package main

import (
	"fmt"
	"os"
	"testing"
)

func TestResolveSocketFromASHLET_SOCKET(t *testing.T) {
	t.Setenv("ASHLET_SOCKET", "/custom/ashlet.sock")
	got := resolveSocketPath()
	if got != "/custom/ashlet.sock" {
		t.Errorf("expected /custom/ashlet.sock, got %s", got)
	}
}

func TestResolveSocketFromXDG_RUNTIME_DIR(t *testing.T) {
	t.Setenv("ASHLET_SOCKET", "")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	got := resolveSocketPath()
	if got != "/run/user/1000/ashlet.sock" {
		t.Errorf("expected /run/user/1000/ashlet.sock, got %s", got)
	}
}

func TestResolveSocketFallback(t *testing.T) {
	t.Setenv("ASHLET_SOCKET", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	got := resolveSocketPath()
	expected := fmt.Sprintf("/tmp/ashlet-%d.sock", os.Getuid())
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestSocketPathMatchesShellClient(t *testing.T) {
	tests := []struct {
		name     string
		envSetup func(t *testing.T)
		expected string
	}{
		{
			name: "ASHLET_SOCKET",
			envSetup: func(t *testing.T) {
				t.Setenv("ASHLET_SOCKET", "/custom/ashlet.sock")
			},
			expected: "/custom/ashlet.sock",
		},
		{
			name: "XDG_RUNTIME_DIR",
			envSetup: func(t *testing.T) {
				t.Setenv("ASHLET_SOCKET", "")
				t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
			},
			expected: "/run/user/1000/ashlet.sock",
		},
		{
			name: "fallback",
			envSetup: func(t *testing.T) {
				t.Setenv("ASHLET_SOCKET", "")
				t.Setenv("XDG_RUNTIME_DIR", "")
			},
			expected: fmt.Sprintf("/tmp/ashlet-%d.sock", os.Getuid()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.envSetup(t)
			got := resolveSocketPath()
			if got != tt.expected {
				t.Errorf("Go resolveSocketPath() = %s, expected %s (should match shell client)", got, tt.expected)
			}
		})
	}
}
