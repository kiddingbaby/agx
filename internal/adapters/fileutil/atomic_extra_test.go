package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFileCreatesNestedPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.txt")
	if err := AtomicWriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("content = %q, want hello", string(data))
	}
}

func TestAtomicWriteFileOverwritesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := AtomicWriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("content = %q, want new", string(data))
	}
}

func TestAtomicWriteFileFailsWhenParentPathIsAFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := AtomicWriteFile(filepath.Join(blocker, "config.txt"), []byte("data"), 0o600); err == nil {
		t.Fatal("AtomicWriteFile() unexpectedly succeeded with file parent")
	}
}

func TestAtomicWriteFileAdditionalErrorBranches(t *testing.T) {
	dir := t.TempDir()

	if err := AtomicWriteFile(filepath.Join(dir, "no-such", "config.txt"), []byte("data"), os.FileMode(0)); err != nil {
		t.Fatalf("AtomicWriteFile(zero mode) error = %v", err)
	}

	missingDir := filepath.Join(dir, "missing-dir")
	if err := syncDir(missingDir); err == nil {
		t.Fatal("syncDir(missing dir) unexpectedly succeeded")
	}
}
