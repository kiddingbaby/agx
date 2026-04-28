package claudeconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func TestDeleteBackupAndMissingRemoveConfig(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	syncer := NewSyncer(settingsPath, filepath.Join(dir, ".config", "agx", "backups"), "agx")

	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig() error = %v", err)
	}

	backupPath, err := syncer.CreateBackup("delete-me", []byte("{}\n"))
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	if err := syncer.DeleteBackup(backupPath); err != nil {
		t.Fatalf("DeleteBackup() error = %v", err)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("backup still exists, err = %v", err)
	}
}

func TestSyncRejectsEmptyHelperCommand(t *testing.T) {
	syncer := NewSyncer(filepath.Join(t.TempDir(), "settings.json"), filepath.Join(t.TempDir(), "backups"), "")
	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}); err == nil {
		t.Fatal("Sync() unexpectedly succeeded without helper command")
	}
}

func TestSyncRestoreSnapshotAndHelperQuoting(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	syncer := NewSyncer(settingsPath, filepath.Join(dir, ".config", "agx", "backups"), "agx helper")

	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	snapshot, err := syncer.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot == nil || !snapshot.Exists || snapshot.ConfigPath != settingsPath {
		t.Fatalf("Snapshot() = %+v, want existing snapshot", snapshot)
	}

	var settings map[string]any
	if err := json.Unmarshal(snapshot.Content, &settings); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := settings["apiKeyHelper"]; !strings.Contains(got.(string), "'agx helper'") {
		t.Fatalf("apiKeyHelper=%v want quoted helper path", got)
	}

	restorePath := filepath.Join(dir, "restore.json")
	if err := os.WriteFile(restorePath, []byte("{\"env\":{\"KEEP\":\"1\"}}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := syncer.Restore(restorePath); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "\"KEEP\"") {
		t.Fatalf("settings=%q want restored content", string(data))
	}
}

func TestRemoveConfigDeleteBackupNoopAndReadSettingsErrors(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	syncer := NewSyncer(settingsPath, filepath.Join(dir, ".config", "agx", "backups"), "agx")

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\"apiKeyHelper\":\"agx __api-key relay-a\",\"env\":{\"ANTHROPIC_BASE_URL\":\"https://relay.example/v1\",\"CLAUDE_CODE_API_KEY_HELPER_TTL_MS\":\"3600000\"}}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig() error = %v", err)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("settings still exists after removing only managed content, err=%v", err)
	}

	if err := syncer.DeleteBackup(""); err != nil {
		t.Fatalf("DeleteBackup(empty) error = %v", err)
	}

	if err := os.WriteFile(settingsPath, []byte("{bad json"), 0o600); err != nil {
		t.Fatalf("WriteFile(invalid) error = %v", err)
	}
	if _, _, _, err := syncer.readSettings(); err == nil || !strings.Contains(err.Error(), "parse claude settings") {
		t.Fatalf("readSettings(invalid) err=%v, want parse error", err)
	}

	if !strings.Contains(shellQuote("hello world"), "'hello world'") {
		t.Fatalf("shellQuote() did not quote spaced value")
	}
	if shellQuote("") != "''" {
		t.Fatalf("shellQuote(empty) = %q, want ''", shellQuote(""))
	}
}

func TestClaudeConfigAdditionalErrorBranches(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	syncer := NewSyncer(settingsPath, filepath.Join(dir, ".config", "agx", "backups"), "agx")

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\"env\":\"bad\"}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}); err == nil || !strings.Contains(err.Error(), "env must be an object") {
		t.Fatalf("Sync(invalid env) err=%v, want env object error", err)
	}

	if err := os.WriteFile(settingsPath, []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile(empty) error = %v", err)
	}
	snapshot, err := syncer.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot(empty) error = %v", err)
	}
	if snapshot == nil || !snapshot.Exists || len(snapshot.Content) != 0 {
		t.Fatalf("Snapshot(empty) = %+v, want existing empty file", snapshot)
	}

	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}
	badSyncer := NewSyncer(filepath.Join(blocker, "settings.json"), filepath.Join(dir, ".config", "agx", "backups"), "agx")
	if _, err := badSyncer.Restore(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("Restore(missing backup) unexpectedly succeeded")
	}
	if _, err := badSyncer.CreateBackup("bad", []byte("{}\n")); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
}

func TestClaudeConfigFileOperationFailures(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}

	syncer := NewSyncer(filepath.Join(blocker, "settings.json"), filepath.Join(dir, "backups"), "agx")
	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}); err == nil {
		t.Fatal("Sync() unexpectedly succeeded with file parent")
	}
	if _, err := syncer.Snapshot(); err == nil {
		t.Fatal("Snapshot() unexpectedly succeeded with file parent")
	}
	if _, err := syncer.CreateBackup("bad", []byte("{}\n")); err != nil {
		t.Fatalf("CreateBackup(file parent on settings only) error = %v", err)
	}

	backupBlocker := filepath.Join(dir, "backup-blocker")
	if err := os.WriteFile(backupBlocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(backup blocker) error = %v", err)
	}
	badBackupSyncer := NewSyncer(filepath.Join(dir, ".claude", "settings.json"), backupBlocker, "agx")
	if _, err := badBackupSyncer.CreateBackup("bad", []byte("{}\n")); err == nil {
		t.Fatal("CreateBackup() unexpectedly succeeded with file parent backup dir")
	}

	writeBlockedPath := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(writeBlockedPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(writeBlockedPath, []byte("{\"apiKeyHelper\":\"agx __api-key relay-a\"}\n"), 0o400); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	removeBlocked := NewSyncer(filepath.Join(writeBlockedPath, "child.json"), filepath.Join(dir, "backups2"), "agx")
	if _, err := removeBlocked.RemoveConfig(); err == nil {
		t.Fatal("RemoveConfig() unexpectedly succeeded with file parent")
	}

	backupPath := filepath.Join(blocker, "backup.json")
	deleteSyncer := NewSyncer(settingsPath, filepath.Join(dir, "backups3"), "agx")
	if err := deleteSyncer.DeleteBackup(backupPath); err == nil {
		t.Fatal("DeleteBackup() unexpectedly succeeded for non-not-exist remove error")
	}

	if _, _, _, err := removeBlocked.readSettings(); err == nil {
		t.Fatal("readSettings() unexpectedly succeeded with file parent")
	}

	if _, err := buildSettings(nil, domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}, "agx"); err != nil {
		t.Fatalf("buildSettings(nil) error = %v", err)
	}
}
