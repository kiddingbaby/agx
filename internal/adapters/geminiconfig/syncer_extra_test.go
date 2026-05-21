package geminiconfig

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
	envPath := filepath.Join(dir, ".gemini", ".env")
	syncer := NewSyncer(envPath, filepath.Join(dir, ".config", "agx", "backups"))

	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig() error = %v", err)
	}

	backupPath, err := syncer.CreateBackup("delete-me", []byte("KEY=value\n"))
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

func TestSyncSnapshotRestoreAndDeleteBackupNoop(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".gemini", ".env")
	syncer := NewSyncer(envPath, filepath.Join(dir, ".config", "agx", "backups"))

	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	snapshot, err := syncer.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot == nil || !snapshot.Exists || snapshot.ConfigPath != envPath {
		t.Fatalf("Snapshot() = %+v, want existing snapshot", snapshot)
	}
	var bundle struct {
		Format string            `json:"format"`
		Files  map[string]string `json:"files"`
	}
	if err := json.Unmarshal(snapshot.Content, &bundle); err != nil {
		t.Fatalf("Unmarshal(snapshot) error = %v", err)
	}
	if bundle.Format != bundleMagic || !strings.Contains(bundle.Files[".env"], "GEMINI_API_KEY=\"sk-a\"") {
		t.Fatalf("snapshot bundle = %+v, want managed API key in .env", bundle)
	}

	restorePath := filepath.Join(dir, "restore.env")
	if err := os.WriteFile(restorePath, []byte("KEEP=1\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := syncer.Restore(restorePath); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "KEEP=1\n" {
		t.Fatalf("env=%q want restored content", string(data))
	}
	settingsPath := filepath.Join(dir, ".gemini", "settings.json")
	settings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(settings) error = %v", err)
	}
	if !strings.Contains(string(settings), `"selectedType": "gemini-api-key"`) {
		t.Fatalf("settings after legacy restore = %q, want managed settings retained", string(settings))
	}

	if err := syncer.DeleteBackup(""); err != nil {
		t.Fatalf("DeleteBackup(empty) error = %v", err)
	}
}

func TestRemoveConfigRemovesManagedBlockAndHelpers(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".gemini", ".env")
	syncer := NewSyncer(envPath, filepath.Join(dir, ".config", "agx", "backups"))

	original := "KEEP=1\n\n" + renderManagedBlock(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}) + "\n"
	if err := os.MkdirAll(filepath.Dir(envPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(envPath, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig() error = %v", err)
	}
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), "AGX managed Gemini env") || !strings.Contains(string(data), "KEEP=1") {
		t.Fatalf("env=%q want preserved unmanaged content only", string(data))
	}
	if _, err := os.Stat(filepath.Join(dir, ".gemini", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("settings still exists after remove, err = %v", err)
	}

	if got := appendManagedBlock("", "BLOCK"); got != "BLOCK\n" {
		t.Fatalf("appendManagedBlock(empty) = %q, want BLOCK newline", got)
	}
}

func TestGeminiConfigAdditionalErrorBranches(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".gemini", ".env")
	syncer := NewSyncer(envPath, filepath.Join(dir, ".config", "agx", "backups"))

	if err := os.MkdirAll(filepath.Dir(envPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(envPath, []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile(empty) error = %v", err)
	}
	snapshot, err := syncer.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot(empty) error = %v", err)
	}
	if snapshot == nil || !snapshot.Exists {
		t.Fatalf("Snapshot(empty) = %+v, want existing empty file bundle", snapshot)
	}
	var bundle struct {
		Format  string            `json:"format"`
		Files   map[string]string `json:"files"`
		Missing []string          `json:"missing"`
	}
	if err := json.Unmarshal(snapshot.Content, &bundle); err != nil {
		t.Fatalf("Unmarshal(snapshot) error = %v", err)
	}
	if bundle.Format != bundleMagic || bundle.Files[".env"] != "" || len(bundle.Missing) != 1 || bundle.Missing[0] != "settings.json" {
		t.Fatalf("Snapshot(empty bundle) = %+v, want empty env plus missing settings marker", bundle)
	}

	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}
	badSyncer := NewSyncer(filepath.Join(blocker, ".env"), filepath.Join(dir, ".config", "agx", "backups"))
	if _, err := badSyncer.Sync(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}); err == nil {
		t.Fatal("Sync() unexpectedly succeeded with file parent")
	}
	if _, err := badSyncer.Restore(filepath.Join(dir, "missing.env")); err == nil {
		t.Fatal("Restore(missing backup) unexpectedly succeeded")
	}
}

func TestGeminiConfigFileOperationFailures(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}

	syncer := NewSyncer(filepath.Join(blocker, ".env"), filepath.Join(dir, "backups"))
	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}); err == nil {
		t.Fatal("Sync() unexpectedly succeeded with file parent")
	}
	if _, err := syncer.Snapshot(); err == nil {
		t.Fatal("Snapshot() unexpectedly succeeded with file parent")
	}

	backupBlocker := filepath.Join(dir, "backup-blocker")
	if err := os.WriteFile(backupBlocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(backup blocker) error = %v", err)
	}
	badBackupSyncer := NewSyncer(filepath.Join(dir, ".gemini", ".env"), backupBlocker)
	if _, err := badBackupSyncer.CreateBackup("bad", []byte("KEY=value\n")); err == nil {
		t.Fatal("CreateBackup() unexpectedly succeeded with file parent backup dir")
	}

	removeBlocked := NewSyncer(filepath.Join(blocker, "child.env"), filepath.Join(dir, "backups2"))
	if _, err := removeBlocked.RemoveConfig(); err == nil {
		t.Fatal("RemoveConfig() unexpectedly succeeded with file parent")
	}

	if err := badBackupSyncer.DeleteBackup(filepath.Join(blocker, "backup.env")); err == nil {
		t.Fatal("DeleteBackup() unexpectedly succeeded for non-not-exist remove error")
	}

	if _, err := stripManagedBlock("prefix\n"+beginMarker+"\nunterminated", ""); err == nil {
		t.Fatal("stripManagedBlock(unterminated) unexpectedly succeeded")
	}
}

func TestGeminiConfigRejectsIncompleteManagedBlock(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".gemini", ".env")
	backupsDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(filepath.Dir(envPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(envPath, []byte("KEEP=1\n"+beginMarker+"\nunterminated\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	syncer := NewSyncer(envPath, backupsDir)
	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}); err == nil {
		t.Fatal("Sync() unexpectedly succeeded with incomplete managed block")
	}
	if _, err := syncer.RemoveConfig(); err == nil {
		t.Fatal("RemoveConfig() unexpectedly succeeded with incomplete managed block")
	}
}

func TestRestoreBundleRemovesManagedSettingsWhenMarkedMissing(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".gemini", ".env")
	settingsPath := filepath.Join(dir, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(envPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{\"managed\":true}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(settings) error = %v", err)
	}

	bundle := snapshotBundle{
		Format:  bundleMagic,
		Files:   map[string]string{".env": "KEEP=1\n"},
		Missing: []string{"settings.json"},
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("Marshal(bundle) error = %v", err)
	}
	backupPath := filepath.Join(dir, "bundle.bak")
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile(backup) error = %v", err)
	}

	syncer := NewSyncer(envPath, filepath.Join(dir, "backups"))
	if _, err := syncer.Restore(backupPath); err != nil {
		t.Fatalf("Restore(bundle) error = %v", err)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("settings still exists, err = %v", err)
	}
}
