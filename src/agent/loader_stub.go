//go:build !linux

package main

import (
	"fmt"
	"sync"
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
