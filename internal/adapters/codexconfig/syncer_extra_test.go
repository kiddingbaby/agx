package codexconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func TestStatusClearDefaultProfileRemoveProfileAndDeleteBackup(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	backupsDir := filepath.Join(dir, ".config", "agx", "backups")
	syncer := NewSyncer(configPath, backupsDir, "agx")

	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay-a.example/v1", APIKey: "sk-a"}, ports.CodexSyncOptions{DefaultProfileName: "relay-a"}); err != nil {
		t.Fatalf("first Sync() error = %v", err)
	}
	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-b", BaseURL: "https://relay-b.example/v1", APIKey: "sk-b"}, ports.CodexSyncOptions{}); err != nil {
		t.Fatalf("second Sync() error = %v", err)
	}

	status, err := syncer.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.DefaultProfileName != "relay-a" || len(status.ManagedProfilesByID) != 2 {
		t.Fatalf("Status() = %+v, want relay-a default and 2 profiles", status)
	}

	if _, err := syncer.ClearDefaultProfile(); err != nil {
		t.Fatalf("ClearDefaultProfile() error = %v", err)
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(content), "profile = \"relay-a\"") {
		t.Fatalf("config=%q want root profile removed", string(content))
	}

	if _, err := syncer.RemoveProfile("relay-a"); err != nil {
		t.Fatalf("RemoveProfile() error = %v", err)
	}
	content, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(content), "[profiles.\"relay-a\"]") {
		t.Fatalf("config=%q want relay-a removed", string(content))
	}

	backupPath, err := syncer.CreateBackup("delete-me", []byte("backup"))
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
	syncer := NewSyncer(filepath.Join(t.TempDir(), "config.toml"), filepath.Join(t.TempDir(), "backups"), "")
	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}, ports.CodexSyncOptions{}); err == nil {
		t.Fatal("Sync() unexpectedly succeeded without helper command")
	}
}

func TestSnapshotRestoreRemoveConfigAndDeleteBackupNoop(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	backupsDir := filepath.Join(dir, ".config", "agx", "backups")
	syncer := NewSyncer(configPath, backupsDir, "agx")

	snapshot, err := syncer.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot == nil || snapshot.Exists || snapshot.ConfigPath != configPath {
		t.Fatalf("Snapshot() = %+v, want non-existing snapshot with config path", snapshot)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "restore.toml"), []byte("profile = \"relay-a\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(restore) error = %v", err)
	}
	if _, err := syncer.Restore(filepath.Join(dir, "restore.toml")); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "profile = \"relay-a\"\n" {
		t.Fatalf("config=%q want restored content", string(data))
	}

	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig() error = %v", err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config still exists, err=%v", err)
	}

	if err := syncer.DeleteBackup(""); err != nil {
		t.Fatalf("DeleteBackup(empty) error = %v", err)
	}
}

func TestRemoveProfileRemovesConfigWhenLastManagedProfileDeleted(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"), "agx")

	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}, ports.CodexSyncOptions{DefaultProfileName: "relay-a"}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if _, err := syncer.RemoveProfile("relay-a"); err != nil {
		t.Fatalf("RemoveProfile() error = %v", err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config still exists after removing last profile, err=%v", err)
	}
}

func TestManagedHelpersAndCommentParsing(t *testing.T) {
	line := `profile = "relay-a # not comment" # keep me`
	if idx := findCommentIndex(line); idx < 0 || line[idx:] != "# keep me" {
		t.Fatalf("findCommentIndex() = %d, want comment after quoted text", idx)
	}
	if idx := findCommentIndex(`profile = 'relay-a # still quoted'`); idx != -1 {
		t.Fatalf("findCommentIndex(single quoted) = %d, want -1", idx)
	}

	rewritten := rewriteProfileLine(`profile = "relay-a"   # trailing`, "relay-b")
	if !strings.Contains(rewritten, `profile = "relay-b"`) || !strings.Contains(rewritten, "# trailing") {
		t.Fatalf("rewriteProfileLine() = %q, want preserved comment", rewritten)
	}

	tables := codexTablePresence("[profiles]\nkey = 1\n")
	if tables.HasModelProviders || !tables.HasProfiles {
		t.Fatalf("codexTablePresence() = %+v, want only profiles table", tables)
	}

	section, name := parseManagedSection(`[model_providers."agx/relay-a".auth]`)
	if section != "" || name != "" {
		t.Fatalf("parseManagedSection(auth) = (%q,%q), want empty values", section, name)
	}
}

func TestStatusAndRootProfileHelpersAdditionalBranches(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"), "agx")

	status, err := syncer.Status()
	if err != nil {
		t.Fatalf("Status(missing) error = %v", err)
	}
	if status.DefaultProfileName != "" || len(status.ManagedProfilesByID) != 0 {
		t.Fatalf("Status(missing) = %+v, want empty status", status)
	}

	if got := findRootProfileName("[profiles]\nmodel = \"x\"\n"); got != "" {
		t.Fatalf("findRootProfileName(table first) = %q, want empty", got)
	}
	if got := normalizeManagedRootProfile("relay-a", map[string]managedProfile{}); got != "" {
		t.Fatalf("normalizeManagedRootProfile(missing) = %q, want empty", got)
	}

	rewritten := upsertRootProfile("# comment only", "relay-a")
	if !strings.Contains(rewritten, "profile = \"relay-a\"") {
		t.Fatalf("upsertRootProfile(comment-only) = %q, want inserted root profile", rewritten)
	}

	removed := removeRootProfile("model = \"gpt-5\"\n[profiles]\n")
	if !strings.Contains(removed, "model = \"gpt-5\"") {
		t.Fatalf("removeRootProfile(no root profile) = %q, want preserved content", removed)
	}
}

func TestCodexConfigHelperBranchesForUnmanagedAndEmptyConfigs(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"), "agx")

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(configPath, []byte("model = \"gpt-5\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := syncer.ClearDefaultProfile(); err != nil {
		t.Fatalf("ClearDefaultProfile(unmanaged only) error = %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "model = \"gpt-5\"" {
		t.Fatalf("config=%q want unmanaged content preserved", string(data))
	}

	if _, err := syncer.RemoveProfile(""); err != nil {
		t.Fatalf("RemoveProfile(empty) error = %v", err)
	}
	if _, err := syncer.RemoveProfile("relay-a"); err != nil {
		t.Fatalf("RemoveProfile(missing managed block) error = %v", err)
	}
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "model = \"gpt-5\"" {
		t.Fatalf("config=%q want unchanged unmanaged content", string(data))
	}

	if got := stripManagedBlock(beginMarker + "\n# orphan"); got != "" {
		t.Fatalf("stripManagedBlock(orphan) = %q, want empty", got)
	}
}

func TestCodexConfigWrapperMethodsAdditionalBranches(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	backupsDir := filepath.Join(dir, "backups")
	syncer := NewSyncer(configPath, backupsDir, "agx")

	backupPath, err := syncer.CreateBackup("empty", []byte(""))
	if err != nil {
		t.Fatalf("CreateBackup(empty) error = %v", err)
	}
	snapshot, err := syncer.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot(missing) error = %v", err)
	}
	if snapshot.Exists {
		t.Fatalf("Snapshot(missing) = %+v, want Exists=false", snapshot)
	}
	if err := syncer.DeleteBackup(backupPath); err != nil {
		t.Fatalf("DeleteBackup(created) error = %v", err)
	}
	if err := syncer.DeleteBackup(backupPath); err != nil {
		t.Fatalf("DeleteBackup(missing) error = %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(configPath, []byte("profile = \"relay-a\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig(existing) error = %v", err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config still exists after RemoveConfig, err=%v", err)
	}
	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig(missing) error = %v", err)
	}
}

func TestCodexConfigAdditionalSuccessAndErrorBranches(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	backupsDir := filepath.Join(dir, "backups")
	syncer := NewSyncer(configPath, backupsDir, "agx")

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(configPath, []byte("profile = \"relay-a\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	snapshot, err := syncer.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot(existing) error = %v", err)
	}
	if !snapshot.Exists || string(snapshot.Content) != "profile = \"relay-a\"\n" {
		t.Fatalf("Snapshot(existing) = %+v, want current content", snapshot)
	}

	backupPath, err := syncer.CreateBackup("restore", snapshot.Content)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	if _, err := syncer.Restore(backupPath); err != nil {
		t.Fatalf("Restore(existing backup) error = %v", err)
	}

	if _, err := syncer.Restore(filepath.Join(dir, "missing.bak")); err == nil {
		t.Fatal("Restore(missing backup) unexpectedly succeeded")
	}

	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay-a.example/v1", APIKey: "sk-a"}, ports.CodexSyncOptions{DefaultProfileName: "relay-a"}); err != nil {
		t.Fatalf("Sync(relay-a) error = %v", err)
	}
	if _, err := syncer.Sync(domainprofile.Profile{Name: "relay-b", BaseURL: "https://relay-b.example/v1", APIKey: "sk-b"}, ports.CodexSyncOptions{}); err != nil {
		t.Fatalf("Sync(relay-b) error = %v", err)
	}
	if _, err := syncer.RemoveProfile("relay-b"); err != nil {
		t.Fatalf("RemoveProfile(relay-b) error = %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "profile = \"relay-a\"") || strings.Contains(text, "[profiles.\"relay-b\"]") {
		t.Fatalf("config=%q want relay-a preserved as root and relay-b removed", text)
	}

	if _, err := syncer.ClearDefaultProfile(); err != nil {
		t.Fatalf("ClearDefaultProfile(after managed sync) error = %v", err)
	}
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "[profiles.\"relay-a\"]") {
		t.Fatalf("config=%q want managed profile retained after clearing default", string(data))
	}
}

func TestCodexConfigDirectoryAndPathErrorBranches(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	backupsDir := filepath.Join(dir, "backups")
	syncer := NewSyncer(configPath, backupsDir, "agx")

	if err := os.MkdirAll(configPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(config dir) error = %v", err)
	}
	if _, err := syncer.Snapshot(); err == nil {
		t.Fatal("Snapshot(directory path) unexpectedly succeeded")
	}
	if _, err := syncer.Status(); err == nil {
		t.Fatal("Status(directory path) unexpectedly succeeded")
	}
	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig(directory path) error = %v", err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config path still exists after RemoveConfig, err=%v", err)
	}

	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}
	badSyncer := NewSyncer(filepath.Join(blocker, "config.toml"), backupsDir, "agx")
	if _, err := badSyncer.Restore(filepath.Join(dir, "missing.toml")); err == nil {
		t.Fatal("Restore(missing backup) unexpectedly succeeded")
	}
	if _, err := badSyncer.CreateBackup("bad", []byte("profile = \"x\"\n")); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	if _, err := badSyncer.Sync(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}, ports.CodexSyncOptions{}); err == nil {
		t.Fatal("Sync(file parent) unexpectedly succeeded")
	}
}

func TestCodexConfigHelperAndErrorBranches(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"), "agx")

	if _, err := syncer.Status(); err != nil {
		t.Fatalf("Status(missing) error = %v", err)
	}

	if got := upsertRootProfile("[profiles]\nname = \"x\"", "relay-a"); !strings.HasPrefix(got, "profile = \"relay-a\"\n[profiles]") {
		t.Fatalf("upsertRootProfile(table first) = %q, want inserted root before table", got)
	}

	if got := removeRootProfile("profile = \"relay-a\"\n"); got != "" {
		t.Fatalf("removeRootProfile(root only) = %q, want empty", got)
	}

	if got := rewriteProfileLine("profile", "relay-a"); got != "profile = \"relay-a\"" {
		t.Fatalf("rewriteProfileLine(no equals) = %q, want normalized profile assignment", got)
	}

	if idx := findCommentIndex(`profile = "relay-a \\\"#\\\"" # trailing`); idx < 0 {
		t.Fatalf("findCommentIndex(escaped quote) = %d, want trailing comment", idx)
	}

	block := strings.Join([]string{
		beginMarker,
		"[profiles.\"relay-a\"]",
		"model_provider = \"agx/relay-a\"",
		"",
		"# note",
		"model = \"gpt-5\"",
		"",
		endMarker,
	}, "\n")
	profiles := extractManagedProfiles(block)
	if got := profiles["relay-a"].Extras; len(got) != 2 || got[0] != "# note" || got[1] != "model = \"gpt-5\"" {
		t.Fatalf("extractManagedProfiles() extras = %v, want trimmed extras", got)
	}

	section, name := parseManagedSection(`[profiles.]`)
	if section != "" || name != "" {
		t.Fatalf("parseManagedSection(invalid profile section) = (%q,%q), want empty", section, name)
	}
}

func TestCodexConfigAdditionalBranchCoverage(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".codex", "config.toml")
	backupsDir := filepath.Join(dir, "backups")
	syncer := NewSyncer(configPath, backupsDir, "agx")

	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}

	if _, err := syncer.Restore(filepath.Join(blocker, "missing.toml")); err == nil {
		t.Fatal("Restore() unexpectedly succeeded with unreadable backup parent")
	}

	blockedBackupSyncer := NewSyncer(filepath.Join(dir, ".codex", "config.toml"), blocker, "agx")
	if _, err := blockedBackupSyncer.CreateBackup("bad", []byte("profile = \"x\"\n")); err == nil {
		t.Fatal("CreateBackup() unexpectedly succeeded with file parent backup dir")
	}

	if _, err := syncer.ClearDefaultProfile(); err != nil {
		t.Fatalf("ClearDefaultProfile(missing config) error = %v", err)
	}

	if _, err := syncer.RemoveProfile(""); err != nil {
		t.Fatalf("RemoveProfile(empty) error = %v", err)
	}
	if _, err := syncer.RemoveProfile("relay-a"); err != nil {
		t.Fatalf("RemoveProfile(missing config) error = %v", err)
	}

	badPathSyncer := NewSyncer(filepath.Join(blocker, "config.toml"), filepath.Join(dir, "other-backups"), "agx")
	if _, err := badPathSyncer.ClearDefaultProfile(); err == nil {
		t.Fatal("ClearDefaultProfile() unexpectedly succeeded with file parent config path")
	}
	if _, err := badPathSyncer.RemoveProfile("relay-a"); err == nil {
		t.Fatal("RemoveProfile() unexpectedly succeeded with file parent config path")
	}

	if err := blockedBackupSyncer.DeleteBackup(filepath.Join(blocker, "backup.toml")); err == nil {
		t.Fatal("DeleteBackup() unexpectedly succeeded for non-not-exist remove error")
	}

	if content, exists, err := readIfExists(filepath.Join(blocker, "config.toml")); err == nil || exists || content != "" {
		t.Fatalf("readIfExists(file parent) = (%q,%v,%v), want error", content, exists, err)
	}

	block := renderManagedBlock(map[string]managedProfile{
		"relay-a": {
			Name:    "relay-a",
			BaseURL: "https://relay.example/v1",
			Extras:  []string{"", "model = \"gpt-5.5\""},
		},
	}, "agx", codexTables{})
	if !strings.Contains(block, "\n\nmodel = \"gpt-5.5\"") {
		t.Fatalf("renderManagedBlock() = %q, want blank extra line preserved", block)
	}

	if got := codexProviderID("relay-a"); got != "agx/relay-a" {
		t.Fatalf("codexProviderID() = %q, want agx/relay-a", got)
	}
	if key, value, ok := parseKeyValueLine("no equals"); ok || key != "" || value != "" {
		t.Fatalf("parseKeyValueLine(no equals) = (%q,%q,%v), want empty false", key, value, ok)
	}
}
