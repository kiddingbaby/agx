package usecase

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

type fakeJournal struct {
	current *ports.OperationRecord
}

type errorLocker struct {
	err error
}

func (e errorLocker) Lock() (func(), error) {
	return nil, e.err
}

type errorProfileRepo struct {
	listErr   error
	getErr    error
	upsertErr error
	deleteErr error
}

func (e *errorProfileRepo) List() ([]domainprofile.Profile, error) {
	if e.listErr != nil {
		return nil, e.listErr
	}
	return nil, nil
}
func (e *errorProfileRepo) Get(string) (*domainprofile.Profile, error) {
	if e.getErr != nil {
		return nil, e.getErr
	}
	return nil, &domainprofile.NotFoundError{Name: "missing"}
}
func (e *errorProfileRepo) Upsert(profile domainprofile.Profile) (*domainprofile.Profile, error) {
	if e.upsertErr != nil {
		return nil, e.upsertErr
	}
	return &profile, nil
}
func (e *errorProfileRepo) Delete(string) error { return e.deleteErr }

type errorStateRepo struct {
	loadErr error
	saveErr error
	state   domainprofile.State
}

func (e *errorStateRepo) Load() (domainprofile.State, error) {
	if e.loadErr != nil {
		return domainprofile.State{}, e.loadErr
	}
	return e.state, nil
}
func (e *errorStateRepo) Save(state domainprofile.State) (domainprofile.State, error) {
	if e.saveErr != nil {
		return domainprofile.State{}, e.saveErr
	}
	e.state = state
	return state, nil
}

type sequenceStateRepo struct {
	loads   []domainprofile.State
	loadErr []error
	saves   []error
	loadN   int
	saveN   int
}

func (s *sequenceStateRepo) Load() (domainprofile.State, error) {
	i := s.loadN
	s.loadN++
	if i < len(s.loadErr) && s.loadErr[i] != nil {
		return domainprofile.State{}, s.loadErr[i]
	}
	if i < len(s.loads) {
		return s.loads[i], nil
	}
	if len(s.loads) > 0 {
		return s.loads[len(s.loads)-1], nil
	}
	return domainprofile.State{}, nil
}

func (s *sequenceStateRepo) Save(state domainprofile.State) (domainprofile.State, error) {
	i := s.saveN
	s.saveN++
	if i < len(s.saves) && s.saves[i] != nil {
		return domainprofile.State{}, s.saves[i]
	}
	if len(s.loads) == 0 {
		s.loads = append(s.loads, state)
	} else {
		s.loads = append(s.loads, state)
	}
	return state, nil
}

type errorJournal struct {
	currentErr error
	beginErr   error
	updateErr  error
	clearErr   error
	current    *ports.OperationRecord
}

func (e *errorJournal) Current() (*ports.OperationRecord, error) {
	if e.currentErr != nil {
		return nil, e.currentErr
	}
	return e.current, nil
}
func (e *errorJournal) Begin(record ports.OperationRecord) error {
	e.current = &record
	return e.beginErr
}
func (e *errorJournal) Update(record ports.OperationRecord) error {
	e.current = &record
	return e.updateErr
}
func (e *errorJournal) Clear(string) error {
	if e.clearErr == nil {
		e.current = nil
	}
	return e.clearErr
}

type stubRestorer struct {
	restorePath string
	removeCalls int
	restoreErr  error
	removeErr   error
}

func (s *stubRestorer) Restore(path string) (string, error) {
	s.restorePath = path
	if s.restoreErr != nil {
		return "", s.restoreErr
	}
	return "restored", nil
}

func (s *stubRestorer) RemoveConfig() (string, error) {
	s.removeCalls++
	if s.removeErr != nil {
		return "", s.removeErr
	}
	return "removed", nil
}

func (f *fakeJournal) Current() (*ports.OperationRecord, error) {
	if f.current == nil {
		return nil, nil
	}
	record := *f.current
	return &record, nil
}

func (f *fakeJournal) Begin(record ports.OperationRecord) error {
	f.current = &record
	return nil
}

func (f *fakeJournal) Update(record ports.OperationRecord) error {
	f.current = &record
	return nil
}

func (f *fakeJournal) Clear(id string) error {
	if f.current != nil && id != "" && f.current.ID != id {
		return errors.New("mismatch")
	}
	f.current = nil
	return nil
}

func TestListSortsProfiles(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-b": {Name: "relay-b"},
		"relay-a": {Name: "relay-a"},
	}}
	svc := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil)

	got, err := svc.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 || got[0].Name != "relay-a" || got[1].Name != "relay-b" {
		t.Fatalf("List() = %+v, want sorted profiles", got)
	}
}

func TestDoctorReportsBrokenState(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	state := &fakeStateRepo{
		state: domainprofile.State{
			Codex: domainprofile.CodexState{
				BindingView: domainprofile.BindingView{SourceProfile: "missing-relay", Status: domainprofile.BindingStatus("broken")},
				Backups: []domainprofile.Backup{
					{ID: "missing-path", RestoreMode: domainprofile.RestoreModeRestoreFile},
					{ID: "missing-file", RestoreMode: domainprofile.RestoreModeRestoreFile, BackupPath: filepath.Join(t.TempDir(), "missing.bak")},
				},
			},
			Claude: domainprofile.AgentBinding{SourceProfile: "missing-relay", Status: domainprofile.BindingStatus("broken")},
		},
	}
	journal := &fakeJournal{
		current: &ports.OperationRecord{
			ID:        "op-1",
			Command:   "set",
			Agent:     domainprofile.AgentCodex,
			Stage:     "started",
			StartedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}
	svc := NewProfileService(repo, state, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil, nil)
	svc.SetOperationJournal(journal)

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if report.OK {
		t.Fatalf("Doctor().OK = true, want false")
	}
	if report.Operation == nil || report.Operation.ID != "op-1" {
		t.Fatalf("Doctor().Operation = %+v, want op-1", report.Operation)
	}
	if len(report.Issues) < 5 {
		t.Fatalf("Doctor().Issues = %+v, want multiple issues", report.Issues)
	}
	foundAction := false
	for _, issue := range report.Issues {
		if issue.Action != "" {
			foundAction = true
			break
		}
	}
	if !foundAction {
		t.Fatalf("Doctor().Issues = %+v, want issue actions", report.Issues)
	}
}

func TestUsecaseErrorMessages(t *testing.T) {
	cases := []string{
		(&ProfileAlreadyExistsError{}).Error(),
		(&ProfileAlreadyExistsError{Name: "relay-a"}).Error(),
		(&DuplicateRelayConfigError{}).Error(),
		(&DuplicateRelayConfigError{ExistingName: "relay-a"}).Error(),
		(&DuplicateRelayConfigError{Name: "relay-b", ExistingName: "relay-a"}).Error(),
		(&InvalidAgentError{}).Error(),
		(&InvalidAgentError{Agent: "bad"}).Error(),
		(&ProfileInUseError{Name: "relay-a"}).Error(),
		(&ProfileInUseError{Name: "relay-a", Agents: []domainprofile.Agent{domainprofile.AgentCodex, domainprofile.AgentClaude}}).Error(),
		(&NoBackupError{}).Error(),
		(&NoBackupError{Agent: domainprofile.AgentCodex}).Error(),
		(&BackupNotFoundError{}).Error(),
		(&BackupNotFoundError{ID: "backup-1"}).Error(),
		(&ConflictingAgentChangesError{}).Error(),
		(&ConflictingAgentChangesError{Agents: []domainprofile.Agent{domainprofile.AgentCodex}}).Error(),
		(&AgentNotBoundToRelayError{Agent: domainprofile.AgentClaude}).Error(),
		(&AgentNotBoundToRelayError{Agent: domainprofile.AgentClaude, Relay: "relay-a"}).Error(),
	}
	for _, msg := range cases {
		if strings.TrimSpace(msg) == "" {
			t.Fatal("expected non-empty error message")
		}
	}
}


func TestClearCurrentOperationJournal(t *testing.T) {
	journal := &fakeJournal{
		current: &ports.OperationRecord{ID: "op-1"},
	}
	svc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, nil, nil)
	svc.SetOperationJournal(journal)

	if err := svc.clearCurrentOperationJournal(); err != nil {
		t.Fatalf("clearCurrentOperationJournal() error = %v", err)
	}
	if journal.current != nil {
		t.Fatalf("journal.current = %+v, want cleared", journal.current)
	}
}

func TestBackupListAndHelpers(t *testing.T) {
	state := &fakeStateRepo{
		state: domainprofile.State{
			Codex: domainprofile.CodexState{
				Backups: []domainprofile.Backup{{ID: "backup-1"}},
			},
		},
	}
	svc := NewProfileService(&fakeProfileRepo{}, state, nil, nil, nil, nil)

	backups, err := svc.BackupList(domainprofile.AgentCodex)
	if err != nil {
		t.Fatalf("BackupList() error = %v", err)
	}
	if len(backups) != 1 || backups[0].ID != "backup-1" {
		t.Fatalf("BackupList() = %+v, want backup-1", backups)
	}
	if _, err := svc.BackupList(domainprofile.Agent("bad")); err == nil {
		t.Fatal("BackupList() unexpectedly succeeded for invalid agent")
	}
	selected, err := selectBackup(backups, "")
	if err != nil || selected.ID != backups[0].ID {
		t.Fatalf("selectBackup(empty) = (%+v,%v), want newest backup", selected, err)
	}
	if codeView := codexBindingView(&state.state); codeView == nil {
		t.Fatal("codexBindingView() returned nil")
	}
}

func TestUseAndBackupEntryPoints(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	state := &fakeStateRepo{}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before")
	codex := &fakeCodexSyncer{codexBase}
	svc := NewProfileService(repo, state, codex, nil, nil, nil)

	setResult, err := svc.Use(domainprofile.AgentCodex, "relay-a")
	if err != nil {
		t.Fatalf("Use() error = %v", err)
	}
	if setResult.Agent != domainprofile.AgentCodex || setResult.Profile == nil || setResult.Profile.Name != "relay-a" {
		t.Fatalf("Use() = %+v", setResult)
	}

	backupResult, err := svc.Backup(domainprofile.AgentCodex)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	if backupResult.Agent != domainprofile.AgentCodex || strings.TrimSpace(backupResult.Backup.ID) == "" {
		t.Fatalf("Backup() = %+v", backupResult)
	}
}

func TestBindingHelpers(t *testing.T) {
	if overlap := overlappingAgents([]domainprofile.Agent{domainprofile.AgentCodex, domainprofile.AgentClaude}, []domainprofile.Agent{domainprofile.AgentClaude, domainprofile.AgentGemini}); len(overlap) != 1 || overlap[0] != domainprofile.AgentClaude {
		t.Fatalf("overlappingAgents() = %v, want [claude]", overlap)
	}
	normalized := normalizeAgents([]domainprofile.Agent{domainprofile.AgentCodex, domainprofile.Agent("bad"), domainprofile.AgentCodex})
	if len(normalized) != 1 || normalized[0] != domainprofile.AgentCodex {
		t.Fatalf("normalizeAgents() = %v, want [codex]", normalized)
	}
}

func TestClearCodexSourceProfileIfMatchesAndRefreshAfterRestore(t *testing.T) {
	state := &fakeStateRepo{
		state: domainprofile.State{
			Codex: domainprofile.CodexState{
				BindingView: domainprofile.BindingView{
					SourceProfile: "relay-a",
					Status:        domainprofile.BindingStatusApplied,
					ConfigPath:    "/tmp/codex/config.toml",
				},
			},
		},
	}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.defaultProfile = "relay-a"
	codex.managedProfiles["relay-a"] = ports.CodexManagedProfile{Name: "relay-a", BaseURL: "https://relay.example/v1"}
	svc := NewProfileService(&fakeProfileRepo{}, state, codex, nil, nil, nil)

	if err := svc.clearCodexSourceProfileIfMatches("relay-a"); err != nil {
		t.Fatalf("clearCodexSourceProfileIfMatches() error = %v", err)
	}
	if state.state.Codex.SourceProfile != "" {
		t.Fatalf("Codex.SourceProfile = %q, want cleared", state.state.Codex.SourceProfile)
	}

	if err := svc.refreshCodexStateAfterRestore(&state.state); err != nil {
		t.Fatalf("refreshCodexStateAfterRestore() error = %v", err)
	}
	if state.state.Codex.SourceProfile != "relay-a" || state.state.Codex.ConfigPath != "/tmp/codex/config.toml" {
		t.Fatalf("state = %+v, want resolved codex binding", state.state.Codex)
	}
}

func TestRestoreAgentSnapshot(t *testing.T) {
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	gemini := &fakeGeminiSyncer{newFakeAgentSyncer("/tmp/gemini/.env")}
	svc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, codex, claude, gemini, nil)

	if err := svc.restoreAgentSnapshot(domainprofile.AgentCodex, ports.AgentConfigSnapshot{Exists: false}); err != nil {
		t.Fatalf("restoreAgentSnapshot(remove codex) error = %v", err)
	}
	if codex.removeCalls != 1 {
		t.Fatalf("codex removeCalls = %d, want 1", codex.removeCalls)
	}

	if err := svc.restoreAgentSnapshot(domainprofile.AgentClaude, ports.AgentConfigSnapshot{Exists: true, Content: []byte("restored claude")}); err != nil {
		t.Fatalf("restoreAgentSnapshot(claude) error = %v", err)
	}
	if string(claude.snapshotContent) != "restored claude" {
		t.Fatalf("claude snapshot = %q, want restored claude", string(claude.snapshotContent))
	}

	if err := svc.restoreAgentSnapshot(domainprofile.AgentGemini, ports.AgentConfigSnapshot{Exists: true, Content: []byte("restored gemini")}); err != nil {
		t.Fatalf("restoreAgentSnapshot(gemini) error = %v", err)
	}
	if string(gemini.snapshotContent) != "restored gemini" {
		t.Fatalf("gemini snapshot = %q, want restored gemini", string(gemini.snapshotContent))
	}
}

func TestRestoreBackupForAgent(t *testing.T) {
	stub := &stubRestorer{}
	if path, err := restoreBackupForAgent(domainprofile.Backup{BackupPath: "/tmp/backup", RestoreMode: domainprofile.RestoreModeRestoreFile}, stub); err != nil || path != "restored" || stub.restorePath != "/tmp/backup" {
		t.Fatalf("restoreBackupForAgent(restore) = (%q,%v), restorePath=%q", path, err, stub.restorePath)
	}
	if path, err := restoreBackupForAgent(domainprofile.Backup{RestoreMode: domainprofile.RestoreModeRemoveCreatedFile}, stub); err != nil || path != "removed" || stub.removeCalls != 1 {
		t.Fatalf("restoreBackupForAgent(remove) = (%q,%v), removeCalls=%d", path, err, stub.removeCalls)
	}
	if _, err := restoreBackupForAgent(domainprofile.Backup{RestoreMode: domainprofile.RestoreMode("broken")}, stub); err == nil {
		t.Fatal("restoreBackupForAgent(invalid) unexpectedly succeeded")
	}
}

func TestDoctorHelpersAndOperationHelpers(t *testing.T) {
	report := &DoctorReport{OK: true}
	checkCodexBindings(report, domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{SourceProfile: "missing", Status: domainprofile.BindingStatus("broken")},
		},
	}, map[string]struct{}{})
	if len(report.Issues) < 2 {
		t.Fatalf("checkCodexBindings() issues = %+v, want multiple issues", report.Issues)
	}

	report = &DoctorReport{OK: true}
	if err := checkBackupMetadata(report, domainprofile.AgentClaude, domainprofile.Backup{ID: "backup-1", RestoreMode: domainprofile.RestoreMode("broken")}); err != nil {
		t.Fatalf("checkBackupMetadata(invalid mode) error = %v", err)
	}
	if len(report.Issues) != 1 {
		t.Fatalf("checkBackupMetadata() issues = %+v, want 1", report.Issues)
	}

	now := time.Now().UTC()
	record := newOperationRecord("set", domainprofile.AgentCodex, "relay-a", now)
	if record.Command != "set" || record.Agent != domainprofile.AgentCodex || record.Profile != "relay-a" || record.Stage != operationStageStarted {
		t.Fatalf("newOperationRecord() = %+v", record)
	}
}

func checkCodexBindings(report *DoctorReport, state domainprofile.State, profileNames map[string]struct{}) {
	svc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, nil, nil)
	svc.checkCodexBindings(report, state, profileNames)
}

func TestRollbackBindingChangesAndSaveProfile(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay-a.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	state := &fakeStateRepo{}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before")
	codex := &fakeCodexSyncer{codexBase}
	svc := NewProfileService(repo, state, codex, nil, nil, nil)

	result, err := svc.AgentSet(domainprofile.AgentCodex, "relay-a")
	if err != nil {
		t.Fatalf("AgentSet() error = %v", err)
	}
	before := cloneState(state.state)
	change := BindingChangeResult{Agent: domainprofile.AgentCodex, Action: "bind", Backup: result.Backup}
	if err := svc.rollbackBindingChanges(before, []BindingChangeResult{change}); err != nil {
		t.Fatalf("rollbackBindingChanges() error = %v", err)
	}

	if _, err := svc.saveProfile(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a"}, false); err != nil {
		t.Fatalf("saveProfile(valid) error = %v", err)
	}
	if _, err := svc.saveProfile(domainprofile.Profile{Name: "relay-a", BaseURL: "ftp://relay.example", APIKey: "sk-a"}, false); err == nil {
		t.Fatal("saveProfile(invalid) unexpectedly succeeded")
	}
}

func TestCloneStateDeepCopiesRegistriesAndBackups(t *testing.T) {
	original := domainprofile.State{
		Codex: domainprofile.CodexState{
			Backups: []domainprofile.Backup{{ID: "codex-backup"}},
		},
		Claude: domainprofile.AgentBinding{
			Backups: []domainprofile.Backup{{ID: "claude-backup"}},
		},
		Gemini: domainprofile.AgentBinding{
			Backups: []domainprofile.Backup{{ID: "gemini-backup"}},
		},
	}

	cloned := cloneState(original)
	cloned.Codex.Backups[0].ID = "codex-backup-new"
	cloned.Claude.Backups[0].ID = "claude-backup-new"
	cloned.Gemini.Backups[0].ID = "gemini-backup-new"

	if got := original.Codex.Backups[0].ID; got != "codex-backup" {
		t.Fatalf("original Codex backups mutated: %q", got)
	}
	if got := original.Claude.Backups[0].ID; got != "claude-backup" {
		t.Fatalf("original Claude backups mutated: %q", got)
	}
	if got := original.Gemini.Backups[0].ID; got != "gemini-backup" {
		t.Fatalf("original Gemini backups mutated: %q", got)
	}

}

func TestLoadStoredStateReturnsDetachedCopy(t *testing.T) {
	repo := &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{BindingView: domainprofile.BindingView{SourceProfile: "relay-a"}},
	}}
	svc := NewProfileService(&fakeProfileRepo{}, repo, nil, nil, nil, nil)

	loaded, err := svc.loadStoredState()
	if err != nil {
		t.Fatalf("loadStoredState() error = %v", err)
	}
	loaded.Codex.SourceProfile = "relay-b"
	if repo.state.Codex.SourceProfile != "relay-a" {
		t.Fatalf("repo state mutated: %q", repo.state.Codex.SourceProfile)
	}
}

func TestMutationGuardErrorBranches(t *testing.T) {
	guardSvc := NewProfileService(&errorProfileRepo{getErr: errors.New("repo get failed")}, &fakeStateRepo{}, nil, nil, nil, nil)
	guard := newMutationGuard(guardSvc)
	if err := guard.CaptureProfile("relay-a"); err == nil {
		t.Fatal("CaptureProfile() unexpectedly succeeded")
	}

	stateSvc := NewProfileService(&fakeProfileRepo{}, &errorStateRepo{loadErr: errors.New("state load failed")}, nil, nil, nil, nil)
	guard = newMutationGuard(stateSvc)
	if err := guard.CaptureState(); err == nil {
		t.Fatal("CaptureState() unexpectedly succeeded")
	}

	guard = newMutationGuard(NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, nil, nil))
	if err := guard.CaptureAgents(domainprofile.Agent("bad")); err != nil {
		t.Fatalf("CaptureAgents(invalid) error = %v, want nil skip", err)
	}

	repoSvc := NewProfileService(&errorProfileRepo{deleteErr: errors.New("delete failed")}, &fakeStateRepo{}, nil, nil, nil, nil)
	guard = newMutationGuard(repoSvc)
	guard.profilesBefore["relay-a"] = nil
	if err := guard.Rollback(); err == nil {
		t.Fatal("Rollback() unexpectedly succeeded with delete failure")
	}
}

func TestOperationAndJournalErrorBranches(t *testing.T) {
	svc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, nil, nil)
	if err := svc.beginOperation(ports.OperationRecord{}); err != nil {
		t.Fatalf("beginOperation(nil journal) error = %v", err)
	}
	if err := svc.updateOperation(ports.OperationRecord{}); err != nil {
		t.Fatalf("updateOperation(nil journal) error = %v", err)
	}
	if err := svc.clearOperation("op-1"); err != nil {
		t.Fatalf("clearOperation(nil journal) error = %v", err)
	}

	journal := &errorJournal{
		current:   &ports.OperationRecord{ID: "op-1"},
		beginErr:  errors.New("begin failed"),
		updateErr: errors.New("update failed"),
		clearErr:  errors.New("clear failed"),
	}
	svc.SetOperationJournal(journal)
	if err := svc.beginOperation(ports.OperationRecord{ID: "op-1"}); err == nil {
		t.Fatal("beginOperation() unexpectedly succeeded")
	}
	if err := svc.updateOperation(ports.OperationRecord{ID: "op-1"}); err == nil {
		t.Fatal("updateOperation() unexpectedly succeeded")
	}
	if err := svc.clearOperation("op-1"); err == nil {
		t.Fatal("clearOperation() unexpectedly succeeded")
	}
}

func TestApplyRelayBindingChangesErrorBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	state := &fakeStateRepo{}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before")
	codex := &fakeCodexSyncer{codexBase}
	svc := NewProfileService(repo, state, codex, nil, nil, nil)

	if _, err := svc.applyRelayBindingChanges("relay-a", []domainprofile.Agent{domainprofile.AgentCodex}, []domainprofile.Agent{domainprofile.AgentCodex}); err == nil {
		t.Fatal("applyRelayBindingChanges(overlap) unexpectedly succeeded")
	}
	if _, err := svc.applyRelayBindingChanges("relay-a", nil, []domainprofile.Agent{domainprofile.AgentCodex}); err == nil {
		t.Fatal("applyRelayBindingChanges(unbind missing) unexpectedly succeeded")
	}
}

func TestApplyRelayBindingChangesRollsBackProfileRegistriesOnLaterFailure(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay-a.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		"relay-b": {Name: "relay-b", BaseURL: "https://relay-b.example/v1", APIKey: "sk-b", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	state := &fakeStateRepo{state: domainprofile.State{
		Claude: domainprofile.AgentBinding{
			SourceProfile: "relay-b",
			Status:        domainprofile.BindingStatusApplied,
			ConfigPath:    "/tmp/claude/settings.json",
		},
	}}

	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.snapshotContent = []byte("before-claude")
	claude := &fakeClaudeSyncer{claudeBase}

	geminiBase := newFakeAgentSyncer("/tmp/gemini/.env")
	geminiBase.syncErr = errors.New("gemini sync failed")
	gemini := &fakeGeminiSyncer{geminiBase}

	svc := NewProfileService(repo, state, nil, claude, gemini, nil)
	before, err := svc.loadStoredState()
	if err != nil {
		t.Fatalf("loadStoredState() error = %v", err)
	}
	if _, err := svc.applyRelayBindingChanges("relay-a", []domainprofile.Agent{domainprofile.AgentClaude, domainprofile.AgentGemini}, nil); err == nil || !strings.Contains(err.Error(), "gemini sync failed") {
		t.Fatalf("applyRelayBindingChanges() err=%v, want gemini sync failed", err)
	}

	if !reflect.DeepEqual(state.state, before) {
		t.Fatalf("state after rollback = %+v, want %+v", state.state, before)
	}
	if got := string(claudeBase.snapshotContent); got != "before-claude" {
		t.Fatalf("claude snapshot after rollback = %q, want before-claude", got)
	}
}

func TestRemoveCodexProfileArtifactsBranches(t *testing.T) {
	state := &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{SourceProfile: "relay-a"},
		},
	}}
	svc := NewProfileService(&fakeProfileRepo{}, state, nil, nil, nil, nil)
	if err := svc.removeCodexProfileArtifacts("relay-a"); err != nil {
		t.Fatalf("removeCodexProfileArtifacts(nil codex) error = %v", err)
	}

	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.managedProfiles["relay-a"] = ports.CodexManagedProfile{Name: "relay-a", BaseURL: "https://relay.example/v1"}
	svc = NewProfileService(&fakeProfileRepo{}, state, codex, nil, nil, nil)
	if err := svc.removeCodexProfileArtifacts("relay-a"); err != nil {
		t.Fatalf("removeCodexProfileArtifacts() error = %v", err)
	}
}

func TestSnapshotAndRestoreAgentConfigBranches(t *testing.T) {
	svc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, nil, nil)

	if snapshot, ok, err := svc.snapshotAgentConfig(domainprofile.AgentCodex); err != nil || ok || snapshot.Exists {
		t.Fatalf("snapshotAgentConfig(nil codex) = (%+v,%v,%v), want empty,false,nil", snapshot, ok, err)
	}
	if _, _, err := svc.snapshotAgentConfig(domainprofile.Agent("bad")); err == nil {
		t.Fatal("snapshotAgentConfig(invalid) unexpectedly succeeded")
	}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotErr = errors.New("snapshot failed")
	codex := &fakeCodexSyncer{codexBase}
	svc = NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, codex, nil, nil, nil)
	if _, _, err := svc.snapshotAgentConfig(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "snapshot failed") {
		t.Fatalf("snapshotAgentConfig(snapshot error) err=%v, want snapshot failure", err)
	}

	if err := svc.restoreAgentSnapshot(domainprofile.Agent("bad"), ports.AgentConfigSnapshot{Exists: false}); err == nil {
		t.Fatal("restoreAgentSnapshot(invalid) unexpectedly succeeded")
	}

	codexBase.removeErr = errors.New("remove failed")
	if err := svc.restoreAgentSnapshot(domainprofile.AgentCodex, ports.AgentConfigSnapshot{Exists: false}); err == nil || !strings.Contains(err.Error(), "remove failed") {
		t.Fatalf("restoreAgentSnapshot(remove error) err=%v, want remove failure", err)
	}

	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.restoreErr = errors.New("restore failed")
	claude := &fakeClaudeSyncer{claudeBase}
	svc = NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, claude, nil, nil)
	if err := svc.restoreAgentSnapshot(domainprofile.AgentClaude, ports.AgentConfigSnapshot{Exists: true, Content: []byte("restored")}); err == nil || !strings.Contains(err.Error(), "restore failed") {
		t.Fatalf("restoreAgentSnapshot(restore error) err=%v, want restore failure", err)
	}

	svc = NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, nil, nil)
	if err := svc.restoreAgentSnapshot(domainprofile.AgentClaude, ports.AgentConfigSnapshot{Exists: true, Content: []byte("ignored")}); err != nil {
		t.Fatalf("restoreAgentSnapshot(nil claude) error = %v", err)
	}
}

func TestRollbackBindingChangesBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	state := &fakeStateRepo{}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotExists = false
	codex := &fakeCodexSyncer{codexBase}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	svc := NewProfileService(repo, state, codex, claude, nil, nil)

	if err := svc.rollbackBindingChanges(domainprofile.State{}, []BindingChangeResult{{
		Agent:  domainprofile.AgentCodex,
		Action: "bind",
		Backup: domainprofile.Backup{RestoreMode: domainprofile.RestoreModeRemoveCreatedFile},
	}}); err != nil {
		t.Fatalf("rollbackBindingChanges(clear) error = %v", err)
	}
	if state.state.Codex.SourceProfile != "" {
		t.Fatalf("Codex.SourceProfile = %q, want empty after rollback clear", state.state.Codex.SourceProfile)
	}

	if err := svc.rollbackBindingChanges(domainprofile.State{}, []BindingChangeResult{{
		Agent:  domainprofile.AgentClaude,
		Action: "unbind",
		Backup: domainprofile.Backup{RestoreMode: domainprofile.RestoreModeRemoveCreatedFile},
	}}); err != nil {
		t.Fatalf("rollbackBindingChanges(no-op unbind) error = %v", err)
	}

	saveErrSvc := NewProfileService(repo, &errorStateRepo{saveErr: errors.New("save failed")}, codex, claude, nil, nil)
	if err := saveErrSvc.rollbackBindingChanges(domainprofile.State{}, nil); err == nil || !strings.Contains(err.Error(), "save failed") {
		t.Fatalf("rollbackBindingChanges(save error) err=%v, want save failure", err)
	}
}

func TestRollbackBindingChangesRestoresOriginalAgentConfig(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}

	t.Run("bind restores prior unbound config", func(t *testing.T) {
		state := &fakeStateRepo{}
		claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
		claudeBase.snapshotContent = []byte("before-claude")
		claude := &fakeClaudeSyncer{claudeBase}
		svc := NewProfileService(repo, state, nil, claude, nil, nil)

		result, err := svc.AgentSet(domainprofile.AgentClaude, "relay-a")
		if err != nil {
			t.Fatalf("AgentSet() error = %v", err)
		}
		if got := string(claudeBase.snapshotContent); got == "before-claude" {
			t.Fatalf("snapshotContent after bind = %q, want AGX-managed config", got)
		}

		if err := svc.rollbackBindingChanges(domainprofile.State{}, []BindingChangeResult{{
			Agent:  domainprofile.AgentClaude,
			Action: "bind",
			Backup: result.Backup,
		}}); err != nil {
			t.Fatalf("rollbackBindingChanges(bind) error = %v", err)
		}
		if got := string(claudeBase.snapshotContent); got != "before-claude" {
			t.Fatalf("snapshotContent after bind rollback = %q, want original content", got)
		}
	})

	t.Run("unbind restores prior bound config", func(t *testing.T) {
		state := &fakeStateRepo{}
		claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
		svc := NewProfileService(repo, state, nil, claude, nil, nil)

		if _, err := svc.AgentSet(domainprofile.AgentClaude, "relay-a"); err != nil {
			t.Fatalf("AgentSet() error = %v", err)
		}
		before := cloneState(state.state)
		beforeContent := string(claude.snapshotContent)

		result, err := svc.clearLocked(domainprofile.AgentClaude)
		if err != nil {
			t.Fatalf("clearLocked() error = %v", err)
		}
		if claude.snapshotExists {
			t.Fatal("snapshotExists after clearLocked = true, want false")
		}

		if err := svc.rollbackBindingChanges(before, []BindingChangeResult{{
			Agent:  domainprofile.AgentClaude,
			Action: "unbind",
			Backup: result.Backup,
		}}); err != nil {
			t.Fatalf("rollbackBindingChanges(unbind) error = %v", err)
		}
		if !claude.snapshotExists {
			t.Fatal("snapshotExists after unbind rollback = false, want true")
		}
		if got := string(claude.snapshotContent); got != beforeContent {
			t.Fatalf("snapshotContent after unbind rollback = %q, want %q", got, beforeContent)
		}
	})
}

func TestRemoveRestoreAndDoctorAdditionalErrorBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	state := &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{BindingView: domainprofile.BindingView{SourceProfile: "relay-a"}},
	}}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.defaultProfile = "relay-a"
	codex.managedProfiles["relay-a"] = ports.CodexManagedProfile{Name: "relay-a", BaseURL: "https://relay.example/v1"}
	codex.removeProfileErr = errors.New("remove profile failed")
	svc := NewProfileService(repo, state, codex, nil, nil, nil)

	if _, err := svc.Remove("relay-a"); err == nil || !strings.Contains(err.Error(), "currently bound to codex") {
		t.Fatalf("Remove() err=%v, want managed codex profile to block removal", err)
	}
	if _, err := svc.Edit("relay-a", EditProfileInput{Unbind: []domainprofile.Agent{domainprofile.AgentCodex}}); err == nil || !strings.Contains(err.Error(), "remove profile failed") {
		t.Fatalf("Edit(unbind codex) err=%v, want codex cleanup failure", err)
	}
	if _, err := repo.Get("relay-a"); err != nil {
		t.Fatalf("profile was not preserved after unbind failure: %v", err)
	}

	svc = NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil, nil)
	if _, err := svc.Restore(domainprofile.Agent("bad"), ""); err == nil {
		t.Fatal("Restore(invalid agent) unexpectedly succeeded")
	}

	restoreState := &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			Backups: []domainprofile.Backup{{ID: "backup-1", RestoreMode: domainprofile.RestoreModeRemoveCreatedFile}},
		},
	}}
	svc = NewProfileService(repo, restoreState, codex, nil, nil, nil)
	if _, err := svc.Restore(domainprofile.AgentCodex, "missing"); err == nil || !strings.Contains(err.Error(), "backup not found") {
		t.Fatalf("Restore(missing backup id) err=%v, want backup not found", err)
	}

	svc.SetOperationJournal(&errorJournal{currentErr: errors.New("current failed")})
	if _, err := svc.Doctor(); err == nil || !strings.Contains(err.Error(), "current failed") {
		t.Fatalf("Doctor() err=%v, want current operation failure", err)
	}

	report := &DoctorReport{OK: true}
	if err := checkBackupMetadata(report, domainprofile.AgentCodex, domainprofile.Backup{ID: "backup-1", RestoreMode: domainprofile.RestoreModeRestoreFile}); err != nil {
		t.Fatalf("checkBackupMetadata(missing path) error = %v", err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Code != "missing_backup_path" {
		t.Fatalf("checkBackupMetadata() issues = %+v, want missing_backup_path", report.Issues)
	}
	if report.Issues[0].Action == "" {
		t.Fatalf("checkBackupMetadata() issue=%+v, want action", report.Issues[0])
	}
}

func TestClearAndRestoreRejectMissingSyncerInsteadOfPanicking(t *testing.T) {
	state := &fakeStateRepo{state: domainprofile.State{
		Claude: domainprofile.AgentBinding{
			SourceProfile: "relay-a",
			Backups: []domainprofile.Backup{
				{ID: "backup-1", RestoreMode: domainprofile.RestoreModeRemoveCreatedFile},
			},
		},
	}}
	svc := NewProfileService(&fakeProfileRepo{}, state, nil, nil, nil, nil)

	if _, err := svc.Clear(domainprofile.AgentClaude); err == nil {
		t.Fatal("Clear(claude without syncer) unexpectedly succeeded")
	}
	if _, err := svc.Restore(domainprofile.AgentClaude, "backup-1"); err == nil {
		t.Fatal("Restore(claude without syncer) unexpectedly succeeded")
	}
}

func TestBindingAndMutationAdditionalBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	state := &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{SourceProfile: "relay-a"},
		},
	}}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before")
	codex := &fakeCodexSyncer{codexBase}
	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.snapshotContent = []byte(`{"before":true}`)
	claude := &fakeClaudeSyncer{claudeBase}
	svc := NewProfileService(repo, state, codex, claude, nil, nil)

	if cleanup := deleteBackupForAgent(domainprofile.AgentGemini, svc); cleanup != nil {
		t.Fatal("deleteBackupForAgent(gemini without syncer) unexpectedly returned cleanup")
	}

	backup, cleanup, _, err := svc.syncProfileToAgent(domainprofile.AgentClaude, repo.profiles["relay-a"], "backup-1", state.state, false)
	if err != nil {
		t.Fatalf("syncProfileToAgent(claude) error = %v", err)
	}
	if backup.ConfigPath != claude.configPath || cleanup == nil {
		t.Fatalf("syncProfileToAgent(claude) = %+v with nil cleanup, want config path and cleanup", backup)
	}

	claudeBase.createBackupErr = errors.New("backup failed")
	if _, err := svc.snapshotCurrentConfig(domainprofile.AgentClaude, "relay-a", "backup-2", time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "backup failed") {
		t.Fatalf("snapshotCurrentConfig(create backup error) err=%v, want backup failure", err)
	}
	claudeBase.createBackupErr = nil

	if err := svc.syncCodexProfile(repo.profiles["relay-a"], &state.state, time.Now().UTC()); err != nil {
		t.Fatalf("syncCodexProfile(active) error = %v", err)
	}
	if state.state.Codex.SourceProfile != "relay-a" || state.state.Codex.LastBackupID == "" {
		t.Fatalf("state.Codex = %+v, want active codex binding refreshed", state.state.Codex)
	}

	state.state.Claude.SourceProfile = "relay-a"
	if err := svc.syncProfileToBoundAgents(repo.profiles["relay-a"], &state.state, time.Now().UTC()); err != nil {
		t.Fatalf("syncProfileToBoundAgents() error = %v", err)
	}
	if state.state.Claude.LastBackupID == "" {
		t.Fatalf("state.Claude = %+v, want backup metadata updated", state.state.Claude)
	}
}

func TestApplyRelayBindingChangesAndRestoreBindingChangeAdditionalBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay-a.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	state := &fakeStateRepo{}
	gemini := &fakeGeminiSyncer{newFakeAgentSyncer("/tmp/gemini/.env")}
	svc := NewProfileService(repo, state, nil, nil, gemini, nil)

	result, err := svc.applyRelayBindingChanges("relay-a", []domainprofile.Agent{domainprofile.AgentGemini}, nil)
	if err != nil {
		t.Fatalf("applyRelayBindingChanges(bind gemini) error = %v", err)
	}
	if len(result.Changed) != 1 || result.Changed[0].Action != "bind" || result.Changed[0].Agent != domainprofile.AgentGemini {
		t.Fatalf("result.Changed = %+v, want single gemini bind", result.Changed)
	}
	if state.state.Gemini.SourceProfile != "relay-a" {
		t.Fatalf("Gemini.SourceProfile = %q, want relay-a", state.state.Gemini.SourceProfile)
	}

	result, err = svc.applyRelayBindingChanges("relay-a", nil, []domainprofile.Agent{domainprofile.AgentGemini})
	if err != nil {
		t.Fatalf("applyRelayBindingChanges(unbind gemini) error = %v", err)
	}
	if len(result.Changed) != 1 || result.Changed[0].Action != "unbind" || result.Changed[0].Agent != domainprofile.AgentGemini {
		t.Fatalf("result.Changed = %+v, want single gemini unbind", result.Changed)
	}
	if state.state.Gemini.SourceProfile != "" {
		t.Fatalf("Gemini.SourceProfile = %q, want empty", state.state.Gemini.SourceProfile)
	}

	loadErrSvc := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("state load failed")}, nil, nil, gemini, nil)
	if _, err := loadErrSvc.applyRelayBindingChanges("relay-a", []domainprofile.Agent{domainprofile.AgentGemini}, nil); err == nil || !strings.Contains(err.Error(), "state load failed") {
		t.Fatalf("applyRelayBindingChanges(load error) err=%v, want load failure", err)
	}

	restoreDir := t.TempDir()
	restorePath := filepath.Join(restoreDir, "claude.bak")
	if err := os.WriteFile(restorePath, []byte("claude restored"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.backups[restorePath] = []byte("claude restored")
	claude := &fakeClaudeSyncer{claudeBase}
	gemini = &fakeGeminiSyncer{newFakeAgentSyncer("/tmp/gemini/.env")}
	restoreSvc := NewProfileService(repo, state, codex, claude, gemini, nil)

	if err := restoreSvc.restoreBindingChange(BindingChangeResult{
		Agent:  domainprofile.AgentCodex,
		Action: "bind",
		Backup: domainprofile.Backup{RestoreMode: domainprofile.RestoreModeRemoveCreatedFile},
	}); err != nil {
		t.Fatalf("restoreBindingChange(codex remove) error = %v", err)
	}
	if codex.removeCalls != 1 {
		t.Fatalf("codex removeCalls = %d, want 1", codex.removeCalls)
	}

	if err := restoreSvc.restoreBindingChange(BindingChangeResult{
		Agent:  domainprofile.AgentClaude,
		Action: "unbind",
		Backup: domainprofile.Backup{BackupPath: restorePath, RestoreMode: domainprofile.RestoreModeRestoreFile},
	}); err != nil {
		t.Fatalf("restoreBindingChange(claude restore) error = %v", err)
	}
	if got := string(claude.snapshotContent); got != "claude restored" {
		t.Fatalf("claude snapshotContent = %q, want restored backup content", got)
	}

	if err := restoreSvc.restoreBindingChange(BindingChangeResult{
		Agent:  domainprofile.AgentGemini,
		Action: "bind",
		Backup: domainprofile.Backup{RestoreMode: domainprofile.RestoreModeRemoveCreatedFile},
	}); err != nil {
		t.Fatalf("restoreBindingChange(gemini remove) error = %v", err)
	}
	if gemini.removeCalls != 1 {
		t.Fatalf("gemini removeCalls = %d, want 1", gemini.removeCalls)
	}

	noClaudeSvc := NewProfileService(repo, state, codex, nil, gemini, nil)
	if err := noClaudeSvc.restoreBindingChange(BindingChangeResult{
		Agent:  domainprofile.AgentClaude,
		Action: "bind",
		Backup: domainprofile.Backup{RestoreMode: domainprofile.RestoreModeRemoveCreatedFile},
	}); err == nil {
		t.Fatal("restoreBindingChange(claude without syncer) unexpectedly succeeded")
	}
	if err := restoreSvc.restoreBindingChange(BindingChangeResult{Agent: domainprofile.Agent("bad")}); err == nil {
		t.Fatal("restoreBindingChange(invalid agent) unexpectedly succeeded")
	}
}

func TestUsecaseErrorEdgeMessagesAndStateHelpers(t *testing.T) {
	if got := (&ProfileAlreadyExistsError{}).Error(); got != "relay already exists" {
		t.Fatalf("ProfileAlreadyExistsError{}.Error() = %q", got)
	}
	if got := (&InvalidAgentError{}).Error(); got != "invalid agent" {
		t.Fatalf("InvalidAgentError{}.Error() = %q", got)
	}
	if got := (&ProfileInUseError{Name: "relay-a"}).Error(); !strings.Contains(got, "relay is currently bound") {
		t.Fatalf("ProfileInUseError(no agents).Error() = %q", got)
	}
	if got := (&NoBackupError{}).Error(); got != "no backup available" {
		t.Fatalf("NoBackupError{}.Error() = %q", got)
	}
	if got := (&BackupNotFoundError{}).Error(); got != "backup not found" {
		t.Fatalf("BackupNotFoundError{}.Error() = %q", got)
	}
	if got := (&ConflictingAgentChangesError{Agents: []domainprofile.Agent{""}}).Error(); !strings.Contains(got, "overlapping agents") {
		t.Fatalf("ConflictingAgentChangesError(empty item).Error() = %q", got)
	}
	if got := (&AgentNotBoundToRelayError{Agent: domainprofile.AgentClaude}).Error(); !strings.Contains(got, "not bound to this relay") {
		t.Fatalf("AgentNotBoundToRelayError{}.Error() = %q", got)
	}

	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.statusErr = errors.New("status failed")
	svc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, codex, nil, nil, nil)
	if _, err := svc.State(); err == nil || !strings.Contains(err.Error(), "status failed") {
		t.Fatalf("State(status error) err=%v, want status failure", err)
	}

	plainState := domainprofile.State{
		Claude: domainprofile.AgentBinding{SourceProfile: "relay-a"},
	}
	svc = NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{state: plainState}, nil, nil, nil, nil)
	gotState, err := svc.State()
	if err != nil {
		t.Fatalf("State(no codex) error = %v", err)
	}
	if gotState.Claude.SourceProfile != "relay-a" {
		t.Fatalf("State(no codex) = %+v, want stored state", gotState)
	}
}

func TestRestoreAndDoctorAdditionalHappyAndErrorBranches(t *testing.T) {
	tmpDir := t.TempDir()
	geminiBackupPath := filepath.Join(tmpDir, "gemini.bak")
	if err := os.WriteFile(geminiBackupPath, []byte("GEMINI_API_KEY=\"restored\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	geminiBase := newFakeAgentSyncer("/tmp/gemini/.env")
	geminiBase.backups[geminiBackupPath] = []byte("GEMINI_API_KEY=\"restored\"\n")
	claude := &fakeClaudeSyncer{claudeBase}
	gemini := &fakeGeminiSyncer{geminiBase}
	state := &fakeStateRepo{state: domainprofile.State{
		Claude: domainprofile.AgentBinding{
			SourceProfile: "relay-a",
			Status:        domainprofile.BindingStatusApplied,
			ConfigPath:    "/tmp/claude/settings.json",
			Backups: []domainprofile.Backup{
				{ID: "claude-1", RestoreMode: domainprofile.RestoreModeRemoveCreatedFile, ConfigPath: "/tmp/claude/settings.json"},
			},
		},
		Gemini: domainprofile.AgentBinding{
			SourceProfile: "relay-a",
			Status:        domainprofile.BindingStatusApplied,
			ConfigPath:    "/tmp/gemini/.env",
			Backups: []domainprofile.Backup{
				{ID: "gemini-1", RestoreMode: domainprofile.RestoreModeRestoreFile, BackupPath: geminiBackupPath, ConfigPath: "/tmp/gemini/.env"},
			},
		},
	}}
	svc := NewProfileService(&fakeProfileRepo{}, state, nil, claude, gemini, nil)

	claudeResult, err := svc.Restore(domainprofile.AgentClaude, "")
	if err != nil {
		t.Fatalf("Restore(claude) error = %v", err)
	}
	if claudeResult.Agent != domainprofile.AgentClaude || claude.removeCalls != 1 {
		t.Fatalf("Restore(claude) result=%+v removeCalls=%d, want claude remove path", claudeResult, claude.removeCalls)
	}

	geminiResult, err := svc.Restore(domainprofile.AgentGemini, "gemini-1")
	if err != nil {
		t.Fatalf("Restore(gemini) error = %v", err)
	}
	if geminiResult.Agent != domainprofile.AgentGemini || gemini.restorePath != geminiBackupPath {
		t.Fatalf("Restore(gemini) result=%+v restorePath=%q, want backup restore", geminiResult, gemini.restorePath)
	}

	report := &DoctorReport{OK: true}
	if err := checkBackupMetadata(report, domainprofile.AgentCodex, domainprofile.Backup{
		ID:          "backup-ok",
		RestoreMode: domainprofile.RestoreModeRestoreFile,
		BackupPath:  geminiBackupPath,
	}); err != nil {
		t.Fatalf("checkBackupMetadata(existing file) error = %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("checkBackupMetadata(existing file) issues = %+v, want none", report.Issues)
	}

	journalSvc := NewProfileService(&fakeProfileRepo{}, state, nil, claude, gemini, nil)
	journalSvc.SetOperationJournal(&errorJournal{beginErr: errors.New("begin failed")})
	if _, err := journalSvc.Restore(domainprofile.AgentGemini, "gemini-1"); err == nil || !strings.Contains(err.Error(), "begin failed") {
		t.Fatalf("Restore(begin failure) err=%v, want begin failure", err)
	}

	if _, err := NewProfileService(&fakeProfileRepo{}, &errorStateRepo{loadErr: errors.New("load failed")}, nil, nil, nil, nil).BackupList(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("BackupList(load failure) err=%v, want load failure", err)
	}

	journal := &fakeJournal{}
	doctorSvc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, nil, nil)
	doctorSvc.SetOperationJournal(journal)
	doctorReport, err := doctorSvc.Doctor()
	if err != nil {
		t.Fatalf("Doctor(happy path) error = %v", err)
	}
	if !doctorReport.OK || len(doctorReport.Issues) != 0 || doctorReport.Operation != nil {
		t.Fatalf("Doctor(happy path) = %+v, want ok report", doctorReport)
	}
}

func TestOperationRollbackForAgentSetClearAndRestore(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before-agent-set")
	codex := &fakeCodexSyncer{codexBase}
	state := &fakeStateRepo{}
	journal := &errorJournal{updateErr: errors.New("update failed")}
	svc := NewProfileService(repo, state, codex, nil, nil, nil)
	svc.SetOperationJournal(journal)
	if _, err := svc.AgentSet(domainprofile.AgentCodex, "relay-a"); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("AgentSet(update failure) err=%v, want update failure", err)
	}
	if string(codex.snapshotContent) != "before-agent-set" {
		t.Fatalf("codex snapshot after AgentSet rollback = %q, want restored content", string(codex.snapshotContent))
	}
	if state.state.Codex.SourceProfile != "" || journal.current != nil {
		t.Fatalf("state/journal after AgentSet rollback = %+v / %+v, want unchanged and cleared", state.state.Codex, journal.current)
	}

	codexBase = newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before-clear")
	codex = &fakeCodexSyncer{codexBase}
	codex.statusErr = errors.New("status failed")
	state = &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{
				SourceProfile: "relay-a",
				Status:        domainprofile.BindingStatusApplied,
				ConfigPath:    "/tmp/codex/config.toml",
			},
		},
	}}
	svc = NewProfileService(repo, state, codex, nil, nil, nil)
	if _, err := svc.Clear(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "status failed") {
		t.Fatalf("Clear(status failure) err=%v, want status failure", err)
	}
	if string(codex.snapshotContent) != "before-clear" {
		t.Fatalf("codex snapshot after Clear rollback = %q, want restored content", string(codex.snapshotContent))
	}
	if state.state.Codex.SourceProfile != "relay-a" {
		t.Fatalf("state after Clear rollback = %+v, want relay-a restored", state.state.Codex)
	}

	codexBase = newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before-restore")
	backupPath := filepath.Join(t.TempDir(), "restore.bak")
	if err := os.WriteFile(backupPath, []byte("restored content"), 0o600); err != nil {
		t.Fatalf("WriteFile(restore backup) error = %v", err)
	}
	codexBase.backups[backupPath] = []byte("restored content")
	codex = &fakeCodexSyncer{codexBase}
	state = &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{
				SourceProfile: "relay-a",
				Status:        domainprofile.BindingStatusApplied,
				ConfigPath:    "/tmp/codex/config.toml",
			},
			Backups: []domainprofile.Backup{
				{ID: "backup-1", BackupPath: backupPath, ConfigPath: "/tmp/codex/config.toml", RestoreMode: domainprofile.RestoreModeRestoreFile},
			},
		},
	}}
	journal = &errorJournal{updateErr: errors.New("update failed")}
	svc = NewProfileService(repo, state, codex, nil, nil, nil)
	svc.SetOperationJournal(journal)
	if _, err := svc.Restore(domainprofile.AgentCodex, "backup-1"); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("Restore(update failure) err=%v, want update failure", err)
	}
	if string(codex.snapshotContent) != "before-restore" {
		t.Fatalf("codex snapshot after Restore rollback = %q, want restored content", string(codex.snapshotContent))
	}
	if state.state.Codex.SourceProfile != "relay-a" || journal.current != nil {
		t.Fatalf("state/journal after Restore rollback = %+v / %+v, want restored and cleared", state.state.Codex, journal.current)
	}
}

func TestAddRemoveRestoreAdditionalFailureBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}
	stateErr := &errorStateRepo{saveErr: errors.New("save failed")}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc := NewProfileService(repo, stateErr, codex, nil, nil, nil)
	if _, err := svc.Add("relay-a", AddProfileInput{
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-a",
		Bind:    []domainprofile.Agent{domainprofile.AgentCodex},
	}); err == nil || !strings.Contains(err.Error(), "save failed") {
		t.Fatalf("Add(save failure) err=%v, want save failure", err)
	}
	if _, err := repo.Get("relay-a"); !IsProfileNotFoundError(err) {
		t.Fatalf("profile after Add rollback err=%v, want not found", err)
	}

	repo = &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	codex = &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.statusErr = errors.New("status failed")
	svc = NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil, nil)
	if _, err := svc.Remove("relay-a"); err == nil || !strings.Contains(err.Error(), "status failed") {
		t.Fatalf("Remove(status failure) err=%v, want status failure", err)
	}
	if _, err := repo.Get("relay-a"); err != nil {
		t.Fatalf("profile after Remove failure err=%v, want still present", err)
	}

	backupPath := filepath.Join(t.TempDir(), "restore.bak")
	if err := os.WriteFile(backupPath, []byte("restored"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before-save-failure")
	codexBase.backups[backupPath] = []byte("restored")
	codex = &fakeCodexSyncer{codexBase}
	restoreState := &errorStateRepo{
		saveErr: errors.New("save failed"),
		state: domainprofile.State{
			Codex: domainprofile.CodexState{
				BindingView: domainprofile.BindingView{
					SourceProfile: "relay-a",
					Status:        domainprofile.BindingStatusApplied,
					ConfigPath:    "/tmp/codex/config.toml",
				},
				Backups: []domainprofile.Backup{
					{ID: "backup-1", BackupPath: backupPath, ConfigPath: "/tmp/codex/config.toml", RestoreMode: domainprofile.RestoreModeRestoreFile},
				},
			},
		},
	}
	svc = NewProfileService(&fakeProfileRepo{}, restoreState, codex, nil, nil, nil)
	if _, err := svc.Restore(domainprofile.AgentCodex, "backup-1"); err == nil || !strings.Contains(err.Error(), "save failed") {
		t.Fatalf("Restore(save failure) err=%v, want save failure", err)
	}
	if string(codex.snapshotContent) != "before-save-failure" {
		t.Fatalf("codex snapshot after Restore(save failure) = %q, want original content", string(codex.snapshotContent))
	}
}

func TestSaveProfileFillsTimestamps(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}
	svc := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil)
	saved, err := svc.saveProfile(domainprofile.Profile{
		Name:    "relay-a",
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-a",
	}, true)
	if err != nil {
		t.Fatalf("saveProfile() error = %v", err)
	}
	if saved.CreatedAt.IsZero() || saved.UpdatedAt.IsZero() {
		t.Fatalf("saveProfile() = %+v, want timestamps set", saved)
	}
}

func TestProfileMutationAdditionalValidationAndFailureBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc := NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil, nil)

	if _, err := svc.Add("bad name", AddProfileInput{
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-a",
		Bind:    []domainprofile.Agent{domainprofile.AgentCodex},
	}); err == nil {
		t.Fatal("Add(invalid name) unexpectedly succeeded")
	}
	if _, err := svc.Add("relay-a", AddProfileInput{
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-a",
		Bind:    []domainprofile.Agent{domainprofile.AgentCodex},
	}); err == nil {
		t.Fatal("Add(duplicate) unexpectedly succeeded")
	}

	errSvc := NewProfileService(&errorProfileRepo{getErr: errors.New("repo failed")}, &fakeStateRepo{}, codex, nil, nil, nil)
	if _, err := errSvc.Add("relay-b", AddProfileInput{
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-a",
		Bind:    []domainprofile.Agent{domainprofile.AgentCodex},
	}); err == nil || !strings.Contains(err.Error(), "repo failed") {
		t.Fatalf("Add(repo get error) err=%v, want repo failure", err)
	}

	result, err := svc.Edit("relay-a", EditProfileInput{})
	if err != nil {
		t.Fatalf("Edit(no-op) error = %v", err)
	}
	if result.Relay.Name != "relay-a" {
		t.Fatalf("Edit(no-op) = %+v, want original relay", result.Relay)
	}

	if _, err := svc.saveProfile(domainprofile.Profile{Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: ""}, false); err == nil {
		t.Fatal("saveProfile(empty api key) unexpectedly succeeded")
	}
}

func TestBindingsOperationAdditionalFailureBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before")
	codex := &fakeCodexSyncer{codexBase}
	state := &fakeStateRepo{}
	svc := NewProfileService(repo, state, codex, nil, nil, nil)
	svc.SetOperationJournal(&errorJournal{beginErr: errors.New("begin failed")})
	if _, err := svc.AgentSet(domainprofile.AgentCodex, "relay-a"); err == nil || !strings.Contains(err.Error(), "begin failed") {
		t.Fatalf("AgentSet(begin failure) err=%v, want begin failure", err)
	}

	codexBase = newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before")
	codex = &fakeCodexSyncer{codexBase}
	codex.clearDefaultErr = errors.New("clear default failed")
	state = &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{SourceProfile: "relay-a"},
		},
	}}
	svc = NewProfileService(repo, state, codex, nil, nil, nil)
	if _, err := svc.Clear(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "clear default failed") {
		t.Fatalf("Clear(clear default failure) err=%v, want clear default failure", err)
	}

	codexBase = newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before")
	codexBase.deleteBackupErr = errors.New("delete backup failed")
	codex = &fakeCodexSyncer{codexBase}
	state = &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{
				SourceProfile: "relay-a",
			},
			Backups: []domainprofile.Backup{
				{ID: "b1", BackupPath: "/tmp/b1"},
				{ID: "b2", BackupPath: "/tmp/b2"},
				{ID: "b3", BackupPath: "/tmp/b3"},
				{ID: "b4", BackupPath: "/tmp/b4"},
				{ID: "b5", BackupPath: "/tmp/b5"},
			},
		},
	}}
	svc = NewProfileService(repo, state, codex, nil, nil, nil)
	if _, err := svc.agentSetLocked(domainprofile.AgentCodex, "relay-a"); err == nil || !strings.Contains(err.Error(), "delete backup failed") {
		t.Fatalf("agentSetLocked(trim cleanup failure) err=%v, want delete backup failure", err)
	}

	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.snapshotContent = []byte(`{"before":true}`)
	claude := &fakeClaudeSyncer{claudeBase}
	saveErrState := &errorStateRepo{saveErr: errors.New("save failed"), state: domainprofile.State{
		Claude: domainprofile.AgentBinding{SourceProfile: "relay-a"},
	}}
	svc = NewProfileService(repo, saveErrState, nil, claude, nil, nil)
	if err := svc.syncProfileToBoundAgents(repo.profiles["relay-a"], &saveErrState.state, time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "save failed") {
		t.Fatalf("syncProfileToBoundAgents(save failure) err=%v, want save failure", err)
	}
}

func TestMutationEntrypointsPropagateLockError(t *testing.T) {
	lockErr := errors.New("lock failed")
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	state := &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			Backups: []domainprofile.Backup{{ID: "backup-1", RestoreMode: domainprofile.RestoreModeRemoveCreatedFile}},
		},
	}}
	svc := NewProfileService(repo, state, nil, nil, nil, nil)
	svc.SetMutationLocker(errorLocker{err: lockErr})

	check := func(name string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "lock failed") {
			t.Fatalf("%s err=%v, want lock failed", name, err)
		}
	}

	_, err := svc.Add("relay-b", AddProfileInput{
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-b",
		Bind:    []domainprofile.Agent{domainprofile.AgentCodex},
	})
	check("Add", err)
	_, err = svc.Edit("relay-a", EditProfileInput{APIKey: ptr("sk-new")})
	check("Edit", err)
	_, err = svc.Remove("relay-a")
	check("Remove", err)
	_, err = svc.AgentSet(domainprofile.AgentCodex, "relay-a")
	check("AgentSet", err)
	_, err = svc.Clear(domainprofile.AgentCodex)
	check("Clear", err)
	_, err = svc.Restore(domainprofile.AgentCodex, "")
	check("Restore", err)
}

func TestMutationGuardRollbackAggregatesFailures(t *testing.T) {
	repo := &errorProfileRepo{deleteErr: errors.New("delete failed")}
	state := &errorStateRepo{saveErr: errors.New("save failed")}
	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.removeErr = errors.New("remove failed")
	codex := &fakeCodexSyncer{codexBase}
	journal := &errorJournal{
		current:  &ports.OperationRecord{ID: "op-1"},
		clearErr: errors.New("clear failed"),
	}
	svc := NewProfileService(repo, state, codex, nil, nil, nil)
	svc.SetOperationJournal(journal)

	guard := newMutationGuard(svc)
	guard.profilesBefore["relay-a"] = nil
	guard.agentSnapshots[domainprofile.AgentCodex] = ports.AgentConfigSnapshot{Exists: false}
	guard.stateBefore = &domainprofile.State{}

	err := guard.Rollback()
	if err == nil {
		t.Fatal("Rollback() unexpectedly succeeded")
	}
	for _, want := range []string{"delete failed", "remove failed", "save failed", "clear failed"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Rollback() err=%v, want substring %q", err, want)
		}
	}
}

func ptr[T any](v T) *T {
	return &v
}

func TestMutationGuardAndRestoreAdditionalBranches(t *testing.T) {
	journal := &errorJournal{}
	svc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, nil, nil)
	svc.SetOperationJournal(journal)
	if err := svc.clearCurrentOperationJournal(); err != nil {
		t.Fatalf("clearCurrentOperationJournal(nil current) error = %v", err)
	}

	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	svc = NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, claude, nil, nil)
	if err := svc.restoreAgentSnapshot(domainprofile.AgentClaude, ports.AgentConfigSnapshot{Exists: false}); err != nil {
		t.Fatalf("restoreAgentSnapshot(remove claude) error = %v", err)
	}
	if claude.removeCalls != 1 {
		t.Fatalf("claude removeCalls = %d, want 1", claude.removeCalls)
	}

	state := &fakeStateRepo{state: domainprofile.State{
		Claude: domainprofile.AgentBinding{
			SourceProfile: "relay-a",
			Backups: []domainprofile.Backup{
				{ID: "backup-1", RestoreMode: domainprofile.RestoreModeRemoveCreatedFile},
			},
		},
	}}
	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.removeErr = errors.New("remove failed")
	claude = &fakeClaudeSyncer{claudeBase}
	svc = NewProfileService(&fakeProfileRepo{}, state, nil, claude, nil, nil)
	if _, err := svc.Restore(domainprofile.AgentClaude, "backup-1"); err == nil || !strings.Contains(err.Error(), "remove failed") {
		t.Fatalf("Restore(remove-config failure) err=%v, want remove failure", err)
	}
}

func TestRestoreAgentSnapshotAdditionalAgents(t *testing.T) {
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	geminiBase := newFakeAgentSyncer("/tmp/gemini/.env")
	geminiBase.restoreErr = errors.New("restore failed")
	gemini := &fakeGeminiSyncer{geminiBase}
	svc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, codex, claude, gemini, nil)

	if err := svc.restoreAgentSnapshot(domainprofile.AgentCodex, ports.AgentConfigSnapshot{Exists: true, Content: []byte("codex restore")}); err != nil {
		t.Fatalf("restoreAgentSnapshot(codex exists) error = %v", err)
	}
	if string(codex.snapshotContent) != "codex restore" {
		t.Fatalf("codex snapshot = %q, want restored content", string(codex.snapshotContent))
	}

	if err := svc.restoreAgentSnapshot(domainprofile.AgentGemini, ports.AgentConfigSnapshot{Exists: false}); err != nil {
		t.Fatalf("restoreAgentSnapshot(gemini remove) error = %v", err)
	}
	if gemini.removeCalls != 1 {
		t.Fatalf("gemini removeCalls = %d, want 1", gemini.removeCalls)
	}

	if err := svc.restoreAgentSnapshot(domainprofile.AgentGemini, ports.AgentConfigSnapshot{Exists: true, Content: []byte("gemini restore")}); err == nil || !strings.Contains(err.Error(), "restore failed") {
		t.Fatalf("restoreAgentSnapshot(gemini restore error) err=%v, want restore failure", err)
	}
}

func TestMutationGuardAdditionalBranches(t *testing.T) {
	guard := newMutationGuard(NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, nil, nil))
	if err := guard.CaptureProfile(""); err != nil {
		t.Fatalf("CaptureProfile(empty) error = %v", err)
	}

	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}
	guard = newMutationGuard(NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil))
	if err := guard.CaptureProfile("relay-a"); err != nil {
		t.Fatalf("CaptureProfile(first) error = %v", err)
	}
	if err := guard.CaptureProfile("relay-a"); err != nil {
		t.Fatalf("CaptureProfile(duplicate) error = %v", err)
	}

	stateGuard := newMutationGuard(NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nil, nil, nil, nil))
	if err := stateGuard.CaptureState(); err != nil {
		t.Fatalf("CaptureState(first) error = %v", err)
	}
	if err := stateGuard.CaptureState(); err != nil {
		t.Fatalf("CaptureState(second) error = %v", err)
	}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotContent = []byte("before")
	svc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, &fakeCodexSyncer{codexBase}, nil, nil, nil)
	guard = newMutationGuard(svc)
	if err := guard.CaptureAgents(domainprofile.AgentCodex, domainprofile.AgentCodex); err != nil {
		t.Fatalf("CaptureAgents(duplicate) error = %v", err)
	}
	if len(guard.agentSnapshots) != 1 {
		t.Fatalf("CaptureAgents(duplicate) snapshots=%d want 1", len(guard.agentSnapshots))
	}

	guard.Commit()
	if err := guard.Rollback(); err != nil {
		t.Fatalf("Rollback(after commit) error = %v", err)
	}

	notFoundRepo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}
	guard = newMutationGuard(NewProfileService(notFoundRepo, &fakeStateRepo{}, nil, nil, nil, nil))
	if err := guard.restoreProfile("relay-a", nil); err != nil {
		t.Fatalf("restoreProfile(not found delete) error = %v", err)
	}

	nilSnapshotCodex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	nilSnapshotCodex.snapshotExists = true
	nilSnapshotSvc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{}, nilSnapshotCodex, nil, nil, nil)
	nilSnapshotCodex.snapshotContent = nil
	snapshot, ok, err := nilSnapshotSvc.snapshotAgentConfig(domainprofile.AgentCodex)
	if err != nil || !ok || len(snapshot.Content) != 0 {
		t.Fatalf("snapshotAgentConfig(empty content) = (%+v,%v,%v), want ok empty content", snapshot, ok, err)
	}
}

func TestBindingsEntrypointsAdditionalFailureBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}

	if _, err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("state load failed")}, nil, nil, nil, nil).AgentSet(domainprofile.AgentCodex, "relay-a"); err == nil || !strings.Contains(err.Error(), "state load failed") {
		t.Fatalf("AgentSet(CaptureState failure) err=%v, want state load failed", err)
	}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codexBase.snapshotErr = errors.New("snapshot failed")
	codex := &fakeCodexSyncer{codexBase}
	if _, err := NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil, nil).AgentSet(domainprofile.AgentCodex, "relay-a"); err == nil || !strings.Contains(err.Error(), "snapshot failed") {
		t.Fatalf("AgentSet(CaptureAgents failure) err=%v, want snapshot failed", err)
	}

	if _, err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("state load failed")}, codex, nil, nil, nil).Clear(domainprofile.AgentCodex); err == nil || !strings.Contains(err.Error(), "state load failed") {
		t.Fatalf("Clear(CaptureState failure) err=%v, want state load failed", err)
	}
}

func TestProfileMutationAdditionalFailureBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}

	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	if _, err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("state load failed")}, codex, nil, nil, nil).Add("relay-b", AddProfileInput{
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-b",
		Bind:    []domainprofile.Agent{domainprofile.AgentCodex},
	}); err == nil || !strings.Contains(err.Error(), "state load failed") {
		t.Fatalf("Add(CaptureState failure) err=%v, want state load failed", err)
	}

	svc := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil)
	if _, err := svc.Edit("missing", EditProfileInput{APIKey: ptr("sk-new")}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Edit(Get failure) err=%v, want not found", err)
	}

	svc = NewProfileService(repo, &errorStateRepo{loadErr: errors.New("state load failed")}, nil, nil, nil, nil)
	if _, err := svc.Edit("relay-a", EditProfileInput{Bind: []domainprofile.Agent{domainprofile.AgentCodex}}); err == nil || !strings.Contains(err.Error(), "state load failed") {
		t.Fatalf("Edit(CaptureState failure) err=%v, want state load failed", err)
	}

	svc = NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil)
	if _, err := svc.Edit("relay-a", EditProfileInput{Bind: []domainprofile.Agent{domainprofile.AgentCodex}, Unbind: []domainprofile.Agent{domainprofile.AgentCodex}}); err == nil || !strings.Contains(err.Error(), "overlapping agents") {
		t.Fatalf("Edit(binding conflict) err=%v, want overlapping agents error", err)
	}

	svc = NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil)
	if _, err := svc.Remove("missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Remove(Get failure) err=%v, want not found", err)
	}

	svc = NewProfileService(repo, &errorStateRepo{loadErr: errors.New("state load failed")}, nil, nil, nil, nil)
	if _, err := svc.Remove("relay-a"); err == nil || !strings.Contains(err.Error(), "state load failed") {
		t.Fatalf("Remove(CaptureState failure) err=%v, want state load failed", err)
	}

	svc = NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil)
	if _, err := svc.Remove("relay-a"); err != nil {
		t.Fatalf("Remove(success) error = %v", err)
	}
}

func TestStateAndBindingsAdditionalBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}

	codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
	codex := &fakeCodexSyncer{codexBase}
	codex.managedProfiles["relay-a"] = ports.CodexManagedProfile{Name: "relay-a", BaseURL: "https://relay.example/v1"}
	stateRepo := &fakeStateRepo{state: domainprofile.State{
	}}
	svc := NewProfileService(repo, stateRepo, codex, nil, nil, nil)
	if err := svc.syncProfileAfterMutation(repo.profiles["relay-a"], true); err != nil {
		t.Fatalf("syncProfileAfterMutation(codex registry) error = %v", err)
	}

	stateRepo = &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{SourceProfile: "relay-b"},
		},
	}}
	codex = &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc = NewProfileService(repo, stateRepo, codex, nil, nil, nil)
	if err := svc.syncCodexProfile(repo.profiles["relay-a"], &stateRepo.state, time.Now().UTC()); err != nil {
		t.Fatalf("syncCodexProfile(inactive source) error = %v", err)
	}
	if stateRepo.state.Codex.SourceProfile != "relay-b" {
		t.Fatalf("Codex.SourceProfile after syncCodexProfile = %q, want relay-b unchanged", stateRepo.state.Codex.SourceProfile)
	}

	stateRepo = &fakeStateRepo{state: domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{SourceProfile: "relay-b"},
		},
	}}
	if agents := affectedAgentsForProfileMutation("relay-a", stateRepo.state, false, []domainprofile.Agent{domainprofile.Agent("bad")}, nil); len(agents) != 0 {
		t.Fatalf("affectedAgentsForProfileMutation(invalid only) = %v, want empty", agents)
	}

	if err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, nil, nil, nil, nil).clearCodexSourceProfileIfMatches("relay-a"); err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("clearCodexSourceProfileIfMatches(load failure) err=%v, want load failed", err)
	}

	codex = &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.statusErr = errors.New("status failed")
	if _, err := NewProfileService(repo, &fakeStateRepo{}, codex, nil, nil, nil).State(); err == nil || !strings.Contains(err.Error(), "status failed") {
		t.Fatalf("State(status failure) err=%v, want status failed", err)
	}

	if _, err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, nil, nil, nil, nil).boundAgentsForProfile("relay-a"); err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("boundAgentsForProfile(load failure) err=%v, want load failed", err)
	}
}

func TestRestoreAndDoctorAdditionalBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}

	seq := &sequenceStateRepo{
		loads: []domainprofile.State{{
			Codex: domainprofile.CodexState{
				BindingView: domainprofile.BindingView{
					SourceProfile: "relay-a",
					Status:        domainprofile.BindingStatusApplied,
					ConfigPath:    "/tmp/codex/config.toml",
				},
				Backups: []domainprofile.Backup{{ID: "backup-1", RestoreMode: domainprofile.RestoreModeRemoveCreatedFile}},
			},
		}},
		saves: []error{errors.New("save failed")},
	}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	if _, err := NewProfileService(repo, seq, codex, nil, nil, nil).Restore(domainprofile.AgentCodex, "backup-1"); err == nil || !strings.Contains(err.Error(), "save failed") {
		t.Fatalf("Restore(save failure) err=%v, want save failed", err)
	}

	seq = &sequenceStateRepo{
		loads: []domainprofile.State{{
			Claude: domainprofile.AgentBinding{
				SourceProfile: "relay-a",
				Backups:       []domainprofile.Backup{{ID: "backup-1", RestoreMode: domainprofile.RestoreModeRemoveCreatedFile}},
			},
		}},
	}
	if _, err := NewProfileService(repo, seq, nil, nil, nil, nil).Restore(domainprofile.AgentClaude, "backup-1"); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("Restore(missing claude syncer) err=%v, want invalid agent", err)
	}

	doctorSvc := NewProfileService(&errorProfileRepo{listErr: errors.New("list failed")}, &fakeStateRepo{}, nil, nil, nil, nil)
	if _, err := doctorSvc.Doctor(); err == nil || !strings.Contains(err.Error(), "list failed") {
		t.Fatalf("Doctor(list failure) err=%v, want list failed", err)
	}

	doctorSvc = NewProfileService(repo, &fakeStateRepo{}, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil, nil)
	doctorSvc.SetOperationJournal(&fakeJournal{current: &ports.OperationRecord{ID: "op-1", Command: "set", Agent: domainprofile.AgentCodex, Stage: "started"}})
	report, err := doctorSvc.Doctor()
	if err != nil {
		t.Fatalf("Doctor(unfinished operation) error = %v", err)
	}
	if report.Operation == nil || report.Operation.ID != "op-1" {
		t.Fatalf("Doctor().Operation = %+v, want op-1", report.Operation)
	}
	if len(report.Issues) == 0 || report.Issues[0].Action == "" {
		t.Fatalf("Doctor().Issues = %+v, want unfinished operation action", report.Issues)
	}
}

func TestBindingHelpersAdditionalErrorBranches(t *testing.T) {
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-a": {Name: "relay-a", BaseURL: "https://relay.example/v1", APIKey: "sk-a", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}}

	if _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).agentSetLocked(domainprofile.Agent("bad"), "relay-a"); err == nil {
		t.Fatal("agentSetLocked(invalid agent) unexpectedly succeeded")
	}

	if _, err := NewProfileService(repo, &errorStateRepo{loadErr: errors.New("load failed")}, nil, nil, nil, nil).agentSetLocked(domainprofile.AgentCodex, "relay-a"); err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("agentSetLocked(load failure) err=%v, want load failed", err)
	}

	claudeBase := newFakeAgentSyncer("/tmp/claude/settings.json")
	claudeBase.syncErr = errors.New("sync failed")
	claude := &fakeClaudeSyncer{claudeBase}
	if _, _, _, err := NewProfileService(repo, &fakeStateRepo{}, nil, claude, nil, nil).syncProfileToAgent(domainprofile.AgentClaude, repo.profiles["relay-a"], "backup-1", domainprofile.State{}, false); err == nil || !strings.Contains(err.Error(), "sync failed") {
		t.Fatalf("syncProfileToAgent(claude sync failure) err=%v, want sync failed", err)
	}

	if _, err := NewProfileService(repo, &fakeStateRepo{}, nil, nil, nil, nil).snapshotCurrentConfig(domainprofile.AgentClaude, "relay-a", "backup-1", time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("snapshotCurrentConfig(missing claude) err=%v, want invalid agent", err)
	}

	state := &fakeStateRepo{state: domainprofile.State{
		Claude: domainprofile.AgentBinding{SourceProfile: "relay-a"},
	}}
	if err := NewProfileService(repo, state, nil, nil, nil, nil).syncProfileToBoundAgents(repo.profiles["relay-a"], &state.state, time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("syncProfileToBoundAgents(missing syncer for bound claude) err=%v, want invalid agent", err)
	}
}

func TestStateBindingsAndRenameCoverageExtras(t *testing.T) {
	stored := domainprofile.State{
	}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	codex.defaultProfile = "relay-old"
	codex.managedProfiles["relay-old"] = ports.CodexManagedProfile{Name: "relay-old", BaseURL: "https://relay.example/v1"}
	openCode := &fakeOpenCodeSyncer{fakeAgentSyncer: newFakeAgentSyncer("/tmp/opencode.json")}
	openCode.status = &ports.OpenCodeConfigStatus{
		ConfigPath:   "/tmp/opencode.json",
		DefaultModel: "agx-relay-old/model-old",
		ManagedProvidersByID: map[string]ports.OpenCodeManagedProvider{
			"agx-relay-old": {ID: "agx-relay-old", Family: domainprofile.OpenCodeProviderFamilyGemini, Model: "model-old"},
		},
	}
	svc := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{state: stored}, codex, nil, nil, openCode)

	resolved, err := svc.loadResolvedState()
	if err != nil {
		t.Fatalf("loadResolvedState() error = %v", err)
	}
	if resolved.Codex.SourceProfile != "relay-old" || resolved.OpenCode.SourceProfile != "relay-old" {
		t.Fatalf("loadResolvedState() = %+v", resolved)
	}

	applyResolvedCodexStatus(&domainprofile.State{}, nil)
	applyResolvedOpenCodeStatus(&domainprofile.State{}, nil)

	if got := agentProfileBindings(stored, domainprofile.Agent("bad")); len(got) != 0 {
		t.Fatalf("agentProfileBindings(invalid) = %+v", got)
	}
	if got := agentProfileBinding(stored, domainprofile.Agent("bad"), "relay-old"); got != (domainprofile.AgentProfileBinding{}) {
		t.Fatalf("agentProfileBinding(invalid) = %+v", got)
	}

	copyState := cloneState(stored)
	copyState.Codex.SourceProfile = "relay-old"
	copyState.OpenCode.SourceProfile = "relay-old"
	renameStateProfileReferences(&copyState, "", "relay-new")
	if copyState.Codex.SourceProfile != "relay-old" {
		t.Fatalf("renameStateProfileReferences(empty old) mutated state = %+v", copyState.Codex)
	}
	renameStateProfileReferences(&copyState, "relay-old", "relay-old")
	if copyState.Codex.SourceProfile != "relay-old" {
		t.Fatalf("renameStateProfileReferences(same names) mutated state = %+v", copyState.Codex)
	}
	renameStateProfileReferences(&copyState, "relay-old", "relay-new")
	if copyState.Codex.SourceProfile != "relay-new" {
		t.Fatalf("renameStateProfileReferences(rename) = %+v", copyState)
	}

	if _, err := NewProfileService(&fakeProfileRepo{}, &errorStateRepo{loadErr: errors.New("codex load failed")}, codex, nil, nil, openCode).loadResolvedState(); err == nil || !strings.Contains(err.Error(), "codex load failed") {
		t.Fatalf("loadResolvedState(load failure) err=%v, want codex load failed", err)
	}
	if _, err := NewProfileService(&fakeProfileRepo{}, &fakeStateRepo{state: stored}, &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}, nil, nil, &fakeOpenCodeSyncer{fakeAgentSyncer: newFakeAgentSyncer("/tmp/opencode.json"), statusErr: errors.New("opencode status failed")}).loadResolvedState(); err == nil || !strings.Contains(err.Error(), "opencode status failed") {
		t.Fatalf("loadResolvedState(status failure) err=%v, want opencode status failed", err)
	}
}

func TestSyncRenamedProfileCoverageExtras(t *testing.T) {
	now := time.Date(2026, 5, 7, 13, 0, 0, 0, time.UTC)
	repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
		"relay-new": {Name: "relay-new", BaseURL: "https://relay.example/v1", APIKey: "sk-new", CreatedAt: now, UpdatedAt: now},
	}}
	state := &domainprofile.State{
		Codex: domainprofile.CodexState{
			BindingView: domainprofile.BindingView{SourceProfile: "relay-old", Status: domainprofile.BindingStatusApplied, ConfigPath: "/tmp/codex/config.toml"},
			Backups:     []domainprofile.Backup{{ID: "backup-codex"}},
		},
		Claude: domainprofile.AgentBinding{
			SourceProfile: "relay-old",
			Status:        domainprofile.BindingStatusApplied,
			ConfigPath:    "/tmp/claude/settings.json",
			Backups:       []domainprofile.Backup{{ID: "backup-claude"}},
		},
		Gemini: domainprofile.AgentBinding{
			SourceProfile: "relay-other",
			Status:        domainprofile.BindingStatusApplied,
			ConfigPath:    "/tmp/gemini/.env",
			Backups:       []domainprofile.Backup{{ID: "backup-gemini"}},
		},
		OpenCode: domainprofile.OpenCodeState{
			BindingView: domainprofile.BindingView{SourceProfile: "relay-old", Status: domainprofile.BindingStatusApplied, ConfigPath: "/tmp/opencode.json"},
			Backups:     []domainprofile.Backup{{ID: "backup-open"}},
		},
	}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	claude := &fakeClaudeSyncer{newFakeAgentSyncer("/tmp/claude/settings.json")}
	gemini := &fakeGeminiSyncer{newFakeAgentSyncer("/tmp/gemini/.env")}
	openCode := &fakeOpenCodeSyncer{fakeAgentSyncer: newFakeAgentSyncer("/tmp/opencode.json")}
	svc := NewProfileService(repo, &fakeStateRepo{state: *state}, codex, claude, gemini, openCode)

	if err := svc.syncRenamedProfile(repo.profiles["relay-new"], "relay-old", state, now); err != nil {
		t.Fatalf("syncRenamedProfile() error = %v", err)
	}
	if codex.syncCalls == 0 || claude.syncCalls == 0 || openCode.syncCalls == 0 {
		t.Fatalf("sync calls = codex:%d claude:%d gemini:%d openCode:%d", codex.syncCalls, claude.syncCalls, gemini.syncCalls, openCode.syncCalls)
	}
	if gemini.syncCalls != 0 {
		t.Fatalf("gemini.syncCalls = %d, want 0 for bound-only registry rename", gemini.syncCalls)
	}
	if state.Codex.SourceProfile != "relay-new" || state.Claude.SourceProfile != "relay-new" {
		t.Fatalf("syncRenamedProfile() state=%+v", state)
	}
}
