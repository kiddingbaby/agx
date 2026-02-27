package usecase

import (
	"errors"
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
