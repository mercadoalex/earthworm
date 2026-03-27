package main

import (
	"fmt"
	"sync"
	"time"
)

// ConnectionRecord represents a single observed network connection.
type ConnectionRecord struct {
	SourcePod string    `json:"sourcePod"`
	SourceNS  string    `json:"sourceNamespace"`
	DstAddr   string    `json:"dstAddr"`
	DstPort   uint16    `json:"dstPort"`
	Protocol  string    `json:"protocol"`
	LastSeen  time.Time `json:"lastSeen"`
	NodeName  string    `json:"nodeName"`
}

// NetworkTopologyMap tracks unique connection tuples within a configurable window.
type NetworkTopologyMap struct {
	mu          sync.RWMutex
	connections map[string]*ConnectionRecord
	window      time.Duration
	hub         *Hub
}

// NewNetworkTopologyMap creates a new topology map with the given expiration window.
func NewNetworkTopologyMap(window time.Duration, hub *Hub) *NetworkTopologyMap {
	return &NetworkTopologyMap{
		connections: make(map[string]*ConnectionRecord),
		window:      window,
		hub:         hub,
	}
}

// connectionKey builds a unique key for a connection tuple.
func connectionKey(pod, ns, addr string, port uint16, proto string) string {
	return fmt.Sprintf("%s|%s|%s|%d|%s", pod, ns, addr, port, proto)
}

// Record inserts or updates a connection from an EnrichedEvent.
// Returns true if this is a new connection (not previously seen).
func (m *NetworkTopologyMap) Record(event EnrichedEvent) bool {
	key := connectionKey(event.PodName, event.Namespace, event.AuditDstAddr, event.AuditDstPort, event.AuditProtocol)

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.connections[key]; ok {
		existing.LastSeen = event.Timestamp
		return false
	}

	rec := &ConnectionRecord{
		SourcePod: event.PodName,
		SourceNS:  event.Namespace,
		DstAddr:   event.AuditDstAddr,
		DstPort:   event.AuditDstPort,
		Protocol:  event.AuditProtocol,
		LastSeen:  event.Timestamp,
		NodeName:  event.NodeName,
	}
	m.connections[key] = rec

	if m.hub != nil {
		m.hub.BroadcastTopologyUpdate(*rec)
	}

	return true
}

// Expire removes connection records older than the configured window.
func (m *NetworkTopologyMap) Expire() {
	cutoff := time.Now().UTC().Add(-m.window)

	m.mu.Lock()
	defer m.mu.Unlock()

	for key, rec := range m.connections {
		if rec.LastSeen.Before(cutoff) {
			delete(m.connections, key)
		}
	}
}

// Connections returns all active connection records.
func (m *NetworkTopologyMap) Connections() []ConnectionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ConnectionRecord, 0, len(m.connections))
	for _, rec := range m.connections {
		result = append(result, *rec)
	}
	return result
}
