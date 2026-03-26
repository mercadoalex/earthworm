package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

	// On non-Linux platforms, /proc and /sys/fs/cgroup are not available.
	// Return early after validating the kubelet API is reachable.
	if runtime.GOOS != "linux" {
		log.Printf("CgroupResolver: skipping /proc scan on %s (not Linux)", runtime.GOOS)
		return nil
	}

	// Build a map from container ID suffix to pod identity for quick lookup.
	// The kubelet containerID field looks like "containerd://abc123..." — we
	// extract the hex ID portion after "://".
	type containerInfo struct {
		pod       PodIdentity
		idSuffix  string // the hex ID after "://"
	}
	var containers []containerInfo
	for _, pod := range podList.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			parts := strings.SplitN(cs.ContainerID, "://", 2)
			if len(parts) != 2 || parts[1] == "" {
				continue
			}
			containers = append(containers, containerInfo{
				pod: PodIdentity{
					PodName:       pod.Metadata.Name,
					Namespace:     pod.Metadata.Namespace,
					ContainerName: cs.Name,
					NodeName:      cr.nodeName,
				},
				idSuffix: parts[1],
			})
		}
	}

	if len(containers) == 0 {
		log.Printf("CgroupResolver: no containers found in kubelet response")
		return nil
	}

	// Scan /proc to find PIDs and their cgroup v2 paths, then match against
	// known container IDs.
	newCache := make(map[uint64]PodIdentity)

	procEntries, err := os.ReadDir("/proc")
	if err != nil {
		return fmt.Errorf("failed to read /proc: %w", err)
	}

	for _, entry := range procEntries {
		if !entry.IsDir() {
			continue
		}
		// Only look at numeric directories (PIDs)
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}

		cgroupPath, err := readProcCgroup(entry.Name())
		if err != nil {
			continue // process may have exited
		}

		// Match this cgroup path against known container IDs
		for _, ci := range containers {
			if !strings.Contains(cgroupPath, ci.idSuffix) {
				continue
			}

			// Found a match — stat the cgroup v2 path to get the inode (cgroup ID)
			cgroupFSPath := filepath.Join("/sys/fs/cgroup", cgroupPath)
			cgroupID, err := cgroupInode(cgroupFSPath)
			if err != nil {
				log.Printf("CgroupResolver: failed to stat cgroup path %s: %v", cgroupFSPath, err)
				continue
			}

			newCache[cgroupID] = ci.pod
			break // this PID matched one container, move to next PID
		}
	}

	// Swap in the new cache atomically
	cr.mu.Lock()
	cr.cache = newCache
	cr.mu.Unlock()

	log.Printf("CgroupResolver: refreshed cache with %d cgroup-to-pod mappings from %d pods",
		len(newCache), len(podList.Items))

	return nil
}

// readProcCgroup reads /proc/<pid>/cgroup and returns the cgroup v2 path.
// For cgroup v2 unified hierarchy, the file contains a single line like:
//
//	0::/kubepods/besteffort/pod<uid>/<container-id>
func readProcCgroup(pid string) (string, error) {
	f, err := os.Open(filepath.Join("/proc", pid, "cgroup"))
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// cgroup v2 lines start with "0::"
		if strings.HasPrefix(line, "0::") {
			return strings.TrimPrefix(line, "0::"), nil
		}
	}
	return "", fmt.Errorf("no cgroup v2 entry found for pid %s", pid)
}

// cgroupInode returns the inode number of the given cgroup filesystem path.
// The kernel uses the cgroup v2 directory inode as the cgroup ID reported
// by bpf_get_current_cgroup_id().
func cgroupInode(path string) (uint64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("failed to get syscall.Stat_t for %s", path)
	}
	return stat.Ino, nil
}
