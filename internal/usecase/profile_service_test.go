package usecase

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kiddingbaby/agx/internal/adapters/lockfile"
	"github.com/kiddingbaby/agx/internal/adapters/profilefile"
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

type fakeProfileRepo struct {
	profiles map[string]domainprofile.Profile
}

func (f *fakeProfileRepo) List() ([]domainprofile.Profile, error) {
	out := make([]domainprofile.Profile, 0, len(f.profiles))
	for _, profile := range f.profiles {
		out = append(out, profile)
	}
	return out, nil
}

func (f *fakeProfileRepo) Get(name string) (*domainprofile.Profile, error) {
	profile, ok := f.profiles[domainprofile.NormalizeProfileName(name)]
	if !ok {
		return nil, &domainprofile.NotFoundError{Name: name}
	}
	return &profile, nil
}

func (f *fakeProfileRepo) Upsert(profile domainprofile.Profile) (*domainprofile.Profile, error) {
	if f.profiles == nil {
		f.profiles = map[string]domainprofile.Profile{}
	}
	f.profiles[profile.Name] = profile
	return &profile, nil
}

func (f *fakeProfileRepo) Delete(name string) error {
	name = domainprofile.NormalizeProfileName(name)
	if _, ok := f.profiles[name]; !ok {
		return &domainprofile.NotFoundError{Name: name}
	}
	delete(f.profiles, name)
	return nil
}

type fakeStateRepo struct {
	state domainprofile.State
}

func (f *fakeStateRepo) Load() (domainprofile.State, error) {
	return f.state, nil
}

func (f *fakeStateRepo) Save(state domainprofile.State) (domainprofile.State, error) {
	f.state = state
	return state, nil
}

type fakeAgentSyncer struct {
	configPath      string
	snapshotExists  bool
	snapshotContent []byte
	backups         map[string][]byte
	deletedBackups  []string
	restorePath     string
	removeCalls     int
	syncCalls       int
	lastProfile     domainprofile.Profile
	defaultProfile  string
	managedProfiles map[string]ports.CodexManagedProfile
	snapshotErr     error
	createBackupErr error
	deleteBackupErr error
	restoreErr      error
	removeErr       error
	statusErr       error
	clearDefaultErr error
	removeProfileErr error
	syncErr         error
}

func newFakeAgentSyncer(configPath string) *fakeAgentSyncer {
	return &fakeAgentSyncer{
		configPath:      configPath,
		snapshotExists:  true,
		backups:         map[string][]byte{},
		managedProfiles: map[string]ports.CodexManagedProfile{},
	}
}

func (f *fakeAgentSyncer) Snapshot() (*ports.AgentConfigSnapshot, error) {
	if f.snapshotErr != nil {
		return nil, f.snapshotErr
	}
	return &ports.AgentConfigSnapshot{
		ConfigPath: f.configPath,
		Exists:     f.snapshotExists,
		Content:    append([]byte(nil), f.snapshotContent...),
	}, nil
}

func (f *fakeAgentSyncer) CreateBackup(id string, content []byte) (string, error) {
	if f.createBackupErr != nil {
		return "", f.createBackupErr
	}
	path := filepath.Join("/tmp/backups", id+".bak")
	f.backups[path] = append([]byte(nil), content...)
	return path, nil
}

func (f *fakeAgentSyncer) Restore(backupPath string) (string, error) {
	if f.restoreErr != nil {
		return "", f.restoreErr
	}
	data, ok := f.backups[backupPath]
	if !ok {
		var err error
		data, err = os.ReadFile(backupPath)
		if err != nil {
			return "", fmt.Errorf("backup missing: %s", backupPath)
		}
	}
	f.snapshotExists = true
	f.snapshotContent = append([]byte(nil), data...)
	f.restorePath = backupPath
	return f.configPath, nil
}

func (f *fakeAgentSyncer) RemoveConfig() (string, error) {
	f.removeCalls++
	if f.removeErr != nil {
		return "", f.removeErr
	}
	f.snapshotExists = false
	f.snapshotContent = nil
	return f.configPath, nil
}

func (f *fakeAgentSyncer) DeleteBackup(backupPath string) error {
	if f.deleteBackupErr != nil {
		return f.deleteBackupErr
	}
	f.deletedBackups = append(f.deletedBackups, backupPath)
	delete(f.backups, backupPath)
	return nil
}

type fakeCodexSyncer struct{ *fakeAgentSyncer }

func (f *fakeCodexSyncer) Sync(profile domainprofile.Profile, options ports.CodexSyncOptions) (*ports.CodexSyncResult, error) {
	if f.syncErr != nil {
		return nil, f.syncErr
	}
	f.syncCalls++
	f.lastProfile = profile
	f.snapshotExists = true
	f.snapshotContent = []byte(fmt.Sprintf("codex:%s:%s:%s", profile.Name, profile.BaseURL, options.DefaultProfileName))
	f.managedProfiles[profile.Name] = ports.CodexManagedProfile{Name: profile.Name, BaseURL: profile.BaseURL}
	if options.DefaultProfileName != "" {
		f.defaultProfile = options.DefaultProfileName
	}
	return &ports.CodexSyncResult{
		ProfileName: profile.Name,
		ConfigPath:  f.configPath,
	}, nil
}

func (f *fakeCodexSyncer) Status() (*ports.CodexConfigStatus, error) {
	if f.statusErr != nil {
		return nil, f.statusErr
	}
	managed := make(map[string]ports.CodexManagedProfile, len(f.managedProfiles))
	for name, profile := range f.managedProfiles {
		managed[name] = profile
	}
	return &ports.CodexConfigStatus{
		ConfigPath:          f.configPath,
		DefaultProfileName:  f.defaultProfile,
		ManagedProfilesByID: managed,
	}, nil
}

func (f *fakeCodexSyncer) ClearDefaultProfile() (string, error) {
	if f.clearDefaultErr != nil {
		return "", f.clearDefaultErr
	}
	f.defaultProfile = ""
	return f.configPath, nil
}

func (f *fakeCodexSyncer) RemoveProfile(name string) (string, error) {
	if f.removeProfileErr != nil {
		return "", f.removeProfileErr
	}
	delete(f.managedProfiles, domainprofile.NormalizeProfileName(name))
	if f.defaultProfile == domainprofile.NormalizeProfileName(name) {
		f.defaultProfile = ""
	}
	return f.configPath, nil
}

type fakeClaudeSyncer struct{ *fakeAgentSyncer }

func (f *fakeClaudeSyncer) Sync(profile domainprofile.Profile) (*ports.ClaudeSyncResult, error) {
	if f.syncErr != nil {
		return nil, f.syncErr
	}
	f.syncCalls++
	f.lastProfile = profile
	f.snapshotExists = true
	f.snapshotContent = []byte(fmt.Sprintf("claude:%s:%s", profile.Name, profile.BaseURL))
	return &ports.ClaudeSyncResult{ConfigPath: f.configPath}, nil
}

type fakeGeminiSyncer struct{ *fakeAgentSyncer }

func (f *fakeGeminiSyncer) Sync(profile domainprofile.Profile) (*ports.GeminiSyncResult, error) {
	if f.syncErr != nil {
		return nil, f.syncErr
	}
	f.syncCalls++
	f.lastProfile = profile
	f.snapshotExists = true
	f.snapshotContent = []byte(fmt.Sprintf("gemini:%s:%s", profile.Name, profile.BaseURL))
	return &ports.GeminiSyncResult{ConfigPath: f.configPath}, nil
}

func TestAddCreatesAndEditUpdatesProfile(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}
	state := &fakeStateRepo{}
	svc := NewProfileService(repo, state, nil, nil, nil)

	added, err := svc.Add("Relay-Prod", AddProfileInput{
		BaseURL: "https://relay.example/v1/",
		APIKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	profile := added.Relay
	if profile.Name != "relay-prod" {
		t.Fatalf("profile.Name = %q, want relay-prod", profile.Name)
	}

	beforeUpdatedAt := profile.UpdatedAt
	newKey := "sk-rotated"
	edited, err := svc.Edit("relay-prod", EditProfileInput{APIKey: &newKey})
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	updated := edited.Relay
	if updated.APIKey != newKey {
		t.Fatalf("updated.APIKey = %q, want %q", updated.APIKey, newKey)
	}
	if !updated.UpdatedAt.After(beforeUpdatedAt) {
		t.Fatalf("UpdatedAt = %v, want after previous UpdatedAt %v", updated.UpdatedAt, beforeUpdatedAt)
	}
}

func TestAddAllowsDuplicateNormalizedBaseURL(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}
	state := &fakeStateRepo{}
	svc := NewProfileService(repo, state, nil, nil, nil)

	if _, err := svc.Add("relay-a", AddProfileInput{
		BaseURL: "HTTPS://relay.example/v1/",
		APIKey:  "sk-a",
	}); err != nil {
		t.Fatalf("first Add() error = %v", err)
	}
	if _, err := svc.Add("relay-b", AddProfileInput{
		BaseURL: "https://relay.example/v1///",
		APIKey:  "sk-b",
	}); err != nil {
		t.Fatalf("second Add() error = %v", err)
	}
}

func TestAddDoesNotTouchAgentsWithoutBind(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}
	state := &fakeStateRepo{}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotExists = false
	codex := &fakeCodexSyncer{codexBase}
	svc := NewProfileService(repo, state, codex, nil, nil)

	if _, err := svc.Add("relay-a", AddProfileInput{
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-a",
	}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if codex.syncCalls != 0 {
		t.Fatalf("codex syncCalls = %d, want 0", codex.syncCalls)
	}
	if len(codex.managedProfiles) != 0 {
		t.Fatalf("managedProfiles = %+v, want empty", codex.managedProfiles)
	}
	if state.state.Codex.SourceProfile != "" {
		t.Fatalf("Codex.SourceProfile = %q, want empty", state.state.Codex.SourceProfile)
	}
	if len(state.state.CodexProfiles) != 0 {
		t.Fatalf("CodexProfiles = %+v, want empty", state.state.CodexProfiles)
	}
	if len(state.state.Codex.Backups) != 0 {
		t.Fatalf("Codex backups = %+v, want empty", state.state.Codex.Backups)
	}
}

func TestAddWithBindSyncsRequestedAgents(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}
	state := &fakeStateRepo{}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before codex")
	codex := &fakeCodexSyncer{codexBase}
	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.snapshotContent = []byte(`{"before":true}`)
	claude := &fakeClaudeSyncer{claudeBase}
	svc := NewProfileService(repo, state, codex, claude, nil)

	result, err := svc.Add("relay-a", AddProfileInput{
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-a",
		Bind:    []domainprofile.Agent{domainprofile.AgentCodex, domainprofile.AgentClaude},
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if result.Bindings == nil || len(result.Bindings.Changed) != 2 {
		t.Fatalf("Bindings = %+v, want 2 binding changes", result.Bindings)
	}

	if state.state.Codex.SourceProfile != "relay-a" {
		t.Fatalf("Codex.SourceProfile = %q, want relay-a", state.state.Codex.SourceProfile)
	}
	if state.state.Claude.SourceProfile != "relay-a" {
		t.Fatalf("Claude.SourceProfile = %q, want relay-a", state.state.Claude.SourceProfile)
	}
	if codex.syncCalls != 1 {
		t.Fatalf("codex syncCalls = %d, want 1", codex.syncCalls)
	}
	if claude.syncCalls != 1 {
		t.Fatalf("claude syncCalls = %d, want 1", claude.syncCalls)
	}
}

func TestAddAnotherProfileDoesNotRewriteActiveCodexBindingMetadata(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}
	state := &fakeStateRepo{}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before")
	codex := &fakeCodexSyncer{codexBase}
	svc := NewProfileService(repo, state, codex, nil, nil)

	if _, err := svc.Add("relay-a", AddProfileInput{
		BaseURL: "https://relay-a.example/v1",
		APIKey:  "sk-a",
	}); err != nil {
		t.Fatalf("Add(relay-a) error = %v", err)
	}
	if _, err := svc.AgentSet(domainprofile.AgentCodex, "relay-a"); err != nil {
		t.Fatalf("AgentSet(codex, relay-a) error = %v", err)
	}

	activeBackupID := state.state.Codex.LastBackupID
	activeAppliedAt := state.state.Codex.LastAppliedAt

	if _, err := svc.Add("relay-b", AddProfileInput{
		BaseURL: "https://relay-b.example/v1",
		APIKey:  "sk-b",
	}); err != nil {
		t.Fatalf("Add(relay-b) error = %v", err)
	}

	if state.state.Codex.SourceProfile != "relay-a" {
		t.Fatalf("Codex.SourceProfile = %q, want relay-a", state.state.Codex.SourceProfile)
	}
	if state.state.Codex.LastBackupID != activeBackupID {
		t.Fatalf("Codex.LastBackupID = %q, want active backup %q unchanged", state.state.Codex.LastBackupID, activeBackupID)
	}
	if !state.state.Codex.LastAppliedAt.Equal(activeAppliedAt) {
		t.Fatalf("Codex.LastAppliedAt = %v, want %v unchanged", state.state.Codex.LastAppliedAt, activeAppliedAt)
	}
	if state.state.CodexProfiles["relay-a"].LastBackupID != activeBackupID {
		t.Fatalf("CodexProfiles[relay-a].LastBackupID = %q, want %q", state.state.CodexProfiles["relay-a"].LastBackupID, activeBackupID)
	}
}

func TestAgentSetChangesOnlyTargetAgent(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay-a.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		"relay-b": {Name: "relay-b", BaseURL: "https://relay-b.example/v1", APIKey: "sk-b", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{
		state: domainprofile.State{
			Codex:  domainprofile.CodexState{BindingView: domainprofile.BindingView{SourceProfile: "relay-a", Status: domainprofile.BindingStatusApplied}},
			Claude: domainprofile.AgentBinding{SourceProfile: "relay-a", Status: domainprofile.BindingStatusApplied},
		},
	}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.snapshotContent = []byte("before codex")
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	svc := NewProfileService(repo, state, codex, claude, nil)

	if _, err := svc.AgentSet(domainprofile.AgentCodex, "relay-b"); err != nil {
		t.Fatalf("AgentSet(codex) error = %v", err)
	}

	if state.state.Codex.SourceProfile != "relay-b" {
		t.Fatalf("Codex.SourceProfile = %q, want relay-b", state.state.Codex.SourceProfile)
	}
	if state.state.Claude.SourceProfile != "relay-a" {
		t.Fatalf("Claude.SourceProfile = %q, want relay-a", state.state.Claude.SourceProfile)
	}
}

func TestAgentSetSupportsMultipleAgentsOnSameProfile(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.snapshotContent = []byte("before codex")
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	claude.snapshotContent = []byte(`{"before":true}`)
	svc := NewProfileService(repo, state, codex, claude, nil)

	if _, err := svc.AgentSet(domainprofile.AgentCodex, "relay-a"); err != nil {
		t.Fatalf("AgentSet(codex) error = %v", err)
	}
	if _, err := svc.AgentSet(domainprofile.AgentClaude, "relay-a"); err != nil {
		t.Fatalf("AgentSet(claude) error = %v", err)
	}

	if state.state.Codex.SourceProfile != "relay-a" || state.state.Claude.SourceProfile != "relay-a" {
		t.Fatalf("state = %+v, want codex and claude active on relay-a", state.state)
	}
}

func TestEditAutoSyncsBoundAgents(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	gemini := &fakeGeminiSyncer{newFakeAgentSyncer("/tmp/gemini/.env")}
	svc := NewProfileService(repo, state, codex, nil, gemini)

	if _, err := svc.AgentSet(domainprofile.AgentCodex, "relay-a"); err != nil {
		t.Fatalf("AgentSet(codex) error = %v", err)
	}
	if _, err := svc.AgentSet(domainprofile.AgentGemini, "relay-a"); err != nil {
		t.Fatalf("AgentSet(gemini) error = %v", err)
	}
	codexCallsBefore := codex.syncCalls
	geminiCallsBefore := gemini.syncCalls

	newURL := "https://relay-new.example/v1"
	newKey := "sk-rotated"
	result, err := svc.Edit("relay-a", EditProfileInput{BaseURL: &newURL, APIKey: &newKey})
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if result.Relay == nil {
		t.Fatal("Edit() relay = nil, want updated relay")
	}

	if codex.syncCalls != codexCallsBefore+1 {
		t.Fatalf("codex syncCalls = %d, want %d", codex.syncCalls, codexCallsBefore+1)
	}
	if gemini.syncCalls != geminiCallsBefore+1 {
		t.Fatalf("gemini syncCalls = %d, want %d", gemini.syncCalls, geminiCallsBefore+1)
	}
	if codex.lastProfile.BaseURL != newURL {
		t.Fatalf("codex last base_url = %q, want %q", codex.lastProfile.BaseURL, newURL)
	}
	if gemini.lastProfile.APIKey != newKey {
		t.Fatalf("gemini last api key = %q, want %q", gemini.lastProfile.APIKey, newKey)
	}
	if state.state.Codex.SourceProfile != "relay-a" || state.state.Gemini.SourceProfile != "relay-a" {
		t.Fatalf("state = %+v, want bindings unchanged", state.state)
	}
}

func TestEditBindReturnsBindingChangesWhenSwitchingRelay(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay-a.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		"relay-b": {Name: "relay-b", BaseURL: "https://relay-b.example/v1", APIKey: "sk-b", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before codex")
	codex := &fakeCodexSyncer{codexBase}
	svc := NewProfileService(repo, state, codex, nil, nil)

	if _, err := svc.Edit("relay-a", EditProfileInput{Bind: []domainprofile.Agent{domainprofile.AgentCodex}}); err != nil {
		t.Fatalf("Edit(bind relay-a) error = %v", err)
	}

	result, err := svc.Edit("relay-b", EditProfileInput{Bind: []domainprofile.Agent{domainprofile.AgentCodex}})
	if err != nil {
		t.Fatalf("Edit(bind relay-b) error = %v", err)
	}
	if result.Bindings == nil || len(result.Bindings.Changed) != 1 {
		t.Fatalf("Bindings = %+v, want exactly 1 change", result.Bindings)
	}
	change := result.Bindings.Changed[0]
	if change.Action != "bind" || change.Agent != domainprofile.AgentCodex {
		t.Fatalf("change = %+v, want bind codex", change)
	}
	if change.Binding == nil || change.Binding.SourceProfile != "relay-b" {
		t.Fatalf("binding = %+v, want relay-b", change.Binding)
	}
}

func TestAddRollsBackProfileAndConfigsWhenBindFails(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}
	state := &fakeStateRepo{}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before codex")
	codex := &fakeCodexSyncer{codexBase}
	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.snapshotContent = []byte("before claude")
	claudeBase.syncErr = errors.New("claude sync failed")
	claude := &fakeClaudeSyncer{claudeBase}
	svc := NewProfileService(repo, state, codex, claude, nil)

	_, err := svc.Add("relay-a", AddProfileInput{
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-a",
		Bind:    []domainprofile.Agent{domainprofile.AgentCodex, domainprofile.AgentClaude},
	})
	if err == nil || !strings.Contains(err.Error(), "claude sync failed") {
		t.Fatalf("Add() err = %v, want claude sync failure", err)
	}
	if _, ok := repo.profiles["relay-a"]; ok {
		t.Fatalf("profiles = %+v, want relay-a rollback", repo.profiles)
	}
	if state.state.Codex.SourceProfile != "" || state.state.Claude.SourceProfile != "" || state.state.Gemini.SourceProfile != "" || len(state.state.CodexProfiles) != 0 || len(state.state.Codex.Backups) != 0 {
		t.Fatalf("state = %+v, want empty rollback state", state.state)
	}
	if string(codex.snapshotContent) != "before codex" {
		t.Fatalf("codex snapshot = %q, want restored content", string(codex.snapshotContent))
	}
	if string(claude.snapshotContent) != "before claude" {
		t.Fatalf("claude snapshot = %q, want restored content", string(claude.snapshotContent))
	}
}

func TestEditRollsBackProfileAndConfigsWhenLaterAgentFails(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before codex")
	codex := &fakeCodexSyncer{codexBase}
	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.snapshotContent = []byte("before claude")
	claude := &fakeClaudeSyncer{claudeBase}
	svc := NewProfileService(repo, state, codex, claude, nil)

	if _, err := svc.AgentSet(domainprofile.AgentCodex, "relay-a"); err != nil {
		t.Fatalf("AgentSet(codex) error = %v", err)
	}
	if _, err := svc.AgentSet(domainprofile.AgentClaude, "relay-a"); err != nil {
		t.Fatalf("AgentSet(claude) error = %v", err)
	}
	codexBeforeEdit := string(codex.snapshotContent)
	claudeBeforeEdit := string(claude.snapshotContent)

	newURL := "https://relay-new.example/v1"
	claudeBase.syncErr = errors.New("claude sync failed")
	err := func() error {
		_, err := svc.Edit("relay-a", EditProfileInput{BaseURL: &newURL})
		return err
	}()
	if err == nil || !strings.Contains(err.Error(), "claude sync failed") {
		t.Fatalf("Edit() err = %v, want claude sync failure", err)
	}

	profile, getErr := repo.Get("relay-a")
	if getErr != nil {
		t.Fatalf("Get() error = %v", getErr)
	}
	if profile.BaseURL != "https://relay.example/v1" {
		t.Fatalf("profile.BaseURL = %q, want rolled back original", profile.BaseURL)
	}
	if state.state.Codex.SourceProfile != "relay-a" {
		t.Fatalf("Codex.SourceProfile = %q, want relay-a", state.state.Codex.SourceProfile)
	}
	if state.state.Claude.SourceProfile != "relay-a" {
		t.Fatalf("Claude.SourceProfile = %q, want relay-a", state.state.Claude.SourceProfile)
	}
	if string(codex.snapshotContent) != codexBeforeEdit {
		t.Fatalf("codex snapshot = %q, want restored content", string(codex.snapshotContent))
	}
	if string(claude.snapshotContent) != claudeBeforeEdit {
		t.Fatalf("claude snapshot = %q, want restored content", string(claude.snapshotContent))
	}
}

func TestClearRemovesActiveBindingButKeepsCodexRegistration(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc := NewProfileService(repo, state, codex, nil, nil)

	if _, err := svc.AgentSet(domainprofile.AgentCodex, "relay-a"); err != nil {
		t.Fatalf("AgentSet(codex) error = %v", err)
	}
	if _, err := svc.Clear(domainprofile.AgentCodex); err != nil {
		t.Fatalf("Clear(codex) error = %v", err)
	}

	if state.state.Codex.SourceProfile != "" {
		t.Fatalf("Codex.SourceProfile = %q, want empty", state.state.Codex.SourceProfile)
	}
	if _, ok := state.state.CodexProfiles["relay-a"]; !ok {
		t.Fatalf("CodexProfiles = %+v, want relay-a registration preserved", state.state.CodexProfiles)
	}
	if codex.defaultProfile != "" {
		t.Fatalf("defaultProfile = %q, want empty", codex.defaultProfile)
	}
}

func TestClearUsesRemoveCreatedFileBackupWhenConfigMissing(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	geminiBase := newFakeAgentSyncer("/tmp/gemini/.env")
	geminiBase.snapshotExists = false
	gemini := &fakeGeminiSyncer{geminiBase}
	svc := NewProfileService(repo, state, nil, nil, gemini)

	if _, err := svc.AgentSet(domainprofile.AgentGemini, "relay-a"); err != nil {
		t.Fatalf("AgentSet() error = %v", err)
	}
	result, err := svc.Clear(domainprofile.AgentGemini)
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if result.Backup.RestoreMode != domainprofile.RestoreModeRestoreFile {
		t.Fatalf("RestoreMode = %q, want restore_file", result.Backup.RestoreMode)
	}
}

func TestRestoreCanTargetSpecificBackupID(t *testing.T) {
	tmpDir := t.TempDir()
	newPath := filepath.Join(tmpDir, "new.bak")
	oldPath := filepath.Join(tmpDir, "old.bak")
	newBody := []byte("profile = \"relay-a\"\n[profiles.\"relay-a\"]\nmodel_provider = \"agx/relay-a\"\n")
	oldBody := []byte("profile = \"relay-a\"\n[profiles.\"relay-a\"]\nmodel_provider = \"agx/relay-a\"\n")
	if err := os.WriteFile(newPath, newBody, 0600); err != nil {
		t.Fatalf("WriteFile(new) error = %v", err)
	}
	if err := os.WriteFile(oldPath, oldBody, 0600); err != nil {
		t.Fatalf("WriteFile(old) error = %v", err)
	}

	state := &fakeStateRepo{
		state: domainprofile.State{
			Codex: domainprofile.CodexState{
				BindingView: domainprofile.BindingView{
					SourceProfile: "relay-a",
					Status:        domainprofile.BindingStatusApplied,
					ConfigPath:    "/tmp/codex/config.toml",
				},
				Backups: []domainprofile.Backup{
					{ID: "new", ConfigPath: "/tmp/codex/config.toml", BackupPath: newPath, RestoreMode: domainprofile.RestoreModeRestoreFile},
					{ID: "old", ConfigPath: "/tmp/codex/config.toml", BackupPath: oldPath, RestoreMode: domainprofile.RestoreModeRestoreFile},
				},
			},
		},
	}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.backups[newPath] = newBody
	codexBase.backups[oldPath] = oldBody
	codex := &fakeCodexSyncer{codexBase}
	svc := NewProfileService(&fakeProfileRepo{}, state, codex, nil, nil)

	result, err := svc.Restore(domainprofile.AgentCodex, "old")
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if result.Backup.ID != "old" {
		t.Fatalf("Backup.ID = %q, want old", result.Backup.ID)
	}
	if codex.restorePath != oldPath {
		t.Fatalf("restorePath = %q, want %q", codex.restorePath, oldPath)
	}
}

func TestBackupHistoryStaysBounded(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before")
	codex := &fakeCodexSyncer{codexBase}
	svc := NewProfileService(repo, state, codex, nil, nil)

	for i := 0; i < 6; i++ {
		codex.snapshotContent = []byte(fmt.Sprintf("before-%d", i))
		if _, err := svc.AgentSet(domainprofile.AgentCodex, "relay-a"); err != nil {
			t.Fatalf("AgentSet() iteration %d error = %v", i, err)
		}
	}

	if len(state.state.Codex.Backups) != 5 {
		t.Fatalf("len(backups) = %d, want 5", len(state.state.Codex.Backups))
	}
	if len(codex.deletedBackups) != 1 {
		t.Fatalf("deletedBackups = %v, want 1 trimmed backup", codex.deletedBackups)
	}
}

func TestRemoveRejectsActiveProfile(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc := NewProfileService(repo, state, codex, nil, nil)

	if _, err := svc.AgentSet(domainprofile.AgentCodex, "relay-a"); err != nil {
		t.Fatalf("AgentSet() error = %v", err)
	}
	_, err := svc.Remove("relay-a")
	var target *ProfileInUseError
	if !errors.As(err, &target) {
		t.Fatalf("Remove() err = %v, want ProfileInUseError", err)
	}
	if len(target.Agents) != 1 || target.Agents[0] != domainprofile.AgentCodex {
		t.Fatalf("Agents = %v, want [codex]", target.Agents)
	}
}

func TestRemoveDeletesUnboundCodexRegisteredProfile(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}
	state := &fakeStateRepo{}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc := NewProfileService(repo, state, codex, nil, nil)

	if _, err := svc.Add("relay-a", AddProfileInput{
		BaseURL: "https://relay.example",
		APIKey:  "sk-a",
	}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, err := svc.Remove("relay-a"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, ok := codex.managedProfiles["relay-a"]; ok {
		t.Fatalf("managedProfiles = %+v, want relay-a removed", codex.managedProfiles)
	}
}

func TestEditRefreshesTrackedInactiveCodexProfile(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before codex")
	codex := &fakeCodexSyncer{codexBase}
	svc := NewProfileService(repo, state, codex, nil, nil)

	if _, err := svc.AgentSet(domainprofile.AgentCodex, "relay-a"); err != nil {
		t.Fatalf("AgentSet() error = %v", err)
	}
	if _, err := svc.Clear(domainprofile.AgentCodex); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	syncCallsBefore := codex.syncCalls

	newURL := "https://relay-new.example/v1"
	if _, err := svc.Edit("relay-a", EditProfileInput{BaseURL: &newURL}); err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if codex.syncCalls != syncCallsBefore+1 {
		t.Fatalf("codex syncCalls = %d, want %d", codex.syncCalls, syncCallsBefore+1)
	}
	if codex.managedProfiles["relay-a"].BaseURL != newURL {
		t.Fatalf("managed profile base_url = %q, want %q", codex.managedProfiles["relay-a"].BaseURL, newURL)
	}
	if state.state.Codex.SourceProfile != "" {
		t.Fatalf("Codex.SourceProfile = %q, want inactive binding preserved", state.state.Codex.SourceProfile)
	}
}

func TestNextBackupIDAddsNumericSuffixOnCollision(t *testing.T) {
	now := time.Date(2026, 4, 24, 1, 2, 3, 0, time.UTC)
	backups := []domainprofile.Backup{
		{ID: "before-codex-sync-20260424T010203Z"},
		{ID: "before-codex-sync-20260424T010203Z-2"},
	}

	got := nextBackupID(domainprofile.AgentCodex, backups, now)
	if got != "before-codex-sync-20260424T010203Z-3" {
		t.Fatalf("nextBackupID() = %q, want before-codex-sync-20260424T010203Z-3", got)
	}
}

func TestAPIKeyReturnsExplicitProfileKey(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	svc := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil)

	got, err := svc.APIKey("relay-a")
	if err != nil {
		t.Fatalf("APIKey() error = %v", err)
	}
	if got != "sk-a" {
		t.Fatalf("APIKey() = %q, want sk-a", got)
	}
}

func TestConcurrentSetDifferentAgentsDoesNotLoseBindings(t *testing.T) {
	home := t.TempDir()
	storeDir := filepath.Join(home, ".config", "agx")
	t.Setenv("HOME", home)

	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("profile = \"before\"\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte("{}\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	serviceA := newProfileServiceForDirForTest(t, storeDir)
	serviceB := newProfileServiceForDirForTest(t, storeDir)
	if _, err := serviceA.Add("relay-a", AddProfileInput{BaseURL: "https://relay.example/v1", APIKey: "sk-a"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := serviceA.AgentSet(domainprofile.AgentCodex, "relay-a")
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := serviceB.AgentSet(domainprofile.AgentClaude, "relay-a")
		errs <- err
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("AgentSet() error = %v", err)
		}
	}

	state, err := serviceA.State()
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if state.Codex.SourceProfile != "relay-a" || state.Claude.SourceProfile != "relay-a" {
		t.Fatalf("state = %+v, want codex and claude active on relay-a", state)
	}
}

func newProfileServiceForDirForTest(t *testing.T, storeDir string) *ProfileService {
	t.Helper()

	if err := os.MkdirAll(storeDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(storeDir) error = %v", err)
	}
	repo, err := profilefile.NewRepository(filepath.Join(storeDir, "profiles"))
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	state := profilefile.NewStateRepository(filepath.Join(storeDir, "state.yaml"))
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	gemini := &fakeGeminiSyncer{newFakeAgentSyncer("/tmp/gemini/.env")}
	service := NewProfileService(repo, state, codex, claude, gemini)
	service.SetMutationLocker(lockfile.New(filepath.Join(storeDir, "agx.lock")))
	return service
}
