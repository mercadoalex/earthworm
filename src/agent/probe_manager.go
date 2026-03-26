package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ProbeManager reads events from BPF ring buffers and forwards them
// to the CgroupResolver for enrichment.
type ProbeManager struct {
	resolver      *CgroupResolver
	eventCh       chan<- EnrichedEvent
	pollInterval  time.Duration
	droppedCnt    atomic.Uint64
	lastDropLog   time.Time
	lastDropLogMu sync.Mutex
}

// NewProbeManager creates a new ProbeManager.
func NewProbeManager(resolver *CgroupResolver, eventCh chan<- EnrichedEvent, pollInterval time.Duration) *ProbeManager {
	if pollInterval <= 0 {
		pollInterval = 100 * time.Millisecond
	}
	return &ProbeManager{
		resolver:     resolver,
		eventCh:      eventCh,
		pollInterval: pollInterval,
	}
}

// Start begins polling the ring buffer. Blocks until ctx is cancelled.
// On non-Linux platforms, this simulates the polling loop without actual BPF.
func (pm *ProbeManager) Start(ctx context.Context) error {
	log.Printf("ProbeManager: starting with poll interval %v", pm.pollInterval)

	ticker := time.NewTicker(pm.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("ProbeManager: shutting down")
			return ctx.Err()
		case <-ticker.C:
			// On Linux, this would read from ringbuf.Reader.
			// The actual ring buffer reading is platform-specific.
		}
	}
}

// ProcessRawEvent decodes a raw binary event, enriches it, and forwards it.
// This is the core event processing pipeline used by both real and test code.
// Dispatch is based on the event_type byte at offset 48:
//   - event_type 0–2 → KernelEvent codec (120 bytes)
//   - event_type 3–7 → ExtendedEvent codec (variable length)
func (pm *ProbeManager) ProcessRawEvent(data []byte) error {
	if len(data) < 49 {
		return fmt.Errorf("record too short: %d bytes", len(data))
	}
	eventType := data[48]
	if eventType <= 2 {
		return pm.processKernelEvent(data)
	}
	return pm.processExtendedEvent(data)
}

// processKernelEvent decodes a legacy 120-byte KernelEvent and forwards it.
func (pm *ProbeManager) processKernelEvent(data []byte) error {
	evt, err := DecodeKernelEvent(data)
	if err != nil {
		return fmt.Errorf("decode event: %w", err)
	}
	return pm.ProcessEvent(evt)
}

// processExtendedEvent decodes an ExtendedEvent, decodes its payload by
// event_type, enriches via CgroupResolver, and forwards to eventCh.
func (pm *ProbeManager) processExtendedEvent(data []byte) error {
	var ext ExtendedEvent
	if err := ext.UnmarshalBinary(data); err != nil {
		return fmt.Errorf("decode extended event: %w", err)
	}

	enriched, err := pm.resolver.EnrichExtended(&ext)
	if err != nil {
		return fmt.Errorf("enrich extended event: %w", err)
	}

	select {
	case pm.eventCh <- enriched:
	default:
		pm.recordDrop()
	}

	return nil
}

// ProcessEvent enriches a decoded KernelEvent and forwards it to the event channel.
func (pm *ProbeManager) ProcessEvent(evt *KernelEvent) error {
	enriched := pm.resolver.Enrich(evt)

	select {
	case pm.eventCh <- enriched:
	default:
		pm.recordDrop()
	}

	return nil
}

// DroppedEvents returns the count of dropped ring buffer events.
func (pm *ProbeManager) DroppedEvents() uint64 {
	return pm.droppedCnt.Load()
}

// recordDrop increments the drop counter and logs at most once per 10 seconds.
func (pm *ProbeManager) recordDrop() {
	pm.droppedCnt.Add(1)

	pm.lastDropLogMu.Lock()
	defer pm.lastDropLogMu.Unlock()

	now := time.Now()
	if now.Sub(pm.lastDropLog) >= 10*time.Second {
		pm.lastDropLog = now
		log.Printf("ProbeManager: events dropped (total: %d)", pm.droppedCnt.Load())
	}
}

// ValidateEventFields checks that a KernelEvent has all required fields
// for its event type. Returns an error describing any missing fields.
func ValidateEventFields(evt *KernelEvent) error {
	// Common fields required for all event types
	if evt.Timestamp == 0 {
		return fmt.Errorf("missing timestamp")
	}
	if evt.PID == 0 {
		return fmt.Errorf("missing PID")
	}
	if evt.CommString() == "" {
		return fmt.Errorf("missing comm")
	}
	if evt.CgroupID == 0 {
		return fmt.Errorf("missing cgroup_id")
	}

	switch evt.EventType {
	case EventTypeSyscall:
		if evt.EntryTs == 0 {
			return fmt.Errorf("syscall event missing entry_ts")
		}
		if evt.ExitTs == 0 {
			return fmt.Errorf("syscall event missing exit_ts")
		}
		// syscall_nr can be 0 (read), so we don't validate it
		// ret_val can be 0 or negative, so we don't validate it

	case EventTypeProcess:
		if evt.PPID == 0 {
			return fmt.Errorf("process event missing PPID")
		}
		// child_pid is only required for fork events, exit_code only for exit events
		// We can't distinguish fork vs exit from the struct alone without additional context

	case EventTypeNetwork:
		if evt.SAddr == 0 {
			return fmt.Errorf("network event missing source address")
		}
		if evt.DAddr == 0 {
			return fmt.Errorf("network event missing destination address")
		}
		if evt.SPort == 0 {
			return fmt.Errorf("network event missing source port")
		}
		if evt.DPort == 0 {
			return fmt.Errorf("network event missing destination port")
		}

	default:
		return fmt.Errorf("unknown event type: %d", evt.EventType)
	}

	return nil
}
