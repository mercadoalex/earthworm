package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

// Event type constants matching the C definitions in common.h.
const (
	EventTypeSyscall uint8 = 0
	EventTypeProcess uint8 = 1
	EventTypeNetwork uint8 = 2
)

// Network sub-event types matching the C definitions.
const (
	NetEventRetransmit uint8 = 0
	NetEventReset      uint8 = 1
	NetEventRTTHigh    uint8 = 2
)

// Thresholds matching the C definitions.
const (
	SlowSyscallNs = 1_000_000_000 // 1 second in nanoseconds
	RTTHighUs     = 500_000        // 500 ms in microseconds
)

// TaskCommLen matches TASK_COMM_LEN in common.h.
const TaskCommLen = 16

// KernelEventSize is the exact binary size of the C kernel_event struct
// including padding for alignment.
const KernelEventSize = 120

// KernelEvent mirrors the C kernel_event struct from common.h.
// The binary layout must match exactly for ring buffer decoding.
//
// C struct layout with padding:
//
//	offset  0: u64 timestamp      (8 bytes)
//	offset  8: u32 pid            (4 bytes)
//	offset 12: u32 ppid           (4 bytes)
//	offset 16: u32 tgid           (4 bytes)
//	offset 20: [4 bytes padding]
//	offset 24: u64 cgroup_id      (8 bytes)
//	offset 32: char comm[16]      (16 bytes)
//	offset 48: u8  event_type     (1 byte)
//	offset 49: [3 bytes padding]
//	offset 52: u32 syscall_nr     (4 bytes)
//	offset 56: s64 ret_val        (8 bytes)
//	offset 64: u64 entry_ts       (8 bytes)
//	offset 72: u64 exit_ts        (8 bytes)
//	offset 80: u8  slow_syscall   (1 byte)
//	offset 81: [3 bytes padding]
//	offset 84: u32 child_pid      (4 bytes)
//	offset 88: s32 exit_code      (4 bytes)
//	offset 92: u8  critical_exit  (1 byte)
//	offset 93: [3 bytes padding]
//	offset 96: u32 saddr          (4 bytes)
//	offset100: u32 daddr          (4 bytes)
//	offset104: u16 sport          (2 bytes)
//	offset106: u16 dport          (2 bytes)
//	offset108: u8  net_event_type (1 byte)
//	offset109: [3 bytes padding]
//	offset112: u32 rtt_us         (4 bytes)
//	offset116: [4 bytes padding to align struct to 8]
//	total: 120 bytes
type KernelEvent struct {
	Timestamp uint64
	PID       uint32
	PPID      uint32
	TGID      uint32
	CgroupID  uint64
	Comm      [TaskCommLen]byte
	EventType uint8

	// Syscall fields
	SyscallNr   uint32
	RetVal      int64
	EntryTs     uint64
	ExitTs      uint64
	SlowSyscall uint8

	// Process fields
	ChildPID     uint32
	ExitCode     int32
	CriticalExit uint8

	// Network fields
	SAddr        uint32
	DAddr        uint32
	SPort        uint16
	DPort        uint16
	NetEventType uint8
	RTTUs        uint32
}

// MarshalBinary encodes a KernelEvent to the BPF binary wire format.
func (e *KernelEvent) MarshalBinary() ([]byte, error) {
	buf := make([]byte, KernelEventSize)

	binary.LittleEndian.PutUint64(buf[0:8], e.Timestamp)
	binary.LittleEndian.PutUint32(buf[8:12], e.PID)
	binary.LittleEndian.PutUint32(buf[12:16], e.PPID)
	binary.LittleEndian.PutUint32(buf[16:20], e.TGID)
	// 4 bytes padding at offset 20
	binary.LittleEndian.PutUint64(buf[24:32], e.CgroupID)
	copy(buf[32:48], e.Comm[:])
	buf[48] = e.EventType
	// 3 bytes padding at offset 49
	binary.LittleEndian.PutUint32(buf[52:56], e.SyscallNr)
	binary.LittleEndian.PutUint64(buf[56:64], uint64(e.RetVal))
	binary.LittleEndian.PutUint64(buf[64:72], e.EntryTs)
	binary.LittleEndian.PutUint64(buf[72:80], e.ExitTs)
	buf[80] = e.SlowSyscall
	// 3 bytes padding at offset 81
	binary.LittleEndian.PutUint32(buf[84:88], e.ChildPID)
	binary.LittleEndian.PutUint32(buf[88:92], uint32(e.ExitCode))
	buf[92] = e.CriticalExit
	// 3 bytes padding at offset 93
	binary.LittleEndian.PutUint32(buf[96:100], e.SAddr)
	binary.LittleEndian.PutUint32(buf[100:104], e.DAddr)
	binary.LittleEndian.PutUint16(buf[104:106], e.SPort)
	binary.LittleEndian.PutUint16(buf[106:108], e.DPort)
	buf[108] = e.NetEventType
	// 3 bytes padding at offset 109
	binary.LittleEndian.PutUint32(buf[112:116], e.RTTUs)
	// 4 bytes trailing padding at offset 116

	return buf, nil
}

// UnmarshalBinary decodes a KernelEvent from the BPF binary wire format.
func (e *KernelEvent) UnmarshalBinary(data []byte) error {
	if len(data) < KernelEventSize {
		return fmt.Errorf("buffer too small: got %d bytes, need %d", len(data), KernelEventSize)
	}

	e.Timestamp = binary.LittleEndian.Uint64(data[0:8])
	e.PID = binary.LittleEndian.Uint32(data[8:12])
	e.PPID = binary.LittleEndian.Uint32(data[12:16])
	e.TGID = binary.LittleEndian.Uint32(data[16:20])
	e.CgroupID = binary.LittleEndian.Uint64(data[24:32])
	copy(e.Comm[:], data[32:48])
	e.EventType = data[48]
	e.SyscallNr = binary.LittleEndian.Uint32(data[52:56])
	e.RetVal = int64(binary.LittleEndian.Uint64(data[56:64]))
	e.EntryTs = binary.LittleEndian.Uint64(data[64:72])
	e.ExitTs = binary.LittleEndian.Uint64(data[72:80])
	e.SlowSyscall = data[80]
	e.ChildPID = binary.LittleEndian.Uint32(data[84:88])
	e.ExitCode = int32(binary.LittleEndian.Uint32(data[88:92]))
	e.CriticalExit = data[92]
	e.SAddr = binary.LittleEndian.Uint32(data[96:100])
	e.DAddr = binary.LittleEndian.Uint32(data[100:104])
	e.SPort = binary.LittleEndian.Uint16(data[104:106])
	e.DPort = binary.LittleEndian.Uint16(data[106:108])
	e.NetEventType = data[108]
	e.RTTUs = binary.LittleEndian.Uint32(data[112:116])

	return nil
}

// DecodeKernelEvent decodes a KernelEvent from a byte slice.
func DecodeKernelEvent(data []byte) (*KernelEvent, error) {
	var evt KernelEvent
	if err := evt.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	return &evt, nil
}

// CommString returns the comm field as a trimmed Go string.
func (e *KernelEvent) CommString() string {
	n := bytes.IndexByte(e.Comm[:], 0)
	if n < 0 {
		n = TaskCommLen
	}
	return string(e.Comm[:n])
}

// EventTypeString returns a human-readable event type name.
func (e *KernelEvent) EventTypeString() string {
	switch e.EventType {
	case EventTypeSyscall:
		return "syscall"
	case EventTypeProcess:
		return "process"
	case EventTypeNetwork:
		return "network"
	default:
		return fmt.Sprintf("unknown(%d)", e.EventType)
	}
}

// NetEventTypeString returns a human-readable network event type name.
func (e *KernelEvent) NetEventTypeString() string {
	switch e.NetEventType {
	case NetEventRetransmit:
		return "retransmit"
	case NetEventReset:
		return "reset"
	case NetEventRTTHigh:
		return "rtt_high"
	default:
		return fmt.Sprintf("unknown(%d)", e.NetEventType)
	}
}

// IPv4String converts a uint32 IP address to dotted-decimal string.
func IPv4String(addr uint32) string {
	ip := make(net.IP, 4)
	binary.LittleEndian.PutUint32(ip, addr)
	return ip.String()
}

// AllowedComms is the set of process names that BPF programs filter on.
var AllowedComms = []string{"kubelet", "containerd", "cri-o"}

// IsAllowedComm checks if a comm string matches one of the monitored process names.
func IsAllowedComm(comm string) bool {
	comm = strings.TrimRight(comm, "\x00")
	for _, allowed := range AllowedComms {
		if comm == allowed {
			return true
		}
	}
	return false
}

// PodIdentity represents the Kubernetes identity for a cgroup.
type PodIdentity struct {
	PodName       string `json:"podName,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	NodeName      string `json:"nodeName"`
}

// EnrichedEvent combines a decoded KernelEvent with pod identity enrichment.
type EnrichedEvent struct {
	// From BPF
	Timestamp time.Time `json:"timestamp"`
	PID       uint32    `json:"pid"`
	PPID      uint32    `json:"ppid"`
	TGID      uint32    `json:"tgid"`
	Comm      string    `json:"comm"`
	CgroupID  uint64    `json:"cgroupId"`
	EventType string    `json:"eventType"`

	// Syscall-specific
	SyscallNr   uint32 `json:"syscallNr,omitempty"`
	ReturnValue int64  `json:"returnValue,omitempty"`
	LatencyNs   uint64 `json:"latencyNs,omitempty"`
	SlowSyscall bool   `json:"slowSyscall,omitempty"`

	// Process-specific
	ChildPID     uint32 `json:"childPid,omitempty"`
	ExitCode     int32  `json:"exitCode,omitempty"`
	CriticalExit bool   `json:"criticalExit,omitempty"`

	// Network-specific
	SrcAddr      string `json:"srcAddr,omitempty"`
	DstAddr      string `json:"dstAddr,omitempty"`
	SrcPort      uint16 `json:"srcPort,omitempty"`
	DstPort      uint16 `json:"dstPort,omitempty"`
	NetEventType string `json:"netEventType,omitempty"`
	RTTUs        uint32 `json:"rttUs,omitempty"`

	// Filesystem I/O fields (event_type "filesystem_io")
	FilePath    string `json:"filePath,omitempty"`
	IOLatencyNs uint64 `json:"ioLatencyNs,omitempty"`
	BytesXfer   uint64 `json:"bytesXfer,omitempty"`
	SlowIO      bool   `json:"slowIO,omitempty"`
	IOOpType    string `json:"ioOpType,omitempty"` // "read" or "write"

	// Memory pressure fields (event_type "memory_pressure")
	OOMSubType  string `json:"oomSubType,omitempty"` // "oom_kill" or "alloc_failure"
	KilledPID   uint32 `json:"killedPid,omitempty"`
	KilledComm  string `json:"killedComm,omitempty"`
	OOMScoreAdj int32  `json:"oomScoreAdj,omitempty"`
	PageOrder   uint32 `json:"pageOrder,omitempty"`
	GFPFlags    uint32 `json:"gfpFlags,omitempty"`

	// DNS resolution fields (event_type "dns_resolution")
	Domain       string `json:"domain,omitempty"`
	DNSLatencyNs uint64 `json:"dnsLatencyNs,omitempty"`
	ResponseCode uint16 `json:"responseCode,omitempty"`
	TimedOut     bool   `json:"timedOut,omitempty"`

	// Cgroup resource fields (event_type "cgroup_resource")
	CPUUsageNs       uint64 `json:"cpuUsageNs,omitempty"`
	MemoryUsageBytes uint64 `json:"memoryUsageBytes,omitempty"`
	MemoryLimitBytes uint64 `json:"memoryLimitBytes,omitempty"`
	MemoryPressure   bool   `json:"memoryPressure,omitempty"`

	// Network audit fields (event_type "network_audit")
	AuditDstAddr  string `json:"auditDstAddr,omitempty"`
	AuditDstPort  uint16 `json:"auditDstPort,omitempty"`
	AuditProtocol string `json:"auditProtocol,omitempty"` // "tcp" or "udp"

	// Enrichment (from CgroupResolver)
	PodName       string `json:"podName,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	NodeName      string `json:"nodeName"`
	HostLevel     bool   `json:"hostLevel,omitempty"`
}

// Enrich converts a KernelEvent to an EnrichedEvent with pod identity.
func (e *KernelEvent) Enrich(pod PodIdentity, hostLevel bool) EnrichedEvent {
	enriched := EnrichedEvent{
		Timestamp: time.Unix(0, int64(e.Timestamp)),
		PID:       e.PID,
		PPID:      e.PPID,
		TGID:      e.TGID,
		Comm:      e.CommString(),
		CgroupID:  e.CgroupID,
		EventType: e.EventTypeString(),
		NodeName:  pod.NodeName,
		HostLevel: hostLevel,
	}

	if !hostLevel {
		enriched.PodName = pod.PodName
		enriched.Namespace = pod.Namespace
		enriched.ContainerName = pod.ContainerName
	}

	switch e.EventType {
	case EventTypeSyscall:
		enriched.SyscallNr = e.SyscallNr
		enriched.ReturnValue = e.RetVal
		if e.ExitTs > e.EntryTs {
			enriched.LatencyNs = e.ExitTs - e.EntryTs
		}
		enriched.SlowSyscall = e.SlowSyscall == 1

	case EventTypeProcess:
		enriched.ChildPID = e.ChildPID
		enriched.ExitCode = e.ExitCode
		enriched.CriticalExit = e.CriticalExit == 1

	case EventTypeNetwork:
		enriched.SrcAddr = IPv4String(e.SAddr)
		enriched.DstAddr = IPv4String(e.DAddr)
		enriched.SrcPort = e.SPort
		enriched.DstPort = e.DPort
		enriched.NetEventType = e.NetEventTypeString()
		enriched.RTTUs = e.RTTUs
	}

	return enriched
}

// ValidateFlags checks that conditional flags are consistent with measured values.
// Returns true if all flags are correctly set.
func (e *KernelEvent) ValidateFlags() bool {
	switch e.EventType {
	case EventTypeSyscall:
		duration := uint64(0)
		if e.ExitTs > e.EntryTs {
			duration = e.ExitTs - e.EntryTs
		}
		isSlow := duration > SlowSyscallNs
		flagSet := e.SlowSyscall == 1
		return isSlow == flagSet

	case EventTypeProcess:
		isCritical := e.CommString() == "kubelet" && e.ExitCode != 0
		flagSet := e.CriticalExit == 1
		return isCritical == flagSet

	case EventTypeNetwork:
		isHighRTT := e.RTTUs > RTTHighUs
		flagSet := e.NetEventType == NetEventRTTHigh
		return isHighRTT == flagSet
	}
	return true
}
