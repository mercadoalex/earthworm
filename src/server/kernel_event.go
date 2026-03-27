package main

import "time"

// EnrichedEvent is the server-side representation of an enriched kernel event
// received from the eBPF agent. It mirrors the agent's EnrichedEvent type.
type EnrichedEvent struct {
	// From BPF
	Timestamp time.Time `json:"timestamp"`
	PID       uint32    `json:"pid"`
	PPID      uint32    `json:"ppid"`
	TGID      uint32    `json:"tgid,omitempty"`
	Comm      string    `json:"comm"`
	CgroupID  uint64    `json:"cgroupId"`
	EventType string    `json:"eventType"` // "syscall", "process", "network"

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
	NetEventType string `json:"netEventType,omitempty"` // "retransmit", "reset", "rtt_high"
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

// CausalChain represents an ordered sequence of correlated events
// explaining why a node transitioned to NotReady.
type CausalChain struct {
	NodeName  string          `json:"nodeName"`
	Timestamp time.Time       `json:"timestamp"`
	Events    []EnrichedEvent `json:"events"`
	Summary   string          `json:"summary"`
	RootCause string          `json:"rootCause"`
}
