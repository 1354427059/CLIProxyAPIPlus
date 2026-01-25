package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultOrchidsConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Orchids.EndpointWS == "" {
		t.Fatalf("orchids ws endpoint not set")
	}
}
