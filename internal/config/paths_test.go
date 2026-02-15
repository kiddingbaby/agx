package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	paths, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths() error = %v", err)
	}

	wantConfigDir := filepath.Join(home, ".config", "agx")
	if paths.ConfigDir != wantConfigDir {
		t.Fatalf("ConfigDir = %q, want %q", paths.ConfigDir, wantConfigDir)
	}
	if paths.StorePath != filepath.Join(wantConfigDir, "keys.yaml") {
		t.Fatalf("StorePath = %q", paths.StorePath)
	}
	if paths.SecretPath != filepath.Join(wantConfigDir, "secret") {
		t.Fatalf("SecretPath = %q", paths.SecretPath)
	}
}
