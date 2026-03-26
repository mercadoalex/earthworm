package main

import (
	"testing"

	"pgregory.net/rapid"
)

// genMonitoredComm generates a comm field with one of the allowed process names.
func genMonitoredComm(t *rapid.T) [TaskCommLen]byte {
	var comm [TaskCommLen]byte
	names := []string{"kubelet", "containerd", "cri-o"}
	name := names[rapid.IntRange(0, len(names)-1).Draw(t, "commIdx")]
	copy(comm[:], name)
	return comm
}

// genValidSyscallEvent generates a KernelEvent with all required syscall fields populated.
func genValidSyscallEvent(t *rapid.T) *KernelEvent {
	entryTs := rapid.Uint64Range(1, 1<<62).Draw(t, "entryTs")
	// exitTs must be >= entryTs and non-zero
	exitTs := entryTs + rapid.Uint64Range(0, 5_000_000_000).Draw(t, "duration")

	duration := exitTs - entryTs
	var slowFlag uint8
	if duration > SlowSyscallNs {
		slowFlag = 1
	}

	return &KernelEvent{
		Timestamp:   rapid.Uint64Min(1).Draw(t, "timestamp"),
		PID:         rapid.Uint32Min(1).Draw(t, "pid"),
		PPID:        rapid.Uint32Min(1).Draw(t, "ppid"),
		TGID:        rapid.Uint32Min(1).Draw(t, "tgid"),
		CgroupID:    rapid.Uint64Min(1).Draw(t, "cgroupID"),
		Comm:        genMonitoredComm(t),
		EventType:   EventTypeSyscall,
		SyscallNr:   rapid.Uint32().Draw(t, "syscallNr"),
		RetVal:      rapid.Int64().Draw(t, "retVal"),
		EntryTs:     entryTs,
		ExitTs:      exitTs,
		SlowSyscall: slowFlag,
	}
}

// genValidProcessEvent generates a KernelEvent with all required process fields populated.
func genValidProcessEvent(t *rapid.T) *KernelEvent {
	comm := genMonitoredComm(t)
	exitCode := rapid.Int32().Draw(t, "exitCode")

	var criticalExit uint8
	commStr := ""
	for i := 0; i < TaskCommLen; i++ {
		if comm[i] == 0 {
			break
		}
		commStr += string(comm[i])
	}
	if commStr == "kubelet" && exitCode != 0 {
		criticalExit = 1
	}

	return &KernelEvent{
		Timestamp:    rapid.Uint64Min(1).Draw(t, "timestamp"),
		PID:          rapid.Uint32Min(1).Draw(t, "pid"),
		PPID:         rapid.Uint32Min(1).Draw(t, "ppid"),
		TGID:         rapid.Uint32Min(1).Draw(t, "tgid"),
		CgroupID:     rapid.Uint64Min(1).Draw(t, "cgroupID"),
		Comm:         comm,
		EventType:    EventTypeProcess,
		ChildPID:     rapid.Uint32().Draw(t, "childPid"),
		ExitCode:     exitCode,
		CriticalExit: criticalExit,
	}
}

// genValidNetworkEvent generates a KernelEvent with all required network fields populated.
func genValidNetworkEvent(t *rapid.T) *KernelEvent {
	rttUs := rapid.Uint32().Draw(t, "rttUs")

	// net_event_type should be RTT_HIGH iff rttUs > RTTHighUs
	var netEventType uint8
	if rttUs > RTTHighUs {
		netEventType = NetEventRTTHigh
	} else {
		netEventType = uint8(rapid.IntRange(0, 1).Draw(t, "netEventType"))
	}

	return &KernelEvent{
		Timestamp:    rapid.Uint64Min(1).Draw(t, "timestamp"),
		PID:          rapid.Uint32Min(1).Draw(t, "pid"),
		PPID:         rapid.Uint32Min(1).Draw(t, "ppid"),
		TGID:         rapid.Uint32Min(1).Draw(t, "tgid"),
		CgroupID:     rapid.Uint64Min(1).Draw(t, "cgroupID"),
		Comm:         genMonitoredComm(t),
		EventType:    EventTypeNetwork,
		SAddr:        rapid.Uint32Min(1).Draw(t, "saddr"),
		DAddr:        rapid.Uint32Min(1).Draw(t, "daddr"),
		SPort:        rapid.Uint16Min(1).Draw(t, "sport"),
		DPort:        rapid.Uint16Min(1).Draw(t, "dport"),
		NetEventType: netEventType,
		RTTUs:        rttUs,
	}
}

// TestEventFieldCompleteness tests Property 2: KernelEvent field completeness.
// **Validates: Requirements 2.2, 3.2, 3.3, 4.3**
// Feature: ebpf-kernel-observability, Property 2: KernelEvent field completeness —
// for any event, all required fields for its type are present.
func TestEventFieldCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		eventType := rapid.IntRange(0, 2).Draw(t, "eventType")

		var evt *KernelEvent
		switch uint8(eventType) {
		case EventTypeSyscall:
			evt = genValidSyscallEvent(t)
		case EventTypeProcess:
			evt = genValidProcessEvent(t)
		case EventTypeNetwork:
			evt = genValidNetworkEvent(t)
		}

		err := ValidateEventFields(evt)
		if err != nil {
			t.Errorf("ValidateEventFields failed for %s event: %v", evt.EventTypeString(), err)
		}
	})
}

// TestConditionalFlagCorrectness tests Property 3: Conditional flag correctness.
// **Validates: Requirements 2.3, 3.4, 4.4**
// Feature: ebpf-kernel-observability, Property 3: Conditional flag correctness —
// slow_syscall, critical_exit, rtt_high flags match thresholds.
func TestConditionalFlagCorrectness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		eventType := rapid.IntRange(0, 2).Draw(t, "eventType")

		var evt *KernelEvent
		switch uint8(eventType) {
		case EventTypeSyscall:
			evt = genValidSyscallEvent(t)
		case EventTypeProcess:
			evt = genValidProcessEvent(t)
		case EventTypeNetwork:
			evt = genValidNetworkEvent(t)
		}

		if !evt.ValidateFlags() {
			t.Errorf("ValidateFlags failed for %s event (comm=%q, exitCode=%d, rttUs=%d, slowSyscall=%d, criticalExit=%d, netEventType=%d)",
				evt.EventTypeString(), evt.CommString(), evt.ExitCode, evt.RTTUs,
				evt.SlowSyscall, evt.CriticalExit, evt.NetEventType)
		}
	})
}
