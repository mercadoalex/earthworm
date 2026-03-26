package main

import (
	"log"
	"os"
	"strconv"
)

// ProbeConfig holds configuration for the advanced eBPF probes.
type ProbeConfig struct {
	SlowIOThresholdMs    int // EARTHWORM_SLOW_IO_THRESHOLD_MS, default 100, range 1-60000
	DNSTimeoutMs         int // EARTHWORM_DNS_TIMEOUT_MS, default 5000, range 100-60000
	CgroupSampleIntervalS int // EARTHWORM_CGROUP_SAMPLE_INTERVAL_S, default 10, range 1-3600
	TopologyWindowS      int // EARTHWORM_TOPOLOGY_WINDOW_S, default 300, range 10-86400
	MemoryPressurePct    int // EARTHWORM_MEMORY_PRESSURE_PCT, default 90, range 1-100
}

// DefaultProbeConfig returns a ProbeConfig with all default values.
func DefaultProbeConfig() ProbeConfig {
	return ProbeConfig{
		SlowIOThresholdMs:    100,
		DNSTimeoutMs:         5000,
		CgroupSampleIntervalS: 10,
		TopologyWindowS:      300,
		MemoryPressurePct:    90,
	}
}

// LoadProbeConfig reads probe configuration from environment variables.
// Out-of-range values log a warning and fall back to defaults.
func LoadProbeConfig() ProbeConfig {
	cfg := DefaultProbeConfig()

	cfg.SlowIOThresholdMs = parseEnvInt("EARTHWORM_SLOW_IO_THRESHOLD_MS", cfg.SlowIOThresholdMs, 1, 60000)
	cfg.DNSTimeoutMs = parseEnvInt("EARTHWORM_DNS_TIMEOUT_MS", cfg.DNSTimeoutMs, 100, 60000)
	cfg.CgroupSampleIntervalS = parseEnvInt("EARTHWORM_CGROUP_SAMPLE_INTERVAL_S", cfg.CgroupSampleIntervalS, 1, 3600)
	cfg.TopologyWindowS = parseEnvInt("EARTHWORM_TOPOLOGY_WINDOW_S", cfg.TopologyWindowS, 10, 86400)
	cfg.MemoryPressurePct = parseEnvInt("EARTHWORM_MEMORY_PRESSURE_PCT", cfg.MemoryPressurePct, 1, 100)

	return cfg
}

// parseEnvInt reads an integer environment variable. If the variable is unset,
// the default is returned silently. If the value is non-numeric or out of range,
// a warning is logged and the default is returned.
func parseEnvInt(envVar string, defaultVal, minVal, maxVal int) int {
	raw := os.Getenv(envVar)
	if raw == "" {
		return defaultVal
	}

	val, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("WARNING: %s=%q is not a valid integer, using default %d", envVar, raw, defaultVal)
		return defaultVal
	}

	if val < minVal || val > maxVal {
		log.Printf("WARNING: %s=%d is out of range [%d, %d], using default %d", envVar, val, minVal, maxVal, defaultVal)
		return defaultVal
	}

	return val
}
