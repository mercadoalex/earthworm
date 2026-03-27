/* SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause */
/*
 * extended_common.h — Extended event definitions for advanced eBPF probes.
 *
 * Shares the same 48-byte common header as kernel_event, then uses a
 * variable-length payload via the extended_event struct.
 */
#ifndef __EARTHWORM_EXTENDED_COMMON_H
#define __EARTHWORM_EXTENDED_COMMON_H

#include "common.h"

/* ── Extended event type constants ────────────────────────────────── */
#define EVENT_TYPE_VFS      3
#define EVENT_TYPE_OOM      4
#define EVENT_TYPE_DNS      5
#define EVENT_TYPE_CGROUP   6
#define EVENT_TYPE_SECURITY 7

/* ── Default thresholds (overridable via BPF maps) ────────────────── */
#define DEFAULT_SLOW_IO_NS      100000000ULL  /* 100ms */
#define DEFAULT_DNS_TIMEOUT_NS  5000000000ULL /* 5s    */
#define DEFAULT_MEM_PRESSURE_PCT 90

/* ── Threshold maps (writable from userspace) ─────────────────────── */
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __type(key, __u32);
    __type(value, __u64);
    __uint(max_entries, 1);
} slow_io_threshold SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __type(key, __u32);
    __type(value, __u64);
    __uint(max_entries, 1);
} dns_timeout_threshold SEC(".maps");

/* ── VFS payload (event_type 3) ───────────────────────────────────── */
struct vfs_payload {
    char  file_path[256];
    __u64 latency_ns;
    __u64 bytes_xfer;
    __u8  slow_io;
    __u8  op_type;       /* 0=read, 1=write */
};

/* ── OOM payload (event_type 4) ───────────────────────────────────── */
struct oom_payload {
    __u8  sub_type;      /* 0=oom_kill, 1=alloc_failure */
    __u32 killed_pid;
    char  killed_comm[TASK_COMM_LEN];
    __s32 oom_score_adj;
    __u32 page_order;
    __u32 gfp_flags;
};

/* ── DNS payload (event_type 5) ───────────────────────────────────── */
struct dns_payload {
    char  domain[253];
    __u64 latency_ns;
    __u16 response_code;
    __u8  timed_out;
};

/* ── Cgroup resource payload (event_type 6) ───────────────────────── */
struct cgroup_resource_payload {
    __u64 cpu_usage_ns;
    __u64 memory_usage_bytes;
    __u64 memory_limit_bytes;
    __u8  memory_pressure;
};

/* ── Network audit payload (event_type 7) ─────────────────────────── */
struct network_audit_payload {
    __u32 dst_addr;
    __u16 dst_port;
    __u8  protocol;      /* IPPROTO_TCP=6, IPPROTO_UDP=17 */
};

/* ── Helper: emit an extended event to the ring buffer ────────────── */
#define EXTENDED_HEADER_SIZE 52

static __always_inline void
fill_extended_header(void *buf, __u8 event_type, __u16 payload_len)
{
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    __u64 *ts  = buf;
    __u32 *pid = buf + 8;
    __u32 *ppid = buf + 12;
    __u32 *tgid = buf + 16;
    /* 4 bytes padding at offset 20 */
    __u64 *cgid = buf + 24;
    char  *comm = buf + 32;
    __u8  *etype = buf + 48;
    /* 1 byte padding at offset 49 */
    __u16 *plen = buf + 50;

    *ts = bpf_ktime_get_ns();
    *pid = bpf_get_current_pid_tgid() & 0xFFFFFFFF;
    *tgid = bpf_get_current_pid_tgid() >> 32;
    BPF_CORE_READ_INTO(ppid, task, real_parent, tgid);
    *cgid = bpf_get_current_cgroup_id();
    bpf_get_current_comm(comm, TASK_COMM_LEN);
    *etype = event_type;
    *plen = payload_len;
}

#endif /* __EARTHWORM_EXTENDED_COMMON_H */
