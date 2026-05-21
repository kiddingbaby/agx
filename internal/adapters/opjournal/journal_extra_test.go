package opjournal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func TestJournalClearMismatch(t *testing.T) {
	journal := New(filepath.Join(t.TempDir(), "ops", "current.yaml"))
	now := time.Now().UTC()
	record := ports.OperationRecord{ID: "op-1", Command: "set", Agent: domainprofile.AgentCodex, Stage: "started", StartedAt: now, UpdatedAt: now}

	if err := journal.Begin(record); err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := journal.Clear("other"); err == nil {
		t.Fatal("Clear() unexpectedly succeeded for mismatched id")
	}
	if err := journal.Clear("op-1"); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if err := journal.Clear("op-1"); err != nil {
		t.Fatalf("Clear() on missing journal error = %v", err)
	}
}

func TestJournalAdditionalErrorBranches(t *testing.T) {
	dir := t.TempDir()
	journal := New(filepath.Join(dir, "ops", "current.yaml"))

	if current, err := journal.Current(); err != nil || current != nil {
		t.Fatalf("Current(missing) = (%v,%v), want nil,nil", current, err)
	}
	if err := journal.Clear(""); err != nil {
		t.Fatalf("Clear(missing) error = %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(journal.path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(journal.path, []byte("bad: [\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := journal.Current(); err == nil {
		t.Fatal("Current(invalid yaml) unexpectedly succeeded")
	}
	if err := journal.Clear(""); err == nil {
		t.Fatal("Clear(invalid yaml) unexpectedly succeeded")
	}

	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}
	badJournal := New(filepath.Join(blocker, "current.yaml"))
	record := ports.OperationRecord{ID: "op-1", Command: "set", Agent: domainprofile.AgentCodex, Stage: "started", StartedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := badJournal.Begin(record); err == nil {
		t.Fatal("Begin(file parent) unexpectedly succeeded")
	}

	if err := os.Remove(journal.path); err != nil {
		t.Fatalf("Remove(invalid yaml) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(journal.path), 0o700); err != nil {
		t.Fatalf("MkdirAll(recreate) error = %v", err)
	}
	if err := journal.Begin(record); err != nil {
		t.Fatalf("Begin(valid) error = %v", err)
	}
	if err := os.Remove(journal.path); err != nil {
		t.Fatalf("Remove(current) error = %v", err)
	}
	if err := journal.Update(record); err != nil {
		t.Fatalf("Update(recreate) error = %v", err)
	}
	current, err := journal.Current()
	if err != nil || current == nil || current.ID != "op-1" {
		t.Fatalf("Current(after update recreate) = (%+v,%v), want op-1", current, err)
	}
}

func TestJournalBeginSameOperationIDAndClearMismatchMessage(t *testing.T) {
	journal := New(filepath.Join(t.TempDir(), "ops", "current.yaml"))
	now := time.Now().UTC()
	record := ports.OperationRecord{ID: "op-1", Command: "set", Agent: domainprofile.AgentCodex, Stage: "started", StartedAt: now, UpdatedAt: now}

	if err := journal.Begin(record); err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	record.Stage = "config_written"
	if err := journal.Begin(record); err != nil {
		t.Fatalf("Begin(same id) error = %v", err)
	}

	err := journal.Clear("other")
	if err == nil || !strings.Contains(err.Error(), "operation journal mismatch") {
		t.Fatalf("Clear(mismatch) err=%v, want mismatch message", err)
	}
}
