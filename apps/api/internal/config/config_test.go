package config

import (
	"testing"
)

func TestLoad_PortResolution(t *testing.T) {
	tests := []struct {
		name     string
		apiPort  string
		port     string
		expected string
	}{
		{
			name:     "API_PORT set",
			apiPort:  "9090",
			port:     "",
			expected: "9090",
		},
		{
			name:     "PORT set, API_PORT unset",
			apiPort:  "",
			port:     "4000",
			expected: "4000",
		},
		{
			name:     "both unset → 8080",
			apiPort:  "",
			port:     "",
			expected: "8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("API_PORT", tt.apiPort)
			t.Setenv("PORT", tt.port)
			cfg := Load()
			if cfg.APIPort != tt.expected {
				t.Errorf("APIPort = %q, want %q", cfg.APIPort, tt.expected)
			}
		})
	}
}

func TestLoad_PortPrecedence(t *testing.T) {
	t.Setenv("API_PORT", "9090")
	t.Setenv("PORT", "4000")
	cfg := Load()
	if cfg.APIPort != "9090" {
		t.Errorf("APIPort = %q, want %q (API_PORT takes precedence)", cfg.APIPort, "9090")
	}
}
