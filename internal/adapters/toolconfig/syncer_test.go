package toolconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kiddingbaby/agx/internal/config"
	domainagent "github.com/kiddingbaby/agx/internal/domain/agent"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
)

func testPaths(t *testing.T) config.Paths {
	t.Helper()
	home := t.TempDir()
	return config.Paths{
		ConfigDir:          filepath.Join(home, ".config", "agx"),
		ClaudeDir:          filepath.Join(home, ".claude"),
		ClaudeSettingsPath: filepath.Join(home, ".claude", "settings.json"),
		CodexDir:           filepath.Join(home, ".codex"),
		CodexAuthPath:      filepath.Join(home, ".codex", "auth.json"),
		CodexConfigPath:    filepath.Join(home, ".codex", "config.toml"),
		GeminiDir:          filepath.Join(home, ".gemini"),
		GeminiEnvPath:      filepath.Join(home, ".gemini", ".env"),
		GeminiSettingsPath: filepath.Join(home, ".gemini", "settings.json"),
	}
}

func TestSyncerApplyCodex(t *testing.T) {
	paths := testPaths(t)
	if err := os.MkdirAll(paths.CodexDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	initialConfig := `model = "gpt-5"
[projects."/tmp"]
model = "project-model"
trust_level = "trusted"
`
	if err := os.WriteFile(paths.CodexConfigPath, []byte(initialConfig), 0600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	syncer := NewSyncer(paths)
	err := syncer.Apply(
		domainagent.Agent{Name: "codex-cli"},
		domainkey.Key{APIKey: "sk-test"},
		domainprovider.Target{
			Name:    "openrouter",
			Family:  domainprovider.FamilyOpenAI,
			Kind:    domainprovider.KindOpenAICompatible,
			Access:  domainprovider.AccessThirdParty,
			Auth:    domainprovider.AuthAPIKey,
			BaseURL: "https://openrouter.ai/api/v1",
		},
	)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	authData, err := os.ReadFile(paths.CodexAuthPath)
	if err != nil {
		t.Fatalf("ReadFile(auth) error = %v", err)
	}
	authText := string(authData)
	if !strings.Contains(authText, `"auth_mode": "apikey"`) {
		t.Fatalf("auth.json missing auth_mode: %s", authText)
	}
	if !strings.Contains(authText, `"OPENAI_API_KEY": "sk-test"`) {
		t.Fatalf("auth.json missing key: %s", authText)
	}

	configData, err := os.ReadFile(paths.CodexConfigPath)
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	configText := string(configData)
	if !strings.Contains(configText, `model_provider = "agx"`) {
		t.Fatalf("config.toml missing model_provider: %s", configText)
	}
	if !strings.Contains(configText, `preferred_auth_method = "apikey"`) {
		t.Fatalf("config.toml missing preferred_auth_method: %s", configText)
	}
	if strings.Contains(configText, `env_key =`) {
		t.Fatalf("config.toml should not require env_key for auth.json-based apikey: %s", configText)
	}
	if !strings.Contains(configText, `base_url = "https://openrouter.ai/api/v1"`) {
		t.Fatalf("config.toml missing base_url: %s", configText)
	}
	if !strings.Contains(configText, `[projects."/tmp"]`) {
		t.Fatalf("config.toml should preserve existing content: %s", configText)
	}
	if !strings.Contains(configText, `model = "project-model"`) {
		t.Fatalf("config.toml should keep subtable model key: %s", configText)
	}
	if strings.Contains(configText, `model = "gpt-5"`) {
		t.Fatalf("config.toml should remove root model when target model is empty: %s", configText)
	}
}

func TestSyncerApplyCodexModel(t *testing.T) {
	paths := testPaths(t)
	if err := os.MkdirAll(paths.CodexDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	initialConfig := `model = "gpt-5"
[projects."/tmp"]
model = "project-model"
trust_level = "trusted"
`
	if err := os.WriteFile(paths.CodexConfigPath, []byte(initialConfig), 0600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	syncer := NewSyncer(paths)
	err := syncer.Apply(
		domainagent.Agent{Name: "codex-cli"},
		domainkey.Key{APIKey: "sk-test"},
		domainprovider.Target{
			Name:    "openrouter",
			Family:  domainprovider.FamilyOpenAI,
			Kind:    domainprovider.KindOpenAICompatible,
			Access:  domainprovider.AccessThirdParty,
			Auth:    domainprovider.AuthAPIKey,
			BaseURL: "https://openrouter.ai/api/v1",
			Model:   "gpt-4.1",
		},
	)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	configData, err := os.ReadFile(paths.CodexConfigPath)
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	configText := string(configData)
	if !strings.Contains(configText, `model = "gpt-4.1"`) {
		t.Fatalf("config.toml missing model override: %s", configText)
	}
	if !strings.Contains(configText, `model = "project-model"`) {
		t.Fatalf("config.toml should keep subtable model key: %s", configText)
	}
}

func TestSyncerApplyClaude(t *testing.T) {
	paths := testPaths(t)
	if err := os.MkdirAll(paths.ClaudeDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	initial := `{"model":"opus","env":{"ANTHROPIC_AUTH_TOKEN":"old-token"}}`
	if err := os.WriteFile(paths.ClaudeSettingsPath, []byte(initial), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	syncer := NewSyncer(paths)
	err := syncer.Apply(
		domainagent.Agent{Name: "claude-code"},
		domainkey.Key{APIKey: "claude-key"},
		domainprovider.Target{
			Name:    "claude-proxy",
			Family:  domainprovider.FamilyClaude,
			Kind:    domainprovider.KindClaude,
			Access:  domainprovider.AccessThirdParty,
			Auth:    domainprovider.AuthAPIKey,
			BaseURL: "https://claude-proxy.local",
		},
	)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	data, err := os.ReadFile(paths.ClaudeSettingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"ANTHROPIC_API_KEY": "claude-key"`) {
		t.Fatalf("settings.json missing api key: %s", text)
	}
	if !strings.Contains(text, `"CLAUDE_API_KEY": "claude-key"`) {
		t.Fatalf("settings.json missing alias key: %s", text)
	}
	if !strings.Contains(text, `"ANTHROPIC_BASE_URL": "https://claude-proxy.local"`) {
		t.Fatalf("settings.json missing base url: %s", text)
	}
	if strings.Contains(text, "ANTHROPIC_AUTH_TOKEN") {
		t.Fatalf("settings.json should remove auth token: %s", text)
	}
}

func TestSyncerApplyClaudeModelAndEnv(t *testing.T) {
	paths := testPaths(t)
	if err := os.MkdirAll(paths.ClaudeDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	initial := `{"model":"opus","env":{"ANTHROPIC_AUTH_TOKEN":"old-token"}}`
	if err := os.WriteFile(paths.ClaudeSettingsPath, []byte(initial), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	syncer := NewSyncer(paths)
	err := syncer.Apply(
		domainagent.Agent{Name: "claude-code"},
		domainkey.Key{APIKey: "claude-key"},
		domainprovider.Target{
			Name:    "claude-proxy",
			Family:  domainprovider.FamilyClaude,
			Kind:    domainprovider.KindClaude,
			Access:  domainprovider.AccessThirdParty,
			Auth:    domainprovider.AuthAPIKey,
			BaseURL: "https://claude-proxy.local",
			Model:   "sonnet",
			Env: map[string]string{
				"FOO": "bar",
			},
		},
	)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	data, err := os.ReadFile(paths.ClaudeSettingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"model": "sonnet"`) {
		t.Fatalf("settings.json missing model override: %s", text)
	}
	if !strings.Contains(text, `"FOO": "bar"`) {
		t.Fatalf("settings.json missing env override: %s", text)
	}
}

func TestSyncerApplyClaudeRemovesManagedEnvKeysWhenUpdated(t *testing.T) {
	paths := testPaths(t)
	if err := os.MkdirAll(paths.ClaudeDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	initial := `{"env":{"KEEP":"yes"}}`
	if err := os.WriteFile(paths.ClaudeSettingsPath, []byte(initial), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	syncer := NewSyncer(paths)
	if err := syncer.Apply(
		domainagent.Agent{Name: "claude-code"},
		domainkey.Key{APIKey: "claude-key"},
		domainprovider.Target{
			Name:    "claude-proxy",
			Family:  domainprovider.FamilyClaude,
			Kind:    domainprovider.KindClaude,
			Access:  domainprovider.AccessThirdParty,
			Auth:    domainprovider.AuthAPIKey,
			BaseURL: "https://claude-proxy.local",
			Env: map[string]string{
				"FOO": "bar",
				"BAR": "baz",
			},
		},
	); err != nil {
		t.Fatalf("Apply(v1) error = %v", err)
	}

	if err := syncer.Apply(
		domainagent.Agent{Name: "claude-code"},
		domainkey.Key{APIKey: "claude-key"},
		domainprovider.Target{
			Name:    "claude-proxy",
			Family:  domainprovider.FamilyClaude,
			Kind:    domainprovider.KindClaude,
			Access:  domainprovider.AccessThirdParty,
			Auth:    domainprovider.AuthAPIKey,
			BaseURL: "https://claude-proxy.local",
			Env: map[string]string{
				"FOO": "bar",
			},
		},
	); err != nil {
		t.Fatalf("Apply(v2) error = %v", err)
	}

	data, err := os.ReadFile(paths.ClaudeSettingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"KEEP": "yes"`) {
		t.Fatalf("settings.json should preserve existing env keys: %s", text)
	}
	if !strings.Contains(text, `"FOO": "bar"`) {
		t.Fatalf("settings.json missing env override: %s", text)
	}
	if strings.Contains(text, `"BAR": "baz"`) {
		t.Fatalf("settings.json should remove managed env keys not in target.Env: %s", text)
	}
}

func TestSyncerApplyGemini(t *testing.T) {
	paths := testPaths(t)
	if err := os.MkdirAll(paths.GeminiDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	initialSettings := `{"ui":{"theme":"ANSI"}}`
	if err := os.WriteFile(paths.GeminiSettingsPath, []byte(initialSettings), 0600); err != nil {
		t.Fatalf("WriteFile(settings) error = %v", err)
	}

	syncer := NewSyncer(paths)
	err := syncer.Apply(
		domainagent.Agent{Name: "gemini-cli"},
		domainkey.Key{APIKey: "gemini-key"},
		domainprovider.Target{
			Name:    "gemini-proxy",
			Family:  domainprovider.FamilyGemini,
			Kind:    domainprovider.KindGemini,
			Access:  domainprovider.AccessThirdParty,
			Auth:    domainprovider.AuthAPIKey,
			BaseURL: "https://gemini-proxy.local",
		},
	)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	settingsData, err := os.ReadFile(paths.GeminiSettingsPath)
	if err != nil {
		t.Fatalf("ReadFile(settings) error = %v", err)
	}
	settingsText := string(settingsData)
	if !strings.Contains(settingsText, `"selectedType": "gemini-api-key"`) {
		t.Fatalf("settings.json missing selectedType: %s", settingsText)
	}
	if !strings.Contains(settingsText, `"theme": "ANSI"`) {
		t.Fatalf("settings.json should preserve existing fields: %s", settingsText)
	}

	envData, err := os.ReadFile(paths.GeminiEnvPath)
	if err != nil {
		t.Fatalf("ReadFile(.env) error = %v", err)
	}
	envText := string(envData)
	if !strings.Contains(envText, "GEMINI_API_KEY=gemini-key") {
		t.Fatalf(".env missing api key: %s", envText)
	}
	if !strings.Contains(envText, "GOOGLE_GEMINI_BASE_URL=https://gemini-proxy.local") {
		t.Fatalf(".env missing base url: %s", envText)
	}
}

func TestSyncerApplyGeminiModelAndEnv(t *testing.T) {
	paths := testPaths(t)
	if err := os.MkdirAll(paths.GeminiDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	initialSettings := `{"ui":{"theme":"ANSI"}}`
	if err := os.WriteFile(paths.GeminiSettingsPath, []byte(initialSettings), 0600); err != nil {
		t.Fatalf("WriteFile(settings) error = %v", err)
	}

	syncer := NewSyncer(paths)
	err := syncer.Apply(
		domainagent.Agent{Name: "gemini-cli"},
		domainkey.Key{APIKey: "gemini-key"},
		domainprovider.Target{
			Name:    "gemini-proxy",
			Family:  domainprovider.FamilyGemini,
			Kind:    domainprovider.KindGemini,
			Access:  domainprovider.AccessThirdParty,
			Auth:    domainprovider.AuthAPIKey,
			BaseURL: "https://gemini-proxy.local",
			Model:   "gemini-2.0-pro",
			Env: map[string]string{
				"FOO": "bar",
			},
		},
	)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	settingsData, err := os.ReadFile(paths.GeminiSettingsPath)
	if err != nil {
		t.Fatalf("ReadFile(settings) error = %v", err)
	}
	if !strings.Contains(string(settingsData), `"model": "gemini-2.0-pro"`) {
		t.Fatalf("settings.json missing model override: %s", string(settingsData))
	}

	envData, err := os.ReadFile(paths.GeminiEnvPath)
	if err != nil {
		t.Fatalf("ReadFile(.env) error = %v", err)
	}
	if !strings.Contains(string(envData), "FOO=bar") {
		t.Fatalf(".env missing env override: %s", string(envData))
	}
}

func TestSyncerApplyGeminiDoesNotDuplicateManagedBlockOnCRLF(t *testing.T) {
	paths := testPaths(t)
	if err := os.MkdirAll(paths.GeminiDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	initialSettings := `{"ui":{"theme":"ANSI"}}`
	if err := os.WriteFile(paths.GeminiSettingsPath, []byte(initialSettings), 0600); err != nil {
		t.Fatalf("WriteFile(settings) error = %v", err)
	}

	syncer := NewSyncer(paths)
	target := domainprovider.Target{
		Name:    "gemini-proxy",
		Family:  domainprovider.FamilyGemini,
		Kind:    domainprovider.KindGemini,
		Access:  domainprovider.AccessThirdParty,
		Auth:    domainprovider.AuthAPIKey,
		BaseURL: "https://gemini-proxy.local",
	}
	if err := syncer.Apply(domainagent.Agent{Name: "gemini-cli"}, domainkey.Key{APIKey: "gemini-key"}, target); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	envData, err := os.ReadFile(paths.GeminiEnvPath)
	if err != nil {
		t.Fatalf("ReadFile(.env) error = %v", err)
	}
	crlf := strings.ReplaceAll(string(envData), "\n", "\r\n")
	if err := os.WriteFile(paths.GeminiEnvPath, []byte(crlf), 0600); err != nil {
		t.Fatalf("WriteFile(.env) error = %v", err)
	}

	if err := syncer.Apply(domainagent.Agent{Name: "gemini-cli"}, domainkey.Key{APIKey: "gemini-key"}, target); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	envData, err = os.ReadFile(paths.GeminiEnvPath)
	if err != nil {
		t.Fatalf("ReadFile(.env) error = %v", err)
	}
	if got := strings.Count(string(envData), managedBlockStartLine); got != 1 {
		t.Fatalf("managed block count = %d, want 1: %s", got, string(envData))
	}
}
