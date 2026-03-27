// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
/*
 * dns_probe.c — DNS resolution tracing via udp_sendmsg/udp_recvmsg kprobes.
 *
 * Tracks DNS queries (dst port 53) and measures response latency.
 * Emits events when responses arrive or when queries time out.
 */
#include "headers/extended_common.h"

/* Key for tracking in-flight DNS queries: pid + a simple counter */
struct dns_key {
    __u32 pid;
    __u16 dport;
};

/* Value: query start timestamp */
struct dns_val {
    __u64 start_ts;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, struct dns_key);
    __type(value, struct dns_val);
    __uint(max_entries, 4096);
} inflight_dns SEC(".maps");

/* kprobe on udp_sendmsg — track DNS queries (dst port 53) */
SEC("kprobe/udp_sendmsg")
int BPF_KPROBE(udp_sendmsg_entry)
{
    /* Read the destination port from the socket */
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    if (!sk)
        return 0;

    __u16 dport = 0;
    BPF_CORE_READ_INTO(&dport, sk, __sk_common.skc_dport);
    dport = __builtin_bswap16(dport); /* network to host byte order */

    if (dport != 53)
        return 0;

    struct dns_key key = {};
    key.pid = bpf_get_current_pid_tgid() & 0xFFFFFFFF;
    key.dport = dport;

    struct dns_val val = {};
    val.start_ts = bpf_ktime_get_ns();

    bpf_map_update_elem(&inflight_dns, &key, &val, BPF_ANY);
    return 0;
}

/* kprobe on udp_recvmsg — match DNS responses and emit events */
SEC("kprobe/udp_recvmsg")
int BPF_KPROBE(udp_recvmsg_entry)
{
    struct dns_key key = {};
    key.pid = bpf_get_current_pid_tgid() & 0xFFFFFFFF;
    key.dport = 53;

    struct dns_val *val = bpf_map_lookup_elem(&inflight_dns, &key);
    if (!val)
        return 0;

    __u64 latency = bpf_ktime_get_ns() - val->start_ts;
    bpf_map_delete_elem(&inflight_dns, &key);

    /* Read timeout threshold from map */
    __u32 tkey = 0;
    __u64 *thresh = bpf_map_lookup_elem(&dns_timeout_threshold, &tkey);
    __u64 timeout = thresh ? *thresh : DEFAULT_DNS_TIMEOUT_NS;

    __u32 payload_size = sizeof(struct dns_payload);
    __u32 total = EXTENDED_HEADER_SIZE + payload_size;
    void *buf = bpf_ringbuf_reserve(&events, total, 0);
    if (!buf)
        return 0;

    fill_extended_header(buf, EVENT_TYPE_DNS, payload_size);

    struct dns_payload *p = buf + EXTENDED_HEADER_SIZE;
    __builtin_memset(p, 0, sizeof(*p));
    p->latency_ns = latency;
    p->response_code = 0; /* would need packet parsing for real RCODE */
    p->timed_out = latency > timeout ? 1 : 0;

    bpf_ringbuf_submit(buf, 0);
    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
