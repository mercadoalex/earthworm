// lease_monitor.go
// This file provides functions to list pods and nodes, extract container IDs and cgroup paths,
// and correlate eBPF process info (cgroup path or container ID) with Kubernetes pods or nodes.
package kubernetes

import (
	"context"
	"fmt"
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// PodInfo holds metadata for mapping eBPF events to Kubernetes pods
type PodInfo struct {
	PodName      string
	Namespace    string
	NodeName     string
	ContainerIDs []string
	CgroupPaths  []string
}

// GetKubeClient initializes a Kubernetes client from kubeconfig
func GetKubeClient(kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}
	return clientset, nil
}

// ListNodes returns a list of node names in the cluster
func ListNodes(clientset *kubernetes.Clientset) ([]string, error) {
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	var nodeNames []string
	for _, node := range nodes.Items {
		nodeNames = append(nodeNames, node.Name)
	}
	return nodeNames, nil
}

// ListPodsWithContainerInfo returns PodInfo for all pods in the cluster
func ListPodsWithContainerInfo(clientset *kubernetes.Clientset) ([]PodInfo, error) {
	pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	var podInfos []PodInfo
	for _, pod := range pods.Items {
		var containerIDs []string
		var cgroupPaths []string
		// Extract container IDs from pod status
		for _, status := range pod.Status.ContainerStatuses {
			// ContainerID format: "docker://<id>" or "containerd://<id>"
			parts := strings.Split(status.ContainerID, "://")
			if len(parts) == 2 {
				containerIDs = append(containerIDs, parts[1])
			}
		}
		// Cgroup paths are not directly available via API, but can be constructed or fetched from the node
		// For demo, we use a placeholder
		for _, cid := range containerIDs {
			cgroupPaths = append(cgroupPaths, fmt.Sprintf("/sys/fs/cgroup/kubepods/%s", cid))
		}
		podInfos = append(podInfos, PodInfo{
			PodName:      pod.Name,
			Namespace:    pod.Namespace,
			NodeName:     pod.Spec.NodeName,
			ContainerIDs: containerIDs,
			CgroupPaths:  cgroupPaths,
		})
	}
	return podInfos, nil
}

// CorrelateEBPFEvent matches eBPF cgroup path or container ID to a pod
func CorrelateEBPFEvent(podInfos []PodInfo, cgroupPath string) *PodInfo {
	for _, pod := range podInfos {
		for _, cg := range pod.CgroupPaths {
			if cg == cgroupPath {
				return &pod
			}
		}
	}
	return nil
}

// Example usage (for demonstration only, not for production)
func ExampleCorrelate() {
	kubeconfig := "/path/to/kubeconfig"
	clientset, err := GetKubeClient(kubeconfig)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	podInfos, err := ListPodsWithContainerInfo(clientset)
	if err != nil {
		log.Fatalf("Error listing pods: %v", err)
	}

	// Simulate an eBPF event with a cgroup path
	ebpfCgroupPath := "/sys/fs/cgroup/kubepods/123abc"
	pod := CorrelateEBPFEvent(podInfos, ebpfCgroupPath)
	if pod != nil {
		fmt.Printf("eBPF event matches pod: %s in namespace: %s on node: %s\n", pod.PodName, pod.Namespace, pod.NodeName)
	} else {
		fmt.Println("No matching pod found for eBPF event")
	}
}
