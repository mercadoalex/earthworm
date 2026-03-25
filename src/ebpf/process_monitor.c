// SPDX-License-Identifier: GPL-2.0
/*
 * process_monitor.c — eBPF program for monitoring kubelet/containerd/cri-o
 * process lifecycle events (fork, exec, exit).
 *
 * Attaches to sched_process_fork, sched_process_exec, and
 * sched_process_exit tracepoints.  Sets the critical_exit flag when
 * kubelet exits with a non-zero exit code.
 *
 * Uses BPF CO-RE (vmlinux.h) for portability across kernel 5.8+.
 */

#include "headers/common.h"

/* ── Helper: populate common event fields ─────────────────────────── */
static __always_inline void fill_common(struct kernel_event *evt,
                                        struct task_struct *task)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    evt->timestamp  = bpf_ktime_get_ns();
    evt->pid        = BPF_CORE_READ(task, pid);
    evt->tgid       = BPF_CORE_READ(task, tgid);
    evt->cgroup_id  = bpf_get_current_cgroup_id();
    evt->event_type = EVENT_TYPE_PROCESS;

    bpf_get_current_comm(&evt->comm, sizeof(evt->comm));

    /* Parent PID via CO-RE */
    evt->ppid = BPF_CORE_READ(task, real_parent, tgid);
}

/* ── sched_process_fork ───────────────────────────────────────────── */

/*
 * Tracepoint args (sched_process_fork):
 *   parent_comm, parent_pid, child_comm, child_pid
 */
SEC("tracepoint/sched/sched_process_fork")
int tracepoint__sched_process_fork(struct trace_event_raw_sched_process_fork *ctx)
{
    char comm[TASK_COMM_LEN];
    bpf_get_current_comm(&comm, sizeof(comm));

    if (!is_monitored_comm(comm))
        return 0;

    struct kernel_event *evt;
    evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    fill_common(evt, task);

    /* Fork-specific: record child PID */
    evt->child_pid = BPF_CORE_READ(ctx, child_pid);

    bpf_ringbuf_submit(evt, 0);
    return 0;
}

/* ── sched_process_exec ───────────────────────────────────────────── */

/*
 * Tracepoint args (sched_process_exec):
 *   filename, pid, old_pid
 */
SEC("tracepoint/sched/sched_process_exec")
int tracepoint__sched_process_exec(struct trace_event_raw_sched_process_exec *ctx)
{
    char comm[TASK_COMM_LEN];
    bpf_get_current_comm(&comm, sizeof(comm));

    if (!is_monitored_comm(comm))
        return 0;

    struct kernel_event *evt;
    evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    fill_common(evt, task);

    bpf_ringbuf_submit(evt, 0);
    return 0;
}

/* ── sched_process_exit ───────────────────────────────────────────── */

/*
 * Tracepoint args (sched_process_exit):
 *   comm, pid, prio
 */
SEC("tracepoint/sched/sched_process_exit")
int tracepoint__sched_process_exit(struct trace_event_raw_sched_process_template *ctx)
{
    char comm[TASK_COMM_LEN];
    bpf_get_current_comm(&comm, sizeof(comm));

    if (!is_monitored_comm(comm))
        return 0;

    struct kernel_event *evt;
    evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    fill_common(evt, task);

    /* Read exit code from task_struct */
    evt->exit_code = BPF_CORE_READ(task, exit_code) >> 8;

    /*
     * critical_exit: set when kubelet exits with a non-zero exit code.
     * Compare against "kubelet" specifically.
     */
    if (evt->exit_code != 0) {
        char kubelet[] = "kubelet";
        bool is_kubelet = true;
        int i;
        #pragma unroll
        for (i = 0; i < 7; i++) {
            if (comm[i] != kubelet[i]) {
                is_kubelet = false;
                break;
            }
        }
        if (is_kubelet && comm[7] == '\0')
            evt->critical_exit = 1;
    }

    bpf_ringbuf_submit(evt, 0);
    return 0;
}

char _license[] SEC("license") = "GPL";
