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

// Store defines the interface for heartbeat event persistence.
type Store interface {
	Save(ctx context.Context, event Heartbeat) error
	GetByTimeRange(ctx context.Context, from, to time.Time) ([]Heartbeat, error)
	GetLatestByNode(ctx context.Context, nodeName string) (*Heartbeat, error)
	Ping(ctx context.Context) error
}

// MemoryStore is an in-memory implementation of the Store interface.
type MemoryStore struct {
	mu         sync.Mutex
	heartbeats []Heartbeat
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
