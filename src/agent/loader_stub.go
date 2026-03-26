//go:build !linux

package main

import (
	"fmt"
	"sync"
)

// BPFLoaderIface defines the interface that both loader_stub.go and
// loader_linux.go must satisfy: NewBPFLoader() → Load() → Close() → Programs().
// This compile-time check ensures interface parity across platforms.
type BPFLoaderIface interface {
	Load() error
	Close() error
	Programs() map[string]*BPFProgram
}

// Compile-time check: *BPFLoader must satisfy BPFLoaderIface.
var _ BPFLoaderIface = (*BPFLoader)(nil)

// Compile-time verification that extended event type constants (defined in
// extended_event.go) are accessible from the non-Linux build context.
// These constants are used by macOS tests to reference event types 3–7.
var (
	_ = EventTypeVFS
	_ = EventTypeOOM
	_ = EventTypeDNS
	_ = EventTypeCgroup
	_ = EventTypeSecurity
)

// BPFProgram is a stub for ebpf.Program on non-Linux platforms.
type BPFProgram struct {
	Name   string
	Closed bool
}

// Close closes the stub program.
func (p *BPFProgram) Close() error {
	p.Closed = true
	return nil
}

// BPFLoader manages the lifecycle of eBPF programs.
// On non-Linux platforms, it returns errors indicating eBPF is unsupported.
type BPFLoader struct {
	mu       sync.Mutex
	programs map[string]*BPFProgram
	closed   bool
}

// NewBPFLoader creates a new BPFLoader instance.
func NewBPFLoader() *BPFLoader {
	return &BPFLoader{
		programs: make(map[string]*BPFProgram),
	}
}

// Load returns an error on non-Linux platforms.
func (l *BPFLoader) Load() error {
	return fmt.Errorf("eBPF not supported on this platform")
}

// Close releases all resources. On non-Linux, clears the programs map.
func (l *BPFLoader) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	for name, prog := range l.programs {
		prog.Close()
		delete(l.programs, name)
	}

	l.closed = true
	return nil
}

// Programs returns loaded program references.
// On non-Linux, returns an empty map after Close().
func (l *BPFLoader) Programs() map[string]*BPFProgram {
	l.mu.Lock()
	defer l.mu.Unlock()

	out := make(map[string]*BPFProgram, len(l.programs))
	for k, v := range l.programs {
		out[k] = v
	}
	return out
}
