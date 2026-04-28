package lockfile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLockSerializesAccess(t *testing.T) {
	lock := New(filepath.Join(t.TempDir(), "agx.lock"))

	unlockA, err := lock.Lock()
	if err != nil {
		t.Fatalf("first Lock() error = %v", err)
	}
	unlocked := false
	defer func() {
		if !unlocked {
			unlockA()
		}
	}()

	ready := make(chan struct{})
	acquired := make(chan struct{})
	go func() {
		close(ready)
		unlockB, err := lock.Lock()
		if err != nil {
			t.Errorf("second Lock() error = %v", err)
			return
		}
		close(acquired)
		unlockB()
	}()

	<-ready
	select {
	case <-acquired:
		t.Fatal("second lock acquired before first unlock")
	case <-time.After(100 * time.Millisecond):
	}

	unlockA()
	unlocked = true

	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("second lock did not acquire after first unlock")
	}
}

func TestLockCreatesParentDirectoryAndUnlocksCleanly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "locks", "agx.lock")
	lock := New(path)

	unlock, err := lock.Lock()
	if err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat(lock file) error = %v", err)
	}
	unlock()

	unlockAgain, err := lock.Lock()
	if err != nil {
		t.Fatalf("second Lock() error = %v", err)
	}
	unlockAgain()
}

func TestLockUnlockIsIdempotent(t *testing.T) {
	lock := New(filepath.Join(t.TempDir(), "agx.lock"))
	unlock, err := lock.Lock()
	if err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	unlock()
	unlock()

	unlockAgain, err := lock.Lock()
	if err != nil {
		t.Fatalf("Lock() after double unlock error = %v", err)
	}
	unlockAgain()
}

func TestNewStoresPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "agx.lock")
	lock := New(path)
	if lock.path != path {
		t.Fatalf("lock.path = %q, want %q", lock.path, path)
	}
}

func TestLockFailsWhenParentPathIsAFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	lock := New(filepath.Join(blocker, "agx.lock"))
	if _, err := lock.Lock(); err == nil {
		t.Fatal("Lock() unexpectedly succeeded with file parent")
	}
}

func TestLockFailsWhenLockPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "agx.lock")
	if err := os.MkdirAll(lockPath, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	lock := New(lockPath)
	if _, err := lock.Lock(); err == nil {
		t.Fatal("Lock() unexpectedly succeeded when lock path is a directory")
	}
}
