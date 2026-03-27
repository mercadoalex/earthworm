package main

import (
	"os"
	"strconv"
	"testing"
)

// TestLoadConfig_Defaults verifies that LoadConfig returns correct defaults
// when no environment variables are set.
func TestLoadConfig_Defaults(t *testing.T) {
	// Clear all relevant env vars to ensure defaults
	envVars := []string{
		"EARTHWORM_PORT",
		"EARTHWORM_LOG_FILE",
		"EARTHWORM_CORS_ORIGINS",
		"EARTHWORM_STORE",
		"EARTHWORM_REDIS_ADDR",
		"EARTHWORM_WARNING_THRESHOLD",
		"EARTHWORM_CRITICAL_THRESHOLD",
		"EARTHWORM_WEBHOOK_URL",
	}
	for _, e := range envVars {
		os.Unsetenv(e)
	}

	cfg := LoadConfig()

	if cfg.Port != 8080 {
		t.Errorf("Port default: got %d, want 8080", cfg.Port)
	}
	if cfg.LogFilePath != "earthworm.log" {
		t.Errorf("LogFilePath default: got %q, want %q", cfg.LogFilePath, "earthworm.log")
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "*" {
		t.Errorf("CORSOrigins default: got %v, want [*]", cfg.CORSOrigins)
	}
	if cfg.StoreType != "memory" {
		t.Errorf("StoreType default: got %q, want %q", cfg.StoreType, "memory")
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr default: got %q, want %q", cfg.RedisAddr, "localhost:6379")
	}
	if cfg.WarningThresholdS != 10 {
		t.Errorf("WarningThresholdS default: got %d, want 10", cfg.WarningThresholdS)
	}
	if cfg.CriticalThresholdS != 40 {
		t.Errorf("CriticalThresholdS default: got %d, want 40", cfg.CriticalThresholdS)
	}
	if cfg.WebhookURL != "" {
		t.Errorf("WebhookURL default: got %q, want empty", cfg.WebhookURL)
	}
}

// TestLoadConfig_AllEnvVars verifies that LoadConfig reads every parameter
// from its corresponding environment variable.
func TestLoadConfig_AllEnvVars(t *testing.T) {
	os.Setenv("EARTHWORM_PORT", "9090")
	os.Setenv("EARTHWORM_LOG_FILE", "/var/log/test.log")
	os.Setenv("EARTHWORM_CORS_ORIGINS", "http://a.com,http://b.com")
	os.Setenv("EARTHWORM_STORE", "redis")
	os.Setenv("EARTHWORM_REDIS_ADDR", "redis.local:6380")
	os.Setenv("EARTHWORM_WARNING_THRESHOLD", "15")
	os.Setenv("EARTHWORM_CRITICAL_THRESHOLD", "60")
	os.Setenv("EARTHWORM_WEBHOOK_URL", "https://hooks.example.com/alert")
	defer func() {
		os.Unsetenv("EARTHWORM_PORT")
		os.Unsetenv("EARTHWORM_LOG_FILE")
		os.Unsetenv("EARTHWORM_CORS_ORIGINS")
		os.Unsetenv("EARTHWORM_STORE")
		os.Unsetenv("EARTHWORM_REDIS_ADDR")
		os.Unsetenv("EARTHWORM_WARNING_THRESHOLD")
		os.Unsetenv("EARTHWORM_CRITICAL_THRESHOLD")
		os.Unsetenv("EARTHWORM_WEBHOOK_URL")
	}()

	cfg := LoadConfig()

	if cfg.Port != 9090 {
		t.Errorf("Port: got %d, want 9090", cfg.Port)
	}
	if cfg.LogFilePath != "/var/log/test.log" {
		t.Errorf("LogFilePath: got %q, want %q", cfg.LogFilePath, "/var/log/test.log")
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[0] != "http://a.com" || cfg.CORSOrigins[1] != "http://b.com" {
		t.Errorf("CORSOrigins: got %v, want [http://a.com http://b.com]", cfg.CORSOrigins)
	}
	if cfg.StoreType != "redis" {
		t.Errorf("StoreType: got %q, want %q", cfg.StoreType, "redis")
	}
	if cfg.RedisAddr != "redis.local:6380" {
		t.Errorf("RedisAddr: got %q, want %q", cfg.RedisAddr, "redis.local:6380")
	}
	if cfg.WarningThresholdS != 15 {
		t.Errorf("WarningThresholdS: got %d, want 15", cfg.WarningThresholdS)
	}
	if cfg.CriticalThresholdS != 60 {
		t.Errorf("CriticalThresholdS: got %d, want 60", cfg.CriticalThresholdS)
	}
	if cfg.WebhookURL != "https://hooks.example.com/alert" {
		t.Errorf("WebhookURL: got %q, want %q", cfg.WebhookURL, "https://hooks.example.com/alert")
	}
}

// TestLoadConfig_InvalidPortFallsBackToDefault verifies that a non-numeric
// EARTHWORM_PORT value is ignored and the default is used.
func TestLoadConfig_InvalidPortFallsBackToDefault(t *testing.T) {
	os.Setenv("EARTHWORM_PORT", "not-a-number")
	defer os.Unsetenv("EARTHWORM_PORT")

	cfg := LoadConfig()
	if cfg.Port != 8080 {
		t.Errorf("Port with invalid env: got %d, want 8080", cfg.Port)
	}
}

// TestLoadConfig_InvalidThresholdsFallBackToDefaults verifies that non-numeric
// threshold values are ignored and defaults are used.
func TestLoadConfig_InvalidThresholdsFallBackToDefaults(t *testing.T) {
	os.Setenv("EARTHWORM_WARNING_THRESHOLD", "abc")
	os.Setenv("EARTHWORM_CRITICAL_THRESHOLD", "xyz")
	defer func() {
		os.Unsetenv("EARTHWORM_WARNING_THRESHOLD")
		os.Unsetenv("EARTHWORM_CRITICAL_THRESHOLD")
	}()

	cfg := LoadConfig()
	if cfg.WarningThresholdS != 10 {
		t.Errorf("WarningThresholdS with invalid env: got %d, want 10", cfg.WarningThresholdS)
	}
	if cfg.CriticalThresholdS != 40 {
		t.Errorf("CriticalThresholdS with invalid env: got %d, want 40", cfg.CriticalThresholdS)
	}
}

// TestLoadConfig_PartialEnvVars verifies that setting only some env vars
// overrides those fields while others keep defaults.
func TestLoadConfig_PartialEnvVars(t *testing.T) {
	// Clear all first
	for _, e := range []string{
		"EARTHWORM_PORT", "EARTHWORM_LOG_FILE", "EARTHWORM_CORS_ORIGINS",
		"EARTHWORM_STORE", "EARTHWORM_REDIS_ADDR",
		"EARTHWORM_WARNING_THRESHOLD", "EARTHWORM_CRITICAL_THRESHOLD",
		"EARTHWORM_WEBHOOK_URL",
	} {
		os.Unsetenv(e)
	}

	// Set only port and webhook
	os.Setenv("EARTHWORM_PORT", strconv.Itoa(3000))
	os.Setenv("EARTHWORM_WEBHOOK_URL", "https://example.com/hook")
	defer func() {
		os.Unsetenv("EARTHWORM_PORT")
		os.Unsetenv("EARTHWORM_WEBHOOK_URL")
	}()

	cfg := LoadConfig()

	if cfg.Port != 3000 {
		t.Errorf("Port: got %d, want 3000", cfg.Port)
	}
	if cfg.WebhookURL != "https://example.com/hook" {
		t.Errorf("WebhookURL: got %q, want %q", cfg.WebhookURL, "https://example.com/hook")
	}
	// Remaining fields should be defaults
	if cfg.LogFilePath != "earthworm.log" {
		t.Errorf("LogFilePath should be default: got %q", cfg.LogFilePath)
	}
	if cfg.StoreType != "memory" {
		t.Errorf("StoreType should be default: got %q", cfg.StoreType)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr should be default: got %q", cfg.RedisAddr)
	}
	if cfg.WarningThresholdS != 10 {
		t.Errorf("WarningThresholdS should be default: got %d", cfg.WarningThresholdS)
	}
	if cfg.CriticalThresholdS != 40 {
		t.Errorf("CriticalThresholdS should be default: got %d", cfg.CriticalThresholdS)
	}
}
