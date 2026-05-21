package codexconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func TestSyncWritesProfileModelIntoCodexConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"), "agx")
	now := time.Now().UTC()

	if _, err := syncer.Sync(domainprofile.Profile{
		Name:      "tmp",
		BaseURL:   "https://relay.example/v1",
		APIKey:    "sk-tmp",
		ModelID:   "bbg/kimi-k2",
		CreatedAt: now,
		UpdatedAt: now,
	}, ports.CodexSyncOptions{DefaultProfileName: "tmp"}); err != nil {
		t.Fatalf("Sync error = %v", err)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	body := string(got)
	if !strings.Contains(body, `[profiles."agx/tmp"]`) {
		t.Fatalf("config missing profile section: %s", body)
	}
	if !strings.Contains(body, `model = "bbg/kimi-k2"`) {
		t.Fatalf("config missing model line for tmp: %s", body)
	}
}

func TestSyncReplacesStaleModelLineWhenProfileModelChanges(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	original := `# >>> AGX managed Codex config >>>
[model_providers]

[profiles]

[model_providers."agx/tmp"]
name = "tmp"
base_url = "https://relay.example/v1"
wire_api = "responses"

[model_providers."agx/tmp".auth]
command = "agx"
args = ["__api-key", "tmp"]

[profiles."agx/tmp"]
model_provider = "agx/tmp"
model = "old-model"
model_reasoning_effort = "high"
# <<< AGX managed Codex config <<<
`
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"), "agx")
	now := time.Now().UTC()
	if _, err := syncer.Sync(domainprofile.Profile{
		Name:      "tmp",
		BaseURL:   "https://relay.example/v1",
		APIKey:    "sk-tmp",
		ModelID:   "new-model",
		CreatedAt: now,
		UpdatedAt: now,
	}, ports.CodexSyncOptions{}); err != nil {
		t.Fatalf("Sync error = %v", err)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	body := string(got)
	if strings.Contains(body, `"old-model"`) {
		t.Fatalf("stale model line should be removed: %s", body)
	}
	if !strings.Contains(body, `model = "new-model"`) {
		t.Fatalf("config missing new model: %s", body)
	}
	if !strings.Contains(body, `model_reasoning_effort = "high"`) {
		t.Fatalf("config should preserve non-model extras: %s", body)
	}
}

func TestSyncPreservesExistingModelWhenProfileModelEmpty(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	original := `# >>> AGX managed Codex config >>>
[model_providers]

[profiles]

[model_providers."agx/tmp"]
name = "tmp"
base_url = "https://relay.example/v1"
wire_api = "responses"

[model_providers."agx/tmp".auth]
command = "agx"
args = ["__api-key", "tmp"]

[profiles."agx/tmp"]
model_provider = "agx/tmp"
model = "carry-over-model"
# <<< AGX managed Codex config <<<
`
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	syncer := NewSyncer(configPath, filepath.Join(dir, "backups"), "agx")
	now := time.Now().UTC()
	if _, err := syncer.Sync(domainprofile.Profile{
		Name:      "tmp",
		BaseURL:   "https://relay.example/v1",
		APIKey:    "sk-tmp",
		ModelID:   "",
		CreatedAt: now,
		UpdatedAt: now,
	}, ports.CodexSyncOptions{}); err != nil {
		t.Fatalf("Sync error = %v", err)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	body := string(got)
	if !strings.Contains(body, `model = "carry-over-model"`) {
		t.Fatalf("empty ModelID should preserve existing model line: %s", body)
	}
}
