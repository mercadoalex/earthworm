// heartbeat.c
// This eBPF program intercepts heartbeat-like signals from Kubernetes clusters by tracing process context switches.
// It collects extended process metadata: PID, parent PID, command name, cgroup info, and timestamp.
// The data is sent to user space via a perf event array for further correlation with Kubernetes resources.

#include <linux/bpf.h>
#include <linux/ptrace.h>
#include <linux/sched.h>
#include <linux/version.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>

// Kernel version check macro
#ifndef LINUX_VERSION_CODE
#define LINUX_VERSION_CODE KERNEL_VERSION(0,0,0)
#endif

// Structure to hold extended heartbeat data
struct heartbeat_data {
    u32 pid;                // Process ID of the heartbeat signal sender
    u32 ppid;               // Parent PID
    char comm[16];          // Command name (TASK_COMM_LEN = 16)
    char cgroup_path[64];   // Cgroup path (truncated for demo)
    u64 timestamp;          // Timestamp of the heartbeat signal
};

// Perf event array map to send data to user space
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __type(key, u32);
    __type(value, u32);
} heartbeat_map SEC(".maps");

// Helper to get cgroup path (robust handling for different kernel versions and null pointers)
static __inline int get_cgroup_path(struct task_struct *task, char *buf, int buflen) {
#if LINUX_VERSION_CODE >= KERNEL_VERSION(4, 18, 0)
    struct cgroup_subsys_state *css = NULL;
    struct cgroup *cgrp = NULL;
    struct kernfs_node *kn = NULL;

    // Safely read cgroups pointer
    struct cgroups *cgs = NULL;
    bpf_probe_read(&cgs, sizeof(cgs), &task->cgroups);
    if (!cgs)
        goto unsupported;

    // Read subsys[0] (usually for cpu controller)
    bpf_probe_read(&css, sizeof(css), &cgs->subsys[0]);
    if (!css)
        goto unsupported;

    bpf_probe_read(&cgrp, sizeof(cgrp), &css->cgroup);
    if (!cgrp)
        goto unsupported;

    bpf_probe_read(&kn, sizeof(kn), &cgrp->kn);
    if (!kn)
        goto unsupported;

    // Read cgroup name
    bpf_probe_read_str(buf, buflen, kn->name);
    return 0;

unsupported:
    bpf_probe_read_str(buf, buflen, "unsupported_or_null");
    return -1;
#else
    // For older kernels, cgroup path extraction may differ or be unsupported
    bpf_probe_read_str(buf, buflen, "unsupported_kernel");
    return -1;
#endif
}

// Helper to print kernel version (for debugging, output to trace pipe)
static __inline void print_kernel_version() {
    // Compose kernel version string
    char version[32];
    int major = (LINUX_VERSION_CODE >> 16) & 0xFF;
    int minor = (LINUX_VERSION_CODE >> 8) & 0xFF;
    int patch = LINUX_VERSION_CODE & 0xFF;
    bpf_trace_printk("Kernel version: %d.%d.%d\n", major, minor, patch);
}

// Tracepoint handler for context switch events
SEC("tracepoint/sched/sched_switch")
int handle_heartbeat(struct trace_event_raw_sched_switch *ctx) {
    struct heartbeat_data data = {};
    struct task_struct *task;

    // Print kernel version for debugging
    print_kernel_version();

    // Get next process PID from context switch event
    data.pid = BPF_CORE_READ(ctx, next_pid);

    // Get current task_struct (process info)
    task = (struct task_struct *)bpf_get_current_task();

    // Get parent PID
    data.ppid = BPF_CORE_READ(task, real_parent, pid);

    // Get command name
    bpf_get_current_comm(&data.comm, sizeof(data.comm));

    // Get cgroup path (truncated, for demo)
    get_cgroup_path(task, data.cgroup_path, sizeof(data.cgroup_path));

    // Get timestamp (nanoseconds since boot)
    data.timestamp = bpf_ktime_get_ns();

    // Send heartbeat data to user space via perf event array
    bpf_perf_event_output(ctx, &heartbeat_map, BPF_F_CURRENT_CPU, &data, sizeof(data));
    return 0;
}

// Required license declaration for eBPF programs
char _license[] SEC("license") = "GPL";