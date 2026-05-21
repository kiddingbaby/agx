package usecase

import (
	"errors"
	"strings"
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func TestStateResolvesRuntimeAgentBindingsFromCurrentConfig(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	claude.snapshotContent = []byte("{\"apiKeyHelper\":\"agx __api-key relay-a\",\"env\":{\"ANTHROPIC_BASE_URL\":\"https://relay.example/v1\"}}\n")
	svc := NewProfileService(repo, state, nil, claude, nil, nil)

	got, err := svc.State()
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if got.Claude.SourceProfile != "relay-a" || got.Claude.Status != domainprofile.BindingStatusApplied {
		t.Fatalf("State() = %+v, want resolved claude binding", got.Claude)
	}
}

func TestEditUsesRuntimeAgentBindings(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	claude.snapshotContent = []byte("{\"apiKeyHelper\":\"agx __api-key relay-a\",\"env\":{\"ANTHROPIC_BASE_URL\":\"https://relay.example/v1\"}}\n")
	svc := NewProfileService(repo, state, nil, claude, nil, nil)

	if _, err := svc.Edit("relay-a", EditProfileInput{BaseURL: ptr("https://relay.example/v2")}); err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if claude.syncCalls != 1 {
		t.Fatalf("claude.syncCalls = %d, want 1", claude.syncCalls)
	}
	if state.state.Claude.SourceProfile != "relay-a" || state.state.Claude.Status != domainprofile.BindingStatusApplied {
		t.Fatalf("state.Claude = %+v, want persisted runtime binding", state.state.Claude)
	}
}

func TestUnbindUsesRuntimeAgentBindingMetadata(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	claude.snapshotContent = []byte("{\"apiKeyHelper\":\"agx __api-key relay-a\",\"env\":{\"ANTHROPIC_BASE_URL\":\"https://relay.example/v1\"}}\n")
	svc := NewProfileService(repo, state, nil, claude, nil, nil)

	result, err := svc.applyRelayBindingChanges("relay-a", nil, []domainprofile.Agent{domainprofile.AgentClaude})
	if err != nil {
		t.Fatalf("applyRelayBindingChanges(unbind runtime) error = %v", err)
	}
	if len(result.Changed) != 1 || result.Changed[0].Backup.AppliedProfile != "relay-a" {
		t.Fatalf("Changed = %+v, want backup applied profile relay-a", result.Changed)
	}
	if state.state.Claude.SourceProfile != "" || state.state.Claude.LastBackupID == "" {
		t.Fatalf("state.Claude = %+v, want cleared binding with backup", state.state.Claude)
	}
}

func TestAgentBindPreservesRuntimeClaudeCurrentWhenStoredStateMissing(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay-a.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		"relay-b": {Name: "relay-b", BaseURL: "https://relay-b.example/v1", APIKey: "sk-b", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	claude.snapshotContent = []byte("{\"apiKeyHelper\":\"agx __api-key relay-a\",\"env\":{\"ANTHROPIC_BASE_URL\":\"https://relay-a.example/v1\"}}\n")
	svc := NewProfileService(repo, state, nil, claude, nil, nil)

	result, err := svc.AgentBind(domainprofile.AgentClaude, "relay-b")
	if err != nil {
		t.Fatalf("AgentBind(claude relay-b) error = %v", err)
	}
	if result.PreviousRelay != "relay-a" {
		t.Fatalf("PreviousRelay = %q, want relay-a", result.PreviousRelay)
	}
	if state.state.Claude.SourceProfile != "relay-a" {
		t.Fatalf("state.Claude.SourceProfile = %q, want relay-a", state.state.Claude.SourceProfile)
	}
	if claude.syncCalls != 0 {
		t.Fatalf("claude.syncCalls = %d, want 0", claude.syncCalls)
	}
}

func TestAgentBindPreservesRuntimeGeminiCurrentWhenStoredStateMissing(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay-a.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		"relay-b": {Name: "relay-b", BaseURL: "https://relay-b.example/v1", APIKey: "sk-b", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	gemini := &fakeGeminiSyncer{newFakeAgentSyncer("/tmp/gemini/.env")}
	gemini.snapshotContent = []byte("GEMINI_API_KEY=\"sk-a\"\nGOOGLE_GEMINI_BASE_URL=\"https://relay-a.example/v1\"\n")
	svc := NewProfileService(repo, state, nil, nil, gemini, nil)

	result, err := svc.AgentBind(domainprofile.AgentGemini, "relay-b")
	if err != nil {
		t.Fatalf("AgentBind(gemini relay-b) error = %v", err)
	}
	if result.PreviousRelay != "relay-a" {
		t.Fatalf("PreviousRelay = %q, want relay-a", result.PreviousRelay)
	}
	if state.state.Gemini.SourceProfile != "relay-a" {
		t.Fatalf("state.Gemini.SourceProfile = %q, want relay-a", state.state.Gemini.SourceProfile)
	}
	if gemini.syncCalls != 0 {
		t.Fatalf("gemini.syncCalls = %d, want 0", gemini.syncCalls)
	}
}

func TestAgentBindPreservesRuntimeCodexCurrentWhenStoredStateMissing(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay-a.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		"relay-b": {Name: "relay-b", BaseURL: "https://relay-b.example/v1", APIKey: "sk-b", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.defaultProfile = "relay-a"
	codex.managedProfiles["relay-a"] = ports.CodexManagedProfile{Name: "relay-a", BaseURL: "https://relay-a.example/v1"}
	svc := NewProfileService(repo, state, codex, nil, nil, nil)

	result, err := svc.AgentBind(domainprofile.AgentCodex, "relay-b")
	if err != nil {
		t.Fatalf("AgentBind(codex relay-b) error = %v", err)
	}
	if result.PreviousRelay != "relay-a" {
		t.Fatalf("PreviousRelay = %q, want relay-a", result.PreviousRelay)
	}
	if state.state.Codex.SourceProfile != "relay-a" {
		t.Fatalf("state.Codex.SourceProfile = %q, want relay-a", state.state.Codex.SourceProfile)
	}
	if codex.defaultProfile != "relay-a" {
		t.Fatalf("codex.defaultProfile = %q, want relay-a", codex.defaultProfile)
	}
}

func TestStateResetsStaleRuntimeBindingMetadata(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	now := time.Now().UTC()
	state := &fakeStateRepo{state: domainprofile.State{
		Claude: domainprofile.AgentBinding{
			SourceProfile: "relay-a",
			Status:        domainprofile.BindingStatus("broken"),
			ConfigPath:    "/tmp/old-claude.json",
			LastAppliedAt: now,
			LastBackupID:  "backup-old",
		},
	}}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	claude.snapshotContent = []byte("{\"apiKeyHelper\":\"agx __api-key relay-a\",\"env\":{\"ANTHROPIC_BASE_URL\":\"https://relay.example/v1\"}}\n")
	svc := NewProfileService(repo, state, nil, claude, nil, nil)

	got, err := svc.State()
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if got.Claude.Status != domainprofile.BindingStatusApplied {
		t.Fatalf("Claude.Status = %q, want applied", got.Claude.Status)
	}
	if !got.Claude.LastAppliedAt.IsZero() || got.Claude.LastBackupID != "" {
		t.Fatalf("Claude metadata = %+v, want stale runtime metadata cleared", got.Claude)
	}
	if got.Claude.ConfigPath != "/tmp/claude/settings.json" {
		t.Fatalf("Claude.ConfigPath = %q, want runtime config path", got.Claude.ConfigPath)
	}
}

func TestStateFallsBackWhenClaudeConfigIsUnreadable(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{state: domainprofile.State{
		Claude: domainprofile.AgentBinding{
			SourceProfile: "relay-a",
			Status:        domainprofile.BindingStatusApplied,
		},
	}}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	claude.snapshotContent = []byte("{broken")
	svc := NewProfileService(repo, state, nil, claude, nil, nil)

	got, err := svc.State()
	if err != nil {
		t.Fatalf("State() error = %v, want fallback to stored state", err)
	}
	if got.Claude.SourceProfile != "relay-a" {
		t.Fatalf("State() = %+v, want stored claude binding preserved", got.Claude)
	}

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if report.OK {
		t.Fatalf("Doctor().OK = true, want runtime config issue")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "runtime_config_unreadable" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Doctor().Issues = %+v, want runtime_config_unreadable", report.Issues)
	}
}

func TestStateAndDoctorFallBackWhenGeminiManagedBlockIsIncomplete(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{state: domainprofile.State{
		Gemini: domainprofile.AgentBinding{
			SourceProfile: "relay-a",
			Status:        domainprofile.BindingStatusApplied,
			ConfigPath:    "/tmp/gemini/.env",
		},
	}}
	gemini := &fakeGeminiSyncer{newFakeAgentSyncer("/tmp/gemini/.env")}
	gemini.snapshotContent = []byte("KEEP=1\n# >>> AGX managed Gemini env >>>\nGEMINI_API_KEY=\"sk-a\"\n")
	svc := NewProfileService(repo, state, nil, nil, gemini, nil)

	got, err := svc.State()
	if err != nil {
		t.Fatalf("State() error = %v, want fallback to stored state", err)
	}
	if got.Gemini.SourceProfile != "relay-a" || got.Gemini.ConfigPath != "/tmp/gemini/.env" {
		t.Fatalf("State() = %+v, want stored gemini binding preserved", got.Gemini)
	}

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if report.OK {
		t.Fatalf("Doctor().OK = true, want managed block issue")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "runtime_config_unreadable" && strings.Contains(issue.Message, "incomplete AGX managed block") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Doctor().Issues = %+v, want incomplete gemini managed block issue", report.Issues)
	}
}

func TestDoctorFallsBackWhenCodexConfigManagedBlockIsIncomplete(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{
				SourceProfile: "relay-a",
				Status:        domainprofile.BindingStatusApplied,
				ConfigPath:    "/tmp/codex/config.toml",
			},
		},
	}}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.statusErr = &ports.IncompleteManagedBlockError{
		Agent:      domainprofile.AgentCodex,
		ConfigPath: "/tmp/codex/config.toml",
	}
	svc := NewProfileService(repo, state, codex, nil, nil, nil)

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if report.OK {
		t.Fatalf("Doctor().OK = true, want managed block issue")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "runtime_config_unreadable" && strings.Contains(issue.Message, "incomplete AGX managed block") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Doctor().Issues = %+v, want incomplete codex managed block issue", report.Issues)
	}
}

func TestDoctorReportsRuntimeBindingConflict(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay-a.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		"relay-b": {Name: "relay-b", BaseURL: "https://relay-b.example/v1", APIKey: "sk-b", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{state: domainprofile.State{
		Claude: domainprofile.AgentBinding{
			SourceProfile: "relay-a",
			Status:        domainprofile.BindingStatusApplied,
		},
	}}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	claude.snapshotContent = []byte("{\"apiKeyHelper\":\"agx __api-key relay-a\",\"env\":{\"ANTHROPIC_BASE_URL\":\"https://relay-b.example/v1\"}}\n")
	svc := NewProfileService(repo, state, nil, claude, nil, nil)

	got, err := svc.State()
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if got.Claude.SourceProfile != "relay-a" {
		t.Fatalf("State() = %+v, want stored relay preserved on conflict", got.Claude)
	}

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "runtime_binding_conflict" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Doctor().Issues = %+v, want runtime_binding_conflict", report.Issues)
	}
}

func TestDoctorReportsStaleClaudeHelperConflict(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-b": {Name: "relay-b", BaseURL: "https://relay-b.example/v1", APIKey: "sk-b", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{state: domainprofile.State{
		Claude: domainprofile.AgentBinding{
			SourceProfile: "relay-b",
			Status:        domainprofile.BindingStatusApplied,
		},
	}}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	claude.snapshotContent = []byte("{\"apiKeyHelper\":\"agx __api-key relay-missing\",\"env\":{\"ANTHROPIC_BASE_URL\":\"https://relay-b.example/v1\"}}\n")
	svc := NewProfileService(repo, state, nil, claude, nil, nil)

	got, err := svc.State()
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if got.Claude.SourceProfile != "relay-b" {
		t.Fatalf("State() = %+v, want stored relay preserved for stale helper", got.Claude)
	}

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "runtime_binding_conflict" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Doctor().Issues = %+v, want runtime_binding_conflict for stale helper", report.Issues)
	}
}

func TestStateFailsClosedWhenRuntimeProfilesUnavailable(t *testing.T) {
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	claude.snapshotContent = []byte("{\"apiKeyHelper\":\"agx __api-key relay-a\",\"env\":{\"ANTHROPIC_BASE_URL\":\"https://relay.example/v1\"}}\n")
	svc := NewProfileService(&errorProfileRepo{listErr: errors.New("list failed")}, &fakeStateRepo{}, nil, claude, nil, nil)

	if _, err := svc.State(); err == nil || !strings.Contains(err.Error(), "list failed") {
		t.Fatalf("State(runtime profiles unavailable) err=%v, want list failed", err)
	}
}
