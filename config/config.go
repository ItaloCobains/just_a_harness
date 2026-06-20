// Package config resolves harness settings from the environment, falling back to
// sensible defaults so the CLIs work out of the box.
package config

import "os"

const (
	defaultModel    = "qwen2.5-coder:7b"
	defaultEndpoint = "http://localhost:11434"
)

// Config holds the runtime settings shared by the agent CLIs.
type Config struct {
	OllamaModel    string
	OllamaEndpoint string
}

// Load reads HARNESS_MODEL and HARNESS_ENDPOINT, using the built-in defaults when
// either is unset or empty.
func Load() Config {
	return Config{
		OllamaModel:    env("HARNESS_MODEL", defaultModel),
		OllamaEndpoint: env("HARNESS_ENDPOINT", defaultEndpoint),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
