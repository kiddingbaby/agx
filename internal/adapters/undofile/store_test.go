package undofile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kiddingbaby/agx/internal/config"
	domainagent "github.com/kiddingbaby/agx/internal/domain/agent"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
	"github.com/kiddingbaby/agx/internal/ports"
)

func TestStoreCaptureAndRestore(t *testing.T) {
	dir := t.TempDir()
	paths := config.Paths{
		ConfigDir:          filepath.Join(dir, "agx"),
		StorePath:          filepath.Join(dir, "agx", "keys.yaml"),
		ProviderConfigPath: filepath.Join(dir, "agx", "providers.yaml"),
		CodexAuthPath:      filepath.Join(dir, "home", ".codex", "auth.json"),
		CodexConfigPath:    filepath.Join(dir, "home", ".codex", "config.toml"),
		ClaudeSettingsPath: filepath.Join(dir, "home", ".claude", "settings.json"),
		GeminiEnvPath:      filepath.Join(dir, "home", ".gemini", ".env"),
		GeminiSettingsPath: filepath.Join(dir, "home", ".gemini", "settings.json"),
	}

	keysV0 := "keys: []\nprofiles: []\n"
	providersV0 := "targets: []\nbindings: []\n"
	authV0 := "{\n  \"OPENAI_API_KEY\": \"old\"\n}\n"
	if err := os.MkdirAll(filepath.Dir(paths.StorePath), 0700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(paths.StorePath, []byte(keysV0), 0600); err != nil {
		t.Fatalf("write keys.yaml: %v", err)
	}
	if err := os.WriteFile(paths.ProviderConfigPath, []byte(providersV0), 0600); err != nil {
		t.Fatalf("write providers.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.CodexAuthPath), 0700); err != nil {
		t.Fatalf("mkdir codex dir: %v", err)
	}
	if err := os.WriteFile(paths.CodexAuthPath, []byte(authV0), 0600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}
	// Intentionally do not create config.toml to cover "delete on restore".

	store := NewStore(paths)
	agent, ok := domainagent.Find("codex-cli")
	if !ok {
		t.Fatalf("agent codex-cli missing")
	}
	meta := ports.UndoCaptureMeta{
		Command: "switch",
		Agent:   agent,
		Target: domainprovider.Target{
			Name:   "openrouter",
			Family: domainprovider.FamilyOpenAI,
		},
	}
	id, err := store.Capture(meta)
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}
	if id == "" {
		t.Fatalf("Capture() returned empty id")
	}

	latest, err := store.LatestID()
	if err != nil {
		t.Fatalf("LatestID() error = %v", err)
	}
	if latest != id {
		t.Fatalf("LatestID() = %q, want %q", latest, id)
	}

	// Mutate files after capture.
	if err := os.WriteFile(paths.StorePath, []byte("changed\n"), 0600); err != nil {
		t.Fatalf("mutate keys.yaml: %v", err)
	}
	if err := os.WriteFile(paths.ProviderConfigPath, []byte("changed\n"), 0600); err != nil {
		t.Fatalf("mutate providers.yaml: %v", err)
	}
	if err := os.WriteFile(paths.CodexAuthPath, []byte("{\"OPENAI_API_KEY\":\"new\"}\n"), 0600); err != nil {
		t.Fatalf("mutate auth.json: %v", err)
	}
	if err := os.WriteFile(paths.CodexConfigPath, []byte("model = \"x\"\n"), 0600); err != nil {
		t.Fatalf("create config.toml: %v", err)
	}

	res, err := store.Restore(id)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if res.ID != id {
		t.Fatalf("Restore().ID = %q, want %q", res.ID, id)
	}

	gotKeys, _ := os.ReadFile(paths.StorePath)
	if string(gotKeys) != keysV0 {
		t.Fatalf("keys.yaml restored mismatch: got=%q want=%q", string(gotKeys), keysV0)
	}
	gotProviders, _ := os.ReadFile(paths.ProviderConfigPath)
	if string(gotProviders) != providersV0 {
		t.Fatalf("providers.yaml restored mismatch: got=%q want=%q", string(gotProviders), providersV0)
	}
	gotAuth, _ := os.ReadFile(paths.CodexAuthPath)
	if string(gotAuth) != authV0 {
		t.Fatalf("auth.json restored mismatch: got=%q want=%q", string(gotAuth), authV0)
	}
	if _, err := os.Stat(paths.CodexConfigPath); !os.IsNotExist(err) {
		t.Fatalf("config.toml should be deleted on restore, stat err=%v", err)
	}
}
