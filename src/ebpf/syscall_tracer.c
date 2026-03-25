// SPDX-License-Identifier: GPL-2.0
/*
 * syscall_tracer.c — eBPF program for tracing kubelet/containerd/cri-o
 * write() and sendto() syscalls.
 *
 * Attaches to sys_enter_write, sys_enter_sendto, sys_exit_write, and
 * sys_exit_sendto tracepoints.  Computes per-syscall latency and sets
 * the slow_syscall flag when duration exceeds 1 second.
 *
 * Uses BPF CO-RE (vmlinux.h) for portability across kernel 5.8+.
 */

#include "headers/common.h"

/* ── Helpers ──────────────────────────────────────────────────────── */

/*
 * Record the entry timestamp for a syscall into the inflight_syscalls
 * hash map, keyed by (tgid << 32 | pid).
 */
static __always_inline int trace_syscall_enter(__u32 syscall_nr)
{
    char comm[TASK_COMM_LEN];
    bpf_get_current_comm(&comm, sizeof(comm));

    if (!is_monitored_comm(comm))
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 ts = bpf_ktime_get_ns();

    bpf_map_update_elem(&inflight_syscalls, &pid_tgid, &ts, BPF_ANY);

    /* Bump per-CPU syscall counter */
    __u32 key = 0;
    __u64 *cnt = bpf_map_lookup_elem(&syscall_count, &key);
    if (cnt)
        __sync_fetch_and_add(cnt, 1);

    return 0;
}

/*
 * On syscall exit, look up the entry timestamp, compute latency,
 * populate a kernel_event, and submit it to the ring buffer.
 */
static __always_inline int trace_syscall_exit(__u32 syscall_nr, __s64 ret)
{
    char comm[TASK_COMM_LEN];
    bpf_get_current_comm(&comm, sizeof(comm));

    if (!is_monitored_comm(comm))
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();

    /* Look up entry timestamp */
    __u64 *entry_ts = bpf_map_lookup_elem(&inflight_syscalls, &pid_tgid);
    if (!entry_ts)
        return 0;

    __u64 entry = *entry_ts;
    bpf_map_delete_elem(&inflight_syscalls, &pid_tgid);

    __u64 exit_ts = bpf_ktime_get_ns();

    /* Reserve space in the ring buffer */
    struct kernel_event *evt;
    evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    /* Zero-init and populate common fields */
    __builtin_memset(evt, 0, sizeof(*evt));

    evt->timestamp  = exit_ts;
    evt->pid        = (__u32)(pid_tgid & 0xFFFFFFFF);
    evt->tgid       = (__u32)(pid_tgid >> 32);
    evt->cgroup_id  = bpf_get_current_cgroup_id();
    evt->event_type = EVENT_TYPE_SYSCALL;

    bpf_get_current_comm(&evt->comm, sizeof(evt->comm));

    /* Read parent PID via CO-RE */
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    evt->ppid = BPF_CORE_READ(task, real_parent, tgid);

    /* Syscall-specific fields */
    evt->syscall_nr = syscall_nr;
    evt->ret_val    = ret;
    evt->entry_ts   = entry;
    evt->exit_ts    = exit_ts;

    /* Flag slow syscalls (> 1 second) */
    if ((exit_ts - entry) > SLOW_SYSCALL_NS)
        evt->slow_syscall = 1;

    bpf_ringbuf_submit(evt, 0);
    return 0;
}

/* ── Tracepoint handlers ─────────────────────────────────────────── */

/*
 * sys_enter_write tracepoint.
 * Tracepoint args: __syscall_nr, unsigned int fd, const char *buf, size_t count
 */
SEC("tracepoint/syscalls/sys_enter_write")
int tracepoint__sys_enter_write(struct trace_event_raw_sys_enter *ctx)
{
    return trace_syscall_enter(ctx->id);
}

/*
 * sys_enter_sendto tracepoint.
 */
SEC("tracepoint/syscalls/sys_enter_sendto")
int tracepoint__sys_enter_sendto(struct trace_event_raw_sys_enter *ctx)
{
    return trace_syscall_enter(ctx->id);
}

/*
 * sys_exit_write tracepoint.
 */
SEC("tracepoint/syscalls/sys_exit_write")
int tracepoint__sys_exit_write(struct trace_event_raw_sys_exit *ctx)
{
    return trace_syscall_exit(__NR_write, ctx->ret);
}

/*
 * sys_exit_sendto tracepoint.
 */
SEC("tracepoint/syscalls/sys_exit_sendto")
int tracepoint__sys_exit_sendto(struct trace_event_raw_sys_exit *ctx)
{
    return trace_syscall_exit(__NR_sendto, ctx->ret);
}

char _license[] SEC("license") = "GPL";
