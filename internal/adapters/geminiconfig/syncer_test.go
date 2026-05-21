package geminiconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func TestSyncWritesRequiredSelections(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".gemini", "settings.json")
	syncer := NewSyncer(settingsPath, filepath.Join(dir, "backups"))

	result, err := syncer.Sync(domainprofile.Profile{Name: "relay-a"})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.ConfigPath != settingsPath {
		t.Fatalf("ConfigPath = %q, want %q", result.ConfigPath, settingsPath)
	}

	settings := readSettings(t, settingsPath)
	if got := nestedString(settings, "security", "auth", "selectedType"); got != "gemini-api-key" {
		t.Fatalf("selectedType = %q, want gemini-api-key", got)
	}
	if got, ok := nestedBool(settings, "tools", "sandbox"); !ok || got {
		t.Fatalf("tools.sandbox = %v, want false", got)
	}
}

func TestSyncPreservesUserKeys(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	existing := `{
  "mcpServers": {
    "filesystem": {"command": "npx", "args": ["fs"]}
  },
  "ui": {"theme": "dark"},
  "tools": {"sandbox": true, "exclude": ["rm"]}
}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	syncer := NewSyncer(settingsPath, filepath.Join(dir, "backups"))
	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-a"}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	settings := readSettings(t, settingsPath)
	// agx-required values
	if got := nestedString(settings, "security", "auth", "selectedType"); got != "gemini-api-key" {
		t.Fatalf("selectedType = %q, want gemini-api-key", got)
	}
	if got, _ := nestedBool(settings, "tools", "sandbox"); got {
		t.Fatalf("tools.sandbox = true, want false (agx must override)")
	}
	// user-added keys preserved
	if mcp, ok := settings["mcpServers"].(map[string]any); !ok || mcp["filesystem"] == nil {
		t.Fatalf("mcpServers.filesystem missing after sync; got %v", settings["mcpServers"])
	}
	if got := nestedString(settings, "ui", "theme"); got != "dark" {
		t.Fatalf("ui.theme = %q, want dark", got)
	}
	// peer keys under tools preserved
	if tools, ok := settings["tools"].(map[string]any); ok {
		if exclude, ok := tools["exclude"].([]any); !ok || len(exclude) != 1 {
			t.Fatalf("tools.exclude lost; got %v", tools["exclude"])
		}
	} else {
		t.Fatalf("tools block lost")
	}
}

func TestSnapshotCreateBackupAndRestore(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	original := `{"mcpServers":{"keep":{"command":"x"}}}`
	if err := os.WriteFile(settingsPath, []byte(original), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	syncer := NewSyncer(settingsPath, filepath.Join(dir, "backups"))
	snapshot, err := syncer.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if !snapshot.Exists || string(snapshot.Content) != original {
		t.Fatalf("snapshot = %+v, want original content", snapshot)
	}
	backupPath, err := syncer.CreateBackup("before-sync", snapshot.Content)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-a"}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	// sync should not have lost the user key
	if got := readSettings(t, settingsPath); got["mcpServers"] == nil {
		t.Fatalf("Sync dropped user key; got %v", got)
	}

	if _, err := syncer.Restore(backupPath); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != original {
		t.Fatalf("restored = %q, want %q", string(got), original)
	}
}

func TestSnapshotMissingFile(t *testing.T) {
	dir := t.TempDir()
	syncer := NewSyncer(filepath.Join(dir, ".gemini", "settings.json"), filepath.Join(dir, "backups"))
	snapshot, err := syncer.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.Exists {
		t.Fatalf("Snapshot Exists = true on missing file, want false")
	}
}

func TestRestoreEmptyBackupRemovesFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"a":1}`), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	emptyBackup := filepath.Join(dir, "empty.bak")
	if err := os.WriteFile(emptyBackup, []byte{}, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	syncer := NewSyncer(settingsPath, filepath.Join(dir, "backups"))
	if _, err := syncer.Restore(emptyBackup); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("settings.json still exists after restoring empty backup, err = %v", err)
	}
}

func TestRemoveConfigDeletesSettings(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{}"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	syncer := NewSyncer(settingsPath, filepath.Join(dir, "backups"))
	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig() error = %v", err)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("settings.json still exists, err = %v", err)
	}
}

func TestRemoveConfigIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".gemini", "settings.json")
	syncer := NewSyncer(settingsPath, filepath.Join(dir, "backups"))
	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig() on missing file error = %v", err)
	}
}

func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return out
}

func nestedString(m map[string]any, path ...string) string {
	cur := m
	for i, key := range path {
		if i == len(path)-1 {
			if v, ok := cur[key].(string); ok {
				return v
			}
			return ""
		}
		next, ok := cur[key].(map[string]any)
		if !ok {
			return ""
		}
		cur = next
	}
	return ""
}

func nestedBool(m map[string]any, path ...string) (bool, bool) {
	cur := m
	for i, key := range path {
		if i == len(path)-1 {
			v, ok := cur[key].(bool)
			return v, ok
		}
		next, ok := cur[key].(map[string]any)
		if !ok {
			return false, false
		}
		cur = next
	}
	return false, false
}
