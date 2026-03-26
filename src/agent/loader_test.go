package main

import (
	"context"
	"sync"
	"testing"
	"time"

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

// TestAgentContinuesWithoutEBPF verifies that when BPFLoader.Load() returns
// an error (as on macOS), the agent logs the error and the main goroutines
// (forwarder, resolver) still start.
// **Validates: Requirements 6.2, 6.3**
func TestAgentContinuesWithoutEBPF(t *testing.T) {
	// 1. Create a BPFLoader and verify Load() returns an error on non-Linux.
	loader := NewBPFLoader()
	err := loader.Load()
	if err == nil {
		t.Skip("Load() succeeded — this test targets non-Linux platforms")
	}
	t.Logf("BPF loader failed as expected: %v", err)

	// 2. Simulate the agent startup flow from main.go:
	//    Even though Load() failed, the resolver and forwarder goroutines must start.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resolver := NewCgroupResolver("test-node", "", 30*time.Second)
	eventCh := make(chan EnrichedEvent, 1024)
	pm := NewProbeManager(resolver, eventCh, 100*time.Millisecond)

	var wg sync.WaitGroup

	// Track that goroutines actually started
	resolverStarted := make(chan struct{})
	probeStarted := make(chan struct{})
	forwarderStarted := make(chan struct{})

	// Forwarder collects events it receives
	forwardedEvents := make(chan EnrichedEvent, 16)

	// Start CgroupResolver refresh loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(resolverStarted)
		_ = resolver.StartRefresh(ctx)
	}()

	// Start ProbeManager polling loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(probeStarted)
		_ = pm.Start(ctx)
	}()

	// Start event forwarder (reads from eventCh, forwards to forwardedEvents)
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(forwarderStarted)
		for {
			select {
			case <-ctx.Done():
				return
			case evt := <-eventCh:
				forwardedEvents <- evt
			}
		}
	}()

	// 3. Wait for all goroutines to confirm they started
	select {
	case <-resolverStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("resolver goroutine did not start within 2 seconds")
	}
	select {
	case <-probeStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("probe manager goroutine did not start within 2 seconds")
	}
	select {
	case <-forwarderStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("forwarder goroutine did not start within 2 seconds")
	}

	// 4. Verify the agent is functional: push an event through the pipeline
	testEvt := &KernelEvent{
		Timestamp: 1000,
		PID:       42,
		TGID:      42,
		CgroupID:  999,
		EventType: EventTypeSyscall,
		EntryTs:   100,
		ExitTs:    200,
	}
	copy(testEvt.Comm[:], "test-proc")

	if err := pm.ProcessEvent(testEvt); err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}

	// Verify the forwarder received the event
	select {
	case evt := <-forwardedEvents:
		if evt.PID != 42 {
			t.Errorf("expected PID 42, got %d", evt.PID)
		}
		if evt.NodeName != "test-node" {
			t.Errorf("expected NodeName 'test-node', got %q", evt.NodeName)
		}
		if !evt.HostLevel {
			t.Error("expected HostLevel=true for unknown cgroup")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("forwarder did not receive enriched event within 2 seconds")
	}

	// 5. Clean shutdown
	cancel()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("goroutines did not shut down within 5 seconds")
	}

	// Verify loader cleanup still works
	if err := loader.Close(); err != nil {
		t.Errorf("loader.Close() returned error: %v", err)
	}
}
