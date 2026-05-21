package usecase

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

type stagedProfileRepo struct {
	profiles   map[string]domainprofile.Profile
	getResults []struct {
		profile *domainprofile.Profile
		err     error
	}
	deleteErr error
}

func (s *stagedProfileRepo) List() ([]domainprofile.Profile, error) {
	out := make([]domainprofile.Profile, 0, len(s.profiles))
	for _, profile := range s.profiles {
		out = append(out, profile)
	}
	return out, nil
}

func (s *stagedProfileRepo) Get(name string) (*domainprofile.Profile, error) {
	name = domainprofile.NormalizeProfileName(name)
	if len(s.getResults) > 0 {
		result := s.getResults[0]
		s.getResults = s.getResults[1:]
		if result.profile != nil {
			copied := *result.profile
			return &copied, result.err
		}
		return nil, result.err
	}
	if profile, ok := s.profiles[name]; ok {
		copied := profile
		return &copied, nil
	}
	return nil, &domainprofile.NotFoundError{Name: name}
}

func (s *stagedProfileRepo) Upsert(profile domainprofile.Profile) (*domainprofile.Profile, error) {
	if s.profiles == nil {
		s.profiles = map[string]domainprofile.Profile{}
	}
	s.profiles[profile.Name] = profile
	copied := profile
	return &copied, nil
}

func (s *stagedProfileRepo) Delete(name string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	name = domainprofile.NormalizeProfileName(name)
	if _, ok := s.profiles[name]; !ok {
		return &domainprofile.NotFoundError{Name: name}
	}
	delete(s.profiles, name)
	return nil
}

func TestBindingsCoverageTargetedBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}

	if _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).agentSetLocked(domainprofile.AgentCodex, "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("agentSetLocked(Get failure) err=%v, want not found", err)
	}

	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc := NewProfileService(repo, &errorStateRepo{saveErr: errors.New("save failed")}, codex, nil, nil, nil)
	if _, err := svc.agentSetLocked(domainprofile.AgentCodex, "relay-a"); err == nil || !strings.Contains(err.Error(), "save failed") {
		t.Fatalf("agentSetLocked(save failure) err=%v, want save failed", err)
	}

	if _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).clearLocked(domainprofile.Agent("bad")); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("clearLocked(invalid agent) err=%v, want invalid agent", err)
	}
	if _, err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, nil, nil, nil, nil).clearLocked(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("clearLocked(load failure) err=%v, want load failed", err)
	}

	bindingState := domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{SourceProfile: "relay-a"},
			Backups: []domainprofile.Backup{
				{ID: "b1", BackupPath: "/tmp/b1"},
				{ID: "b2", BackupPath: "/tmp/b2"},
				{ID: "b3", BackupPath: "/tmp/b3"},
				{ID: "b4", BackupPath: "/tmp/b4"},
				{ID: "b5", BackupPath: "/tmp/b5"},
			},
		},
	}
	if _, err := NewProfileService(repo, &fakeStateRepo{state: bindingState}, nil, nil, nil, nil).clearLocked(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("clearLocked(missing codex) err=%v, want invalid agent", err)
	}
	if _, err := NewProfileService(repo, &fakeStateRepo{state: domainprofile.State{Claude: domainprofile.AgentBinding{SourceProfile: "relay-a"}}}, nil, nil, nil, nil).clearLocked(domainprofile.AgentClaude); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("clearLocked(missing claude) err=%v, want invalid agent", err)
	}
	if _, err := NewProfileService(repo, &fakeStateRepo{state: domainprofile.State{Gemini: domainprofile.AgentBinding{SourceProfile: "relay-a"}}}, nil, nil, nil, nil).clearLocked(domainprofile.AgentGemini); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("clearLocked(missing gemini) err=%v, want invalid agent", err)
	}

	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.removeErr = errors.New("remove failed")
	claude := &fakeClaudeSyncer{claudeBase}
	if _, err := NewProfileService(repo, &fakeStateRepo{state: domainprofile.State{Claude: domainprofile.AgentBinding{SourceProfile: "relay-a"}}}, nil, claude, nil, nil).clearLocked(domainprofile.AgentClaude); err == nil || !strings.Contains(err.Error(), "remove failed") {
		t.Fatalf("clearLocked(remove failure) err=%v, want remove failed", err)
	}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.deleteBackupErr = errors.New("delete backup failed")
	codex = &fakeCodexSyncer{codexBase}
	if _, err := NewProfileService(repo, &fakeStateRepo{state: bindingState}, codex, nil, nil, nil).clearLocked(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "delete backup failed") {
		t.Fatalf("clearLocked(trim cleanup failure) err=%v, want delete backup failed", err)
	}

	journalErr := &errorJournal{updateErr: errors.New("update failed")}
	codex = &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc = NewProfileService(repo, &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{BindingView: domainprofile.BindingView{SourceProfile: "relay-a"}},
	}}, codex, nil, nil, nil)
	svc.SetOperationJournal(journalErr)
	if _, err := svc.clearLocked(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("clearLocked(update failure) err=%v, want update failed", err)
	}

	journalErr = &errorJournal{clearErr: errors.New("clear failed")}
	codex = &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc = NewProfileService(repo, &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{BindingView: domainprofile.BindingView{SourceProfile: "relay-a"}},
	}}, codex, nil, nil, nil)
	svc.SetOperationJournal(journalErr)
	if _, err := svc.clearLocked(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "clear failed") {
		t.Fatalf("clearLocked(clear failure) err=%v, want clear failed", err)
	}

	codex = &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc = NewProfileService(repo, &errorStateRepo{saveErr: errors.New("save failed"), state: domainprofile.State{
		Codex: domainprofile.CodexState{BindingView: domainprofile.BindingView{SourceProfile: "relay-a"}},
	}}, codex, nil, nil, nil)
	if _, err := svc.clearLocked(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "save failed") {
		t.Fatalf("clearLocked(save failure) err=%v, want save failed", err)
	}

	claude = &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	svc = NewProfileService(repo, &fakeStateRepo{}, nil, claude, nil, nil)
	if _, _, _, err := svc.syncProfileToAgent(domainprofile.AgentCodex, repo.profiles["relay-a"], "backup-1", domainprofile.State{}, false); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("syncProfileToAgent(missing codex) err=%v, want invalid agent", err)
	}
	if _, _, _, err := svc.syncProfileToAgent(domainprofile.AgentGemini, repo.profiles["relay-a"], "backup-1", domainprofile.State{}, false); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("syncProfileToAgent(missing gemini) err=%v, want invalid agent", err)
	}
	if _, _, _, err := svc.syncProfileToAgent(domainprofile.Agent("bad"), repo.profiles["relay-a"], "backup-1", domainprofile.State{}, false); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("syncProfileToAgent(invalid agent) err=%v, want invalid agent", err)
	}

	codexBase = newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.syncErr = errors.New("codex sync failed")
	codex = &fakeCodexSyncer{codexBase}
	if _, _, _, err := NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil, nil).syncProfileToAgent(domainprofile.AgentCodex, repo.profiles["relay-a"], "backup-1", domainprofile.State{}, true); err == nil || !strings.Contains(err.Error(), "codex sync failed") {
		t.Fatalf("syncProfileToAgent(codex sync failure) err=%v, want codex sync failed", err)
	}

	geminiBase := newFakeAgentSyncer("/tmp/gemini/.env")
	geminiBase.syncErr = errors.New("gemini sync failed")
	gemini := &fakeGeminiSyncer{geminiBase}
	if _, _, _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, gemini, nil).syncProfileToAgent(domainprofile.AgentGemini, repo.profiles["relay-a"], "backup-1", domainprofile.State{}, false); err == nil || !strings.Contains(err.Error(), "gemini sync failed") {
		t.Fatalf("syncProfileToAgent(gemini sync failure) err=%v, want gemini sync failed", err)
	}

	svc = NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil)
	if _, err := svc.snapshotCurrentConfig(domainprofile.AgentCodex, "relay-a", "backup-1", time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("snapshotCurrentConfig(missing codex) err=%v, want invalid agent", err)
	}
	if _, err := svc.snapshotCurrentConfig(domainprofile.AgentGemini, "relay-a", "backup-1", time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("snapshotCurrentConfig(missing gemini) err=%v, want invalid agent", err)
	}
	if _, err := svc.snapshotCurrentConfig(domainprofile.Agent("bad"), "relay-a", "backup-1", time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("snapshotCurrentConfig(invalid agent) err=%v, want invalid agent", err)
	}
}

func TestStateCoverageTargetedBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.syncErr = errors.New("codex sync failed")
	codex := &fakeCodexSyncer{codexBase}
	stateRepo := &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{BindingView: domainprofile.BindingView{SourceProfile: "relay-a"}},
	}}
	if err := NewProfileService(repo, stateRepo, codex, nil, nil, nil).syncProfileAfterMutation(repo.profiles["relay-a"], true); err == nil || !strings.Contains(err.Error(), "codex sync failed") {
		t.Fatalf("syncProfileAfterMutation(codex sync failure) err=%v, want codex sync failed", err)
	}

	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.syncErr = errors.New("claude sync failed")
	claude := &fakeClaudeSyncer{claudeBase}
	stateRepo = &fakeStateRepo{state: domainprofile.State{
		Claude: domainprofile.AgentBinding{SourceProfile: "relay-a"},
	}}
	if err := NewProfileService(repo, stateRepo, nil, claude, nil, nil).syncProfileAfterMutation(repo.profiles["relay-a"], false); err == nil || !strings.Contains(err.Error(), "claude sync failed") {
		t.Fatalf("syncProfileAfterMutation(bound agent sync failure) err=%v, want claude sync failed", err)
	}

	codex = &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.defaultProfile = "relay-a"
	codex.managedProfiles["relay-a"] = ports.CodexManagedProfile{Name: "relay-a", BaseURL: "https://relay.example/v1"}
	stateRepo = &fakeStateRepo{state: domainprofile.State{
		Codex:  domainprofile.CodexState{BindingView: domainprofile.BindingView{SourceProfile: "relay-a"}},
		Claude: domainprofile.AgentBinding{SourceProfile: "relay-a"},
		Gemini: domainprofile.AgentBinding{SourceProfile: "relay-a"},
	}}
	agents, err := NewProfileService(repo, stateRepo, codex, nil, nil, nil).boundAgentsForProfile("relay-a")
	if err != nil {
		t.Fatalf("boundAgentsForProfile() error = %v", err)
	}
	if len(agents) != 3 {
		t.Fatalf("boundAgentsForProfile() = %v, want all 3 agents", agents)
	}

	codexBase = newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.removeProfileErr = errors.New("remove profile failed")
	codex = &fakeCodexSyncer{codexBase}
	if err := NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil, nil).removeCodexProfileArtifacts("relay-a"); err == nil || !strings.Contains(err.Error(), "remove profile failed") {
		t.Fatalf("removeCodexProfileArtifacts(remove failure) err=%v, want remove profile failed", err)
	}
	if err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil, nil).removeCodexProfileArtifacts("relay-a"); err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("removeCodexProfileArtifacts(load failure) err=%v, want load failed", err)
	}

	state := domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{SourceProfile: "relay-a", Status: domainprofile.BindingStatusApplied},
		},
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).refreshCodexStateAfterRestore(&state); err != nil {
		t.Fatalf("refreshCodexStateAfterRestore(nil codex) error = %v", err)
	}
	codex = &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.statusErr = errors.New("status failed")
	if err := NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil, nil).refreshCodexStateAfterRestore(&state); err == nil || !strings.Contains(err.Error(), "status failed") {
		t.Fatalf("refreshCodexStateAfterRestore(status failure) err=%v, want status failed", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil, nil).refreshCodexStateAfterRestore(&domainprofile.State{}); err != nil {
		t.Fatalf("refreshCodexStateAfterRestore(nil status path) error = %v", err)
	}

	codexBase = newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.deleteBackupErr = errors.New("delete backup failed")
	codex = &fakeCodexSyncer{codexBase}
	stateWithBackups := domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{SourceProfile: "relay-b"},
			Backups: []domainprofile.Backup{
				{ID: "b1", BackupPath: "/tmp/b1"},
				{ID: "b2", BackupPath: "/tmp/b2"},
				{ID: "b3", BackupPath: "/tmp/b3"},
				{ID: "b4", BackupPath: "/tmp/b4"},
				{ID: "b5", BackupPath: "/tmp/b5"},
			},
		},
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil, nil).syncCodexProfile(repo.profiles["relay-a"], &stateWithBackups, time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "delete backup failed") {
		t.Fatalf("syncCodexProfile(trim cleanup failure) err=%v, want delete backup failed", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).syncCodexProfile(repo.profiles["relay-a"], &state, time.Now().UTC()); err != nil {
		t.Fatalf("syncCodexProfile(nil codex) error = %v", err)
	}
}

func TestProfileMutationCoverageTargetedBranches(t *testing.T) {
	baseProfile := domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{"relay-a": baseProfile}}

	stagedAddRepo := &stagedProfileRepo{
		getResults: []struct {
			profile *domainprofile.Profile
			err     error
		}{
			{err: &domainprofile.NotFoundError{Name: "relay-b"}},
			{err: errors.New("capture profile failed")},
		},
	}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	if _, err := NewProfileService(stagedAddRepo, &fakeStateRepo{}, codex, nil, nil, nil).Add("relay-b", AddProfileInput{
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-b",
		Bind:    []domainprofile.Agent{domainprofile.AgentCodex},
	}); err == nil || !strings.Contains(err.Error(), "capture profile failed") {
		t.Fatalf("Add(CaptureProfile failure) err=%v, want capture profile failed", err)
	}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotErr = errors.New("snapshot failed")
	if _, err := NewProfileService(&fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}, &fakeStateRepo{}, &fakeCodexSyncer{codexBase}, nil, nil, nil).Add("relay-b", AddProfileInput{
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-b",
		Bind:    []domainprofile.Agent{domainprofile.AgentCodex},
	}); err == nil || !strings.Contains(err.Error(), "snapshot failed") {
		t.Fatalf("Add(bind capture agent failure) err=%v, want snapshot failed", err)
	}

	stagedEditRepo := &stagedProfileRepo{
		profiles: map[string]domainprofile.Profile{"relay-a": baseProfile},
		getResults: []struct {
			profile *domainprofile.Profile
			err     error
		}{
			{profile: &baseProfile},
			{err: errors.New("capture profile failed")},
		},
	}
	if _, err := NewProfileService(stagedEditRepo, &fakeStateRepo{}, nil, nil, nil, nil).Edit("relay-a", EditProfileInput{APIKey: ptr("sk-new")}); err == nil || !strings.Contains(err.Error(), "capture profile failed") {
		t.Fatalf("Edit(CaptureProfile failure) err=%v, want capture profile failed", err)
	}

	svc := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil)
	if _, err := svc.Edit("relay-a", EditProfileInput{APIKey: ptr(" "), Bind: []domainprofile.Agent{domainprofile.AgentClaude}}); err == nil || !strings.Contains(err.Error(), "api key is required") {
		t.Fatalf("Edit(saveProfile failure) err=%v, want api key is required", err)
	}

	codexBase = newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotErr = errors.New("snapshot failed")
	stateRepo := &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{BindingView: domainprofile.BindingView{SourceProfile: "relay-a"}},
	}}
	codex = &fakeCodexSyncer{codexBase}
	codex.defaultProfile = "relay-a"
	codex.managedProfiles["relay-a"] = ports.CodexManagedProfile{Name: "relay-a", BaseURL: "https://relay.example/v1"}
	if _, err := NewProfileService(repo, stateRepo, codex, nil, nil, nil).Edit("relay-a", EditProfileInput{APIKey: ptr("sk-new")}); err == nil || !strings.Contains(err.Error(), "snapshot failed") {
		t.Fatalf("Edit(sync after mutation failure) err=%v, want snapshot failed", err)
	}

	if _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).Edit("relay-a", EditProfileInput{Bind: []domainprofile.Agent{domainprofile.AgentClaude}}); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("Edit(binding change failure) err=%v, want invalid agent", err)
	}

	stagedRemoveRepo := &stagedProfileRepo{
		profiles: map[string]domainprofile.Profile{"relay-a": baseProfile},
		getResults: []struct {
			profile *domainprofile.Profile
			err     error
		}{
			{profile: &baseProfile},
			{err: errors.New("capture profile failed")},
		},
	}
	if _, err := NewProfileService(stagedRemoveRepo, &fakeStateRepo{}, nil, nil, nil, nil).Remove("relay-a"); err == nil || !strings.Contains(err.Error(), "capture profile failed") {
		t.Fatalf("Remove(CaptureProfile failure) err=%v, want capture profile failed", err)
	}
	if _, err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, nil, nil, nil, nil).Remove("relay-a"); err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("Remove(boundAgents failure) err=%v, want load failed", err)
	}

	deleteErrRepo := &stagedProfileRepo{
		profiles:  map[string]domainprofile.Profile{"relay-a": baseProfile},
		deleteErr: errors.New("delete failed"),
	}
	if _, err := NewProfileService(deleteErrRepo, &fakeStateRepo{}, nil, nil, nil, nil).Remove("relay-a"); err == nil || !strings.Contains(err.Error(), "delete failed") {
		t.Fatalf("Remove(delete failure) err=%v, want delete failed", err)
	}
}

func TestRestoreDoctorAndGuardCoverageTargetedBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}

	if _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).Restore(domainprofile.Agent("bad"), ""); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("Restore(invalid agent) err=%v, want invalid agent", err)
	}
	if _, err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, nil, nil, nil, nil).Restore(domainprofile.AgentCodex, ""); err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("Restore(CaptureState failure) err=%v, want load failed", err)
	}
	if _, err := NewProfileService(repo, &fakeStateRepo{}, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil, nil).Restore(domainprofile.AgentCodex, ""); err == nil || !strings.Contains(err.Error(), "no codex backup available") {
		t.Fatalf("Restore(no backup) err=%v, want no backup", err)
	}

	state := &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			Backups: []domainprofile.Backup{{ID: "backup-1", RestoreMode: domainprofile.RestoreModeRemoveCreatedFile}},
		},
	}}
	if _, err := NewProfileService(repo, state, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil, nil).Restore(domainprofile.AgentCodex, "missing"); err == nil || !strings.Contains(err.Error(), "backup not found") {
		t.Fatalf("Restore(select backup failure) err=%v, want backup not found", err)
	}

	svc := NewProfileService(repo, state, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil, nil)
	svc.SetOperationJournal(&errorJournal{beginErr: errors.New("begin failed")})
	if _, err := svc.Restore(domainprofile.AgentCodex, "backup-1"); err == nil || !strings.Contains(err.Error(), "begin failed") {
		t.Fatalf("Restore(begin failure) err=%v, want begin failed", err)
	}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.removeErr = errors.New("remove failed")
	svc = NewProfileService(repo, state, &fakeCodexSyncer{codexBase}, nil, nil, nil)
	if _, err := svc.Restore(domainprofile.AgentCodex, "backup-1"); err == nil || !strings.Contains(err.Error(), "remove failed") {
		t.Fatalf("Restore(restore backup failure) err=%v, want remove failed", err)
	}

	svc = NewProfileService(repo, state, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil, nil)
	svc.SetOperationJournal(&errorJournal{updateErr: errors.New("update failed")})
	if _, err := svc.Restore(domainprofile.AgentCodex, "backup-1"); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("Restore(update failure) err=%v, want update failed", err)
	}

	svc = NewProfileService(repo, state, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil, nil)
	svc.SetOperationJournal(&errorJournal{clearErr: errors.New("clear failed")})
	if _, err := svc.Restore(domainprofile.AgentCodex, "backup-1"); err == nil || !strings.Contains(err.Error(), "clear failed") {
		t.Fatalf("Restore(clear failure) err=%v, want clear failed", err)
	}

	doctorSvc := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, nil, nil, nil, nil)
	if _, err := doctorSvc.Doctor(); err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("Doctor(State failure) err=%v, want load failed", err)
	}

	report := &DoctorReport{}
	if err := checkBackupMetadata(report, domainprofile.AgentClaude, domainprofile.Backup{ID: "backup-1", RestoreMode: domainprofile.RestoreModeRemoveCreatedFile}); err != nil {
		t.Fatalf("checkBackupMetadata(remove created) error = %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("checkBackupMetadata(remove created) issues=%+v, want none", report.Issues)
	}

	dir := t.TempDir()
	report = &DoctorReport{}
	missingPath := filepath.Join(dir, "missing.bak")
	if err := checkBackupMetadata(report, domainprofile.AgentClaude, domainprofile.Backup{
		ID:          "backup-2",
		RestoreMode: domainprofile.RestoreModeRestoreFile,
		BackupPath:  missingPath,
	}); err != nil {
		t.Fatalf("checkBackupMetadata(missing file) error = %v", err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Code != "missing_backup_file" {
		t.Fatalf("checkBackupMetadata(missing file) issues=%+v, want missing_backup_file", report.Issues)
	}

	existing := filepath.Join(dir, "codex.bak")
	if err := os.WriteFile(existing, []byte("backup"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	report = &DoctorReport{}
	if err := checkBackupMetadata(report, domainprofile.AgentCodex, domainprofile.Backup{
		ID:          "backup-3",
		RestoreMode: domainprofile.RestoreModeRestoreFile,
		BackupPath:  existing,
	}); err != nil {
		t.Fatalf("checkBackupMetadata(codex existing file) error = %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("checkBackupMetadata(codex existing file) issues=%+v, want none", report.Issues)
	}

	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).checkAgentBackups(&DoctorReport{}, domainprofile.State{
		Claude: domainprofile.AgentBinding{
			Backups: []domainprofile.Backup{{ID: "backup-4", RestoreMode: domainprofile.RestoreModeRestoreFile, BackupPath: existing}},
		},
	}); err != nil {
		t.Fatalf("checkAgentBackups(existing backup) error = %v", err)
	}

	geminiBase := newFakeAgentSyncer("/tmp/gemini/.env")
	geminiBase.snapshotErr = errors.New("snapshot failed")
	if _, _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, &fakeGeminiSyncer{geminiBase}, nil).snapshotAgentConfig(domainprofile.AgentGemini); err == nil || !strings.Contains(err.Error(), "snapshot failed") {
		t.Fatalf("snapshotAgentConfig(gemini snapshot error) err=%v, want snapshot failed", err)
	}

	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).restoreAgentSnapshot(domainprofile.AgentCodex, ports.AgentConfigSnapshot{Exists: false}); err != nil {
		t.Fatalf("restoreAgentSnapshot(missing codex syncer remove) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).restoreAgentSnapshot(domainprofile.AgentClaude, ports.AgentConfigSnapshot{Exists: false}); err != nil {
		t.Fatalf("restoreAgentSnapshot(missing claude syncer remove) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).restoreAgentSnapshot(domainprofile.AgentGemini, ports.AgentConfigSnapshot{Exists: false}); err != nil {
		t.Fatalf("restoreAgentSnapshot(missing gemini syncer remove) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).restoreAgentSnapshot(domainprofile.Agent("bad"), ports.AgentConfigSnapshot{Exists: false}); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("restoreAgentSnapshot(invalid remove agent) err=%v, want invalid agent", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).restoreAgentSnapshot(domainprofile.AgentCodex, ports.AgentConfigSnapshot{Exists: true, Content: []byte("restore")}); err != nil {
		t.Fatalf("restoreAgentSnapshot(missing codex syncer restore) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).restoreAgentSnapshot(domainprofile.AgentClaude, ports.AgentConfigSnapshot{Exists: true, Content: []byte("restore")}); err != nil {
		t.Fatalf("restoreAgentSnapshot(missing claude syncer restore) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).restoreAgentSnapshot(domainprofile.AgentGemini, ports.AgentConfigSnapshot{Exists: true, Content: []byte("restore")}); err != nil {
		t.Fatalf("restoreAgentSnapshot(missing gemini syncer restore) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).restoreAgentSnapshot(domainprofile.Agent("bad"), ports.AgentConfigSnapshot{Exists: true, Content: []byte("restore")}); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("restoreAgentSnapshot(invalid restore agent) err=%v, want invalid agent", err)
	}

	svc = NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil)
	svc.SetOperationJournal(&errorJournal{currentErr: errors.New("current failed")})
	if err := svc.clearCurrentOperationJournal(); err == nil || !strings.Contains(err.Error(), "current failed") {
		t.Fatalf("clearCurrentOperationJournal(current failure) err=%v, want current failed", err)
	}
}

func TestOpenCodeStateAndRuntimeCoverageTargetedBranches(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	old := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	profiles := []domainprofile.Profile{
		{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: old, UpdatedAt: now},
		{Name: "relay-b", BaseURL: "https://relay-b.example/v1", APIKey: "sk-b", CreatedAt: old, UpdatedAt: now},
		{Name: "relay-c", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: old, UpdatedAt: now},
	}

	if issues := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, nil, nil).enrichRuntimeState(nil, profiles); issues != nil {
		t.Fatalf("enrichRuntimeState(nil) = %+v, want nil", issues)
	}
	if NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, nil, nil).hasRuntimeManagedAgents() {
		t.Fatal("hasRuntimeManagedAgents() = true, want false")
	}
	if !NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, &fakeClaudeSyncer{}, nil, nil).hasRuntimeManagedAgents() {
		t.Fatal("hasRuntimeManagedAgents() = false, want true")
	}

	if got := runtimeSnapshotErrorIssue(domainprofile.AgentClaude, errors.New("boom")); got.Code != "runtime_config_unreadable" || !strings.Contains(got.Message, "boom") {
		t.Fatalf("runtimeSnapshotErrorIssue() = %+v", got)
	}
	if !hasIncompleteManagedBlock([]byte("prefix\n# >>> AGX managed Gemini env >>>\nGOOGLE_GEMINI_BASE_URL=x"), geminiManagedBlockBeginMarker, geminiManagedBlockEndMarker) {
		t.Fatal("hasIncompleteManagedBlock() = false, want true")
	}
	if relay := extractRelayNameFromHelper("agx __api-key relay-a"); relay != "relay-a" {
		t.Fatalf("extractRelayNameFromHelper() = %q, want relay-a", relay)
	}
	if relay := extractRelayNameFromHelper("agx __api-key"); relay != "" {
		t.Fatalf("extractRelayNameFromHelper(missing relay) = %q, want empty", relay)
	}

	helperRelay, baseURL, err := parseClaudeBindingSnapshot([]byte(`{"apiKeyHelper":"agx __api-key relay-a","env":{"ANTHROPIC_BASE_URL":"https://relay.example"}}`))
	if err != nil || helperRelay != "relay-a" || baseURL != "https://relay.example" {
		t.Fatalf("parseClaudeBindingSnapshot() = (%q,%q,%v)", helperRelay, baseURL, err)
	}
	if _, _, err := parseClaudeBindingSnapshot([]byte("{")); err == nil {
		t.Fatal("parseClaudeBindingSnapshot(invalid JSON) unexpectedly succeeded")
	}

	if baseURL, apiKey := parseGeminiBindingSnapshot([]byte("GOOGLE_GEMINI_BASE_URL='https://relay.example'\nGEMINI_API_KEY=\" sk-a \"")); baseURL != "https://relay.example" || apiKey != "sk-a" {
		t.Fatalf("parseGeminiBindingSnapshot() = (%q,%q), want normalized values", baseURL, apiKey)
	}
	bundle, err := json.Marshal(map[string]any{
		"format": geminiSnapshotBundleFormat,
		"files": map[string]string{
			".env": "GOOGLE_GEMINI_BASE_URL='https://relay-b.example'\nGEMINI_API_KEY=\"sk-b\"\n",
		},
	})
	if err != nil {
		t.Fatalf("Marshal(bundle) error = %v", err)
	}
	if baseURL, apiKey := parseGeminiBindingSnapshot(bundle); baseURL != "https://relay-b.example" || apiKey != "sk-b" {
		t.Fatalf("parseGeminiBindingSnapshot(bundle) = (%q,%q), want normalized bundle values", baseURL, apiKey)
	}
	if got := string(geminiSnapshotEnvContent(bundle)); !strings.Contains(got, "GEMINI_API_KEY=\"sk-b\"") {
		t.Fatalf("geminiSnapshotEnvContent(bundle) = %q, want .env payload", got)
	}
	assignments := parseEnvAssignments([]byte("export FOO='bar'\nBAZ=qux\n# comment\nBROKEN\n"))
	if assignments["FOO"] != "bar" || assignments["BAZ"] != "qux" || len(assignments) != 2 {
		t.Fatalf("parseEnvAssignments() = %+v", assignments)
	}
	if got := parseEnvValue(`"quoted"`); got != "quoted" {
		t.Fatalf("parseEnvValue(double quoted) = %q, want quoted", got)
	}
	if got := parseEnvValue("'single quoted'"); got != "single quoted" {
		t.Fatalf("parseEnvValue(single quoted) = %q, want single quoted", got)
	}
	if got := parseEnvValue("plain"); got != "plain" {
		t.Fatalf("parseEnvValue(plain) = %q, want plain", got)
	}

	if relay, outcome := chooseClaudeRelay(profiles, "relay-a", "https://relay.example", "relay-a"); relay != "relay-a" || outcome != claudeBindingResolved {
		t.Fatalf("chooseClaudeRelay(resolved) = (%q,%q)", relay, outcome)
	}
	if relay, outcome := chooseClaudeRelay(profiles, "relay-z", "https://relay.example", "relay-a"); relay != "relay-a" || outcome != claudeBindingStaleHelper {
		t.Fatalf("chooseClaudeRelay(stale helper) = (%q,%q)", relay, outcome)
	}
	if relay, outcome := chooseClaudeRelay(profiles, "relay-a", "https://other.example", "relay-a"); relay != "relay-a" || outcome != claudeBindingConflict {
		t.Fatalf("chooseClaudeRelay(conflict) = (%q,%q)", relay, outcome)
	}
	if relay, outcome := chooseClaudeRelay(profiles, "", "", ""); relay != "" || outcome != claudeBindingNone {
		t.Fatalf("chooseClaudeRelay(none) = (%q,%q)", relay, outcome)
	}
	if relay, ambiguous := chooseGeminiRelay(profiles, "https://relay-b.example", "sk-b", ""); relay != "relay-b" || ambiguous {
		t.Fatalf("chooseGeminiRelay(resolved) = (%q,%v)", relay, ambiguous)
	}
	if relay, ambiguous := chooseGeminiRelay(profiles, "https://relay.example", "sk-a", "relay-c"); relay != "relay-c" || !ambiguous {
		t.Fatalf("chooseGeminiRelay(ambiguous current) = (%q,%v)", relay, ambiguous)
	}
	if relay, ambiguous := chooseGeminiRelay(profiles, "", "", "relay-a"); relay != "" || ambiguous {
		t.Fatalf("chooseGeminiRelay(missing data) = (%q,%v)", relay, ambiguous)
	}
	if matches := matchingProfilesByBaseURL(profiles, "https://relay.example"); len(matches) != 2 || !containsProfile(matches, "relay-a") || !containsProfile(matches, "relay-c") {
		t.Fatalf("matchingProfilesByBaseURL() = %+v", matches)
	}
	if !profileMatchesAgentBaseURL(profiles, "relay-a", domainprofile.AgentClaude, "https://relay.example") {
		t.Fatal("profileMatchesAgentBaseURL() = false, want true")
	}
	if !profileBaseURLMatchesAgent(profiles[0], domainprofile.AgentOpenCode, "https://relay.example/v1") {
		t.Fatal("profileBaseURLMatchesAgent(openCode) = false, want true")
	}

	state := &domainprofile.State{
		Claude: domainprofile.AgentBinding{
			SourceProfile: "relay-a",
			Status:        domainprofile.BindingStatusApplied,
			ConfigPath:    "/tmp/claude/settings.json",
			LastAppliedAt: old,
			LastBackupID:  "backup-old",
		},
		Gemini: domainprofile.AgentBinding{
			SourceProfile: "relay-b",
			Status:        domainprofile.BindingStatusApplied,
			ConfigPath:    "/tmp/gemini/.env",
			LastAppliedAt: old,
			LastBackupID:  "backup-old",
		},
	}
	svc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil,
		&fakeClaudeSyncer{&fakeAgentSyncer{configPath: "/tmp/claude/settings.json", snapshotExists: true, snapshotContent: []byte(`{"apiKeyHelper":"agx __api-key relay-a","env":{"ANTHROPIC_BASE_URL":"https://relay.example"}}`)}},
		&fakeGeminiSyncer{&fakeAgentSyncer{configPath: "/tmp/gemini/.env", snapshotExists: true, snapshotContent: []byte("GOOGLE_GEMINI_BASE_URL='https://relay-b.example'\nGEMINI_API_KEY=\"sk-b\"\n")}},
		nil,
	)
	issues := svc.enrichRuntimeState(state, profiles)
	if len(issues) != 0 {
		t.Fatalf("enrichRuntimeState() issues=%+v, want none", issues)
	}
	if state.Claude.SourceProfile != "relay-a" || state.Gemini.SourceProfile != "relay-b" {
		t.Fatalf("runtime state bindings = %+v, want relay-a", state)
	}

	claudeErr := &fakeClaudeSyncer{&fakeAgentSyncer{configPath: "/tmp/claude/settings.json", snapshotErr: errors.New("snap failed")}}
	claudeState := &domainprofile.State{Claude: domainprofile.AgentBinding{SourceProfile: "relay-a", Status: domainprofile.BindingStatusApplied, ConfigPath: "/tmp/old", LastAppliedAt: old, LastBackupID: "backup-old"}}
	issues = NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, claudeErr, nil, nil).applyRuntimeAgentBinding(claudeState, profiles, domainprofile.AgentClaude)
	if len(issues) != 1 || issues[0].Code != "runtime_config_unreadable" {
		t.Fatalf("applyRuntimeAgentBinding(snapshot error) issues=%+v", issues)
	}
	if claudeState.Claude.SourceProfile != "relay-a" || claudeState.Claude.ConfigPath != "/tmp/old" || claudeState.Claude.LastBackupID != "backup-old" {
		t.Fatalf("applyRuntimeAgentBinding(snapshot error) state=%+v, want preserve existing", claudeState.Claude)
	}

	geminiMissing := &fakeGeminiSyncer{&fakeAgentSyncer{configPath: "/tmp/gemini/.env", snapshotExists: false}}
	geminiState := &domainprofile.State{Gemini: domainprofile.AgentBinding{SourceProfile: "relay-a", Status: domainprofile.BindingStatusApplied, ConfigPath: "/tmp/old", LastAppliedAt: old, LastBackupID: "backup-old"}}
	issues = NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, geminiMissing, nil).applyRuntimeAgentBinding(geminiState, profiles, domainprofile.AgentGemini)
	if len(issues) != 1 || issues[0].Code != "runtime_binding_missing" {
		t.Fatalf("applyRuntimeAgentBinding(missing config) issues=%+v", issues)
	}
	if geminiState.Gemini.SourceProfile != "" || geminiState.Gemini.ConfigPath != "" || !geminiState.Gemini.LastAppliedAt.IsZero() || geminiState.Gemini.LastBackupID != "" {
		t.Fatalf("applyRuntimeAgentBinding(missing config) state=%+v, want cleared", geminiState.Gemini)
	}

	resolvedClaude := &fakeClaudeSyncer{&fakeAgentSyncer{configPath: "/tmp/claude/settings.json", snapshotExists: true, snapshotContent: []byte(`{"apiKeyHelper":"agx __api-key relay-a","env":{"ANTHROPIC_BASE_URL":"https://relay.example"}}`)}}
	claudeState = &domainprofile.State{Claude: domainprofile.AgentBinding{SourceProfile: "relay-a", Status: "", ConfigPath: "/tmp/old", LastAppliedAt: old, LastBackupID: "backup-old"}}
	issues = NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, resolvedClaude, nil, nil).applyRuntimeAgentBinding(claudeState, profiles, domainprofile.AgentClaude)
	if len(issues) != 0 {
		t.Fatalf("applyRuntimeAgentBinding(resolved) issues=%+v, want none", issues)
	}
	if claudeState.Claude.SourceProfile != "relay-a" || claudeState.Claude.ConfigPath != "/tmp/claude/settings.json" || !claudeState.Claude.LastAppliedAt.IsZero() || claudeState.Claude.LastBackupID != "" {
		t.Fatalf("applyRuntimeAgentBinding(resolved stale metadata) state=%+v", claudeState.Claude)
	}
}


func TestOpenCodeProfileNormalizationAndSyncCoverageTargetedBranches(t *testing.T) {
	now := time.Date(2026, 5, 6, 13, 0, 0, 0, time.UTC)
	normalized := normalizeOpenCodeProfileBinding("relay-open", nil)
	if normalized.ProviderID != "agx-relay-open" || normalized.ProviderFamily != domainprofile.OpenCodeProviderFamilyOpenAICompatible || normalized.ModelID != "relay-open" || normalized.ModelName != "relay-open" {
		t.Fatalf("normalizeOpenCodeProfileBinding(nil) = %+v", normalized)
	}
	normalized = normalizeOpenCodeProfileBinding("relay-open", &domainprofile.OpenCodeProfileBinding{
		ModelID:   "gpt-4o",
		ModelName: "GPT-4o",
	})
	if normalized.ProviderID != "agx-relay-open" || normalized.ProviderFamily != domainprofile.OpenCodeProviderFamilyOpenAICompatible || normalized.ModelID != "gpt-4o" || normalized.ModelName != "GPT-4o" {
		t.Fatalf("normalizeOpenCodeProfileBinding(custom) = %+v", normalized)
	}

	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-open": {
			Name:           "relay-open",
			BaseURL:        "https://relay.example",
			APIKey:         "sk-open",
			ModelID:        "model-a",
			ProviderFamily: domainprofile.OpenCodeProviderFamilyGemini,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}}
	state := &domainprofile.State{
		OpenCode: domainprofile.OpenCodeState{
			BindingView: domainprofile.BindingView{
				SourceProfile: "relay-open",
				Status:        domainprofile.BindingStatusApplied,
				ConfigPath:    "/tmp/opencode.json",
				LastAppliedAt: now,
				LastBackupID:  "backup-old",
			},
		},
	}
	openCode := &fakeOpenCodeSyncer{fakeAgentSyncer: newFakeAgentSyncer("/tmp/opencode.json")}
	openCode.status = &ports.OpenCodeConfigStatus{
		ConfigPath:   "/tmp/opencode.json",
		DefaultModel: "agx-relay-open/model-a",
		ManagedProvidersByID: map[string]ports.OpenCodeManagedProvider{
			"agx-relay-open": {ID: "agx-relay-open", Family: domainprofile.OpenCodeProviderFamilyGemini, Model: "model-a"},
		},
	}
	openCode.snapshotContent = []byte("before opencode")
	svc := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, openCode)
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).syncOpenCodeProfile(repo.profiles["relay-open"], &domainprofile.State{}, now); err != nil {
		t.Fatalf("syncOpenCodeProfile(nil openCode) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).refreshOpenCodeStateAfterRestore(&domainprofile.State{}); err != nil {
		t.Fatalf("refreshOpenCodeStateAfterRestore(nil openCode) error = %v", err)
	}

	if err := svc.refreshOpenCodeStateAfterRestore(state); err != nil {
		t.Fatalf("refreshOpenCodeStateAfterRestore() error = %v", err)
	}
	if state.OpenCode.SourceProfile != "relay-open" || state.OpenCode.ConfigPath != "/tmp/opencode.json" {
		t.Fatalf("refreshOpenCodeStateAfterRestore() state=%+v", state.OpenCode)
	}

	state.OpenCode.SourceProfile = "relay-open"
	if err := svc.syncOpenCodeProfile(repo.profiles["relay-open"], state, now); err != nil {
		t.Fatalf("syncOpenCodeProfile() error = %v", err)
	}
	if openCode.syncCalls != 1 {
		t.Fatalf("openCode syncCalls = %d, want 1", openCode.syncCalls)
	}
	if openCode.lastOptions.ProviderFamily != domainprofile.OpenCodeProviderFamilyGemini || openCode.lastOptions.ModelID != "model-a" || openCode.lastOptions.ModelName != "model-a" || openCode.lastOptions.ProviderName != "relay-open" || !openCode.lastOptions.SetAsCurrent {
		t.Fatalf("syncOpenCodeProfile options = %+v", openCode.lastOptions)
	}
	if state.OpenCode.SourceProfile != "relay-open" || state.OpenCode.LastBackupID == "" {
		t.Fatalf("syncOpenCodeProfile state=%+v", state.OpenCode)
	}

	beforeRename := domainprofile.State{
		OpenCode: state.OpenCode,
	}
	if err := svc.removeOpenCodeProfileAfterRename("relay-open", beforeRename); err != nil {
		t.Fatalf("removeOpenCodeProfileAfterRename() error = %v", err)
	}
	if len(openCode.removeProfileCalls) != 1 || openCode.removeProfileCalls[0] != "relay-open" {
		t.Fatalf("removeOpenCodeProfileAfterRename removeProfileCalls=%+v", openCode.removeProfileCalls)
	}
}
