package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Extended event type constants for new probe categories.
const (
	EventTypeVFS      uint8 = 3
	EventTypeOOM      uint8 = 4
	EventTypeDNS      uint8 = 5
	EventTypeCgroup   uint8 = 6
	EventTypeSecurity uint8 = 7
)

// ExtendedEventHeaderSize is the fixed header size before the variable payload.
const ExtendedEventHeaderSize = 52

// ExtendedEvent is a variable-length event sharing the first 48 bytes with
// KernelEvent, then carrying a payload_len + payload for probe-specific data.
//
// Binary layout:
//
//	offset  0: u64 timestamp      (8 bytes)
//	offset  8: u32 pid            (4 bytes)
//	offset 12: u32 ppid           (4 bytes)
//	offset 16: u32 tgid           (4 bytes)
//	offset 20: [4 bytes padding]
//	offset 24: u64 cgroup_id      (8 bytes)
//	offset 32: char comm[16]      (16 bytes)
//	offset 48: u8  event_type     (1 byte)
//	offset 49: [1 byte padding]
//	offset 50: u16 payload_len    (2 bytes)
//	offset 52: payload            (payload_len bytes)
type ExtendedEvent struct {
	Timestamp  uint64
	PID        uint32
	PPID       uint32
	TGID       uint32
	CgroupID   uint64
	Comm       [TaskCommLen]byte
	EventType  uint8
	PayloadLen uint16
	Payload    []byte
}

// MarshalBinary encodes an ExtendedEvent to the BPF binary wire format.
// The 52-byte header is written first, followed by the variable-length payload.
func (e *ExtendedEvent) MarshalBinary() ([]byte, error) {
	buf := make([]byte, ExtendedEventHeaderSize+len(e.Payload))

	binary.LittleEndian.PutUint64(buf[0:8], e.Timestamp)
	binary.LittleEndian.PutUint32(buf[8:12], e.PID)
	binary.LittleEndian.PutUint32(buf[12:16], e.PPID)
	binary.LittleEndian.PutUint32(buf[16:20], e.TGID)
	// 4 bytes padding at offset 20
	binary.LittleEndian.PutUint64(buf[24:32], e.CgroupID)
	copy(buf[32:48], e.Comm[:])
	buf[48] = e.EventType
	// 1 byte padding at offset 49
	binary.LittleEndian.PutUint16(buf[50:52], e.PayloadLen)
	if len(e.Payload) > 0 {
		copy(buf[52:], e.Payload)
	}

	return buf, nil
}

// UnmarshalBinary decodes an ExtendedEvent from the BPF binary wire format.
func (e *ExtendedEvent) UnmarshalBinary(data []byte) error {
	if len(data) < ExtendedEventHeaderSize {
		return fmt.Errorf("buffer too small: got %d bytes, need at least %d", len(data), ExtendedEventHeaderSize)
	}

	e.Timestamp = binary.LittleEndian.Uint64(data[0:8])
	e.PID = binary.LittleEndian.Uint32(data[8:12])
	e.PPID = binary.LittleEndian.Uint32(data[12:16])
	e.TGID = binary.LittleEndian.Uint32(data[16:20])
	e.CgroupID = binary.LittleEndian.Uint64(data[24:32])
	copy(e.Comm[:], data[32:48])
	e.EventType = data[48]
	e.PayloadLen = binary.LittleEndian.Uint16(data[50:52])

	totalNeeded := ExtendedEventHeaderSize + int(e.PayloadLen)
	if len(data) < totalNeeded {
		return fmt.Errorf("buffer too small for payload: got %d bytes, need %d (header %d + payload %d)",
			len(data), totalNeeded, ExtendedEventHeaderSize, e.PayloadLen)
	}

	if e.PayloadLen > 0 {
		e.Payload = make([]byte, e.PayloadLen)
		copy(e.Payload, data[52:52+int(e.PayloadLen)])
	} else {
		e.Payload = nil
	}

	return nil
}

// CommString returns the comm field as a trimmed Go string.
func (e *ExtendedEvent) CommString() string {
	n := bytes.IndexByte(e.Comm[:], 0)
	if n < 0 {
		n = TaskCommLen
	}
	return string(e.Comm[:n])
}

// EventTypeString returns a human-readable event type name for extended events.
func (e *ExtendedEvent) EventTypeString() string {
	switch e.EventType {
	case EventTypeVFS:
		return "filesystem_io"
	case EventTypeOOM:
		return "memory_pressure"
	case EventTypeDNS:
		return "dns_resolution"
	case EventTypeCgroup:
		return "cgroup_resource"
	case EventTypeSecurity:
		return "network_audit"
	default:
		return fmt.Sprintf("unknown(%d)", e.EventType)
	}
}

// ---------------------------------------------------------------------------
// Payload sizes
// ---------------------------------------------------------------------------

const (
	VFSPayloadSize             = 280
	OOMPayloadSize             = 36
	DNSPayloadSize             = 268
	CgroupResourcePayloadSize  = 32
	NetworkAuditPayloadSize    = 8
)

// ---------------------------------------------------------------------------
// VFSPayload (event_type 3) — 280 bytes
// ---------------------------------------------------------------------------
//
// Layout:
//   offset   0: char file_path[256]  (256 bytes)
//   offset 256: u64  latency_ns      (8 bytes)
//   offset 264: u64  bytes_xfer      (8 bytes)
//   offset 272: u8   slow_io         (1 byte)
//   offset 273: u8   op_type         (1 byte)  0=read, 1=write
//   offset 274: [6 bytes padding]
//   total: 280 bytes

type VFSPayload struct {
	FilePath  [256]byte
	LatencyNs uint64
	BytesXfer uint64
	SlowIO    uint8
	OpType    uint8
}

func (p *VFSPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, VFSPayloadSize)
	copy(buf[0:256], p.FilePath[:])
	binary.LittleEndian.PutUint64(buf[256:264], p.LatencyNs)
	binary.LittleEndian.PutUint64(buf[264:272], p.BytesXfer)
	buf[272] = p.SlowIO
	buf[273] = p.OpType
	// 6 bytes padding at offset 274
	return buf, nil
}

func (p *VFSPayload) UnmarshalBinary(data []byte) error {
	if len(data) < VFSPayloadSize {
		return fmt.Errorf("VFSPayload buffer too small: got %d, need %d", len(data), VFSPayloadSize)
	}
	copy(p.FilePath[:], data[0:256])
	p.LatencyNs = binary.LittleEndian.Uint64(data[256:264])
	p.BytesXfer = binary.LittleEndian.Uint64(data[264:272])
	p.SlowIO = data[272]
	p.OpType = data[273]
	return nil
}

// ---------------------------------------------------------------------------
// OOMPayload (event_type 4) — 36 bytes
// ---------------------------------------------------------------------------
//
// Layout:
//   offset  0: u8   sub_type       (1 byte)  0=oom_kill, 1=alloc_failure
//   offset  1: [3 bytes padding]
//   offset  4: u32  killed_pid     (4 bytes)
//   offset  8: char killed_comm[16](16 bytes)
//   offset 24: s32  oom_score_adj  (4 bytes)
//   offset 28: u32  page_order     (4 bytes)
//   offset 32: u32  gfp_flags      (4 bytes)
//   total: 36 bytes

type OOMPayload struct {
	SubType     uint8
	KilledPID   uint32
	KilledComm  [TaskCommLen]byte
	OOMScoreAdj int32
	PageOrder   uint32
	GFPFlags    uint32
}

func (p *OOMPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, OOMPayloadSize)
	buf[0] = p.SubType
	// 3 bytes padding at offset 1
	binary.LittleEndian.PutUint32(buf[4:8], p.KilledPID)
	copy(buf[8:24], p.KilledComm[:])
	binary.LittleEndian.PutUint32(buf[24:28], uint32(p.OOMScoreAdj))
	binary.LittleEndian.PutUint32(buf[28:32], p.PageOrder)
	binary.LittleEndian.PutUint32(buf[32:36], p.GFPFlags)
	return buf, nil
}

func (p *OOMPayload) UnmarshalBinary(data []byte) error {
	if len(data) < OOMPayloadSize {
		return fmt.Errorf("OOMPayload buffer too small: got %d, need %d", len(data), OOMPayloadSize)
	}
	p.SubType = data[0]
	p.KilledPID = binary.LittleEndian.Uint32(data[4:8])
	copy(p.KilledComm[:], data[8:24])
	p.OOMScoreAdj = int32(binary.LittleEndian.Uint32(data[24:28]))
	p.PageOrder = binary.LittleEndian.Uint32(data[28:32])
	p.GFPFlags = binary.LittleEndian.Uint32(data[32:36])
	return nil
}

// ---------------------------------------------------------------------------
// DNSPayload (event_type 5) — 268 bytes
// ---------------------------------------------------------------------------
//
// Layout:
//   offset   0: char domain[253]    (253 bytes)
//   offset 253: [3 bytes padding]
//   offset 256: u64  latency_ns     (8 bytes)
//   offset 264: u16  response_code  (2 bytes)
//   offset 266: u8   timed_out      (1 byte)
//   offset 267: [1 byte padding]
//   total: 268 bytes

type DNSPayload struct {
	Domain       [253]byte
	LatencyNs    uint64
	ResponseCode uint16
	TimedOut     uint8
}

func (p *DNSPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, DNSPayloadSize)
	copy(buf[0:253], p.Domain[:])
	// 3 bytes padding at offset 253
	binary.LittleEndian.PutUint64(buf[256:264], p.LatencyNs)
	binary.LittleEndian.PutUint16(buf[264:266], p.ResponseCode)
	buf[266] = p.TimedOut
	// 1 byte padding at offset 267
	return buf, nil
}

func (p *DNSPayload) UnmarshalBinary(data []byte) error {
	if len(data) < DNSPayloadSize {
		return fmt.Errorf("DNSPayload buffer too small: got %d, need %d", len(data), DNSPayloadSize)
	}
	copy(p.Domain[:], data[0:253])
	p.LatencyNs = binary.LittleEndian.Uint64(data[256:264])
	p.ResponseCode = binary.LittleEndian.Uint16(data[264:266])
	p.TimedOut = data[266]
	return nil
}

// ---------------------------------------------------------------------------
// CgroupResourcePayload (event_type 6) — 32 bytes
// ---------------------------------------------------------------------------
//
// Layout:
//   offset  0: u64 cpu_usage_ns       (8 bytes)
//   offset  8: u64 memory_usage_bytes (8 bytes)
//   offset 16: u64 memory_limit_bytes (8 bytes)
//   offset 24: u8  memory_pressure    (1 byte)
//   offset 25: [7 bytes padding]
//   total: 32 bytes

type CgroupResourcePayload struct {
	CPUUsageNs       uint64
	MemoryUsageBytes uint64
	MemoryLimitBytes uint64
	MemoryPressure   uint8
}

func (p *CgroupResourcePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, CgroupResourcePayloadSize)
	binary.LittleEndian.PutUint64(buf[0:8], p.CPUUsageNs)
	binary.LittleEndian.PutUint64(buf[8:16], p.MemoryUsageBytes)
	binary.LittleEndian.PutUint64(buf[16:24], p.MemoryLimitBytes)
	buf[24] = p.MemoryPressure
	// 7 bytes padding at offset 25
	return buf, nil
}

func (p *CgroupResourcePayload) UnmarshalBinary(data []byte) error {
	if len(data) < CgroupResourcePayloadSize {
		return fmt.Errorf("CgroupResourcePayload buffer too small: got %d, need %d", len(data), CgroupResourcePayloadSize)
	}
	p.CPUUsageNs = binary.LittleEndian.Uint64(data[0:8])
	p.MemoryUsageBytes = binary.LittleEndian.Uint64(data[8:16])
	p.MemoryLimitBytes = binary.LittleEndian.Uint64(data[16:24])
	p.MemoryPressure = data[24]
	return nil
}

// ---------------------------------------------------------------------------
// NetworkAuditPayload (event_type 7) — 8 bytes
// ---------------------------------------------------------------------------
//
// Layout:
//   offset 0: u32 dst_addr   (4 bytes)
//   offset 4: u16 dst_port   (2 bytes)
//   offset 6: u8  protocol   (1 byte)  6=TCP, 17=UDP
//   offset 7: [1 byte padding]
//   total: 8 bytes

type NetworkAuditPayload struct {
	DstAddr  uint32
	DstPort  uint16
	Protocol uint8
}

func (p *NetworkAuditPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, NetworkAuditPayloadSize)
	binary.LittleEndian.PutUint32(buf[0:4], p.DstAddr)
	binary.LittleEndian.PutUint16(buf[4:6], p.DstPort)
	buf[6] = p.Protocol
	// 1 byte padding at offset 7
	return buf, nil
}

func (p *NetworkAuditPayload) UnmarshalBinary(data []byte) error {
	if len(data) < NetworkAuditPayloadSize {
		return fmt.Errorf("NetworkAuditPayload buffer too small: got %d, need %d", len(data), NetworkAuditPayloadSize)
	}
	p.DstAddr = binary.LittleEndian.Uint32(data[0:4])
	p.DstPort = binary.LittleEndian.Uint16(data[4:6])
	p.Protocol = data[6]
	return nil
}
