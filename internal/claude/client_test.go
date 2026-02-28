package claude

import (
	"testing"
	"time"
)

func TestNewClient_DefaultModel(t *testing.T) {
	t.Parallel()
	c, err := NewClient("", 5*time.Minute, "")
	if err != nil {
		t.Skip("claude CLI not available in test environment")
	}
	if c.model != "sonnet" {
		t.Errorf("model = %q, want %q", c.model, "sonnet")
	}
}

func TestNewClient_DefaultTimeout(t *testing.T) {
	t.Parallel()
	c, err := NewClient("", 0, "opus")
	if err != nil {
		t.Skip("claude CLI not available in test environment")
	}
	if c.timeout != 5*time.Minute {
		t.Errorf("timeout = %v, want %v", c.timeout, 5*time.Minute)
	}
}

func TestNewClient_PathNotFound(t *testing.T) {
	t.Parallel()
	_, err := NewClient("nonexistent_binary_xyz_123", time.Minute, "sonnet")
	if err == nil {
		t.Error("expected error for non-existent binary, got nil")
	}
}

func TestWithModel_ClonesCorrectly(t *testing.T) {
	t.Parallel()
	c, err := NewClient("", time.Minute, "sonnet")
	if err != nil {
		t.Skip("claude CLI not available in test environment")
	}

	clone := c.WithModel("opus")
	if clone.model != "opus" {
		t.Errorf("clone.model = %q, want %q", clone.model, "opus")
	}
	if c.model != "sonnet" {
		t.Errorf("original.model = %q, want %q (should be unchanged)", c.model, "sonnet")
	}
}

func TestWithMaxTurns_ClonesCorrectly(t *testing.T) {
	t.Parallel()
	c, err := NewClient("", time.Minute, "sonnet")
	if err != nil {
		t.Skip("claude CLI not available in test environment")
	}

	clone := c.WithMaxTurns(5)
	if clone.maxTurns != 5 {
		t.Errorf("clone.maxTurns = %d, want 5", clone.maxTurns)
	}
	if c.maxTurns != 1 {
		t.Errorf("original.maxTurns = %d, want 1 (should be unchanged)", c.maxTurns)
	}
}

func TestWithEnvVars_ClonesCorrectly(t *testing.T) {
	t.Parallel()
	c, err := NewClient("", time.Minute, "sonnet")
	if err != nil {
		t.Skip("claude CLI not available in test environment")
	}

	envVars := map[string]string{"KEY": "value"}
	clone := c.WithEnvVars(envVars)

	if clone.envVars["KEY"] != "value" {
		t.Errorf("clone.envVars[KEY] = %q, want %q", clone.envVars["KEY"], "value")
	}
	if len(c.envVars) != 0 {
		t.Errorf("original.envVars should be empty, got %v", c.envVars)
	}
}

func TestWithEnvVars_IsolatesFromOriginal(t *testing.T) {
	t.Parallel()
	c, err := NewClient("", time.Minute, "sonnet")
	if err != nil {
		t.Skip("claude CLI not available in test environment")
	}

	envVars := map[string]string{"KEY": "value"}
	clone := c.WithEnvVars(envVars)

	// Mutate clone's env vars — should not affect input map
	clone.envVars["NEW_KEY"] = "new_value"
	if _, ok := envVars["NEW_KEY"]; ok {
		t.Error("mutating clone's envVars should not affect the input map")
	}

	// Mutate input map — should not affect clone
	envVars["ANOTHER"] = "another"
	if _, ok := clone.envVars["ANOTHER"]; ok {
		t.Error("mutating input map should not affect clone's envVars")
	}
}
