package usecase

import (
	"os"
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

// TestRestoreClearsStaleJournalEntry verifies that agx restore is able to
// take over an unrelated stale journal entry left behind by a previously
// crashed command. Earlier the second Begin() call would refuse to record
// the new restore operation because the prior entry was still present,
// trapping users in a loop where doctor recommended `agx restore` and
// restore itself refused to run.
func TestRestoreClearsStaleJournalEntry(t *testing.T) {
	backupDir := t.TempDir()
	backupPath := backupDir + "/snap.toml"
	if err := os.WriteFile(backupPath, []byte("# managed snapshot\n"), 0o600); err != nil {
		t.Fatalf("seed backup: %v", err)
	}

	now := time.Now().UTC()
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"work": {Name: "work", BaseURL: "https://relay.example/v1", APIKey: "sk-test", CreatedAt: now, UpdatedAt: now},
	}}
	state := &fakeStateRepo{
		state: domainprofile.State{
			Codex: domainprofile.CodexState{
				BindingView: domainprofile.BindingView{
					SourceProfile: "work",
					Status:        domainprofile.BindingStatusApplied,
					ConfigPath:    "/tmp/codex/config.toml",
					LastAppliedAt: now,
					LastBackupID:  "b-1",
				},
				Backups: []domainprofile.Backup{
					{
						ID:          "b-1",
						ConfigPath:  "/tmp/codex/config.toml",
						BackupPath:  backupPath,
						RestoreMode: domainprofile.RestoreModeRestoreFile,
						CreatedAt:   now,
					},
				},
			},
		},
	}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	journal := &fakeJournal{current: &ports.OperationRecord{ID: "stale-add-codex", Command: "add", Agent: domainprofile.AgentCodex, Stage: "started"}}

	svc := NewProfileService(repo, state, codex, nil, nil, nil)
	svc.SetOperationJournal(journal)

	if _, err := svc.Restore(domainprofile.AgentCodex, ""); err != nil {
		t.Fatalf("Restore() should clear stale journal entry, got error: %v", err)
	}
	if journal.current != nil {
		t.Fatalf("journal should be cleared after successful restore, current = %+v", journal.current)
	}
}
