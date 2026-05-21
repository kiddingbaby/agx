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

func TestActivateManagedProfileCreatesDerivedRelayTargetButListStaysProfileFirst(t *testing.T) {
	repo := &fakeProfileRepo{
		profiles: map[string]domainprofile.Profile{
			"work": {
				Name:      "work",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-work",
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
		},
	}
	state := &fakeStateRepo{}
	svc := NewProfileService(repo, state, nil, nil, nil, nil)

	contextsDir := filepath.Join(t.TempDir(), "contexts")
	backupsDir := filepath.Join(t.TempDir(), "backups")
	var codexSyncers []*fakeCodexSyncer
	svc.SetManagedRuntime(
		ManagedPaths{
			ContextsDir:   contextsDir,
			BackupsDir:    backupsDir,
			HelperCommand: "agx",
		},
		ManagedSyncerFactory{
			NewCodex: func(configPath, backupsDir, helperCommand string) ports.CodexSyncer {
				syncer := &fakeCodexSyncer{newFakeAgentSyncer(configPath)}
				codexSyncers = append(codexSyncers, syncer)
				return syncer
			},
		},
	)

	result, err := svc.ActivateManagedProfile(domainprofile.AgentCodex, "work")
	if err != nil {
		t.Fatalf("ActivateManagedProfile() error = %v", err)
	}
	if result.Target.Kind != domainprofile.TargetKindRelay || result.Target.Relay.ProfileName != "codex.work" {
		t.Fatalf("ActivateManagedProfile() target = %+v", result.Target)
	}
	if len(codexSyncers) != 1 || codexSyncers[0].lastProfile.Name != "codex.work" {
		t.Fatalf("managed codex syncers = %+v", codexSyncers)
	}
	if _, ok := repo.profiles["codex.work"]; !ok {
		t.Fatalf("repo profiles = %+v, want derived codex.work", repo.profiles)
	}

	profiles, current, err := svc.ListManagedProfiles()
	if err != nil {
		t.Fatalf("ListManagedProfiles() error = %v", err)
	}
	if current != "" {
		t.Fatalf("ListManagedProfiles() current = %q, want empty", current)
	}
	if len(profiles) != 1 || profiles[0].Profile.Name != "work" {
		t.Fatalf("ListManagedProfiles() profiles = %+v, want only top-level work", profiles)
	}
}

func TestAddManagedProfileRejectsReservedDerivedName(t *testing.T) {
	repo := &fakeProfileRepo{}
	state := &fakeStateRepo{}
	svc := NewProfileService(repo, state, nil, nil, nil, nil)

	if _, err := svc.AddManagedProfile("codex.work", domainprofile.Profile{
		Kind:    domainprofile.ProfileKindRelay,
		BaseURL: "https://relay.example/v1",
		APIKey:  "sk-work",
	}); err == nil {
		t.Fatal("AddManagedProfile() unexpectedly succeeded for reserved name")
	}
}

func TestListManagedProfilesHidesReservedInternalNames(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeProfileRepo{
		profiles: map[string]domainprofile.Profile{
			"work": {
				Name:      "work",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-work",
				CreatedAt: now,
				UpdatedAt: now,
			},
			"codex.work": {
				Name:      "codex.work",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-internal",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	state := &fakeStateRepo{}
	svc := NewProfileService(repo, state, nil, nil, nil, nil)

	profiles, _, err := svc.ListManagedProfiles()
	if err != nil {
		t.Fatalf("ListManagedProfiles() error = %v", err)
	}
	if len(profiles) != 1 || profiles[0].Profile.Name != "work" {
		t.Fatalf("ListManagedProfiles() profiles = %+v, want only top-level work", profiles)
	}

	allProfiles, _, err := svc.ListAllProfiles()
	if err != nil {
		t.Fatalf("ListAllProfiles() error = %v", err)
	}
	if len(allProfiles) != 2 {
		t.Fatalf("ListAllProfiles() profiles = %+v, want both work and codex.work", allProfiles)
	}
	names := map[string]bool{}
	for _, summary := range allProfiles {
		names[summary.Profile.Name] = true
	}
	if !names["work"] || !names["codex.work"] {
		t.Fatalf("ListAllProfiles() names = %v, want both work and codex.work", names)
	}
}

func TestEditManagedProfileResyncsAllReferencingTargets(t *testing.T) {
	now := time.Now().UTC()
	contextsDir := filepath.Join(t.TempDir(), "contexts")
	repo := &fakeProfileRepo{
		profiles: map[string]domainprofile.Profile{
			"work": {
				Name:      "work",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-old",
				ModelID:   "gpt-old",
				CreatedAt: now,
				UpdatedAt: now,
			},
			"codex.work": {
				Name:      "codex.work",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-old",
				ModelID:   "gpt-old",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	state := &fakeStateRepo{
		state: domainprofile.State{
			ManagedAgents: map[domainprofile.Agent]domainprofile.ManagedAgentState{
				domainprofile.AgentCodex: {
					Targets: map[string]domainprofile.TargetState{
						"work": {
							Kind: domainprofile.TargetKindRelay,
							ContextPath: filepath.Join(contextsDir, "codex", "targets", "work"),
							Relay: domainprofile.RelayTargetState{
								ProfileName: "codex.work",
								BaseURL:     "https://relay.example/v1",
								ModelID:     "gpt-old",
								ModelName:   "gpt-old",
							},
							CreatedAt: now,
							UpdatedAt: now,
						},
					},
				},
			},
		},
	}
	codexSyncer := &fakeCodexSyncer{newFakeAgentSyncer(filepath.Join(contextsDir, "codex", "targets", "work", "config.toml"))}
	svc := NewProfileService(repo, state, nil, nil, nil, nil)
	svc.SetManagedRuntime(
		ManagedPaths{
			ContextsDir:   contextsDir,
			BackupsDir:    filepath.Join(t.TempDir(), "backups"),
			HelperCommand: "agx",
		},
		ManagedSyncerFactory{
			NewCodex: func(configPath, backupsDir, helperCommand string) ports.CodexSyncer {
				return codexSyncer
			},
		},
	)

	result, err := svc.EditManagedProfile("work", UpdateManagedProfileInput{ModelID: ptr("gpt-new")})
	if err != nil {
		t.Fatalf("EditManagedProfile error = %v", err)
	}
	if result.Profile == nil || result.Profile.ModelID != "gpt-new" {
		t.Fatalf("result.Profile = %+v, want ModelID=gpt-new", result.Profile)
	}
	if len(result.ResyncedTargets) != 1 || result.ResyncedTargets[0].Agent != domainprofile.AgentCodex || result.ResyncedTargets[0].TargetName != "work" {
		t.Fatalf("result.ResyncedTargets = %+v, want codex/work", result.ResyncedTargets)
	}
	if len(result.FailedTargets) != 0 {
		t.Fatalf("result.FailedTargets = %+v, want empty", result.FailedTargets)
	}
	if codexSyncer.lastProfile.ModelID != "gpt-new" {
		t.Fatalf("codex syncer received profile = %+v, want ModelID=gpt-new", codexSyncer.lastProfile)
	}
	derived, ok := repo.profiles["codex.work"]
	if !ok || derived.ModelID != "gpt-new" {
		t.Fatalf("derived profile = %+v, want ModelID=gpt-new", derived)
	}
	target := state.state.ManagedAgents[domainprofile.AgentCodex].Targets["work"]
	if target.Relay.ModelID != "gpt-new" {
		t.Fatalf("target.Relay.ModelID = %q, want gpt-new", target.Relay.ModelID)
	}
}

func TestEditManagedProfileSurvivesPartialSyncFailure(t *testing.T) {
	now := time.Now().UTC()
	contextsDir := filepath.Join(t.TempDir(), "contexts")
	repo := &fakeProfileRepo{
		profiles: map[string]domainprofile.Profile{
			"work": {
				Name:      "work",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-old",
				ModelID:   "gpt-old",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	state := &fakeStateRepo{
		state: domainprofile.State{
			ManagedAgents: map[domainprofile.Agent]domainprofile.ManagedAgentState{
				domainprofile.AgentCodex: {
					Targets: map[string]domainprofile.TargetState{
						"work": {
							Kind:        domainprofile.TargetKindRelay,
							ContextPath: filepath.Join(contextsDir, "codex", "targets", "work"),
							Relay:       domainprofile.RelayTargetState{ProfileName: "codex.work", BaseURL: "https://relay.example/v1"},
							CreatedAt:   now,
							UpdatedAt:   now,
						},
					},
				},
			},
		},
	}
	failingSyncer := &fakeCodexSyncer{newFakeAgentSyncer(filepath.Join(contextsDir, "codex", "targets", "work", "config.toml"))}
	failingSyncer.syncErr = errors.New("codex out of disk")
	svc := NewProfileService(repo, state, nil, nil, nil, nil)
	svc.SetManagedRuntime(
		ManagedPaths{ContextsDir: contextsDir, BackupsDir: filepath.Join(t.TempDir(), "backups"), HelperCommand: "agx"},
		ManagedSyncerFactory{NewCodex: func(configPath, backupsDir, helperCommand string) ports.CodexSyncer { return failingSyncer }},
	)

	result, err := svc.EditManagedProfile("work", UpdateManagedProfileInput{ModelID: ptr("gpt-new")})
	if err != nil {
		t.Fatalf("EditManagedProfile error = %v", err)
	}
	if result.Profile == nil || result.Profile.ModelID != "gpt-new" {
		t.Fatalf("result.Profile = %+v, want ModelID=gpt-new committed regardless of sync failure", result.Profile)
	}
	if len(result.ResyncedTargets) != 0 {
		t.Fatalf("result.ResyncedTargets = %+v, want empty", result.ResyncedTargets)
	}
	if len(result.FailedTargets) != 1 || result.FailedTargets[0].Agent != domainprofile.AgentCodex {
		t.Fatalf("result.FailedTargets = %+v, want codex failure", result.FailedTargets)
	}
}

func TestEditManagedProfileRenamesProfileAndManagedTargets(t *testing.T) {
	now := time.Now().UTC()
	contextsDir := filepath.Join(t.TempDir(), "contexts")
	oldContextPath := filepath.Join(contextsDir, "codex", "targets", "work")
	if err := os.MkdirAll(oldContextPath, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	repo := &fakeProfileRepo{
		profiles: map[string]domainprofile.Profile{
			"work": {
				Name:      "work",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-work",
				ModelID:   "gpt-5.5",
				CreatedAt: now,
				UpdatedAt: now,
			},
			"codex.work": {
				Name:      "codex.work",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-work",
				ModelID:   "gpt-5.5",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	state := &fakeStateRepo{
		state: domainprofile.State{
			CurrentProfile: "work",
			ManagedAgents: map[domainprofile.Agent]domainprofile.ManagedAgentState{
				domainprofile.AgentCodex: {
					CurrentTarget: domainprofile.CurrentTarget{Kind: domainprofile.TargetKindRelay, Name: "work"},
					Targets: map[string]domainprofile.TargetState{
						"work": {
							Kind:        domainprofile.TargetKindRelay,
							ContextPath: oldContextPath,
							ConfigPath:  filepath.Join(oldContextPath, "config.toml"),
							Relay: domainprofile.RelayTargetState{
								ProfileName:    "codex.work",
								BaseURL:        "https://relay.example/v1",
								ProviderFamily: domainprofile.OpenCodeProviderFamilyOpenAICompatible,
								ModelID:        "gpt-5.5",
								ModelName:      "gpt-5.5",
							},
							CreatedAt: now,
							UpdatedAt: now,
						},
					},
				},
			},
		},
	}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc := NewProfileService(repo, state, codex, nil, nil, nil)
	svc.SetManagedRuntime(
		ManagedPaths{ContextsDir: contextsDir, BackupsDir: filepath.Join(t.TempDir(), "backups"), HelperCommand: "agx"},
		ManagedSyncerFactory{
			NewCodex: func(configPath, backupsDir, helperCommand string) ports.CodexSyncer {
				return codex
			},
		},
	)

	result, err := svc.EditManagedProfile("work", UpdateManagedProfileInput{Name: ptr("focus")})
	if err != nil {
		t.Fatalf("EditManagedProfile(rename) error = %v", err)
	}
	if result.Profile == nil || result.Profile.Name != "focus" {
		t.Fatalf("EditManagedProfile(rename) result = %+v, want focus", result)
	}
	if _, ok := repo.profiles["work"]; ok {
		t.Fatalf("profiles = %+v, want work removed", repo.profiles)
	}
	if _, ok := repo.profiles["focus"]; !ok {
		t.Fatalf("profiles = %+v, want focus present", repo.profiles)
	}
	if _, ok := repo.profiles["codex.work"]; ok {
		t.Fatalf("profiles = %+v, want codex.work removed", repo.profiles)
	}
	if state.state.CurrentProfile != "focus" {
		t.Fatalf("CurrentProfile = %q, want focus", state.state.CurrentProfile)
	}
	managed := state.state.ManagedAgents[domainprofile.AgentCodex]
	if managed.CurrentTarget.Name != "focus" {
		t.Fatalf("CurrentTarget = %+v, want focus", managed.CurrentTarget)
	}
	if _, ok := managed.Targets["work"]; ok {
		t.Fatalf("Targets = %+v, want work removed", managed.Targets)
	}
	target, ok := managed.Targets["focus"]
	if !ok {
		t.Fatalf("Targets = %+v, want focus present", managed.Targets)
	}
	if target.Relay.ProfileName != "codex.focus" {
		t.Fatalf("target = %+v, want derived profile codex.focus", target)
	}
	if target.ContextPath != filepath.Join(contextsDir, "codex", "targets", "focus") {
		t.Fatalf("target context = %q, want renamed managed context", target.ContextPath)
	}
	if _, err := os.Stat(oldContextPath); !os.IsNotExist(err) {
		t.Fatalf("Stat(oldContextPath) err = %v, want not exists", err)
	}
	if _, err := os.Stat(target.ContextPath); err != nil {
		t.Fatalf("Stat(newContextPath) error = %v", err)
	}
	if codex.lastProfile.Name != "codex.focus" {
		t.Fatalf("codex last profile = %+v, want codex.focus", codex.lastProfile)
	}
	if _, ok := codex.managedProfiles["codex.work"]; ok {
		t.Fatalf("managedProfiles = %+v, want codex.work removed", codex.managedProfiles)
	}
	if _, ok := codex.managedProfiles["codex.focus"]; !ok {
		t.Fatalf("managedProfiles = %+v, want codex.focus present", codex.managedProfiles)
	}
}

func TestActivateManagedProfileOpenCodeWithoutModelGivesActionableHint(t *testing.T) {
	now := time.Now().UTC()
	contextsDir := filepath.Join(t.TempDir(), "contexts")
	repo := &fakeProfileRepo{
		profiles: map[string]domainprofile.Profile{
			"work": {
				Name:      "work",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-work",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	state := &fakeStateRepo{}
	svc := NewProfileService(repo, state, nil, nil, nil, nil)
	svc.SetManagedRuntime(
		ManagedPaths{ContextsDir: contextsDir, BackupsDir: filepath.Join(t.TempDir(), "backups"), HelperCommand: "agx"},
		ManagedSyncerFactory{
			NewOpenCode: func(configPath, backupsDir string) ports.OpenCodeSyncer {
				return &fakeOpenCodeSyncer{fakeAgentSyncer: newFakeAgentSyncer(configPath)}
			},
		},
	)

	_, err := svc.ActivateManagedProfile(domainprofile.AgentOpenCode, "work")
	if err == nil {
		t.Fatal("ActivateManagedProfile expected error when profile has no model")
	}
	msg := err.Error()
	if !strings.Contains(msg, "opencode model is required") {
		t.Fatalf("error %q should include underlying validation message", msg)
	}
	if !strings.Contains(msg, "agx edit work --model") {
		t.Fatalf("error %q should hint `agx edit work --model <id>`", msg)
	}
}

func TestManagedProfileVisibleWithLegacyTargetReference(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeProfileRepo{
		profiles: map[string]domainprofile.Profile{
			"tmp": {
				Name:      "tmp",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-tmp",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	state := &fakeStateRepo{
		state: domainprofile.State{
			CurrentProfile: "tmp",
			ManagedAgents: map[domainprofile.Agent]domainprofile.ManagedAgentState{
				domainprofile.AgentCodex: {
					CurrentTarget: domainprofile.CurrentTarget{Kind: domainprofile.TargetKindRelay, Name: "tmp"},
					Targets: map[string]domainprofile.TargetState{
						"tmp": {
							Kind: domainprofile.TargetKindRelay,
							Relay: domainprofile.RelayTargetState{
								ProfileName: "tmp",
								BaseURL:     "https://relay.example/v1",
							},
							CreatedAt: now,
							UpdatedAt: now,
						},
					},
				},
			},
		},
	}
	svc := NewProfileService(repo, state, nil, nil, nil, nil)

	got, err := svc.ManagedProfile("tmp")
	if err != nil {
		t.Fatalf("ManagedProfile(tmp) error = %v", err)
	}
	if got.Name != "tmp" {
		t.Fatalf("ManagedProfile(tmp) = %+v, want tmp", got)
	}

	current, err := svc.CurrentManagedProfile()
	if err != nil {
		t.Fatalf("CurrentManagedProfile() error = %v", err)
	}
	if current == nil || current.Name != "tmp" {
		t.Fatalf("CurrentManagedProfile() = %+v, want tmp", current)
	}

	profiles, listedCurrent, err := svc.ListManagedProfiles()
	if err != nil {
		t.Fatalf("ListManagedProfiles() error = %v", err)
	}
	if listedCurrent != "tmp" {
		t.Fatalf("ListManagedProfiles() current = %q, want tmp", listedCurrent)
	}
	if len(profiles) != 1 || profiles[0].Profile.Name != "tmp" {
		t.Fatalf("ListManagedProfiles() profiles = %+v, want only tmp", profiles)
	}
}

func TestBoundAgentsIncludesManagedTargets(t *testing.T) {
	now := time.Now().UTC()
	state := domainprofile.State{
		ManagedAgents: map[domainprofile.Agent]domainprofile.ManagedAgentState{
			domainprofile.AgentCodex: {
				Targets: map[string]domainprofile.TargetState{
					"tmp": {
						Kind:      domainprofile.TargetKindRelay,
						Relay:     domainprofile.RelayTargetState{ProfileName: "codex.tmp"},
						CreatedAt: now,
						UpdatedAt: now,
					},
				},
			},
		},
	}

	got := boundAgents("tmp", state)
	if len(got) != 1 || got[0] != domainprofile.AgentCodex {
		t.Fatalf("boundAgents(tmp) = %v, want [codex]", got)
	}
}

func TestDoctorIgnoresProfilesBoundOnlyByManagedTargets(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeProfileRepo{
		profiles: map[string]domainprofile.Profile{
			"tmp": {
				Name:      "tmp",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-tmp",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	state := &fakeStateRepo{
		state: domainprofile.State{
			ManagedAgents: map[domainprofile.Agent]domainprofile.ManagedAgentState{
				domainprofile.AgentCodex: {
					Targets: map[string]domainprofile.TargetState{
						"tmp": {
							Kind:      domainprofile.TargetKindRelay,
							Relay:     domainprofile.RelayTargetState{ProfileName: "codex.tmp"},
							CreatedAt: now,
							UpdatedAt: now,
						},
					},
				},
			},
		},
	}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc := NewProfileService(repo, state, codex, nil, nil, nil)

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	for _, issue := range report.Issues {
		if issue.Code == "unconfigured_relay" {
			t.Fatalf("Doctor() reported unconfigured_relay for managed-target profile: %+v", issue)
		}
	}
}

func TestDoctorSkipsDerivedProfilesInUnconfiguredCheck(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeProfileRepo{
		profiles: map[string]domainprofile.Profile{
			"tmp": {
				Name:      "tmp",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-tmp",
				CreatedAt: now,
				UpdatedAt: now,
			},
			"codex.tmp": {
				Name:      "codex.tmp",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-tmp",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	state := &fakeStateRepo{
		state: domainprofile.State{
			ManagedAgents: map[domainprofile.Agent]domainprofile.ManagedAgentState{
				domainprofile.AgentCodex: {
					Targets: map[string]domainprofile.TargetState{
						"tmp": {
							Kind:      domainprofile.TargetKindRelay,
							Relay:     domainprofile.RelayTargetState{ProfileName: "codex.tmp"},
							CreatedAt: now,
							UpdatedAt: now,
						},
					},
				},
			},
		},
	}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc := NewProfileService(repo, state, codex, nil, nil, nil)

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	for _, issue := range report.Issues {
		if issue.Code == "unconfigured_relay" {
			t.Fatalf("Doctor() reported unconfigured_relay for derived profile: %+v", issue)
		}
	}
}
