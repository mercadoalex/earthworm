package main

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all configurable parameters for the Earthworm server.
type Config struct {
	Port               int
	LogFilePath        string
	CORSOrigins        []string
	StoreType          string
	RedisAddr          string
	WarningThresholdS  int
	CriticalThresholdS int
	WebhookURL         string
}

// LoadConfig reads configuration from environment variables with sensible defaults.
func LoadConfig() Config {
	cfg := Config{
		Port:               8080,
		LogFilePath:        "earthworm.log",
		CORSOrigins:        []string{"*"},
		StoreType:          "memory",
		RedisAddr:          "localhost:6379",
		WarningThresholdS:  10,
		CriticalThresholdS: 40,
		WebhookURL:         "",
	}

	if v := os.Getenv("EARTHWORM_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("EARTHWORM_LOG_FILE"); v != "" {
		cfg.LogFilePath = v
	}
	if v := os.Getenv("EARTHWORM_CORS_ORIGINS"); v != "" {
		cfg.CORSOrigins = strings.Split(v, ",")
	}
	if v := os.Getenv("EARTHWORM_STORE"); v != "" {
		cfg.StoreType = v
	}
	if v := os.Getenv("EARTHWORM_REDIS_ADDR"); v != "" {
		cfg.RedisAddr = v
	}
	if v := os.Getenv("EARTHWORM_WARNING_THRESHOLD"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			cfg.WarningThresholdS = t
		}
	}
	if v := os.Getenv("EARTHWORM_CRITICAL_THRESHOLD"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			cfg.CriticalThresholdS = t
		}
	}
	if v := os.Getenv("EARTHWORM_WEBHOOK_URL"); v != "" {
		cfg.WebhookURL = v
	}

	return cfg
}
