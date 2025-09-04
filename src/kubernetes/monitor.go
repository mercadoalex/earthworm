// monitor.go
package kubernetes

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationclient "k8s.io/client-go/kubernetes/typed/coordination/v1"
)

// HeartbeatMonitor is a struct that holds the Kubernetes client and the channel for heartbeat data.
type HeartbeatMonitor struct {
	clientset        *kubernetes.Clientset
	heartbeatChannel chan string
}

// NewHeartbeatMonitor initializes a new HeartbeatMonitor with the given kubeconfig path.
func NewHeartbeatMonitor(kubeconfig string) (*HeartbeatMonitor, error) {
	// Load the kubeconfig file to create a Kubernetes client
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %v", err)
	}

	// Create a new Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	// Initialize the heartbeat channel
	heartbeatChannel := make(chan string)

	return &HeartbeatMonitor{
		clientset:        clientset,
		heartbeatChannel: heartbeatChannel,
	}, nil
}

// StartMonitoring begins watching for heartbeat events in the Kubernetes cluster.
func (hm *HeartbeatMonitor) StartMonitoring() {
	// Use a context with a timeout to manage the lifecycle of the monitoring
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Watch for events in the Kubernetes cluster
	watcher, err := hm.clientset.CoreV1().Pods("").Watch(ctx, v1.ListOptions{})
	if err != nil {
		log.Fatalf("failed to watch pods: %v", err)
	}

	// Handle events from the watcher
	go func() {
		for event := range watcher.ResultChan() {
			switch event.Type {
			case watch.Added:
				// Handle added pod event
				hm.handleHeartbeat(event.Object)
			case watch.Modified:
				// Handle modified pod event
				hm.handleHeartbeat(event.Object)
			case watch.Deleted:
				// Handle deleted pod event
				log.Printf("Pod deleted: %s", event.Object.GetName())
			}
		}
	}()
}

// handleHeartbeat processes the heartbeat data from the Kubernetes event.
func (hm *HeartbeatMonitor) handleHeartbeat(obj interface{}) {
	// Extract the pod name and status from the event object
	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Printf("unexpected type: %T", obj)
		return
	}

	// Send the heartbeat data to the channel
	heartbeatData := fmt.Sprintf("Pod: %s, Status: %s", pod.Name, pod.Status.Phase)
	hm.heartbeatChannel <- heartbeatData
	log.Printf("Heartbeat data sent: %s", heartbeatData)
}

// WatchLeaseHeartbeats watches Lease objects for node heartbeat updates.
func (hm *HeartbeatMonitor) WatchLeaseHeartbeats() {
	leaseClient := hm.clientset.CoordinationV1().Leases("kube-node-lease")
	watcher, err := leaseClient.Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("failed to watch Lease objects: %v", err)
	}

	go func() {
		for event := range watcher.ResultChan() {
			lease, ok := event.Object.(*coordinationv1.Lease)
			if !ok {
				log.Printf("unexpected type: %T", event.Object)
				continue
			}
			// Each update to a Lease object is a node heartbeat
			heartbeatData := fmt.Sprintf("Node: %s, Lease RenewTime: %v", lease.Name, lease.Spec.RenewTime)
			hm.heartbeatChannel <- heartbeatData
			log.Printf("Lease heartbeat: %s", heartbeatData)
		}
	}()
}

// GetHeartbeatChannel returns the channel for receiving heartbeat data.
func (hm *HeartbeatMonitor) GetHeartbeatChannel() chan string {
	return hm.heartbeatChannel
}

// Mock namespace and node data
var mockNamespaces = []string{
	"default",
	"kube-system",
	"earthworm-monitoring",
}

var mockNodes = []string{
	"node-1",
	"node-2",
	"node-3",
}

// Function to list namespaces
func ListNamespaces() []string {
	return mockNamespaces
}

// Function to list nodes
func ListNodes() []string {
	return mockNodes
}

// MockNode represents a simulated Kubernetes node
type MockNode struct {
	Name      string
	LastLease time.Time
	Status    string
}

// GenerateMockNodes creates a slice of 50 mock nodes
func GenerateMockNodes() []MockNode {
	nodes := make([]MockNode, 50)
	for i := 0; i < 50; i++ {
		nodes[i] = MockNode{
			Name:      fmt.Sprintf("node-%02d", i+1),
			LastLease: time.Now().Add(-time.Duration(rand.Intn(10)) * time.Second),
			Status:    "Ready",
		}
	}
	return nodes
}

// SimulateHeartbeats randomly updates node lease times and status
func SimulateHeartbeats(nodes []MockNode) {
	for i := range nodes {
		// Randomly simulate lease renewal and possible status changes
		nodes[i].LastLease = time.Now().Add(-time.Duration(rand.Intn(10)) * time.Second)
		if rand.Float32() < 0.05 {
			nodes[i].Status = "NotReady"
		} else {
			nodes[i].Status = "Ready"
		}
	}
}
