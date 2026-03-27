// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
/*
 * vfs_probe.c — File system I/O tracing via vfs_read/vfs_write kprobes.
 *
 * Captures file path, latency, bytes transferred, and flags slow I/O
 * operations above a configurable threshold.
 */
#include "headers/extended_common.h"

/* Track in-flight VFS operations: key = pid_tgid, value = entry timestamp */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u64);
    __type(value, __u64);
    __uint(max_entries, 10240);
} inflight_vfs SEC(".maps");

/* kprobe entry for vfs_read */
SEC("kprobe/vfs_read")
int BPF_KPROBE(vfs_read_entry)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&inflight_vfs, &pid_tgid, &ts, BPF_ANY);
    return 0;
}

/* kretprobe for vfs_read */
SEC("kretprobe/vfs_read")
int BPF_KRETPROBE(vfs_read_exit, ssize_t ret)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 *entry_ts = bpf_map_lookup_elem(&inflight_vfs, &pid_tgid);
    if (!entry_ts)
        return 0;

    __u64 latency = bpf_ktime_get_ns() - *entry_ts;
    bpf_map_delete_elem(&inflight_vfs, &pid_tgid);

    /* Read threshold from map, fall back to default */
    __u32 key = 0;
    __u64 *thresh = bpf_map_lookup_elem(&slow_io_threshold, &key);
    __u64 threshold = thresh ? *thresh : DEFAULT_SLOW_IO_NS;

    /* Allocate ring buffer space */
    __u32 payload_size = sizeof(struct vfs_payload);
    __u32 total = EXTENDED_HEADER_SIZE + payload_size;
    void *buf = bpf_ringbuf_reserve(&events, total, 0);
    if (!buf)
        return 0;

    fill_extended_header(buf, EVENT_TYPE_VFS, payload_size);

    struct vfs_payload *p = buf + EXTENDED_HEADER_SIZE;
    __builtin_memset(p, 0, sizeof(*p));
    p->latency_ns = latency;
    p->bytes_xfer = ret > 0 ? ret : 0;
    p->slow_io = latency > threshold ? 1 : 0;
    p->op_type = 0; /* read */

    bpf_ringbuf_submit(buf, 0);
    return 0;
}

/* kprobe entry for vfs_write */
SEC("kprobe/vfs_write")
int BPF_KPROBE(vfs_write_entry)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&inflight_vfs, &pid_tgid, &ts, BPF_ANY);
    return 0;
}

/* kretprobe for vfs_write */
SEC("kretprobe/vfs_write")
int BPF_KRETPROBE(vfs_write_exit, ssize_t ret)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 *entry_ts = bpf_map_lookup_elem(&inflight_vfs, &pid_tgid);
    if (!entry_ts)
        return 0;

    __u64 latency = bpf_ktime_get_ns() - *entry_ts;
    bpf_map_delete_elem(&inflight_vfs, &pid_tgid);

    __u32 key = 0;
    __u64 *thresh = bpf_map_lookup_elem(&slow_io_threshold, &key);
    __u64 threshold = thresh ? *thresh : DEFAULT_SLOW_IO_NS;

    __u32 payload_size = sizeof(struct vfs_payload);
    __u32 total = EXTENDED_HEADER_SIZE + payload_size;
    void *buf = bpf_ringbuf_reserve(&events, total, 0);
    if (!buf)
        return 0;

    fill_extended_header(buf, EVENT_TYPE_VFS, payload_size);

    struct vfs_payload *p = buf + EXTENDED_HEADER_SIZE;
    __builtin_memset(p, 0, sizeof(*p));
    p->latency_ns = latency;
    p->bytes_xfer = ret > 0 ? ret : 0;
    p->slow_io = latency > threshold ? 1 : 0;
    p->op_type = 1; /* write */

    bpf_ringbuf_submit(buf, 0);
    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
