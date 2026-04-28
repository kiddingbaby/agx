package usecase

import (
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

	if _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).agentSetLocked(domainprofile.AgentCodex, "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("agentSetLocked(Get failure) err=%v, want not found", err)
	}

	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc := NewProfileService(repo, &errorStateRepo{saveErr: errors.New("save failed")}, codex, nil, nil)
	if _, err := svc.agentSetLocked(domainprofile.AgentCodex, "relay-a"); err == nil || !strings.Contains(err.Error(), "save failed") {
		t.Fatalf("agentSetLocked(save failure) err=%v, want save failed", err)
	}

	if _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).clearLocked(domainprofile.Agent("bad")); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("clearLocked(invalid agent) err=%v, want invalid agent", err)
	}
	if _, err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, nil, nil, nil).clearLocked(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "load failed") {
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
	if _, err := NewProfileService(repo, &fakeStateRepo{state: bindingState}, nil, nil, nil).clearLocked(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("clearLocked(missing codex) err=%v, want invalid agent", err)
	}
	if _, err := NewProfileService(repo, &fakeStateRepo{state: domainprofile.State{Claude: domainprofile.AgentBinding{SourceProfile: "relay-a"}}}, nil, nil, nil).clearLocked(domainprofile.AgentClaude); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("clearLocked(missing claude) err=%v, want invalid agent", err)
	}
	if _, err := NewProfileService(repo, &fakeStateRepo{state: domainprofile.State{Gemini: domainprofile.AgentBinding{SourceProfile: "relay-a"}}}, nil, nil, nil).clearLocked(domainprofile.AgentGemini); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("clearLocked(missing gemini) err=%v, want invalid agent", err)
	}

	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.removeErr = errors.New("remove failed")
	claude := &fakeClaudeSyncer{claudeBase}
	if _, err := NewProfileService(repo, &fakeStateRepo{state: domainprofile.State{Claude: domainprofile.AgentBinding{SourceProfile: "relay-a"}}}, nil, claude, nil).clearLocked(domainprofile.AgentClaude); err == nil || !strings.Contains(err.Error(), "remove failed") {
		t.Fatalf("clearLocked(remove failure) err=%v, want remove failed", err)
	}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.deleteBackupErr = errors.New("delete backup failed")
	codex = &fakeCodexSyncer{codexBase}
	if _, err := NewProfileService(repo, &fakeStateRepo{state: bindingState}, codex, nil, nil).clearLocked(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "delete backup failed") {
		t.Fatalf("clearLocked(trim cleanup failure) err=%v, want delete backup failed", err)
	}

	journalErr := &errorJournal{updateErr: errors.New("update failed")}
	codex = &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc = NewProfileService(repo, &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{BindingView: domainprofile.BindingView{SourceProfile: "relay-a"}},
	}}, codex, nil, nil)
	svc.SetOperationJournal(journalErr)
	if _, err := svc.clearLocked(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("clearLocked(update failure) err=%v, want update failed", err)
	}

	journalErr = &errorJournal{clearErr: errors.New("clear failed")}
	codex = &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc = NewProfileService(repo, &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{BindingView: domainprofile.BindingView{SourceProfile: "relay-a"}},
	}}, codex, nil, nil)
	svc.SetOperationJournal(journalErr)
	if _, err := svc.clearLocked(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "clear failed") {
		t.Fatalf("clearLocked(clear failure) err=%v, want clear failed", err)
	}

	codex = &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc = NewProfileService(repo, &errorStateRepo{saveErr: errors.New("save failed"), state: domainprofile.State{
		Codex: domainprofile.CodexState{BindingView: domainprofile.BindingView{SourceProfile: "relay-a"}},
	}}, codex, nil, nil)
	if _, err := svc.clearLocked(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "save failed") {
		t.Fatalf("clearLocked(save failure) err=%v, want save failed", err)
	}

	claude = &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	svc = NewProfileService(repo, &fakeStateRepo{}, nil, claude, nil)
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
	if _, _, _, err := NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil).syncProfileToAgent(domainprofile.AgentCodex, repo.profiles["relay-a"], "backup-1", domainprofile.State{}, true); err == nil || !strings.Contains(err.Error(), "codex sync failed") {
		t.Fatalf("syncProfileToAgent(codex sync failure) err=%v, want codex sync failed", err)
	}

	geminiBase := newFakeAgentSyncer("/tmp/gemini/.env")
	geminiBase.syncErr = errors.New("gemini sync failed")
	gemini := &fakeGeminiSyncer{geminiBase}
	if _, _, _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, gemini).syncProfileToAgent(domainprofile.AgentGemini, repo.profiles["relay-a"], "backup-1", domainprofile.State{}, false); err == nil || !strings.Contains(err.Error(), "gemini sync failed") {
		t.Fatalf("syncProfileToAgent(gemini sync failure) err=%v, want gemini sync failed", err)
	}

	gemini = &fakeGeminiSyncer{newFakeAgentSyncer("/tmp/gemini/.env")}
	svc = NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil)
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
		CodexProfiles: map[string]domainprofile.CodexProfileBinding{"relay-a": {Status: domainprofile.BindingStatusApplied}},
	}}
	if err := NewProfileService(repo, stateRepo, codex, nil, nil).syncProfileAfterMutation(repo.profiles["relay-a"], true); err == nil || !strings.Contains(err.Error(), "codex sync failed") {
		t.Fatalf("syncProfileAfterMutation(codex sync failure) err=%v, want codex sync failed", err)
	}

	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.syncErr = errors.New("claude sync failed")
	claude := &fakeClaudeSyncer{claudeBase}
	stateRepo = &fakeStateRepo{state: domainprofile.State{
		Claude: domainprofile.AgentBinding{SourceProfile: "relay-a"},
	}}
	if err := NewProfileService(repo, stateRepo, nil, claude, nil).syncProfileAfterMutation(repo.profiles["relay-a"], false); err == nil || !strings.Contains(err.Error(), "claude sync failed") {
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
	agents, err := NewProfileService(repo, stateRepo, codex, nil, nil).boundAgentsForProfile("relay-a")
	if err != nil {
		t.Fatalf("boundAgentsForProfile() error = %v", err)
	}
	if len(agents) != 3 {
		t.Fatalf("boundAgentsForProfile() = %v, want all 3 agents", agents)
	}

	codexBase = newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.removeProfileErr = errors.New("remove profile failed")
	codex = &fakeCodexSyncer{codexBase}
	if err := NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil).removeCodexProfileArtifacts("relay-a"); err == nil || !strings.Contains(err.Error(), "remove profile failed") {
		t.Fatalf("removeCodexProfileArtifacts(remove failure) err=%v, want remove profile failed", err)
	}
	if err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil).removeCodexProfileArtifacts("relay-a"); err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("removeCodexProfileArtifacts(load failure) err=%v, want load failed", err)
	}

	state := domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{SourceProfile: "relay-a", Status: domainprofile.BindingStatusApplied},
		},
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).refreshCodexStateAfterRestore(&state); err != nil {
		t.Fatalf("refreshCodexStateAfterRestore(nil codex) error = %v", err)
	}
	codex = &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.statusErr = errors.New("status failed")
	if err := NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil).refreshCodexStateAfterRestore(&state); err == nil || !strings.Contains(err.Error(), "status failed") {
		t.Fatalf("refreshCodexStateAfterRestore(status failure) err=%v, want status failed", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil).refreshCodexStateAfterRestore(&domainprofile.State{}); err != nil {
		t.Fatalf("refreshCodexStateAfterRestore(nil status path) error = %v", err)
	}

	resolved := resolveCodexProfiles(map[string]domainprofile.CodexProfileBinding{
		"Relay-A": {Status: domainprofile.BindingStatusApplied},
	}, nil)
	if _, ok := resolved["relay-a"]; !ok {
		t.Fatalf("resolveCodexProfiles(nil status) = %+v, want normalized stored binding", resolved)
	}

	resolved = resolveCodexProfiles(map[string]domainprofile.CodexProfileBinding{
		"relay-a": {},
		"relay-b": {},
	}, &ports.CodexConfigStatus{
		ConfigPath:          "/tmp/codex/config.toml",
		ManagedProfilesByID: map[string]ports.CodexManagedProfile{"relay-a": {Name: "relay-a"}},
	})
	if binding := resolved["relay-a"]; binding.Status != domainprofile.BindingStatusApplied || binding.ConfigPath == "" {
		t.Fatalf("resolveCodexProfiles(managed) relay-a=%+v, want applied with config path", binding)
	}
	if _, ok := resolved["relay-b"]; ok {
		t.Fatalf("resolveCodexProfiles() kept unmanaged binding: %+v", resolved)
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
	if err := NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil).syncCodexProfile(repo.profiles["relay-a"], &stateWithBackups, time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "delete backup failed") {
		t.Fatalf("syncCodexProfile(trim cleanup failure) err=%v, want delete backup failed", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).syncCodexProfile(repo.profiles["relay-a"], &state, time.Now().UTC()); err != nil {
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
	if _, err := NewProfileService(stagedAddRepo, &fakeStateRepo{}, nil, nil, nil).Add("relay-b", AddProfileInput{BaseURL: "https://relay.example/v1", APIKey: "sk-b"}); err == nil || !strings.Contains(err.Error(), "capture profile failed") {
		t.Fatalf("Add(CaptureProfile failure) err=%v, want capture profile failed", err)
	}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotErr = errors.New("snapshot failed")
	if _, err := NewProfileService(&fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}, &fakeStateRepo{}, &fakeCodexSyncer{codexBase}, nil, nil).Add("relay-b", AddProfileInput{
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
	if _, err := NewProfileService(stagedEditRepo, &fakeStateRepo{}, nil, nil, nil).Edit("relay-a", EditProfileInput{APIKey: ptr("sk-new")}); err == nil || !strings.Contains(err.Error(), "capture profile failed") {
		t.Fatalf("Edit(CaptureProfile failure) err=%v, want capture profile failed", err)
	}

	svc := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil)
	if _, err := svc.Edit("relay-a", EditProfileInput{APIKey: ptr(" "), Bind: []domainprofile.Agent{domainprofile.AgentClaude}}); err == nil || !strings.Contains(err.Error(), "api key is required") {
		t.Fatalf("Edit(saveProfile failure) err=%v, want api key is required", err)
	}

	codexBase = newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotErr = errors.New("snapshot failed")
	stateRepo := &fakeStateRepo{state: domainprofile.State{
		CodexProfiles: map[string]domainprofile.CodexProfileBinding{"relay-a": {Status: domainprofile.BindingStatusApplied}},
	}}
	if _, err := NewProfileService(repo, stateRepo, &fakeCodexSyncer{codexBase}, nil, nil).Edit("relay-a", EditProfileInput{APIKey: ptr("sk-new")}); err == nil || !strings.Contains(err.Error(), "snapshot failed") {
		t.Fatalf("Edit(sync after mutation failure) err=%v, want snapshot failed", err)
	}

	if _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).Edit("relay-a", EditProfileInput{Bind: []domainprofile.Agent{domainprofile.AgentClaude}}); err == nil || !strings.Contains(err.Error(), "invalid agent") {
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
	if _, err := NewProfileService(stagedRemoveRepo, &fakeStateRepo{}, nil, nil, nil).Remove("relay-a"); err == nil || !strings.Contains(err.Error(), "capture profile failed") {
		t.Fatalf("Remove(CaptureProfile failure) err=%v, want capture profile failed", err)
	}
	if _, err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, nil, nil, nil).Remove("relay-a"); err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("Remove(boundAgents failure) err=%v, want load failed", err)
	}

	deleteErrRepo := &stagedProfileRepo{
		profiles:  map[string]domainprofile.Profile{"relay-a": baseProfile},
		deleteErr: errors.New("delete failed"),
	}
	if _, err := NewProfileService(deleteErrRepo, &fakeStateRepo{}, nil, nil, nil).Remove("relay-a"); err == nil || !strings.Contains(err.Error(), "delete failed") {
		t.Fatalf("Remove(delete failure) err=%v, want delete failed", err)
	}
}

func TestRestoreDoctorAndGuardCoverageTargetedBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}

	if _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).Restore(domainprofile.Agent("bad"), ""); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("Restore(invalid agent) err=%v, want invalid agent", err)
	}
	if _, err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, nil, nil, nil).Restore(domainprofile.AgentCodex, ""); err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("Restore(CaptureState failure) err=%v, want load failed", err)
	}
	if _, err := NewProfileService(repo, &fakeStateRepo{}, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil).Restore(domainprofile.AgentCodex, ""); err == nil || !strings.Contains(err.Error(), "no codex backup available") {
		t.Fatalf("Restore(no backup) err=%v, want no backup", err)
	}

	state := &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			Backups: []domainprofile.Backup{{ID: "backup-1", RestoreMode: domainprofile.RestoreModeRemoveCreatedFile}},
		},
	}}
	if _, err := NewProfileService(repo, state, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil).Restore(domainprofile.AgentCodex, "missing"); err == nil || !strings.Contains(err.Error(), "backup not found") {
		t.Fatalf("Restore(select backup failure) err=%v, want backup not found", err)
	}

	svc := NewProfileService(repo, state, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil)
	svc.SetOperationJournal(&errorJournal{beginErr: errors.New("begin failed")})
	if _, err := svc.Restore(domainprofile.AgentCodex, "backup-1"); err == nil || !strings.Contains(err.Error(), "begin failed") {
		t.Fatalf("Restore(begin failure) err=%v, want begin failed", err)
	}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.removeErr = errors.New("remove failed")
	svc = NewProfileService(repo, state, &fakeCodexSyncer{codexBase}, nil, nil)
	if _, err := svc.Restore(domainprofile.AgentCodex, "backup-1"); err == nil || !strings.Contains(err.Error(), "remove failed") {
		t.Fatalf("Restore(restore backup failure) err=%v, want remove failed", err)
	}

	svc = NewProfileService(repo, state, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil)
	svc.SetOperationJournal(&errorJournal{updateErr: errors.New("update failed")})
	if _, err := svc.Restore(domainprofile.AgentCodex, "backup-1"); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("Restore(update failure) err=%v, want update failed", err)
	}

	svc = NewProfileService(repo, state, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil)
	svc.SetOperationJournal(&errorJournal{clearErr: errors.New("clear failed")})
	if _, err := svc.Restore(domainprofile.AgentCodex, "backup-1"); err == nil || !strings.Contains(err.Error(), "clear failed") {
		t.Fatalf("Restore(clear failure) err=%v, want clear failed", err)
	}

	doctorSvc := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, nil, nil, nil)
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

	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).checkAgentBackups(&DoctorReport{}, domainprofile.State{
		Claude: domainprofile.AgentBinding{
			Backups: []domainprofile.Backup{{ID: "backup-4", RestoreMode: domainprofile.RestoreModeRestoreFile, BackupPath: existing}},
		},
	}); err != nil {
		t.Fatalf("checkAgentBackups(existing backup) error = %v", err)
	}

	geminiBase := newFakeAgentSyncer("/tmp/gemini/.env")
	geminiBase.snapshotErr = errors.New("snapshot failed")
	if _, _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, &fakeGeminiSyncer{geminiBase}).snapshotAgentConfig(domainprofile.AgentGemini); err == nil || !strings.Contains(err.Error(), "snapshot failed") {
		t.Fatalf("snapshotAgentConfig(gemini snapshot error) err=%v, want snapshot failed", err)
	}

	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).restoreAgentSnapshot(domainprofile.AgentCodex, ports.AgentConfigSnapshot{Exists: false}); err != nil {
		t.Fatalf("restoreAgentSnapshot(missing codex syncer remove) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).restoreAgentSnapshot(domainprofile.AgentClaude, ports.AgentConfigSnapshot{Exists: false}); err != nil {
		t.Fatalf("restoreAgentSnapshot(missing claude syncer remove) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).restoreAgentSnapshot(domainprofile.AgentGemini, ports.AgentConfigSnapshot{Exists: false}); err != nil {
		t.Fatalf("restoreAgentSnapshot(missing gemini syncer remove) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).restoreAgentSnapshot(domainprofile.Agent("bad"), ports.AgentConfigSnapshot{Exists: false}); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("restoreAgentSnapshot(invalid remove agent) err=%v, want invalid agent", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).restoreAgentSnapshot(domainprofile.AgentCodex, ports.AgentConfigSnapshot{Exists: true, Content: []byte("restore")}); err != nil {
		t.Fatalf("restoreAgentSnapshot(missing codex syncer restore) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).restoreAgentSnapshot(domainprofile.AgentClaude, ports.AgentConfigSnapshot{Exists: true, Content: []byte("restore")}); err != nil {
		t.Fatalf("restoreAgentSnapshot(missing claude syncer restore) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).restoreAgentSnapshot(domainprofile.AgentGemini, ports.AgentConfigSnapshot{Exists: true, Content: []byte("restore")}); err != nil {
		t.Fatalf("restoreAgentSnapshot(missing gemini syncer restore) error = %v", err)
	}
	if err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil).restoreAgentSnapshot(domainprofile.Agent("bad"), ports.AgentConfigSnapshot{Exists: true, Content: []byte("restore")}); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("restoreAgentSnapshot(invalid restore agent) err=%v, want invalid agent", err)
	}

	svc = NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil)
	svc.SetOperationJournal(&errorJournal{currentErr: errors.New("current failed")})
	if err := svc.clearCurrentOperationJournal(); err == nil || !strings.Contains(err.Error(), "current failed") {
		t.Fatalf("clearCurrentOperationJournal(current failure) err=%v, want current failed", err)
	}
}
