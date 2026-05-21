package opencodeconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func TestSyncWritesManagedProviderAndCurrentModel(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"))

	_, err := syncer.Sync(profile("relay-a", "https://relay.example", "sk-a"), ports.OpenCodeSyncOptions{
		ModelID:      "gpt-4o",
		ModelName:    "GPT-4o",
		SetAsCurrent: true,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	settings := readJSONConfig(t, configPath)
	if got := settings["model"]; got != "agx-relay-a-openai-compatible/gpt-4o" {
		t.Fatalf("model = %v, want agx-relay-a-openai-compatible/gpt-4o (default heuristic)", got)
	}
	providers := settings["provider"].(map[string]any)
	for _, suffix := range []string{"openai-compatible", "anthropic", "gemini"} {
		id := "agx-relay-a-" + suffix
		if _, ok := providers[id].(map[string]any); !ok {
			t.Fatalf("provider %q missing in %+v", id, providers)
		}
	}
	openaiProvider := providers["agx-relay-a-openai-compatible"].(map[string]any)
	if got := openaiProvider["npm"]; got != "@ai-sdk/openai-compatible" {
		t.Fatalf("openai npm = %v, want @ai-sdk/openai-compatible", got)
	}
	options := openaiProvider["options"].(map[string]any)
	if got := options["baseURL"]; got != "https://relay.example/v1" {
		t.Fatalf("openai baseURL = %v, want https://relay.example/v1", got)
	}
	if got := options["apiKey"]; got != "sk-a" {
		t.Fatalf("apiKey = %v, want sk-a", got)
	}
	model := openaiProvider["models"].(map[string]any)["gpt-4o"].(map[string]any)
	if got := model["name"]; got != "GPT-4o" {
		t.Fatalf("model display name = %v, want GPT-4o", got)
	}
	anthropicProvider := providers["agx-relay-a-anthropic"].(map[string]any)
	if got := anthropicProvider["npm"]; got != "@ai-sdk/anthropic" {
		t.Fatalf("anthropic npm = %v, want @ai-sdk/anthropic", got)
	}
	geminiProvider := providers["agx-relay-a-gemini"].(map[string]any)
	if got := geminiProvider["npm"]; got != "@ai-sdk/google" {
		t.Fatalf("gemini npm = %v, want @ai-sdk/google", got)
	}
}

func TestSyncDefaultProviderFollowsModelHeuristic(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"))

	if _, err := syncer.Sync(profile("relay", "https://relay.example", "sk"), ports.OpenCodeSyncOptions{
		ModelID:      "claude-sonnet-4-5",
		SetAsCurrent: true,
	}); err != nil {
		t.Fatalf("Sync(claude) error = %v", err)
	}
	settings := readJSONConfig(t, configPath)
	if got := settings["model"]; got != "agx-relay-anthropic/claude-sonnet-4-5" {
		t.Fatalf("model = %v, want agx-relay-anthropic/claude-sonnet-4-5", got)
	}

	if _, err := syncer.Sync(profile("relay", "https://relay.example", "sk"), ports.OpenCodeSyncOptions{
		ModelID:      "gemini-2.5-pro",
		SetAsCurrent: true,
	}); err != nil {
		t.Fatalf("Sync(gemini) error = %v", err)
	}
	settings = readJSONConfig(t, configPath)
	if got := settings["model"]; got != "agx-relay-gemini/gemini-2.5-pro" {
		t.Fatalf("model = %v, want agx-relay-gemini/gemini-2.5-pro", got)
	}
}

func TestSyncCanAddNonCurrentCustomFamilyProvider(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"))

	if _, err := syncer.Sync(profile("relay-a", "https://relay-a.example/v1", "sk-a"), ports.OpenCodeSyncOptions{
		ModelID:      "model-a",
		SetAsCurrent: true,
	}); err != nil {
		t.Fatalf("Sync(current) error = %v", err)
	}
	if _, err := syncer.Sync(profile("relay-b", "https://relay-b.example/v1", "sk-b"), ports.OpenCodeSyncOptions{
		ModelID:      "claude-sonnet-4-5",
		SetAsCurrent: false,
	}); err != nil {
		t.Fatalf("Sync(non-current) error = %v", err)
	}

	settings := readJSONConfig(t, configPath)
	if got := settings["model"]; got != "agx-relay-a-openai-compatible/model-a" {
		t.Fatalf("model changed to %v, want agx-relay-a-openai-compatible/model-a", got)
	}
	providers := settings["provider"].(map[string]any)
	bProvider := providers["agx-relay-b-anthropic"].(map[string]any)
	if got := bProvider["npm"]; got != "@ai-sdk/anthropic" {
		t.Fatalf("relay-b anthropic npm = %v, want @ai-sdk/anthropic", got)
	}
	if _, ok := providers["agx-relay-b-openai-compatible"].(map[string]any); !ok {
		t.Fatalf("relay-b openai provider missing: %+v", providers)
	}
}

func TestRemoveProfileClearsMatchingTopLevelModel(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"))

	if _, err := syncer.Sync(profile("relay-a", "https://relay-a.example/v1", "sk-a"), ports.OpenCodeSyncOptions{
		ModelID:      "model-a",
		SetAsCurrent: true,
	}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if _, err := syncer.RemoveProfile("relay-a"); err != nil {
		t.Fatalf("RemoveProfile() error = %v", err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config path exists after removing only managed provider: %v", err)
	}
}

func TestOpenCodeHelpersAndLifecycleBranches(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"))

	raw := `// leading comment
	{
	  "$schema": "old",
	  "model": " agx-relay-a/gemini-2.5-pro ",
	  "provider": {
	    "agx-relay-a": {
	      "npm": "@ai-sdk/google",
	      "name": "Relay A",
	      "models": {
	        "gemini-2.5-pro": {
	          "name": "Gemini Pro"
	        }
	      },
	      "options": {
	        "baseURL": "https://relay.example/v1",
	        "apiKey": "sk-a"
	      }
	    },
	    "third-party": {
	      "name": "Other"
	    }
	  }
	}`
	if err := os.WriteFile(configPath, []byte(raw), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	status, err := syncer.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.DefaultModel != "agx-relay-a/gemini-2.5-pro" {
		t.Fatalf("DefaultModel = %q, want agx-relay-a/gemini-2.5-pro", status.DefaultModel)
	}
	provider, ok := status.ManagedProvidersByID["agx-relay-a"]
	if !ok {
		t.Fatalf("ManagedProvidersByID = %+v, want agx-relay-a", status.ManagedProvidersByID)
	}
	if provider.Family != domainprofile.OpenCodeProviderFamilyGemini || provider.Model != "gemini-2.5-pro" || provider.Name != "Relay A" {
		t.Fatalf("provider = %+v, want gemini relay", provider)
	}
	if _, ok := status.ManagedProvidersByID["third-party"]; ok {
		t.Fatalf("Status() kept non-managed provider: %+v", status.ManagedProvidersByID)
	}

	snapshot, err := syncer.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if !snapshot.Exists || string(snapshot.Content) != raw {
		t.Fatalf("Snapshot() = %+v, want original file content", snapshot)
	}

	backupPath, err := syncer.CreateBackup("backup-1", []byte("backup-data"))
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	if data, err := os.ReadFile(backupPath); err != nil || string(data) != "backup-data" {
		t.Fatalf("backup file = %q, %v", string(data), err)
	}
	if err := syncer.DeleteBackup(""); err != nil {
		t.Fatalf("DeleteBackup(empty) error = %v", err)
	}

	if got, want := providerBaseURL(domainprofile.OpenCodeProviderFamilyOpenAICompatible, "https://relay.example"), "https://relay.example/v1"; got != want {
		t.Fatalf("providerBaseURL(openai-compatible) = %q, want %q", got, want)
	}
	if got, want := providerBaseURL(domainprofile.OpenCodeProviderFamilyAnthropic, "https://relay.example/v1"), "https://relay.example/v1"; got != want {
		t.Fatalf("providerBaseURL(anthropic) = %q, want %q", got, want)
	}
	if got, want := providerBaseURL(domainprofile.OpenCodeProviderFamilyGemini, "https://relay.example/v1"), "https://relay.example"; got != want {
		t.Fatalf("providerBaseURL(gemini) = %q, want %q", got, want)
	}
	if got, want := providerBaseURL(domainprofile.OpenCodeProviderFamily("other"), "https://relay.example/v1/"), "https://relay.example/v1"; got != want {
		t.Fatalf("providerBaseURL(default) = %q, want %q", got, want)
	}
	if got, want := providerNPM(domainprofile.OpenCodeProviderFamilyGemini), "@ai-sdk/google"; got != want {
		t.Fatalf("providerNPM(gemini) = %q, want %q", got, want)
	}
	if got := providerNPM(domainprofile.OpenCodeProviderFamily("other")); got != "" {
		t.Fatalf("providerNPM(default) = %q, want empty", got)
	}
	if got, want := providerFamilyFromProvider(map[string]any{"npm": "@ai-sdk/openai"}), domainprofile.OpenCodeProviderFamilyOpenAICompatible; got != want {
		t.Fatalf("providerFamilyFromProvider(openai) = %q, want %q", got, want)
	}
	if got, want := providerFamilyFromProvider(map[string]any{"npm": "@ai-sdk/anthropic"}), domainprofile.OpenCodeProviderFamilyAnthropic; got != want {
		t.Fatalf("providerFamilyFromProvider(anthropic) = %q, want %q", got, want)
	}
	if got, want := providerModel(map[string]any{"models": map[string]any{"m1": map[string]any{"name": "Model 1"}}}); got != "m1" || want != "Model 1" {
		t.Fatalf("providerModel() = (%q,%q), want (m1,Model 1)", got, want)
	}
	if got := stringValue(nil, "name"); got != "" {
		t.Fatalf("stringValue(nil) = %q, want empty", got)
	}
	if !isEmptyManagedConfig(map[string]any{"$schema": schemaURL}) {
		t.Fatal("isEmptyManagedConfig(schema only) = false, want true")
	}
	if isEmptyManagedConfig(map[string]any{"provider": map[string]any{}}) {
		t.Fatal("isEmptyManagedConfig(provider present) = true, want false")
	}
	if parsed, err := parseJSONC([]byte(`// comment
		{"provider":{"agx-relay-a":{}}}`)); err != nil || len(parsed) != 1 {
		t.Fatalf("parseJSONC() = (%v,%v), want parsed config", parsed, err)
	}

	if _, err := syncer.ClearDefaultModel(); err != nil {
		t.Fatalf("ClearDefaultModel() error = %v", err)
	}
	cleared := readJSONConfig(t, configPath)
	if _, ok := cleared["model"]; ok {
		t.Fatalf("config after ClearDefaultModel still has model: %+v", cleared)
	}
	if _, ok := cleared["provider"].(map[string]any)["agx-relay-a"]; !ok {
		t.Fatalf("config after ClearDefaultModel lost provider: %+v", cleared)
	}

	if got, err := syncer.RemoveProfile(" "); err != nil || got != configPath {
		t.Fatalf("RemoveProfile(blank) = (%q,%v), want config path and nil", got, err)
	}
	if _, err := syncer.RemoveProfile("relay-a"); err != nil {
		t.Fatalf("RemoveProfile() error = %v", err)
	}
	afterRemove := readJSONConfig(t, configPath)
	if _, ok := afterRemove["provider"].(map[string]any)["agx-relay-a"]; ok {
		t.Fatalf("config after RemoveProfile still has managed provider: %+v", afterRemove)
	}
	if _, ok := afterRemove["provider"].(map[string]any)["third-party"]; !ok {
		t.Fatalf("config after RemoveProfile lost third-party provider: %+v", afterRemove)
	}
	if _, err := syncer.RemoveConfig(); err != nil {
		t.Fatalf("RemoveConfig() error = %v", err)
	}
	if err := syncer.DeleteBackup(backupPath); err != nil {
		t.Fatalf("DeleteBackup() error = %v", err)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("backup exists after DeleteBackup: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "restore.bak"), []byte(`{"restored":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile(restore.bak) error = %v", err)
	}
	restorePath := filepath.Join(dir, "restore.bak")
	if _, err := syncer.Restore(restorePath); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if data, err := os.ReadFile(configPath); err != nil || string(data) != `{"restored":true}` {
		t.Fatalf("restored config = %q, %v", string(data), err)
	}
}

func TestSyncerErrorAndCleanupBranches(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"))

	if _, err := syncer.Sync(profile("relay-a", "https://relay.example", "sk-a"), ports.OpenCodeSyncOptions{}); err == nil {
		t.Fatal("Sync(missing model) unexpectedly succeeded")
	}

	snapshot, err := syncer.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot(missing) error = %v", err)
	}
	if snapshot.Exists {
		t.Fatalf("Snapshot(missing).Exists = true, want false")
	}
	if got, err := syncer.ClearDefaultModel(); err != nil || got != configPath {
		t.Fatalf("ClearDefaultModel(missing) = (%q,%v), want config path and nil", got, err)
	}

	if err := os.WriteFile(configPath, []byte(`{"$schema":"https://opencode.ai/config.json","model":"agx-relay-a/gpt-4o"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(schema-only) error = %v", err)
	}
	if _, err := syncer.ClearDefaultModel(); err != nil {
		t.Fatalf("ClearDefaultModel(schema-only) error = %v", err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("schema-only config still exists after ClearDefaultModel: %v", err)
	}

	if err := os.WriteFile(configPath, []byte(`{"provider":`), 0o600); err != nil {
		t.Fatalf("WriteFile(invalid) error = %v", err)
	}
	if _, err := syncer.Status(); err == nil {
		t.Fatal("Status(invalid config) unexpectedly succeeded")
	}
	if _, err := parseJSONC([]byte(`{"provider":`)); err == nil {
		t.Fatal("parseJSONC(invalid) unexpectedly succeeded")
	}
	if parsed := string(stripJSONComments([]byte(`{"url":"https://example.test//v1"} // drop
/* keep newline */`))); !strings.Contains(parsed, `https://example.test//v1`) || strings.Contains(parsed, "drop") {
		t.Fatalf("stripJSONComments() = %q, want comments removed outside strings", parsed)
	}
	if modelID, modelName := providerModel(map[string]any{"models": map[string]any{"bad": "shape"}}); modelID != "" || modelName != "" {
		t.Fatalf("providerModel(invalid shape) = (%q,%q), want empty", modelID, modelName)
	}

	if _, err := syncer.Restore(filepath.Join(dir, "missing.bak")); err == nil {
		t.Fatal("Restore(missing backup) unexpectedly succeeded")
	}
	if got, err := syncer.RemoveConfig(); err != nil || got != configPath {
		t.Fatalf("RemoveConfig() = (%q,%v), want config path and nil", got, err)
	}

	blockedBackupRoot := filepath.Join(dir, "blocked-backups")
	if err := os.WriteFile(blockedBackupRoot, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocked backup root) error = %v", err)
	}
	blockedBackupSyncer := NewSyncer(filepath.Join(dir, "blocked-opencode.json"), blockedBackupRoot)
	if _, err := blockedBackupSyncer.CreateBackup("backup-1", []byte("backup")); err == nil {
		t.Fatal("CreateBackup(blocked backup root) unexpectedly succeeded")
	}

	blockedConfigPath := filepath.Join(dir, "blocked-config")
	if err := os.Mkdir(blockedConfigPath, 0o700); err != nil {
		t.Fatalf("Mkdir(blocked config) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(blockedConfigPath, "child"), []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocked config child) error = %v", err)
	}
	blockedConfigSyncer := NewSyncer(blockedConfigPath, filepath.Join(dir, "unused-backups"))
	if _, err := blockedConfigSyncer.RemoveConfig(); err == nil {
		t.Fatal("RemoveConfig(non-empty directory) unexpectedly succeeded")
	}
	restorePath := filepath.Join(dir, "restore-source.bak")
	if err := os.WriteFile(restorePath, []byte("restored"), 0o600); err != nil {
		t.Fatalf("WriteFile(restore source) error = %v", err)
	}
	if _, err := blockedConfigSyncer.Restore(restorePath); err == nil {
		t.Fatal("Restore(to directory path) unexpectedly succeeded")
	}
}

func profile(name, baseURL, apiKey string) domainprofile.Profile {
	now := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	return domainprofile.Profile{
		Name:      name,
		BaseURL:   baseURL,
		APIKey:    apiKey,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func readJSONConfig(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return settings
}
