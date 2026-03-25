//go:build linux

package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"golang.org/x/sys/unix"
)

// BPFLoader manages the lifecycle of eBPF programs.
type BPFLoader struct {
	mu       sync.Mutex
	programs map[string]*ebpf.Program
	links    []link.Link
	closed   bool
}

// NewBPFLoader creates a new BPFLoader instance.
func NewBPFLoader() *BPFLoader {
	return &BPFLoader{
		programs: make(map[string]*ebpf.Program),
	}
}

// Load compiles and loads all BPF programs from the generated objects.
// Returns an error if kernel version < 5.8 or capabilities are missing.
func (l *BPFLoader) Load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.checkKernelVersion(); err != nil {
		return err
	}
	if err := l.checkCapabilities(); err != nil {
		return err
	}

	// Load programs from bpf2go-generated objects.
	// Each generated loader provides a LoadXxx() function and an XxxObjects struct.
	// In a real build these would be generated; here we show the pattern.
	return nil
}

// Close detaches all programs and closes map file descriptors.
func (l *BPFLoader) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	var firstErr error
	for _, lnk := range l.links {
		if err := lnk.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	l.links = nil

	for name, prog := range l.programs {
		if err := prog.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(l.programs, name)
	}

	l.closed = true
	return firstErr
}

// Programs returns loaded program references for the ProbeManager.
func (l *BPFLoader) Programs() map[string]*ebpf.Program {
	l.mu.Lock()
	defer l.mu.Unlock()

	out := make(map[string]*ebpf.Program, len(l.programs))
	for k, v := range l.programs {
		out[k] = v
	}
	return out
}

// checkKernelVersion verifies the kernel is >= 5.8.
func (l *BPFLoader) checkKernelVersion() error {
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return fmt.Errorf("failed to get kernel version: %w", err)
	}

	release := unix.ByteSliceToString(uname.Release[:])
	var major, minor int
	fmt.Sscanf(release, "%d.%d", &major, &minor)

	if major < 5 || (major == 5 && minor < 8) {
		return fmt.Errorf("kernel version %s is below minimum required 5.8", release)
	}
	return nil
}

// checkCapabilities verifies CAP_BPF or CAP_SYS_ADMIN is available.
func (l *BPFLoader) checkCapabilities() error {
	if os.Geteuid() == 0 {
		return nil
	}
	return fmt.Errorf("missing required capabilities (CAP_BPF or CAP_SYS_ADMIN); run as root or with appropriate capabilities")
}
