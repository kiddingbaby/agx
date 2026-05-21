package usecase

import (
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

// The four interfaces below name the logical concerns that
// ProfileService composes today. Splitting ProfileService into four
// concrete services is deferred to v0.3; for now these contracts:
//
//   - document the seam (cf. docs/ARCHITECTURE.md)
//   - let downstream code (CLI commands, tests) accept the narrower
//     surface it actually needs instead of the full *ProfileService
//   - get compile-time checks via the var _ assignments at the end so
//     any drift between concern and implementation is caught the same
//     day it happens.
//
// All four are satisfied by *ProfileService.

// RelayService owns relay profile CRUD and the relay-only target
// surface (CreateRelayTarget / EditRelayTarget). It does not touch the
// agent-binding pipeline or the managed-runtime per-target contexts.
type RelayService interface {
	List() ([]domainprofile.Profile, error)
	Get(name string) (*domainprofile.Profile, error)
	APIKey(name string) (string, error)
	Add(name string, input AddProfileInput) (*AddProfileResult, error)
	Edit(name string, input EditProfileInput) (*EditProfileResult, error)
	Remove(name string) (*domainprofile.Profile, error)

	ListTargets(agent domainprofile.Agent) ([]TargetResult, error)
	Target(agent domainprofile.Agent, name string) (TargetResult, error)
	CreateRelayTarget(agent domainprofile.Agent, name string, input RelayTargetInput) (TargetResult, error)
	EditRelayTarget(agent domainprofile.Agent, name string, input RelayTargetInput) (TargetResult, error)
	UseTarget(agent domainprofile.Agent, name string) (TargetResult, error)
	RemoveTarget(agent domainprofile.Agent, name string) (TargetResult, error)
}

// BindingCoordinator drives the agent ↔ profile binding pipeline:
// AgentSet/AgentBind/Clear/Use bound through withMutationGuard, plus
// the explicit Backup/Restore entry points.
type BindingCoordinator interface {
	AgentSet(agent domainprofile.Agent, name string) (*AgentSetResult, error)
	AgentBind(agent domainprofile.Agent, name string) (*BindingChangeResult, error)
	Use(agent domainprofile.Agent, name string) (*AgentSetResult, error)
	Clear(agent domainprofile.Agent) (*RestoreResult, error)
	Backup(agent domainprofile.Agent) (*BackupResult, error)
	Restore(agent domainprofile.Agent, backupID string) (*RestoreResult, error)
	BackupList(agent domainprofile.Agent) ([]domainprofile.Backup, error)

	State() (domainprofile.State, error)
}

// ManagedProfileService owns the managed-profile lifecycle plus the
// per-target context roots used by the launcher commands.
type ManagedProfileService interface {
	ListManagedProfiles() ([]ManagedProfileSummary, string, error)
	ListAllProfiles() ([]ManagedProfileSummary, string, error)
	ManagedProfile(name string) (*domainprofile.Profile, error)
	CurrentManagedProfile() (*domainprofile.Profile, error)
	AddManagedProfile(name string, profile domainprofile.Profile) (*domainprofile.Profile, error)
	EditManagedProfile(name string, input UpdateManagedProfileInput) (*EditManagedProfileResult, error)
	RemoveManagedProfile(name string) (*domainprofile.Profile, error)
	UseManagedProfile(name string) (*domainprofile.Profile, error)
	ActivateManagedProfile(agent domainprofile.Agent, name string) (TargetResult, error)

	ManagedAgent(agent domainprofile.Agent) (domainprofile.ManagedAgentState, error)
	CurrentTargetContext(agent domainprofile.Agent) (domainprofile.CurrentTarget, string, error)
	BackupManagedTarget(agent domainprofile.Agent, kind domainprofile.TargetKind, name string) (domainprofile.ContextBackup, error)
	RestoreManagedTarget(agent domainprofile.Agent, kind domainprofile.TargetKind, name, backupID string) (domainprofile.ContextBackup, error)
	RestoreCurrentTarget(agent domainprofile.Agent) (domainprofile.ContextBackup, error)
}

// DoctorService produces the diagnostic report used by `agx doctor`.
// It is read-only against the world; mutating callers should go
// through BindingCoordinator instead.
type DoctorService interface {
	Doctor() (*DoctorReport, error)
}

// Compile-time guarantees that *ProfileService still satisfies every
// concern interface. If ProfileService is later split into four
// concrete types, only these assertions need to move.
var (
	_ RelayService          = (*ProfileService)(nil)
	_ BindingCoordinator    = (*ProfileService)(nil)
	_ ManagedProfileService = (*ProfileService)(nil)
	_ DoctorService         = (*ProfileService)(nil)
)
