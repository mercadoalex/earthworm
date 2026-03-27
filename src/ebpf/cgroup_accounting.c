// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
/*
 * cgroup_accounting.c — Per-cgroup CPU and memory resource accounting.
 *
 * Emits cgroup resource events on every context switch for the current
 * cgroup, allowing the agent to build per-pod resource metrics.
 * In production, this would use a BPF timer for periodic sampling;
 * here we piggyback on sched_switch for simplicity.
 */
#include "headers/extended_common.h"

/* Sampling: only emit one event per cgroup per N context switches.
 * This prevents flooding the ring buffer on busy systems. */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u64);   /* cgroup_id */
    __type(value, __u64); /* last emit timestamp */
    __uint(max_entries, 4096);
} cgroup_last_emit SEC(".maps");

#define CGROUP_SAMPLE_INTERVAL_NS 10000000000ULL /* 10 seconds */

SEC("tp_btf/sched_switch")
int BPF_PROG(cgroup_accounting, bool preempt,
             struct task_struct *prev, struct task_struct *next)
{
    __u64 cgid = bpf_get_current_cgroup_id();
    __u64 now = bpf_ktime_get_ns();

    /* Rate-limit: one event per cgroup per sample interval */
    __u64 *last = bpf_map_lookup_elem(&cgroup_last_emit, &cgid);
    if (last && (now - *last) < CGROUP_SAMPLE_INTERVAL_NS)
        return 0;

    bpf_map_update_elem(&cgroup_last_emit, &cgid, &now, BPF_ANY);

    __u32 payload_size = sizeof(struct cgroup_resource_payload);
    __u32 total = EXTENDED_HEADER_SIZE + payload_size;
    void *buf = bpf_ringbuf_reserve(&events, total, 0);
    if (!buf)
        return 0;

    fill_extended_header(buf, EVENT_TYPE_CGROUP, payload_size);

    struct cgroup_resource_payload *p = buf + EXTENDED_HEADER_SIZE;
    __builtin_memset(p, 0, sizeof(*p));

    /* Note: Reading actual cgroup CPU/memory stats from BPF requires
     * cgroup-local storage or reading from /sys/fs/cgroup in userspace.
     * The agent enriches these fields via the CgroupResolver.
     * Here we emit the cgroup_id so the agent can look up the stats. */
    p->cpu_usage_ns = 0;
    p->memory_usage_bytes = 0;
    p->memory_limit_bytes = 0;
    p->memory_pressure = 0;

    bpf_ringbuf_submit(buf, 0);
    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
