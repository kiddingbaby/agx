package opjournal

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func TestJournalLifecycle(t *testing.T) {
	journal := New(filepath.Join(t.TempDir(), "ops", "current.yaml"))
	record := ports.OperationRecord{
		ID:        "op-1",
		Command:   "set",
		Agent:     domainprofile.AgentCodex,
		Profile:   "relay-a",
		Stage:     "started",
		StartedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if current, err := journal.Current(); err != nil || current != nil {
		t.Fatalf("Current() = (%v, %v), want nil nil", current, err)
	}
	if err := journal.Begin(record); err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	record.Stage = "config_written"
	record.BackupID = "backup-1"
	if err := journal.Update(record); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	current, err := journal.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current == nil || current.Stage != "config_written" || current.BackupID != "backup-1" {
		t.Fatalf("current = %#v, want updated record", current)
	}

	if err := journal.Clear("op-1"); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if current, err := journal.Current(); err != nil || current != nil {
		t.Fatalf("Current() after clear = (%v, %v), want nil nil", current, err)
	}
}

func TestJournalBeginRefusesUnfinishedOperation(t *testing.T) {
	journal := New(filepath.Join(t.TempDir(), "ops", "current.yaml"))
	now := time.Now().UTC()

	if err := journal.Begin(ports.OperationRecord{ID: "op-1", Command: "set", Agent: domainprofile.AgentCodex, Stage: "started", StartedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	err := journal.Begin(ports.OperationRecord{ID: "op-2", Command: "restore", Agent: domainprofile.AgentCodex, Stage: "started", StartedAt: now, UpdatedAt: now})
	if err == nil || !strings.Contains(err.Error(), "unfinished AGX operation op-1") {
		t.Fatalf("Begin() err = %v, want unfinished operation", err)
	}
}
