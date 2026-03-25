package main

import (
	"context"
	"sync"
	"time"
)

// Heartbeat represents a heartbeat event from a Kubernetes node.
type Heartbeat struct {
	NodeName  string    `json:"nodeName"`
	Namespace string    `json:"namespace"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
	EbpfPID   uint32    `json:"ebpfPid,omitempty"`
	EbpfComm  string    `json:"ebpfComm,omitempty"`
}

// Store defines the interface for heartbeat and kernel event persistence.
type Store interface {
	Save(ctx context.Context, event Heartbeat) error
	GetByTimeRange(ctx context.Context, from, to time.Time) ([]Heartbeat, error)
	GetLatestByNode(ctx context.Context, nodeName string) (*Heartbeat, error)
	Ping(ctx context.Context) error

	// Kernel event methods
	SaveKernelEvent(ctx context.Context, event EnrichedEvent) error
	GetKernelEvents(ctx context.Context, nodeName string, from, to time.Time) ([]EnrichedEvent, error)
	GetKernelEventsByType(ctx context.Context, nodeName string, eventType string, from, to time.Time) ([]EnrichedEvent, error)

	// Causal chain methods
	SaveCausalChain(ctx context.Context, chain CausalChain) error
	GetCausalChains(ctx context.Context, nodeName string, from, to time.Time) ([]CausalChain, error)
}

// MemoryStore is an in-memory implementation of the Store interface.
type MemoryStore struct {
	mu           sync.Mutex
	heartbeats   []Heartbeat
	kernelEvents []EnrichedEvent
	causalChains []CausalChain
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (m *MemoryStore) Save(_ context.Context, event Heartbeat) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.heartbeats = append(m.heartbeats, event)
	return nil
}

func (m *MemoryStore) GetByTimeRange(_ context.Context, from, to time.Time) ([]Heartbeat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Heartbeat
	for _, hb := range m.heartbeats {
		if !hb.Timestamp.Before(from) && !hb.Timestamp.After(to) {
			result = append(result, hb)
		}
	}
	return result, nil
}

func (m *MemoryStore) GetLatestByNode(_ context.Context, nodeName string) (*Heartbeat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var latest *Heartbeat
	for i := range m.heartbeats {
		if m.heartbeats[i].NodeName == nodeName {
			if latest == nil || m.heartbeats[i].Timestamp.After(latest.Timestamp) {
				latest = &m.heartbeats[i]
			}
		}
	}
	return latest, nil
}

func (m *MemoryStore) Ping(_ context.Context) error {
	return nil
}

func (m *MemoryStore) SaveKernelEvent(_ context.Context, event EnrichedEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kernelEvents = append(m.kernelEvents, event)
	return nil
}

func (m *MemoryStore) GetKernelEvents(_ context.Context, nodeName string, from, to time.Time) ([]EnrichedEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []EnrichedEvent
	for _, e := range m.kernelEvents {
		if e.NodeName == nodeName && !e.Timestamp.Before(from) && !e.Timestamp.After(to) {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *MemoryStore) GetKernelEventsByType(_ context.Context, nodeName string, eventType string, from, to time.Time) ([]EnrichedEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []EnrichedEvent
	for _, e := range m.kernelEvents {
		if e.NodeName == nodeName && e.EventType == eventType && !e.Timestamp.Before(from) && !e.Timestamp.After(to) {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *MemoryStore) SaveCausalChain(_ context.Context, chain CausalChain) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.causalChains = append(m.causalChains, chain)
	return nil
}

func (m *MemoryStore) GetCausalChains(_ context.Context, nodeName string, from, to time.Time) ([]CausalChain, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []CausalChain
	for _, c := range m.causalChains {
		if c.NodeName == nodeName && !c.Timestamp.Before(from) && !c.Timestamp.After(to) {
			result = append(result, c)
		}
	}
	return result, nil
}
