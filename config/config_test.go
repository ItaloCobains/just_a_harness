package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HARNESS_MODEL", "")
	t.Setenv("HARNESS_ENDPOINT", "")
	c := Load()
	if c.OllamaModel != defaultModel {
		t.Errorf("OllamaModel = %q, want default %q", c.OllamaModel, defaultModel)
	}
	if c.OllamaEndpoint != defaultEndpoint {
		t.Errorf("OllamaEndpoint = %q, want default %q", c.OllamaEndpoint, defaultEndpoint)
	}
}

func TestLoadOverride(t *testing.T) {
	t.Setenv("HARNESS_MODEL", "llama3")
	t.Setenv("HARNESS_ENDPOINT", "http://remote:1234")
	c := Load()
	if c.OllamaModel != "llama3" {
		t.Errorf("OllamaModel = %q, want %q", c.OllamaModel, "llama3")
	}
	if c.OllamaEndpoint != "http://remote:1234" {
		t.Errorf("OllamaEndpoint = %q, want %q", c.OllamaEndpoint, "http://remote:1234")
	}
}
