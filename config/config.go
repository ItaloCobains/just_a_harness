// Package config resolves harness settings from the environment, falling back to
// sensible defaults so the CLIs work out of the box.
package config

import (
	"os"
	"strconv"
	"time"
)

const (
	defaultModel      = "qwen2.5-coder:7b"
	defaultEndpoint   = "http://localhost:11434"
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 3
)

// Config holds the runtime settings shared by the agent CLIs.
type Config struct {
	OllamaModel    string
	OllamaEndpoint string
	HTTPTimeout    time.Duration
	HTTPMaxRetries int
}

// Load reads the HARNESS_* environment variables, using the built-in defaults
// when one is unset, empty, or unparseable.
func Load() Config {
	return Config{
		OllamaModel:    env("HARNESS_MODEL", defaultModel),
		OllamaEndpoint: env("HARNESS_ENDPOINT", defaultEndpoint),
		HTTPTimeout:    envDuration("HARNESS_HTTP_TIMEOUT", defaultTimeout),
		HTTPMaxRetries: envInt("HARNESS_HTTP_MAX_RETRIES", defaultMaxRetries),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
