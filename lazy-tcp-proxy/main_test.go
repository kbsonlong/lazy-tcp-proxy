package main

import (
	"testing"
	"time"
)

func TestResolveIdleTimeout_Default(t *testing.T) {
	t.Setenv("IDLE_TIMEOUT_SECS", "")
	if got := resolveIdleTimeout(); got != defaultIdleTimeout {
		t.Errorf("got %s, want %s", got, defaultIdleTimeout)
	}
}

func TestResolveIdleTimeout_ValidValue(t *testing.T) {
	t.Setenv("IDLE_TIMEOUT_SECS", "60")
	if got := resolveIdleTimeout(); got != 60*time.Second {
		t.Errorf("got %s, want 60s", got)
	}
}

func TestResolveIdleTimeout_ZeroFallsBackToDefault(t *testing.T) {
	t.Setenv("IDLE_TIMEOUT_SECS", "0")
	if got := resolveIdleTimeout(); got != defaultIdleTimeout {
		t.Errorf("got %s, want default %s", got, defaultIdleTimeout)
	}
}

func TestResolveIdleTimeout_NegativeFallsBackToDefault(t *testing.T) {
	t.Setenv("IDLE_TIMEOUT_SECS", "-5")
	if got := resolveIdleTimeout(); got != defaultIdleTimeout {
		t.Errorf("got %s, want default %s", got, defaultIdleTimeout)
	}
}

func TestResolveIdleTimeout_InvalidFallsBackToDefault(t *testing.T) {
	t.Setenv("IDLE_TIMEOUT_SECS", "notanumber")
	if got := resolveIdleTimeout(); got != defaultIdleTimeout {
		t.Errorf("got %s, want default %s", got, defaultIdleTimeout)
	}
}

func TestResolvePollInterval_Default(t *testing.T) {
	t.Setenv("POLL_INTERVAL_SECS", "")
	if got := resolvePollInterval(); got != defaultPollInterval {
		t.Errorf("got %s, want %s", got, defaultPollInterval)
	}
}

func TestResolvePollInterval_ValidValue(t *testing.T) {
	t.Setenv("POLL_INTERVAL_SECS", "30")
	if got := resolvePollInterval(); got != 30*time.Second {
		t.Errorf("got %s, want 30s", got)
	}
}

func TestResolvePollInterval_InvalidFallsBackToDefault(t *testing.T) {
	t.Setenv("POLL_INTERVAL_SECS", "bad")
	if got := resolvePollInterval(); got != defaultPollInterval {
		t.Errorf("got %s, want default %s", got, defaultPollInterval)
	}
}

func TestResolveStatusPort_Default(t *testing.T) {
	t.Setenv("STATUS_PORT", "")
	if got := resolveStatusPort(); got != defaultStatusPort {
		t.Errorf("got %d, want %d", got, defaultStatusPort)
	}
}

func TestResolveStatusPort_ValidValue(t *testing.T) {
	t.Setenv("STATUS_PORT", "9090")
	if got := resolveStatusPort(); got != 9090 {
		t.Errorf("got %d, want 9090", got)
	}
}

func TestResolveStatusPort_ZeroDisables(t *testing.T) {
	t.Setenv("STATUS_PORT", "0")
	if got := resolveStatusPort(); got != 0 {
		t.Errorf("got %d, want 0 (disabled)", got)
	}
}

func TestResolveStatusPort_InvalidFallsBackToDefault(t *testing.T) {
	t.Setenv("STATUS_PORT", "notaport")
	if got := resolveStatusPort(); got != defaultStatusPort {
		t.Errorf("got %d, want default %d", got, defaultStatusPort)
	}
}

func TestResolveStatusPort_NegativeFallsBackToDefault(t *testing.T) {
	t.Setenv("STATUS_PORT", "-1")
	if got := resolveStatusPort(); got != defaultStatusPort {
		t.Errorf("got %d, want default %d", got, defaultStatusPort)
	}
}
