// monitor.go

package kubernetes

import (
    "context"
    "fmt"
    "log"
    "time"

    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/tools/remotecommand"
    "k8s.io/apimachinery/pkg/watch"
    "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HeartbeatMonitor is a struct that holds the Kubernetes client and the channel for heartbeat data.
type HeartbeatMonitor struct {
    clientset *kubernetes.Clientset
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
        clientset: clientset,
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

// GetHeartbeatChannel returns the channel for receiving heartbeat data.
func (hm *HeartbeatMonitor) GetHeartbeatChannel() chan string {
    return hm.heartbeatChannel
}