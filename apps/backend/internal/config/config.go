package config

import (
	"os"
	"time"
)

// Config holds service configuration loaded from environment variables.
type Config struct {
	HTTPAddr        string
	ShutdownTimeout time.Duration
}

// FromEnv loads config from environment variables with safe defaults.
func FromEnv() Config {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":3001"
	}

	// Keep it simple for now; can be made configurable later.
	return Config{
		HTTPAddr:        addr,
		ShutdownTimeout: 10 * time.Second,
	}
}
