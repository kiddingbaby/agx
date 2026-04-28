package claudeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func TestSyncUpdatesSettingsWithoutAutoBackup(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	backupsDir := filepath.Join(dir, ".config", "agx", "backups")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	original := "{\n  \"theme\": \"dark\",\n  \"env\": {\n    \"KEEP_ME\": \"1\"\n  }\n}\n"
	if err := os.WriteFile(settingsPath, []byte(original), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	syncer := NewSyncer(settingsPath, backupsDir, "agx")
	result, err := syncer.Sync(domainprofile.Profile{
		Name:      "relay-a",
		BaseURL:   "https://relay.example/v1",
		APIKey:    "sk-a",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.ConfigPath != settingsPath {
		t.Fatalf("ConfigPath = %q, want %q", result.ConfigPath, settingsPath)
	}

	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(settings) error = %v", err)
	}
	text := string(got)
	if !strings.Contains(text, "\"apiKeyHelper\": \"agx __api-key relay-a\"") {
		t.Fatalf("settings=%q want apiKeyHelper", text)
	}
	if !strings.Contains(text, "\"ANTHROPIC_BASE_URL\": \"https://relay.example/v1\"") {
		t.Fatalf("settings=%q want base url", text)
	}
}

func TestSnapshotCreateBackupAndRestore(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	backupsDir := filepath.Join(dir, ".config", "agx", "backups")
	original := "{\n  \"theme\": \"dark\"\n}\n"
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(original), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	syncer := NewSyncer(settingsPath, backupsDir, "agx")
	snapshot, err := syncer.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	backupPath, err := syncer.CreateBackup("before-claude-sync", snapshot.Content)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-b", BaseURL: "https://relay-b.example/v1", APIKey: "sk-b"}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if _, err := syncer.Restore(backupPath); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(settings) error = %v", err)
	}
	if string(got) != original {
		t.Fatalf("restored settings = %q, want %q", string(got), original)
	}
}

func TestRemoveConfigRemovesOnlyManagedFields(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	syncer := NewSyncer(settingsPath, filepath.Join(dir, ".config", "agx", "backups"), "agx")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "{\n  \"theme\": \"dark\",\n  \"apiKeyHelper\": \"agx __api-key relay-a\",\n  \"env\": {\n    \"KEEP_ME\": \"1\",\n    \"ANTHROPIC_BASE_URL\": \"https://relay.example/v1\",\n    \"CLAUDE_CODE_API_KEY_HELPER_TTL_MS\": \"3600000\"\n  }\n}\n"
	if err := os.WriteFile(settingsPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig() error = %v", err)
	}

	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(settings) error = %v", err)
	}
	text := string(got)
	if !strings.Contains(text, "\"theme\": \"dark\"") || !strings.Contains(text, "\"KEEP_ME\": \"1\"") {
		t.Fatalf("settings=%q want non-AGX fields preserved", text)
	}
	if strings.Contains(text, "apiKeyHelper") || strings.Contains(text, "ANTHROPIC_BASE_URL") || strings.Contains(text, "CLAUDE_CODE_API_KEY_HELPER_TTL_MS") {
		t.Fatalf("settings=%q want AGX fields removed", text)
	}
}

func TestRemoveConfigDeletesPureManagedSettings(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	syncer := NewSyncer(settingsPath, filepath.Join(dir, ".config", "agx", "backups"), "agx")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "{\n  \"apiKeyHelper\": \"agx __api-key relay-a\",\n  \"env\": {\n    \"ANTHROPIC_BASE_URL\": \"https://relay.example/v1\",\n    \"CLAUDE_CODE_API_KEY_HELPER_TTL_MS\": \"3600000\"\n  }\n}\n"
	if err := os.WriteFile(settingsPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig() error = %v", err)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("settings still exists, err = %v", err)
	}
}
