package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDefaultBundlePath_HomeConfigBundleYml(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".config", "agx", "agx.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(path, []byte("keys: []\n"), 0600); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	got, ok := findDefaultConfigPath(func() (string, error) { return t.TempDir(), nil })
	if !ok {
		t.Fatalf("findDefaultConfigPath ok=false, want true")
	}
	if filepath.Clean(got) != filepath.Clean(path) {
		t.Fatalf("findDefaultConfigPath path=%q want %q", got, path)
	}
}

func TestFindDefaultBundlePath_HomeConfigBundleYaml(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".config", "agx", "agx.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(path, []byte("keys: []\n"), 0600); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	got, ok := findDefaultConfigPath(func() (string, error) { return t.TempDir(), nil })
	if !ok {
		t.Fatalf("findDefaultConfigPath ok=false, want true")
	}
	if filepath.Clean(got) != filepath.Clean(path) {
		t.Fatalf("findDefaultConfigPath path=%q want %q", got, path)
	}
}

func TestFindDefaultBundlePath_IgnoresCWD(t *testing.T) {
	home := t.TempDir()
	wd := t.TempDir()
	t.Setenv("HOME", home)

	homePath := filepath.Join(home, ".config", "agx", "agx.yml")
	if err := os.MkdirAll(filepath.Dir(homePath), 0700); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.WriteFile(homePath, []byte("keys: []\n"), 0600); err != nil {
		t.Fatalf("WriteFile(home) error = %v", err)
	}

	wdPath := filepath.Join(wd, "agx.yml")
	if err := os.WriteFile(wdPath, []byte("keys: []\n"), 0600); err != nil {
		t.Fatalf("WriteFile(wd) error = %v", err)
	}

	got, ok := findDefaultConfigPath(func() (string, error) { return wd, nil })
	if !ok {
		t.Fatalf("findDefaultConfigPath ok=false, want true")
	}
	if filepath.Clean(got) != filepath.Clean(homePath) {
		t.Fatalf("findDefaultConfigPath path=%q want %q", got, homePath)
	}
}

func TestResolveBundlePath_Directory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agx.yml")
	if err := os.WriteFile(path, []byte("keys: []\n"), 0600); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	got, err := resolveConfigPath(dir)
	if err != nil {
		t.Fatalf("resolveConfigPath error = %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(path) {
		t.Fatalf("resolveConfigPath path=%q want %q", got, path)
	}
}

func TestResolveBundlePath_DirectoryMissingBundle(t *testing.T) {
	dir := t.TempDir()
	if _, err := resolveConfigPath(dir); err == nil {
		t.Fatalf("resolveConfigPath error=nil, want error")
	}
}
