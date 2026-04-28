package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFileCreatesAndReplacesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.txt")

	if err := AtomicWriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatalf("AtomicWriteFile() error = %v", err)
	}
	if err := AtomicWriteFile(path, []byte("second"), 0600); err != nil {
		t.Fatalf("AtomicWriteFile() replace error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "second" {
		t.Fatalf("content = %q, want second", string(data))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("mode = %v, want 0600", got)
	}
}
