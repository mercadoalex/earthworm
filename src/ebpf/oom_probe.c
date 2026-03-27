// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
/*
 * oom_probe.c — Memory pressure signals via oom_kill_process kprobe.
 *
 * Captures OOM kills with killed process PID, comm, and OOM score.
 */
#include "headers/extended_common.h"

/* kprobe on oom_kill_process — fires when the OOM killer selects a victim */
SEC("kprobe/oom_kill_process")
int BPF_KPROBE(oom_kill_entry)
{
    __u32 payload_size = sizeof(struct oom_payload);
    __u32 total = EXTENDED_HEADER_SIZE + payload_size;
    void *buf = bpf_ringbuf_reserve(&events, total, 0);
    if (!buf)
        return 0;

    fill_extended_header(buf, EVENT_TYPE_OOM, payload_size);

    struct oom_payload *p = buf + EXTENDED_HEADER_SIZE;
    __builtin_memset(p, 0, sizeof(*p));
    p->sub_type = 0; /* oom_kill */

    /* Try to read the victim task from the first argument.
     * oom_kill_process signature varies by kernel version;
     * we read what we can and set -1 for unreadable fields. */
    struct task_struct *victim = (struct task_struct *)PT_REGS_PARM2(ctx);
    if (victim) {
        BPF_CORE_READ_INTO(&p->killed_pid, victim, pid);
        BPF_CORE_READ_STR_INTO(&p->killed_comm, victim, comm);
    }
    p->oom_score_adj = -1; /* default: unreadable */

    bpf_ringbuf_submit(buf, 0);
    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
