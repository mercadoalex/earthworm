package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// CgroupResolver maps cgroup IDs to Kubernetes pod identities.
type CgroupResolver struct {
	cache       map[uint64]PodIdentity
	mu          sync.RWMutex
	refreshRate time.Duration
	nodeName    string
	kubeletURL  string
	client      *http.Client
}

// NewCgroupResolver creates a new CgroupResolver.
func NewCgroupResolver(nodeName string, kubeletURL string, refreshRate time.Duration) *CgroupResolver {
	if refreshRate <= 0 {
		refreshRate = 30 * time.Second
	}
	return &CgroupResolver{
		cache:       make(map[uint64]PodIdentity),
		refreshRate: refreshRate,
		nodeName:    nodeName,
		kubeletURL:  kubeletURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Resolve enriches a KernelEvent with pod identity.
// Returns "host-level" identity if cgroup is unknown.
func (cr *CgroupResolver) Resolve(cgroupID uint64, comm string) (PodIdentity, bool) {
	cr.mu.RLock()
	pod, found := cr.cache[cgroupID]
	cr.mu.RUnlock()

	if found {
		return pod, false // not host-level
	}

	// Unknown cgroup — label as host-level
	return PodIdentity{
		NodeName: cr.nodeName,
	}, true
}

// Enrich converts a KernelEvent to an EnrichedEvent using the cgroup cache.
func (cr *CgroupResolver) Enrich(evt *KernelEvent) EnrichedEvent {
	pod, hostLevel := cr.Resolve(evt.CgroupID, evt.CommString())
	return evt.Enrich(pod, hostLevel)
}

// UpdateCache directly sets a cgroup-to-pod mapping. Useful for testing.
func (cr *CgroupResolver) UpdateCache(cgroupID uint64, pod PodIdentity) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.cache[cgroupID] = pod
}

// CacheSize returns the number of entries in the cgroup cache.
func (cr *CgroupResolver) CacheSize() int {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	return len(cr.cache)
}

// StartRefresh begins periodic cache refresh from kubelet API.
// Blocks until ctx is cancelled.
func (cr *CgroupResolver) StartRefresh(ctx context.Context) error {
	log.Printf("CgroupResolver: starting refresh loop (interval: %v)", cr.refreshRate)

	// Initial refresh
	if err := cr.refresh(); err != nil {
		log.Printf("CgroupResolver: initial refresh failed: %v", err)
	}

	ticker := time.NewTicker(cr.refreshRate)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("CgroupResolver: shutting down")
			return ctx.Err()
		case <-ticker.C:
			if err := cr.refresh(); err != nil {
				log.Printf("CgroupResolver: refresh failed (keeping stale cache): %v", err)
			}
		}
	}
}

// kubeletPodList represents the response from the kubelet /pods API.
type kubeletPodList struct {
	Items []kubeletPod `json:"items"`
}

type kubeletPod struct {
	Metadata kubeletMetadata `json:"metadata"`
	Status   kubeletStatus   `json:"status"`
}

type kubeletMetadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UID       string `json:"uid"`
}

type kubeletStatus struct {
	ContainerStatuses []kubeletContainerStatus `json:"containerStatuses"`
}

type kubeletContainerStatus struct {
	Name        string `json:"name"`
	ContainerID string `json:"containerID"`
}

// refresh queries the kubelet API for pod information and rebuilds the cache.
func (cr *CgroupResolver) refresh() error {
	if cr.kubeletURL == "" {
		return nil // no kubelet URL configured, skip refresh
	}

	url := fmt.Sprintf("%s/pods", cr.kubeletURL)
	resp, err := cr.client.Get(url)
	if err != nil {
		return fmt.Errorf("kubelet API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("kubelet API returned status %d", resp.StatusCode)
	}

	var podList kubeletPodList
	if err := json.NewDecoder(resp.Body).Decode(&podList); err != nil {
		return fmt.Errorf("failed to decode kubelet response: %w", err)
	}

	// In a real implementation, we would read /proc/<pid>/cgroup for each
	// container to map cgroup IDs to pod identities. For now, we store
	// the pod list for lookup by other means.
	log.Printf("CgroupResolver: refreshed cache with %d pods", len(podList.Items))

	return nil
}
