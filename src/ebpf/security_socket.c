// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
/*
 * security_socket.c — Network policy auditing via security_socket_connect kprobe.
 *
 * Captures outbound connection attempts with destination IP, port, and protocol.
 * Used to build a live network topology map of pod-to-pod and pod-to-external connections.
 */
#include "headers/extended_common.h"

SEC("kprobe/security_socket_connect")
int BPF_KPROBE(security_socket_connect_entry)
{
    /* Read the sockaddr from the second argument */
    struct sockaddr *addr = (struct sockaddr *)PT_REGS_PARM2(ctx);
    if (!addr)
        return 0;

    /* Only handle AF_INET (IPv4) for now */
    __u16 family = 0;
    BPF_CORE_READ_INTO(&family, addr, sa_family);
    if (family != 2) /* AF_INET = 2 */
        return 0;

    struct sockaddr_in *sin = (struct sockaddr_in *)addr;

    __u32 dst_addr = 0;
    __u16 dst_port = 0;
    BPF_CORE_READ_INTO(&dst_addr, sin, sin_addr.s_addr);
    BPF_CORE_READ_INTO(&dst_port, sin, sin_port);
    dst_port = __builtin_bswap16(dst_port);

    /* Read protocol from the socket (first argument) */
    struct socket *sock = (struct socket *)PT_REGS_PARM1(ctx);
    __u8 protocol = 0;
    if (sock) {
        struct sock *sk = NULL;
        BPF_CORE_READ_INTO(&sk, sock, sk);
        if (sk) {
            BPF_CORE_READ_INTO(&protocol, sk, sk_protocol);
        }
    }

    __u32 payload_size = sizeof(struct network_audit_payload);
    __u32 total = EXTENDED_HEADER_SIZE + payload_size;
    void *buf = bpf_ringbuf_reserve(&events, total, 0);
    if (!buf)
        return 0;

    fill_extended_header(buf, EVENT_TYPE_SECURITY, payload_size);

    struct network_audit_payload *p = buf + EXTENDED_HEADER_SIZE;
    __builtin_memset(p, 0, sizeof(*p));
    p->dst_addr = dst_addr;
    p->dst_port = dst_port;
    p->protocol = protocol;

    bpf_ringbuf_submit(buf, 0);
    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
