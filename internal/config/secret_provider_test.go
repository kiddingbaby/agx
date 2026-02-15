package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecretProviderLoad_EnvPriority(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		ConfigDir:  dir,
		StorePath:  filepath.Join(dir, "keys.yaml"),
		SecretPath: filepath.Join(dir, "secret"),
	}
	if err := os.WriteFile(paths.SecretPath, []byte("abcdefghijklmnopqrstuvwxyz1234567890"), 0600); err != nil {
		t.Fatalf("WriteFile(secret) error = %v", err)
	}

	envSecret := "abcdefghijklmnopqrstuvwxyz123456"
	t.Setenv("AGX_SECRET", envSecret)

	got, err := NewSecretProvider(paths).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if string(got) != envSecret {
		t.Fatalf("Load() = %q, want %q", string(got), envSecret)
	}
}

func TestSecretProviderLoad_InvalidEnvLength(t *testing.T) {
	t.Setenv("AGX_SECRET", "short")

	_, err := NewSecretProvider(Paths{}).Load()
	if err == nil {
		t.Fatal("Load() expected error, got nil")
	}
}

func TestSecretProviderLoad_FileFallback(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		ConfigDir:  dir,
		StorePath:  filepath.Join(dir, "keys.yaml"),
		SecretPath: filepath.Join(dir, "secret"),
	}
	t.Setenv("AGX_SECRET", "")

	fileSecret := []byte("abcdefghijklmnopqrstuvwxyz1234567890")
	if err := os.WriteFile(paths.SecretPath, fileSecret, 0600); err != nil {
		t.Fatalf("WriteFile(secret) error = %v", err)
	}

	got, err := NewSecretProvider(paths).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if string(got) != string(fileSecret[:32]) {
		t.Fatalf("Load() = %q, want %q", string(got), string(fileSecret[:32]))
	}
}

func TestSecretProviderLoad_MigrationError(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		ConfigDir:  dir,
		StorePath:  filepath.Join(dir, "keys.yaml"),
		SecretPath: filepath.Join(dir, "secret"),
	}
	t.Setenv("AGX_SECRET", "")

	if err := os.WriteFile(paths.StorePath, []byte("keys: []"), 0600); err != nil {
		t.Fatalf("WriteFile(store) error = %v", err)
	}

	_, err := NewSecretProvider(paths).Load()
	if err == nil {
		t.Fatal("Load() expected error, got nil")
	}
}

func TestSecretProviderLoad_GenerateOnFirstRun(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		ConfigDir:  filepath.Join(dir, "nested"),
		StorePath:  filepath.Join(dir, "nested", "keys.yaml"),
		SecretPath: filepath.Join(dir, "nested", "secret"),
	}
	t.Setenv("AGX_SECRET", "")

	got, err := NewSecretProvider(paths).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("len(secret) = %d, want 32", len(got))
	}

	saved, err := os.ReadFile(paths.SecretPath)
	if err != nil {
		t.Fatalf("ReadFile(secret) error = %v", err)
	}
	if string(saved) != string(got) {
		t.Fatalf("saved secret does not match generated secret")
	}
}
