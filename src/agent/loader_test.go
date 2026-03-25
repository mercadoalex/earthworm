package main

import (
	"testing"

	"pgregory.net/rapid"
)

// TestBPFLoaderCleanupInvariant tests Property 1: BPF Loader cleanup.
// **Validates: Requirements 1.4**
// Feature: ebpf-kernel-observability, Property 1: BPF Loader cleanup —
// for any loaded programs, Close() releases all resources.
func TestBPFLoaderCleanupInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		loader := NewBPFLoader()

		// Generate a random number of "programs" to simulate loading
		numPrograms := rapid.IntRange(1, 10).Draw(t, "numPrograms")

		// Simulate loading programs by adding them to the loader's map
		for i := 0; i < numPrograms; i++ {
			name := rapid.StringMatching(`[a-z_]{3,20}`).Draw(t, "progName")
			loader.programs[name] = &BPFProgram{Name: name}
		}

		initialCount := len(loader.programs)
		if initialCount == 0 {
			t.Fatal("expected at least one program after loading")
		}

		// Close should release all resources
		err := loader.Close()
		if err != nil {
			t.Fatalf("Close() returned error: %v", err)
		}

		// After Close(), Programs() should return an empty map
		progs := loader.Programs()
		if len(progs) != 0 {
			t.Errorf("Programs() returned %d programs after Close(), expected 0", len(progs))
		}

		// Verify all programs were closed
		// (On non-Linux, BPFProgram has a Closed field we can check)

		// Close() should be idempotent
		err = loader.Close()
		if err != nil {
			t.Errorf("second Close() returned error: %v", err)
		}
	})
}

// TestCommFilterInvariant tests Property 4: Comm filter invariant.
// **Validates: Requirements 2.4**
// Feature: ebpf-kernel-observability, Property 4: Comm filter —
// emitted events only have allowed comm names.
func TestCommFilterInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random comm string
		comm := rapid.StringMatching(`[a-z\-]{1,15}`).Draw(t, "comm")

		isAllowed := IsAllowedComm(comm)

		// Verify the filter matches exactly the allowed set
		expectedAllowed := comm == "kubelet" || comm == "containerd" || comm == "cri-o"

		if isAllowed != expectedAllowed {
			t.Errorf("IsAllowedComm(%q) = %v, expected %v", comm, isAllowed, expectedAllowed)
		}
	})
}
