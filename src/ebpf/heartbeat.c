// SPDX-License-Identifier: GPL-2.0
/*
 * heartbeat.c — eBPF program that intercepts heartbeat-like signals from
 * Kubernetes clusters by tracing process context switches.
 *
 * Collects extended process metadata (PID, parent PID, command name,
 * cgroup ID, timestamp) and emits events to the shared BPF ring buffer.
 *
 * Refactored from the original perf-event-array version to use:
 *   - BPF ring buffer (BPF_MAP_TYPE_RINGBUF) via common.h
 *   - BPF CO-RE helpers (vmlinux.h + BPF_CORE_READ) for portability
 */

#include "headers/common.h"

/*
 * Tracepoint handler for context switch events.
 *
 * On every sched_switch we capture the incoming process's metadata and
 * submit a kernel_event to the ring buffer.  The event_type is set to
 * EVENT_TYPE_PROCESS since this tracks scheduling / process activity.
 */
SEC("tracepoint/sched/sched_switch")
int handle_heartbeat(struct trace_event_raw_sched_switch *ctx)
{
    struct kernel_event *evt;

    evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));

    /* Common fields */
    evt->timestamp = bpf_ktime_get_ns();
    evt->event_type = EVENT_TYPE_PROCESS;

    /* PID of the process being switched in */
    evt->pid = BPF_CORE_READ(ctx, next_pid);

    /* Current task metadata via CO-RE */
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();

    evt->ppid      = BPF_CORE_READ(task, real_parent, tgid);
    evt->tgid      = BPF_CORE_READ(task, tgid);
    evt->cgroup_id = bpf_get_current_cgroup_id();

    /* Command name */
    bpf_get_current_comm(&evt->comm, sizeof(evt->comm));

    bpf_ringbuf_submit(evt, 0);
    return 0;
}

char _license[] SEC("license") = "GPL";
