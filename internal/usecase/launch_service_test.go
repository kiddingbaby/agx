package usecase

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainsession "github.com/kiddingbaby/agx/internal/domain/session"
)

type recordingRuntime struct {
	lastConfig domainsession.SessionConfig
	launchErr  error
	launched   bool
}

func (r *recordingRuntime) Launch(cfg domainsession.SessionConfig) error {
	r.lastConfig = cfg
	r.launched = true
	return r.launchErr
}

func (r *recordingRuntime) Attach(sessionName string) error {
	return nil
}

func (r *recordingRuntime) ListSessions() ([]domainsession.SessionInfo, error) {
	return nil, nil
}

func (r *recordingRuntime) KillSession(name string) error {
	return nil
}

func TestLaunchServiceBuildSessionConfig(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("CLAUDE_API_KEY", "")
	t.Setenv("ANTHROPIC_BASE_URL", "")

	repo := &fakeKeyRepo{
		keys: []domainkey.Key{
			{
				ID:       "k1",
				Provider: domainkey.ProviderClaude,
				Profile:  domainkey.DefaultProfile,
				Name:     "claude-main",
				APIKey:   "plain-key",
				BaseURL:  "https://api.proxy.local",
				Active:   true,
			},
		},
	}
	keySvc := NewKeyService(repo)
	rt := &recordingRuntime{}
	launchSvc := NewLaunchService(keySvc, rt)

	cfg, err := launchSvc.BuildSessionConfig("claude", "/tmp", "-c")
	if err != nil {
		t.Fatalf("BuildSessionConfig() error = %v", err)
	}
	if cfg.Agent != "ai-claude-code" {
		t.Fatalf("cfg.Agent = %s, want ai-claude-code", cfg.Agent)
	}
	if cfg.Command != "claude -c" {
		t.Fatalf("cfg.Command = %s, want 'claude -c'", cfg.Command)
	}
	if cfg.EnvVars["ANTHROPIC_API_KEY"] != "plain-key" {
		t.Fatalf("missing api key env in cfg: %+v", cfg.EnvVars)
	}
	if cfg.EnvVars["CLAUDE_API_KEY"] != "plain-key" {
		t.Fatalf("missing alias api key env in cfg: %+v", cfg.EnvVars)
	}
	if cfg.EnvVars["ANTHROPIC_BASE_URL"] != "https://api.proxy.local" {
		t.Fatalf("missing base url env in cfg: %+v", cfg.EnvVars)
	}
}

func TestLaunchServiceBuildSessionConfigWithOptions(t *testing.T) {
	repo := &fakeKeyRepo{
		keys: []domainkey.Key{
			{
				ID:       "k1",
				Provider: domainkey.ProviderClaude,
				Profile:  "prod",
				Name:     "claude-prod",
				APIKey:   "prod-key",
				Active:   true,
			},
		},
	}
	keySvc := NewKeyService(repo)
	rt := &recordingRuntime{}
	launchSvc := NewLaunchService(keySvc, rt)

	cfg, err := launchSvc.BuildSessionConfigWithOptions("claude", "/tmp", LaunchOptions{
		Profile:       "prod",
		KeyIdentifier: "claude-prod",
		ExtraArgs:     "--continue",
	})
	if err != nil {
		t.Fatalf("BuildSessionConfigWithOptions() error = %v", err)
	}
	if cfg.Command != "claude --continue" {
		t.Fatalf("cfg.Command = %s, want 'claude --continue'", cfg.Command)
	}
	if cfg.EnvVars["ANTHROPIC_API_KEY"] != "prod-key" {
		t.Fatalf("cfg.EnvVars ANTHROPIC_API_KEY = %q, want prod-key", cfg.EnvVars["ANTHROPIC_API_KEY"])
	}
}

func TestLaunchServiceEnvOverridesStore(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "env-key")
	t.Setenv("ANTHROPIC_BASE_URL", "https://env-base.local")

	keySvc := NewKeyService(&fakeKeyRepo{})
	rt := &recordingRuntime{}
	launchSvc := NewLaunchService(keySvc, rt)

	cfg, err := launchSvc.BuildSessionConfig("claude", "/tmp", "")
	if err != nil {
		t.Fatalf("BuildSessionConfig() error = %v", err)
	}
	if cfg.EnvVars["ANTHROPIC_API_KEY"] != "env-key" {
		t.Fatalf("cfg.EnvVars[ANTHROPIC_API_KEY] = %q, want env-key", cfg.EnvVars["ANTHROPIC_API_KEY"])
	}
	if cfg.EnvVars["ANTHROPIC_BASE_URL"] != "https://env-base.local" {
		t.Fatalf("cfg.EnvVars[ANTHROPIC_BASE_URL] = %q, want env base", cfg.EnvVars["ANTHROPIC_BASE_URL"])
	}
}

func TestLaunchServicePrefersStoreOverEnvWhenStoreHasKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "env-key")
	t.Setenv("ANTHROPIC_BASE_URL", "https://env-base.local")

	repo := &fakeKeyRepo{
		keys: []domainkey.Key{
			{
				ID:       "k-claude-1",
				Provider: domainkey.ProviderClaude,
				Profile:  domainkey.DefaultProfile,
				Name:     "claude-store",
				APIKey:   "store-key",
				BaseURL:  "https://store-base.local",
				Active:   true,
			},
		},
	}
	keySvc := NewKeyService(repo)
	launchSvc := NewLaunchService(keySvc, &recordingRuntime{})

	cfg, err := launchSvc.BuildSessionConfig("claude", "/tmp", "")
	if err != nil {
		t.Fatalf("BuildSessionConfig(claude) error = %v", err)
	}
	if cfg.EnvVars["ANTHROPIC_API_KEY"] != "store-key" {
		t.Fatalf("cfg.EnvVars[ANTHROPIC_API_KEY] = %q, want store-key", cfg.EnvVars["ANTHROPIC_API_KEY"])
	}
	if cfg.EnvVars["ANTHROPIC_BASE_URL"] != "https://store-base.local" {
		t.Fatalf("cfg.EnvVars[ANTHROPIC_BASE_URL] = %q, want store-base.local", cfg.EnvVars["ANTHROPIC_BASE_URL"])
	}
}

func TestLaunchServiceCodexUsesSingleLatestBaseURLEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("HOME", t.TempDir())

	repo := &fakeKeyRepo{
		keys: []domainkey.Key{
			{
				ID:       "k-openai-1",
				Provider: domainkey.ProviderOpenAI,
				Profile:  domainkey.DefaultProfile,
				Name:     "openai-main",
				APIKey:   "openai-key",
				BaseURL:  "https://example-openai.local/v1",
				Active:   true,
			},
		},
	}
	keySvc := NewKeyService(repo)
	rt := &recordingRuntime{}
	launchSvc := NewLaunchService(keySvc, rt)

	cfg, err := launchSvc.BuildSessionConfig("codex", "/tmp", "")
	if err != nil {
		t.Fatalf("BuildSessionConfig(codex) error = %v", err)
	}
	if cfg.EnvVars["OPENAI_API_KEY"] != "openai-key" {
		t.Fatalf("cfg.EnvVars[OPENAI_API_KEY] = %q, want openai-key", cfg.EnvVars["OPENAI_API_KEY"])
	}
	if cfg.EnvVars["OPENAI_BASE_URL"] != "https://example-openai.local/v1" {
		t.Fatalf("cfg.EnvVars[OPENAI_BASE_URL] = %q, want base url", cfg.EnvVars["OPENAI_BASE_URL"])
	}
	if _, exists := cfg.EnvVars["OPENAI_API_BASE"]; exists {
		t.Fatalf("cfg.EnvVars should not include deprecated OPENAI_API_BASE: %+v", cfg.EnvVars)
	}
}

func TestLaunchServiceCodexUsesEnvKeyFromConfigToml(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")

	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("mkdir .codex: %v", err)
	}
	config := `model_provider = "custom"

[model_providers.custom]
name = "custom"
base_url = "https://proxy.example/v1"
env_key = "MY_PROVIDER_KEY"
wire_api = "responses"
requires_openai_auth = true
`
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatalf("write codex config: %v", err)
	}

	repo := &fakeKeyRepo{
		keys: []domainkey.Key{
			{
				ID:       "k-openai-2",
				Provider: domainkey.ProviderOpenAI,
				Profile:  domainkey.DefaultProfile,
				Name:     "openai-custom-env-key",
				APIKey:   "openai-key-2",
				BaseURL:  "https://example-openai.local/v1",
				Active:   true,
			},
		},
	}
	keySvc := NewKeyService(repo)
	launchSvc := NewLaunchService(keySvc, &recordingRuntime{})

	cfg, err := launchSvc.BuildSessionConfig("codex", "/tmp", "")
	if err != nil {
		t.Fatalf("BuildSessionConfig(codex) error = %v", err)
	}
	if cfg.EnvVars["MY_PROVIDER_KEY"] != "openai-key-2" {
		t.Fatalf("cfg.EnvVars[MY_PROVIDER_KEY] = %q, want openai-key-2", cfg.EnvVars["MY_PROVIDER_KEY"])
	}
	if _, exists := cfg.EnvVars["OPENAI_API_KEY"]; exists {
		t.Fatalf("cfg.EnvVars should not include OPENAI_API_KEY when env_key is custom: %+v", cfg.EnvVars)
	}
}

func TestLaunchServiceErrors(t *testing.T) {
	keySvc := NewKeyService(&fakeKeyRepo{})
	rt := &recordingRuntime{}
	launchSvc := NewLaunchService(keySvc, rt)

	if _, err := launchSvc.BuildSessionConfig("not-exists", "/tmp", ""); !IsUnknownAgentError(err) {
		t.Fatalf("unknown agent err = %v", err)
	}

	if _, err := launchSvc.BuildSessionConfig("claude", "/tmp", ""); !IsNoActiveKeyError(err) {
		t.Fatalf("no active key err = %v", err)
	}
}

func TestLaunchServiceExplicitKeyErrorsPreserveCause(t *testing.T) {
	repo := &fakeKeyRepo{
		keys: []domainkey.Key{
			{ID: "dup-1111", Name: "claude-main", Provider: domainkey.ProviderClaude, Profile: "prod", APIKey: "k1", Active: true},
			{ID: "dup-2222", Name: "claude-backup", Provider: domainkey.ProviderClaude, Profile: "prod", APIKey: "k2"},
		},
	}
	launchSvc := NewLaunchService(NewKeyService(repo), &recordingRuntime{})

	if _, err := launchSvc.BuildSessionConfigWithOptions("claude", "/tmp", LaunchOptions{Profile: "prod", KeyIdentifier: "missing"}); !IsKeyNotFoundError(err) {
		t.Fatalf("explicit missing key err = %v, want KeyNotFoundError", err)
	}
	if _, err := launchSvc.BuildSessionConfigWithOptions("claude", "/tmp", LaunchOptions{Profile: "prod", KeyIdentifier: "dup"}); !IsAmbiguousKeyIdentifierError(err) {
		t.Fatalf("explicit ambiguous key err = %v, want AmbiguousKeyIdentifierError", err)
	}
}

func TestLaunchServiceLaunchPropagatesRuntimeError(t *testing.T) {
	repo := &fakeKeyRepo{
		keys: []domainkey.Key{
			{
				ID:       "k1",
				Provider: domainkey.ProviderClaude,
				Profile:  domainkey.DefaultProfile,
				Name:     "claude-main",
				APIKey:   "plain-key",
				Active:   true,
			},
		},
	}
	keySvc := NewKeyService(repo)
	rt := &recordingRuntime{launchErr: errors.New("tmux down")}
	launchSvc := NewLaunchService(keySvc, rt)

	err := launchSvc.Launch("claude", "/tmp", "")
	if err == nil {
		t.Fatal("Launch() should return runtime error")
	}
	if !IsRuntimeError(err) {
		t.Fatalf("Launch() err = %v, want RuntimeError", err)
	}
	if !errors.Is(err, rt.launchErr) {
		t.Fatalf("Launch() err should unwrap runtime error, got %v", err)
	}
	if !rt.launched {
		t.Fatal("runtime Launch should be called")
	}
	if rt.lastConfig.Agent != "ai-claude-code" {
		t.Fatalf("runtime cfg.Agent = %s, want ai-claude-code", rt.lastConfig.Agent)
	}
}
