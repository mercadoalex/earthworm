//go:build ignore

package main

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -target bpfel -cflags "-O2 -g -Wall -Werror" syscallTracer ../ebpf/syscall_tracer.c -- -I../ebpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -target bpfel -cflags "-O2 -g -Wall -Werror" processMonitor ../ebpf/process_monitor.c -- -I../ebpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -target bpfel -cflags "-O2 -g -Wall -Werror" networkProbe ../ebpf/network_probe.c -- -I../ebpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -target bpfel -cflags "-O2 -g -Wall -Werror" heartbeat ../ebpf/heartbeat.c -- -I../ebpf/headers
