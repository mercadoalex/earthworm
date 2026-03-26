package main

import (
	"log"
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
	TopologyWindowS    int
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
		TopologyWindowS:    300,
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
	if v := os.Getenv("EARTHWORM_TOPOLOGY_WINDOW_S"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			if t >= 10 && t <= 86400 {
				cfg.TopologyWindowS = t
			} else {
				log.Printf("EARTHWORM_TOPOLOGY_WINDOW_S=%d out of range [10,86400], using default %d", t, cfg.TopologyWindowS)
			}
		}
	}

	return cfg
}
