/* SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause */
/*
 * common.h — Shared definitions for Earthworm eBPF programs.
 *
 * Contains the kernel_event struct, event type constants, and BPF map
 * definitions used by all eBPF programs (syscall tracer, process monitor,
 * network probe, heartbeat).
 */
#ifndef __EARTHWORM_COMMON_H
#define __EARTHWORM_COMMON_H

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>

/* ── Event type constants ─────────────────────────────────────────── */
#define EVENT_TYPE_SYSCALL  0
#define EVENT_TYPE_PROCESS  1
#define EVENT_TYPE_NETWORK  2

/* ── Network sub-event types ──────────────────────────────────────── */
#define NET_EVENT_RETRANSMIT 0
#define NET_EVENT_RESET      1
#define NET_EVENT_RTT_HIGH   2

/* ── Thresholds ───────────────────────────────────────────────────── */
#define SLOW_SYSCALL_NS  1000000000ULL   /* 1 second in nanoseconds */
#define RTT_HIGH_US      500000          /* 500 ms in microseconds  */

/* ── Comm names for filtering ─────────────────────────────────────── */
#define TASK_COMM_LEN 16

/*
 * Helper: returns true if comm matches one of the monitored process names.
 * Compares against "kubelet", "containerd", and "cri-o".
 */
static __always_inline bool is_monitored_comm(const char *comm)
{
    char kubelet[]    = "kubelet";
    char containerd[] = "containerd";
    char crio[]       = "cri-o";

    /* bpf_strncmp is available on kernel 5.17+; fall back to manual
       byte comparison for broader compatibility. */
    int i;

    /* Check "kubelet" (7 chars) */
    bool match = true;
    #pragma unroll
    for (i = 0; i < 7; i++) {
        if (comm[i] != kubelet[i]) {
            match = false;
            break;
        }
    }
    if (match && comm[7] == '\0')
        return true;

    /* Check "containerd" (10 chars) */
    match = true;
    #pragma unroll
    for (i = 0; i < 10; i++) {
        if (comm[i] != containerd[i]) {
            match = false;
            break;
        }
    }
    if (match && comm[10] == '\0')
        return true;

    /* Check "cri-o" (5 chars) */
    match = true;
    #pragma unroll
    for (i = 0; i < 5; i++) {
        if (comm[i] != crio[i]) {
            match = false;
            break;
        }
    }
    if (match && comm[5] == '\0')
        return true;

    return false;
}

/* ── Shared kernel event structure ────────────────────────────────── */
struct kernel_event {
    __u64 timestamp;        /* ktime_get_ns()                       */
    __u32 pid;
    __u32 ppid;
    __u32 tgid;
    __u64 cgroup_id;        /* bpf_get_current_cgroup_id()          */
    char  comm[TASK_COMM_LEN];
    __u8  event_type;       /* EVENT_TYPE_SYSCALL / PROCESS / NETWORK */

    /* ── Syscall fields ── */
    __u32 syscall_nr;
    __s64 ret_val;
    __u64 entry_ts;
    __u64 exit_ts;
    __u8  slow_syscall;     /* 1 if duration > 1 s                  */

    /* ── Process fields ── */
    __u32 child_pid;
    __s32 exit_code;
    __u8  critical_exit;    /* 1 if non-zero exit from kubelet      */

    /* ── Network fields ── */
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u8  net_event_type;   /* NET_EVENT_RETRANSMIT / RESET / RTT_HIGH */
    __u32 rtt_us;
};

/* ── BPF Maps ─────────────────────────────────────────────────────── */

/* Ring buffer for all kernel events (shared across programs).
 * Default size: 256 KB. */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

/* Per-CPU syscall counter for overhead monitoring. */
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key, __u32);
    __type(value, __u64);
    __uint(max_entries, 1);
} syscall_count SEC(".maps");

/* Hash map for tracking in-flight syscalls (entry timestamp keyed by tgid_pid). */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u64);
    __type(value, __u64);
    __uint(max_entries, 10240);
} inflight_syscalls SEC(".maps");

#endif /* __EARTHWORM_COMMON_H */
