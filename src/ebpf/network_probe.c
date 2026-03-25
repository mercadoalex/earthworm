// SPDX-License-Identifier: GPL-2.0
/*
 * network_probe.c — eBPF program for capturing TCP retransmits, resets,
 * and high-RTT events.
 *
 * Attaches to tcp_retransmit_skb and tcp_reset kprobes.  Tracks
 * per-connection RTT and emits an rtt_high event when RTT > 500 ms.
 *
 * Uses BPF CO-RE (vmlinux.h) for portability across kernel 5.8+.
 */

#include "headers/common.h"

/* ── Helper: extract TCP 4-tuple from sock and populate event ─────── */
static __always_inline void fill_network_fields(struct kernel_event *evt,
                                                struct sock *sk)
{
    /* Source / destination addresses (IPv4) */
    evt->saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    evt->daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);

    /* Ports — network byte order in the kernel struct */
    evt->sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    evt->dport = __bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
}

/* ── Helper: populate common fields for network events ────────────── */
static __always_inline void fill_common_net(struct kernel_event *evt)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    evt->timestamp  = bpf_ktime_get_ns();
    evt->pid        = (__u32)(pid_tgid & 0xFFFFFFFF);
    evt->tgid       = (__u32)(pid_tgid >> 32);
    evt->cgroup_id  = bpf_get_current_cgroup_id();
    evt->event_type = EVENT_TYPE_NETWORK;

    bpf_get_current_comm(&evt->comm, sizeof(evt->comm));

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    evt->ppid = BPF_CORE_READ(task, real_parent, tgid);
}

/* ── kprobe: tcp_retransmit_skb ────────────────────────────────────── */

/*
 * tcp_retransmit_skb(struct sock *sk, struct sk_buff *skb, int segs)
 *
 * Fires on every TCP retransmission.  We also read the smoothed RTT
 * from tcp_sock and emit an rtt_high event if it exceeds 500 ms.
 */
SEC("kprobe/tcp_retransmit_skb")
int BPF_KPROBE(kprobe__tcp_retransmit_skb, struct sock *sk, struct sk_buff *skb)
{
    struct kernel_event *evt;
    evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    fill_common_net(evt);
    fill_network_fields(evt, sk);

    evt->net_event_type = NET_EVENT_RETRANSMIT;

    /*
     * Read smoothed RTT from tcp_sock (srtt_us is stored as srtt << 3
     * in some kernels; CO-RE handles the layout).
     */
    struct tcp_sock *tp = (struct tcp_sock *)sk;
    __u32 srtt = BPF_CORE_READ(tp, srtt_us) >> 3;  /* convert to µs */
    evt->rtt_us = srtt;

    bpf_ringbuf_submit(evt, 0);

    /*
     * If RTT exceeds the high-RTT threshold, emit a separate rtt_high
     * event so the userspace pipeline can flag it independently.
     */
    if (srtt > RTT_HIGH_US) {
        struct kernel_event *rtt_evt;
        rtt_evt = bpf_ringbuf_reserve(&events, sizeof(*rtt_evt), 0);
        if (!rtt_evt)
            return 0;

        __builtin_memset(rtt_evt, 0, sizeof(*rtt_evt));
        fill_common_net(rtt_evt);
        fill_network_fields(rtt_evt, sk);

        rtt_evt->net_event_type = NET_EVENT_RTT_HIGH;
        rtt_evt->rtt_us = srtt;

        bpf_ringbuf_submit(rtt_evt, 0);
    }

    return 0;
}

/* ── kprobe: tcp_reset ────────────────────────────────────────────── */

/*
 * tcp_reset(struct sock *sk, struct sk_buff *skb)
 *
 * Fires when a TCP RST is received on a connection.
 */
SEC("kprobe/tcp_reset")
int BPF_KPROBE(kprobe__tcp_reset, struct sock *sk)
{
    struct kernel_event *evt;
    evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    fill_common_net(evt);
    fill_network_fields(evt, sk);

    evt->net_event_type = NET_EVENT_RESET;

    /* Also capture current RTT for context */
    struct tcp_sock *tp = (struct tcp_sock *)sk;
    evt->rtt_us = BPF_CORE_READ(tp, srtt_us) >> 3;

    bpf_ringbuf_submit(evt, 0);
    return 0;
}

char _license[] SEC("license") = "GPL";
