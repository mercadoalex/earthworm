// heartbeat.c
// This eBPF program intercepts heartbeat signals from Kubernetes clusters.
// It attaches to the appropriate hooks in the kernel to collect heartbeat data.
// The collected data is then sent to a user-defined map for further processing.

#include <linux/bpf.h>
#include <linux/ptrace.h>
#include <linux/sched.h>
#include <linux/version.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>

// Define a structure to hold heartbeat data
struct heartbeat_data {
    u32 pid;          // Process ID of the heartbeat signal sender
    u64 timestamp;    // Timestamp of the heartbeat signal
};

// Define a map to store heartbeat data
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY); // Use a perf event array to send data to user space
    __type(key, u32); // Key type is u32 (CPU ID)
    __type(value, u32); // Value type is u32 (not used here)
} heartbeat_map SEC(".maps");

// Function to handle the heartbeat signal
SEC("tracepoint/sched/sched_switch")
int handle_heartbeat(struct trace_event_raw_sched_switch *ctx) {
    struct heartbeat_data data = {};
    
    // Capture the process ID and timestamp
    data.pid = BPF_CORE_READ(ctx, next_pid);
    data.timestamp = bpf_ktime_get_ns(); // Get the current time in nanoseconds

    // Send the heartbeat data to the user space
    bpf_perf_event_output(ctx, &heartbeat_map, BPF_F_CURRENT_CPU, &data, sizeof(data));

    return 0; // Return 0 to indicate successful execution
}

// Define the license for the eBPF program
char _license[] SEC("license") = "GPL"; // GPL license is required for eBPF programs

// The end of the eBPF program
// This program will be loaded into the kernel and will start intercepting heartbeat signals
// from Kubernetes clusters, allowing for monitoring and visualization of the heartbeat data.