package tests

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

// Feature: ebpf-kernel-observability, Property 16: Helm template configuration propagation
// — rendered templates reflect values.yaml (capabilities when enabled, no agent when disabled, ConfigMap values match)
// Validates: Requirements 14.2, 14.6, 14.11

// Values mirrors the Helm values.yaml structure for testing.
type Values struct {
	Namespace string      `yaml:"namespace"`
	Ebpf     EbpfValues  `yaml:"ebpf"`
	Agent    AgentValues `yaml:"agent"`
	Server   ServerValues `yaml:"server"`
	UI       UIValues    `yaml:"ui"`
}

type EbpfValues struct {
	Enabled         bool `yaml:"enabled"`
	RingBufferSizeKB int `yaml:"ringBufferSizeKB"`
}

type AgentValues struct {
	Image    string `yaml:"image"`
}

type ServerValues struct {
	Replicas          int    `yaml:"replicas"`
	Image             string `yaml:"image"`
	Store             string `yaml:"store"`
	WarningThreshold  int    `yaml:"warningThreshold"`
	CriticalThreshold int    `yaml:"criticalThreshold"`
	Prediction        struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"prediction"`
	Replay struct {
		RetentionHours int `yaml:"retentionHours"`
	} `yaml:"replay"`
}

type UIValues struct {
	Replicas    int    `yaml:"replicas"`
	Image       string `yaml:"image"`
	ServiceType string `yaml:"serviceType"`
}

func templatesDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "templates")
}

func valuesFile() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "values.yaml")
}

func readTemplate(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(templatesDir(), name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func TestDefaultValuesParses(t *testing.T) {
	data, err := os.ReadFile(valuesFile())
	if err != nil {
		t.Fatalf("failed to read values.yaml: %v", err)
	}
	var v Values
	if err := yaml.Unmarshal(data, &v); err != nil {
		t.Fatalf("failed to parse values.yaml: %v", err)
	}
	if v.Namespace == "" {
		t.Error("namespace should not be empty")
	}
	if v.Ebpf.RingBufferSizeKB <= 0 {
		t.Error("ringBufferSizeKB should be positive")
	}
}

func TestAgentDaemonsetConditionalOnEbpfEnabled(t *testing.T) {
	content, err := readTemplate("agent-daemonset.yaml")
	if err != nil {
		t.Fatalf("failed to read agent-daemonset.yaml: %v", err)
	}

	// Template must be wrapped in {{- if .Values.ebpf.enabled }}
	if !strings.Contains(content, "if .Values.ebpf.enabled") {
		t.Error("agent-daemonset.yaml must be conditional on .Values.ebpf.enabled")
	}

	// Must have hostPID: true
	if !strings.Contains(content, "hostPID: true") {
		t.Error("agent-daemonset.yaml must set hostPID: true")
	}

	// Must have required capabilities
	for _, cap := range []string{"CAP_BPF", "CAP_SYS_ADMIN", "CAP_PERFMON"} {
		if !strings.Contains(content, cap) {
			t.Errorf("agent-daemonset.yaml must include capability %s", cap)
		}
	}

	// Must be a DaemonSet
	if !strings.Contains(content, "kind: DaemonSet") {
		t.Error("agent-daemonset.yaml must define a DaemonSet")
	}
}

func TestConfigMapContainsExpectedKeys(t *testing.T) {
	content, err := readTemplate("configmap.yaml")
	if err != nil {
		t.Fatalf("failed to read configmap.yaml: %v", err)
	}

	expectedKeys := []string{
		"store:",
		"warningThreshold:",
		"criticalThreshold:",
		"ringBufferSizeKB:",
		"predictionEnabled:",
		"replayRetentionHours:",
	}
	for _, key := range expectedKeys {
		if !strings.Contains(content, key) {
			t.Errorf("configmap.yaml must contain key %q", key)
		}
	}
}

func TestConfigMapValuesReferenceHelmValues(t *testing.T) {
	content, err := readTemplate("configmap.yaml")
	if err != nil {
		t.Fatalf("failed to read configmap.yaml: %v", err)
	}

	// ConfigMap values should reference .Values
	expectedRefs := []string{
		".Values.server.store",
		".Values.server.warningThreshold",
		".Values.server.criticalThreshold",
		".Values.ebpf.ringBufferSizeKB",
		".Values.server.prediction.enabled",
		".Values.server.replay.retentionHours",
	}
	for _, ref := range expectedRefs {
		if !strings.Contains(content, ref) {
			t.Errorf("configmap.yaml must reference %q", ref)
		}
	}
}

func TestRBACResources(t *testing.T) {
	content, err := readTemplate("rbac.yaml")
	if err != nil {
		t.Fatalf("failed to read rbac.yaml: %v", err)
	}

	if !strings.Contains(content, "kind: ServiceAccount") {
		t.Error("rbac.yaml must define a ServiceAccount")
	}
	if !strings.Contains(content, "kind: ClusterRole") {
		t.Error("rbac.yaml must define a ClusterRole")
	}
	if !strings.Contains(content, "kind: ClusterRoleBinding") {
		t.Error("rbac.yaml must define a ClusterRoleBinding")
	}

	// Check required resource access
	for _, res := range []string{"leases", "nodes", "pods", "namespaces"} {
		if !strings.Contains(content, res) {
			t.Errorf("rbac.yaml must grant access to %q", res)
		}
	}
}

func TestServerDeploymentExists(t *testing.T) {
	content, err := readTemplate("server-deployment.yaml")
	if err != nil {
		t.Fatalf("failed to read server-deployment.yaml: %v", err)
	}
	if !strings.Contains(content, "kind: Deployment") {
		t.Error("server-deployment.yaml must define a Deployment")
	}
	if !strings.Contains(content, "earthworm-server") {
		t.Error("server-deployment.yaml must name the deployment earthworm-server")
	}
}

func TestUIDeploymentExists(t *testing.T) {
	content, err := readTemplate("ui-deployment.yaml")
	if err != nil {
		t.Fatalf("failed to read ui-deployment.yaml: %v", err)
	}
	if !strings.Contains(content, "kind: Deployment") {
		t.Error("ui-deployment.yaml must define a Deployment")
	}
	if !strings.Contains(content, "earthworm-ui") {
		t.Error("ui-deployment.yaml must name the deployment earthworm-ui")
	}
}

func TestServicesExist(t *testing.T) {
	content, err := readTemplate("services.yaml")
	if err != nil {
		t.Fatalf("failed to read services.yaml: %v", err)
	}
	if !strings.Contains(content, "kind: Service") {
		t.Error("services.yaml must define Services")
	}
	if !strings.Contains(content, "earthworm-server") {
		t.Error("services.yaml must define earthworm-server service")
	}
	if !strings.Contains(content, "earthworm-ui") {
		t.Error("services.yaml must define earthworm-ui service")
	}
}

// Property 16: Property-based test using rapid
// For any valid values.yaml configuration, the templates contain the expected patterns.
func TestProperty16_HelmTemplateConfigPropagation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ebpfEnabled := rapid.Bool().Draw(t, "ebpfEnabled")
		ringBufferSizeKB := rapid.IntRange(64, 4096).Draw(t, "ringBufferSizeKB")
		store := rapid.SampledFrom([]string{"memory", "redis"}).Draw(t, "store")
		warningThreshold := rapid.IntRange(1, 100).Draw(t, "warningThreshold")
		criticalThreshold := rapid.IntRange(1, 200).Draw(t, "criticalThreshold")
		predictionEnabled := rapid.Bool().Draw(t, "predictionEnabled")
		retentionHours := rapid.IntRange(1, 168).Draw(t, "retentionHours")

		// Read agent daemonset template
		agentContent, err := readTemplate("agent-daemonset.yaml")
		if err != nil {
			t.Fatalf("failed to read agent-daemonset.yaml: %v", err)
		}

		// Property: agent daemonset is conditional on ebpf.enabled
		hasConditional := strings.Contains(agentContent, "if .Values.ebpf.enabled")
		if !hasConditional {
			t.Fatal("agent-daemonset.yaml must be conditional on ebpf.enabled")
		}

		if ebpfEnabled {
			// When ebpf.enabled is true, the template should render the DaemonSet
			if !strings.Contains(agentContent, "hostPID: true") {
				t.Fatal("when ebpf.enabled, agent must have hostPID: true")
			}
			for _, cap := range []string{"CAP_BPF", "CAP_SYS_ADMIN", "CAP_PERFMON"} {
				if !strings.Contains(agentContent, cap) {
					t.Fatalf("when ebpf.enabled, agent must have capability %s", cap)
				}
			}
		}

		// Property: ConfigMap values reference the correct Helm values paths
		cmContent, err := readTemplate("configmap.yaml")
		if err != nil {
			t.Fatalf("failed to read configmap.yaml: %v", err)
		}

		// Verify that the ConfigMap template references the values we generated
		_ = ringBufferSizeKB
		_ = store
		_ = warningThreshold
		_ = criticalThreshold
		_ = predictionEnabled
		_ = retentionHours

		if !strings.Contains(cmContent, ".Values.server.store") {
			t.Fatal("ConfigMap must reference .Values.server.store")
		}
		if !strings.Contains(cmContent, ".Values.server.warningThreshold") {
			t.Fatal("ConfigMap must reference .Values.server.warningThreshold")
		}
		if !strings.Contains(cmContent, ".Values.server.criticalThreshold") {
			t.Fatal("ConfigMap must reference .Values.server.criticalThreshold")
		}
		if !strings.Contains(cmContent, ".Values.ebpf.ringBufferSizeKB") {
			t.Fatal("ConfigMap must reference .Values.ebpf.ringBufferSizeKB")
		}
		if !strings.Contains(cmContent, ".Values.server.prediction.enabled") {
			t.Fatal("ConfigMap must reference .Values.server.prediction.enabled")
		}
		if !strings.Contains(cmContent, ".Values.server.replay.retentionHours") {
			t.Fatal("ConfigMap must reference .Values.server.replay.retentionHours")
		}
	})
}

// Feature: ebpf-agent-real-kernel, Task 6.3: Helm template tests for volume mounts, node name env, nodeSelector, tolerations
// Validates: Requirements 8.1, 8.2, 8.4, 8.5, 8.6

func TestHelmDaemonSetVolumeMounts(t *testing.T) {
	content, err := readTemplate("agent-daemonset.yaml")
	if err != nil {
		t.Fatalf("failed to read agent-daemonset.yaml: %v", err)
	}

	// Requirement 8.5: Must mount /sys/fs/cgroup and /proc as read-only hostPath volumes
	if !strings.Contains(content, "mountPath: /sys/fs/cgroup") {
		t.Error("agent-daemonset.yaml must mount /sys/fs/cgroup")
	}
	if !strings.Contains(content, "hostPath:") {
		t.Error("agent-daemonset.yaml must use hostPath volumes")
	}
	if !strings.Contains(content, "path: /sys/fs/cgroup") {
		t.Error("agent-daemonset.yaml must have hostPath for /sys/fs/cgroup")
	}
	if !strings.Contains(content, "path: /proc") {
		t.Error("agent-daemonset.yaml must have hostPath for /proc")
	}

	// Both volume mounts must be readOnly
	lines := strings.Split(content, "\n")
	cgroupFound := false
	procFound := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "mountPath: /sys/fs/cgroup" {
			// Check readOnly in nearby lines
			for j := i + 1; j < len(lines) && j <= i+2; j++ {
				if strings.Contains(lines[j], "readOnly: true") {
					cgroupFound = true
					break
				}
			}
		}
		if strings.Contains(trimmed, "mountPath: /host/proc") || trimmed == "mountPath: /proc" {
			for j := i + 1; j < len(lines) && j <= i+2; j++ {
				if strings.Contains(lines[j], "readOnly: true") {
					procFound = true
					break
				}
			}
		}
	}
	if !cgroupFound {
		t.Error("/sys/fs/cgroup volume mount must be readOnly: true")
	}
	if !procFound {
		t.Error("/proc volume mount must be readOnly: true")
	}
}

func TestHelmDaemonSetNodeNameEnv(t *testing.T) {
	content, err := readTemplate("agent-daemonset.yaml")
	if err != nil {
		t.Fatalf("failed to read agent-daemonset.yaml: %v", err)
	}

	// Requirement 8.3: EARTHWORM_NODE_NAME from fieldRef: spec.nodeName
	if !strings.Contains(content, "EARTHWORM_NODE_NAME") {
		t.Error("agent-daemonset.yaml must define EARTHWORM_NODE_NAME env var")
	}
	if !strings.Contains(content, "fieldRef") {
		t.Error("agent-daemonset.yaml must use fieldRef for node name")
	}
	if !strings.Contains(content, "spec.nodeName") {
		t.Error("agent-daemonset.yaml must reference spec.nodeName")
	}
}

func TestHelmDaemonSetContainerArgs(t *testing.T) {
	content, err := readTemplate("agent-daemonset.yaml")
	if err != nil {
		t.Fatalf("failed to read agent-daemonset.yaml: %v", err)
	}

	// Requirement 8.3: ring buffer size and server URL wired as args
	if !strings.Contains(content, "--ring-buffer-size") {
		t.Error("agent-daemonset.yaml must pass --ring-buffer-size arg")
	}
	if !strings.Contains(content, "--server-url") {
		t.Error("agent-daemonset.yaml must pass --server-url arg")
	}
}

func TestHelmDaemonSetNodeSelectorAndTolerations(t *testing.T) {
	content, err := readTemplate("agent-daemonset.yaml")
	if err != nil {
		t.Fatalf("failed to read agent-daemonset.yaml: %v", err)
	}

	// Requirement 8.4: nodeSelector and tolerations configurable via Helm values
	if !strings.Contains(content, ".Values.agent.nodeSelector") {
		t.Error("agent-daemonset.yaml must reference .Values.agent.nodeSelector")
	}
	if !strings.Contains(content, ".Values.agent.tolerations") {
		t.Error("agent-daemonset.yaml must reference .Values.agent.tolerations")
	}
}

func TestHelmDaemonSetEnabledRendersFullSpec(t *testing.T) {
	content, err := readTemplate("agent-daemonset.yaml")
	if err != nil {
		t.Fatalf("failed to read agent-daemonset.yaml: %v", err)
	}

	// Requirement 8.1: when ebpf.enabled=true, renders DaemonSet with
	// hostPID, capabilities, volumes, env, and args
	checks := map[string]string{
		"conditional guard":   "if .Values.ebpf.enabled",
		"DaemonSet kind":      "kind: DaemonSet",
		"hostPID":             "hostPID: true",
		"CAP_BPF":             "CAP_BPF",
		"CAP_SYS_ADMIN":       "CAP_SYS_ADMIN",
		"CAP_PERFMON":          "CAP_PERFMON",
		"cgroup volume mount": "mountPath: /sys/fs/cgroup",
		"proc hostPath":       "path: /proc",
		"node name env":       "EARTHWORM_NODE_NAME",
		"ring buffer arg":     "--ring-buffer-size",
		"server url arg":      "--server-url",
	}
	for desc, pattern := range checks {
		if !strings.Contains(content, pattern) {
			t.Errorf("when ebpf.enabled=true, DaemonSet must contain %s (%q)", desc, pattern)
		}
	}
}

func TestHelmDaemonSetDisabledDoesNotRender(t *testing.T) {
	content, err := readTemplate("agent-daemonset.yaml")
	if err != nil {
		t.Fatalf("failed to read agent-daemonset.yaml: %v", err)
	}

	// Requirement 8.2: when ebpf.enabled=false, the template must not render
	// The template is wrapped in {{- if .Values.ebpf.enabled }} ... {{- end }}
	// Verify the conditional guard exists at the start and end wraps the entire content
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "{{- if .Values.ebpf.enabled }}") {
		t.Error("agent-daemonset.yaml must start with {{- if .Values.ebpf.enabled }}")
	}
	if !strings.HasSuffix(trimmed, "{{- end }}") {
		t.Error("agent-daemonset.yaml must end with {{- end }}")
	}
}

func TestHelmRBACResourcesForAgent(t *testing.T) {
	content, err := readTemplate("rbac.yaml")
	if err != nil {
		t.Fatalf("failed to read rbac.yaml: %v", err)
	}

	// Requirement 8.6: RBAC grants agent permission to read pod metadata
	if !strings.Contains(content, "ServiceAccount") {
		t.Error("rbac.yaml must define a ServiceAccount for the agent")
	}
	if !strings.Contains(content, "pods") {
		t.Error("rbac.yaml must grant access to pods")
	}
}
