package main

import (
	"testing"

	"pgregory.net/rapid"
)

// genComm generates a valid comm field: one of the allowed comm names
// or a random string up to TaskCommLen bytes, null-terminated.
func genComm(t *rapid.T) [TaskCommLen]byte {
	var comm [TaskCommLen]byte
	// Pick a random comm string (1-15 printable ASCII chars)
	length := rapid.IntRange(1, TaskCommLen-1).Draw(t, "commLen")
	for i := 0; i < length; i++ {
		comm[i] = byte(rapid.IntRange(0x20, 0x7E).Draw(t, "commChar"))
	}
	// Remaining bytes are zero (null-terminated)
	return comm
}

// genKernelEvent generates a random valid KernelEvent for property testing.
func genKernelEvent(t *rapid.T) *KernelEvent {
	eventType := uint8(rapid.IntRange(0, 2).Draw(t, "eventType"))

	evt := &KernelEvent{
		Timestamp: rapid.Uint64().Draw(t, "timestamp"),
		PID:       rapid.Uint32().Draw(t, "pid"),
		PPID:      rapid.Uint32().Draw(t, "ppid"),
		TGID:      rapid.Uint32().Draw(t, "tgid"),
		CgroupID:  rapid.Uint64().Draw(t, "cgroupID"),
		Comm:      genComm(t),
		EventType: eventType,
	}

	switch eventType {
	case EventTypeSyscall:
		evt.SyscallNr = rapid.Uint32().Draw(t, "syscallNr")
		evt.RetVal = rapid.Int64().Draw(t, "retVal")
		evt.EntryTs = rapid.Uint64().Draw(t, "entryTs")
		evt.ExitTs = rapid.Uint64().Draw(t, "exitTs")
		evt.SlowSyscall = uint8(rapid.IntRange(0, 1).Draw(t, "slowSyscall"))

	case EventTypeProcess:
		evt.ChildPID = rapid.Uint32().Draw(t, "childPid")
		evt.ExitCode = rapid.Int32().Draw(t, "exitCode")
		evt.CriticalExit = uint8(rapid.IntRange(0, 1).Draw(t, "criticalExit"))

	case EventTypeNetwork:
		evt.SAddr = rapid.Uint32().Draw(t, "saddr")
		evt.DAddr = rapid.Uint32().Draw(t, "daddr")
		evt.SPort = rapid.Uint16().Draw(t, "sport")
		evt.DPort = rapid.Uint16().Draw(t, "dport")
		evt.NetEventType = uint8(rapid.IntRange(0, 2).Draw(t, "netEventType"))
		evt.RTTUs = rapid.Uint32().Draw(t, "rttUs")
	}

	return evt
}

// TestKernelEventBinaryRoundTrip tests Property 15: KernelEvent binary round-trip.
// **Validates: Requirements 13.6**
// Feature: ebpf-kernel-observability, Property 15: KernelEvent binary round-trip —
// for any valid KernelEvent, encode then decode produces equivalent struct.
func TestKernelEventBinaryRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := genKernelEvent(t)

		// Encode to binary
		data, err := original.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}

		if len(data) != KernelEventSize {
			t.Fatalf("encoded size %d != expected %d", len(data), KernelEventSize)
		}

		// Decode from binary
		var decoded KernelEvent
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}

		// Verify all fields match
		if original.Timestamp != decoded.Timestamp {
			t.Errorf("Timestamp mismatch: %d != %d", original.Timestamp, decoded.Timestamp)
		}
		if original.PID != decoded.PID {
			t.Errorf("PID mismatch: %d != %d", original.PID, decoded.PID)
		}
		if original.PPID != decoded.PPID {
			t.Errorf("PPID mismatch: %d != %d", original.PPID, decoded.PPID)
		}
		if original.TGID != decoded.TGID {
			t.Errorf("TGID mismatch: %d != %d", original.TGID, decoded.TGID)
		}
		if original.CgroupID != decoded.CgroupID {
			t.Errorf("CgroupID mismatch: %d != %d", original.CgroupID, decoded.CgroupID)
		}
		if original.Comm != decoded.Comm {
			t.Errorf("Comm mismatch: %v != %v", original.Comm, decoded.Comm)
		}
		if original.EventType != decoded.EventType {
			t.Errorf("EventType mismatch: %d != %d", original.EventType, decoded.EventType)
		}
		if original.SyscallNr != decoded.SyscallNr {
			t.Errorf("SyscallNr mismatch: %d != %d", original.SyscallNr, decoded.SyscallNr)
		}
		if original.RetVal != decoded.RetVal {
			t.Errorf("RetVal mismatch: %d != %d", original.RetVal, decoded.RetVal)
		}
		if original.EntryTs != decoded.EntryTs {
			t.Errorf("EntryTs mismatch: %d != %d", original.EntryTs, decoded.EntryTs)
		}
		if original.ExitTs != decoded.ExitTs {
			t.Errorf("ExitTs mismatch: %d != %d", original.ExitTs, decoded.ExitTs)
		}
		if original.SlowSyscall != decoded.SlowSyscall {
			t.Errorf("SlowSyscall mismatch: %d != %d", original.SlowSyscall, decoded.SlowSyscall)
		}
		if original.ChildPID != decoded.ChildPID {
			t.Errorf("ChildPID mismatch: %d != %d", original.ChildPID, decoded.ChildPID)
		}
		if original.ExitCode != decoded.ExitCode {
			t.Errorf("ExitCode mismatch: %d != %d", original.ExitCode, decoded.ExitCode)
		}
		if original.CriticalExit != decoded.CriticalExit {
			t.Errorf("CriticalExit mismatch: %d != %d", original.CriticalExit, decoded.CriticalExit)
		}
		if original.SAddr != decoded.SAddr {
			t.Errorf("SAddr mismatch: %d != %d", original.SAddr, decoded.SAddr)
		}
		if original.DAddr != decoded.DAddr {
			t.Errorf("DAddr mismatch: %d != %d", original.DAddr, decoded.DAddr)
		}
		if original.SPort != decoded.SPort {
			t.Errorf("SPort mismatch: %d != %d", original.SPort, decoded.SPort)
		}
		if original.DPort != decoded.DPort {
			t.Errorf("DPort mismatch: %d != %d", original.DPort, decoded.DPort)
		}
		if original.NetEventType != decoded.NetEventType {
			t.Errorf("NetEventType mismatch: %d != %d", original.NetEventType, decoded.NetEventType)
		}
		if original.RTTUs != decoded.RTTUs {
			t.Errorf("RTTUs mismatch: %d != %d", original.RTTUs, decoded.RTTUs)
		}

		// Also verify re-encoding produces the same bytes (bidirectional round-trip)
		reEncoded, err := decoded.MarshalBinary()
		if err != nil {
			t.Fatalf("re-MarshalBinary failed: %v", err)
		}
		for i := range data {
			if data[i] != reEncoded[i] {
				t.Errorf("re-encoded byte %d differs: 0x%02x != 0x%02x", i, data[i], reEncoded[i])
				break
			}
		}
	})
}
