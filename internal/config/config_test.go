package config

import (
	"testing"
	"time"
)

func TestLoadConfigReloadInterval(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
		want  time.Duration
	}{
		{name: "unset uses default", set: false, want: 60 * time.Second},
		{name: "valid value", value: "30s", set: true, want: 30 * time.Second},
		{name: "zero falls back to default", value: "0s", set: true, want: 60 * time.Second},
		{name: "negative falls back to default", value: "-5s", set: true, want: 60 * time.Second},
		{name: "unparsable falls back to default", value: "soon", set: true, want: 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://test")
			t.Setenv("CONFIG_RELOAD_INTERVAL", "")
			if tt.set {
				t.Setenv("CONFIG_RELOAD_INTERVAL", tt.value)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.ConfigReloadInterval != tt.want {
				t.Errorf("ConfigReloadInterval = %v, want %v", cfg.ConfigReloadInterval, tt.want)
			}
		})
	}
}
