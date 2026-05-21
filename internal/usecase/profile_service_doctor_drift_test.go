package usecase

import (
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func TestDoctorReportsOrphanDerivedProfile(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeProfileRepo{
		profiles: map[string]domainprofile.Profile{
			"codex.ghost": {
				Name:      "codex.ghost",
				Kind:      domainprofile.ProfileKindRelay,
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-ghost",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	state := &fakeStateRepo{}
	codex := &fakeCodexSyncer{newFakeAgentSyncer("/tmp/codex/config.toml")}
	svc := NewProfileService(repo, state, codex, nil, nil, nil)

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if !hasIssueCode(report, "orphan_derived_profile") {
		t.Fatalf("Doctor() issues = %+v, want orphan_derived_profile", report.Issues)
	}
}

func TestDoctorDoesNotReportOrphanWhenManagedTargetExists(t *testing.T) {
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
				APIKey:    "sk-work",
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
							Kind:      domainprofile.TargetKindRelay,
							Relay:     domainprofile.RelayTargetState{ProfileName: "codex.work"},
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
	if hasIssueCode(report, "orphan_derived_profile") {
		t.Fatalf("Doctor() should not report orphan_derived_profile: %+v", report.Issues)
	}
}

func TestDoctorReportsOrphanManagedTarget(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeProfileRepo{}
	state := &fakeStateRepo{
		state: domainprofile.State{
			ManagedAgents: map[domainprofile.Agent]domainprofile.ManagedAgentState{
				domainprofile.AgentCodex: {
					Targets: map[string]domainprofile.TargetState{
						"ghost": {
							Kind:      domainprofile.TargetKindRelay,
							Relay:     domainprofile.RelayTargetState{ProfileName: "codex.ghost"},
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
	if !hasIssueCode(report, "orphan_managed_target") {
		t.Fatalf("Doctor() issues = %+v, want orphan_managed_target", report.Issues)
	}
}

func TestDoctorAcceptsUserProfileOrDerivedAsTargetBacking(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name     string
		profiles map[string]domainprofile.Profile
	}{
		{
			name: "user profile only",
			profiles: map[string]domainprofile.Profile{
				"work": {Name: "work", Kind: domainprofile.ProfileKindRelay, BaseURL: "u", APIKey: "k", CreatedAt: now, UpdatedAt: now},
			},
		},
		{
			name: "derived profile only",
			profiles: map[string]domainprofile.Profile{
				"codex.work": {Name: "codex.work", Kind: domainprofile.ProfileKindRelay, BaseURL: "u", APIKey: "k", CreatedAt: now, UpdatedAt: now},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeProfileRepo{profiles: tc.profiles}
			state := &fakeStateRepo{
				state: domainprofile.State{
					ManagedAgents: map[domainprofile.Agent]domainprofile.ManagedAgentState{
						domainprofile.AgentCodex: {
							Targets: map[string]domainprofile.TargetState{
								"work": {Kind: domainprofile.TargetKindRelay, Relay: domainprofile.RelayTargetState{ProfileName: "codex.work"}, CreatedAt: now, UpdatedAt: now},
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
			if hasIssueCode(report, "orphan_managed_target") {
				t.Fatalf("Doctor() should not report orphan_managed_target when %s: %+v", tc.name, report.Issues)
			}
		})
	}
}

func TestDoctorReportsModelDrift(t *testing.T) {
	now := time.Now().UTC()
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
		},
	}
	state := &fakeStateRepo{
		state: domainprofile.State{
			ManagedAgents: map[domainprofile.Agent]domainprofile.ManagedAgentState{
				domainprofile.AgentCodex: {
					Targets: map[string]domainprofile.TargetState{
						"work": {
							Kind: domainprofile.TargetKindRelay,
							Relay: domainprofile.RelayTargetState{
								ProfileName: "codex.work",
								ModelID:     "gpt-5.3-codex",
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

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if !hasIssueCode(report, "model_id_drift") {
		t.Fatalf("Doctor() issues = %+v, want model_id_drift", report.Issues)
	}
}

func TestDoctorIgnoresModelDriftWhenEitherSideEmpty(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name   string
		user   string
		target string
	}{
		{name: "user empty", user: "", target: "gpt-5.5"},
		{name: "target empty", user: "gpt-5.5", target: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeProfileRepo{
				profiles: map[string]domainprofile.Profile{
					"work": {Name: "work", Kind: domainprofile.ProfileKindRelay, BaseURL: "u", APIKey: "k", ModelID: tc.user, CreatedAt: now, UpdatedAt: now},
				},
			}
			state := &fakeStateRepo{
				state: domainprofile.State{
					ManagedAgents: map[domainprofile.Agent]domainprofile.ManagedAgentState{
						domainprofile.AgentCodex: {
							Targets: map[string]domainprofile.TargetState{
								"work": {Kind: domainprofile.TargetKindRelay, Relay: domainprofile.RelayTargetState{ProfileName: "codex.work", ModelID: tc.target}, CreatedAt: now, UpdatedAt: now},
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
			if hasIssueCode(report, "model_id_drift") {
				t.Fatalf("Doctor() should not report model_id_drift when %s: %+v", tc.name, report.Issues)
			}
		})
	}
}

func hasIssueCode(report *DoctorReport, code string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

