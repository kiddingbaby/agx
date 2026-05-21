package usecase

import (
	"fmt"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func (s *ProfileService) AgentSet(agent domainprofile.Agent, name string) (*AgentSetResult, error) {
	return withMutationGuard(s,
		func(g *mutationGuard) error {
			if err := g.CaptureState(); err != nil {
				return err
			}
			return g.CaptureAgents(agent)
		},
		func() (*AgentSetResult, error) { return s.agentSetLocked(agent, name) },
	)
}

func (s *ProfileService) AgentBind(agent domainprofile.Agent, name string) (*BindingChangeResult, error) {
	return withMutationGuard(s,
		func(g *mutationGuard) error {
			if err := g.CaptureState(); err != nil {
				return err
			}
			return g.CaptureAgents(agent)
		},
		func() (*BindingChangeResult, error) { return s.agentBindLocked(agent, name) },
	)
}

func (s *ProfileService) Use(agent domainprofile.Agent, name string) (*AgentSetResult, error) {
	return s.AgentSet(agent, name)
}

func (s *ProfileService) agentSetLocked(agent domainprofile.Agent, name string) (*AgentSetResult, error) {
	if !agent.Valid() {
		return nil, &InvalidAgentError{Agent: string(agent)}
	}

	profile, err := s.Get(name)
	if err != nil {
		return nil, err
	}

	state, err := s.loadStoredState()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	backupID := nextBackupID(agent, currentBinding(state, agent).Backups, now)
	operation := newOperationRecord("set", agent, profile.Name, now)
	if err := s.beginOperation(operation); err != nil {
		return nil, err
	}

	backup, cleanup, codexProfileName, err := s.syncProfileToAgent(agent, *profile, backupID, state, true)
	if err != nil {
		_ = s.clearOperation(operation.ID)
		return nil, err
	}

	binding := currentBinding(state, agent)
	setBindingApplied(&binding, profile.Name, backup.ConfigPath, now, backup.ID)
	if err := prependBackupAndTrim(&binding, backup, cleanup); err != nil {
		_ = s.clearOperation(operation.ID)
		return nil, err
	}

	assignBinding(&state, agent, binding)
	state.UpdatedAt = now

	operation.BackupID = backup.ID
	operation.ConfigPath = backup.ConfigPath
	operation.BackupPath = backup.BackupPath
	operation.Stage = operationStageConfigWritten
	operation.UpdatedAt = time.Now().UTC()
	if err := s.updateOperation(operation); err != nil {
		return nil, err
	}
	if err := s.saveState(state); err != nil {
		return nil, err
	}
	if err := s.clearOperation(operation.ID); err != nil {
		return nil, err
	}

	return &AgentSetResult{
		Agent:            agent,
		Profile:          profile,
		Binding:          binding,
		Backup:           backup,
		CodexProfileName: codexProfileName,
	}, nil
}

func (s *ProfileService) agentBindLocked(agent domainprofile.Agent, name string) (*BindingChangeResult, error) {
	if !agent.Valid() {
		return nil, &InvalidAgentError{Agent: string(agent)}
	}

	profile, err := s.Get(name)
	if err != nil {
		return nil, err
	}
	state, err := s.loadEffectiveState()
	if err != nil {
		return nil, err
	}

	current := currentBinding(state, agent)
	if current.SourceProfile == "" {
		setResult, err := s.agentSetLocked(agent, profile.Name)
		if err != nil {
			return nil, err
		}
		binding := setResult.Binding
		return &BindingChangeResult{
			Agent:         agent,
			Action:        "bind",
			PreviousRelay: "",
			Binding:       &binding,
			Backup:        setResult.Backup,
			CodexProfile:  setResult.CodexProfileName,
		}, nil
	}

	now := time.Now().UTC()
	backupID := nextBackupID(agent, current.Backups, now)
	operation := newOperationRecord("bind", agent, profile.Name, now)
	if err := s.beginOperation(operation); err != nil {
		return nil, err
	}

	var codexProfileName string
	var backup domainprofile.Backup
	var cleanup func(string) error
	switch agent {
	case domainprofile.AgentCodex:
		var err error
		backup, cleanup, codexProfileName, err = s.syncProfileToAgent(agent, *profile, backupID, state, false)
		if err != nil {
			_ = s.clearOperation(operation.ID)
			return nil, err
		}
		if current.SourceProfile == profile.Name {
			setBindingApplied(&current, profile.Name, backup.ConfigPath, now, backup.ID)
		}
		if err := prependBackupAndTrim(&current, backup, cleanup); err != nil {
			_ = s.clearOperation(operation.ID)
			return nil, err
		}
		assignBinding(&state, agent, current)
	default:
		var err error
		backup, err = s.snapshotCurrentConfig(agent, current.SourceProfile, backupID, now)
		if err != nil {
			_ = s.clearOperation(operation.ID)
			return nil, err
		}
		cleanup = deleteBackupForAgent(agent, s)
		if current.SourceProfile == profile.Name {
			setBindingApplied(&current, profile.Name, backup.ConfigPath, now, backup.ID)
		}
		if err := prependBackupAndTrim(&current, backup, cleanup); err != nil {
			_ = s.clearOperation(operation.ID)
			return nil, err
		}
		assignBinding(&state, agent, current)
		if current.SourceProfile == profile.Name {
			updateAgentProfileBinding(&state, agent, profile.Name, func(binding *domainprofile.AgentProfileBinding) {
				binding.Status = domainprofile.BindingStatusApplied
				binding.ConfigPath = backup.ConfigPath
				binding.LastAppliedAt = now
				binding.LastBackupID = backup.ID
			})
		} else {
			updateAgentProfileBinding(&state, agent, profile.Name, func(binding *domainprofile.AgentProfileBinding) {
				binding.Status = domainprofile.BindingStatusBound
				binding.ConfigPath = backup.ConfigPath
				binding.LastBackupID = backup.ID
			})
		}
	}

	operation.BackupID = backup.ID
	operation.ConfigPath = backup.ConfigPath
	operation.BackupPath = backup.BackupPath
	operation.Stage = operationStageConfigWritten
	operation.UpdatedAt = time.Now().UTC()
	if err := s.updateOperation(operation); err != nil {
		return nil, err
	}
	state.UpdatedAt = now
	if err := s.saveState(state); err != nil {
		return nil, err
	}
	if err := s.clearOperation(operation.ID); err != nil {
		return nil, err
	}

	resultBinding := domainprofile.AgentBinding{
		SourceProfile: profile.Name,
		Status:        domainprofile.BindingStatusBound,
		ConfigPath:    backup.ConfigPath,
		LastBackupID:  backup.ID,
	}
	if current.SourceProfile == profile.Name {
		resultBinding = current
	}
	return &BindingChangeResult{
		Agent:         agent,
		Action:        "bind",
		PreviousRelay: current.SourceProfile,
		Binding:       &resultBinding,
		Backup:        backup,
		CodexProfile:  codexProfileName,
	}, nil
}

func (s *ProfileService) Clear(agent domainprofile.Agent) (*RestoreResult, error) {
	return withMutationGuard(s,
		func(g *mutationGuard) error {
			if err := g.CaptureState(); err != nil {
				return err
			}
			return g.CaptureAgents(agent)
		},
		func() (*RestoreResult, error) { return s.clearLocked(agent) },
	)
}

func (s *ProfileService) Backup(agent domainprofile.Agent) (*BackupResult, error) {
	return withMutationGuard(s,
		func(g *mutationGuard) error {
			if err := g.CaptureState(); err != nil {
				return err
			}
			return g.CaptureAgents(agent)
		},
		func() (*BackupResult, error) { return s.backupLocked(agent) },
	)
}

func (s *ProfileService) backupLocked(agent domainprofile.Agent) (*BackupResult, error) {
	if !agent.Valid() {
		return nil, &InvalidAgentError{Agent: string(agent)}
	}

	state, err := s.loadEffectiveState()
	if err != nil {
		return nil, err
	}

	binding := currentBinding(state, agent)
	now := time.Now().UTC()
	backupID := nextBackupID(agent, binding.Backups, now)
	operation := newOperationRecord("backup", agent, binding.SourceProfile, now)
	if err := s.beginOperation(operation); err != nil {
		return nil, err
	}

	backup, err := s.snapshotCurrentConfig(agent, binding.SourceProfile, backupID, now)
	if err != nil {
		_ = s.clearOperation(operation.ID)
		return nil, err
	}

	if cleanup := deleteBackupForAgent(agent, s); cleanup != nil {
		if err := prependBackupAndTrim(&binding, backup, cleanup); err != nil {
			_ = s.clearOperation(operation.ID)
			return nil, err
		}
	}
	if binding.ConfigPath == "" {
		binding.ConfigPath = backup.ConfigPath
	}
	binding.LastBackupID = backup.ID
	assignBinding(&state, agent, binding)
	state.UpdatedAt = now

	operation.BackupID = backup.ID
	operation.ConfigPath = backup.ConfigPath
	operation.BackupPath = backup.BackupPath
	operation.Stage = operationStageConfigWritten
	operation.UpdatedAt = time.Now().UTC()
	if err := s.updateOperation(operation); err != nil {
		return nil, err
	}
	if err := s.saveState(state); err != nil {
		return nil, err
	}
	if err := s.clearOperation(operation.ID); err != nil {
		return nil, err
	}

	return &BackupResult{
		Agent:  agent,
		Backup: backup,
	}, nil
}

func (s *ProfileService) clearLocked(agent domainprofile.Agent) (*RestoreResult, error) {
	if !agent.Valid() {
		return nil, &InvalidAgentError{Agent: string(agent)}
	}

	state, err := s.loadEffectiveState()
	if err != nil {
		return nil, err
	}

	binding := currentBinding(state, agent)
	now := time.Now().UTC()
	backupID := nextBackupID(agent, binding.Backups, now)
	operation := newOperationRecord("clear", agent, binding.SourceProfile, now)
	if err := s.beginOperation(operation); err != nil {
		return nil, err
	}

	backup, err := s.snapshotCurrentConfig(agent, binding.SourceProfile, backupID, now)
	if err != nil {
		_ = s.clearOperation(operation.ID)
		return nil, err
	}

	var configPath string
	switch agent {
	case domainprofile.AgentCodex:
		if s.codex == nil {
			return nil, &InvalidAgentError{Agent: string(agent)}
		}
		configPath, err = s.codex.ClearDefaultProfile()
	case domainprofile.AgentClaude:
		if s.claude == nil {
			return nil, &InvalidAgentError{Agent: string(agent)}
		}
		configPath, err = s.claude.RemoveConfig()
	case domainprofile.AgentGemini:
		if s.gemini == nil {
			return nil, &InvalidAgentError{Agent: string(agent)}
		}
		configPath, err = s.gemini.RemoveConfig()
	case domainprofile.AgentOpenCode:
		if s.openCode == nil {
			return nil, &InvalidAgentError{Agent: string(agent)}
		}
		configPath, err = s.openCode.ClearDefaultModel()
	default:
		err = &InvalidAgentError{Agent: string(agent)}
	}
	if err != nil {
		_ = s.clearOperation(operation.ID)
		return nil, err
	}

	clearResolvedBinding(&binding)
	binding.ConfigPath = configPath
	if cleanup := deleteBackupForAgent(agent, s); cleanup != nil {
		if err := prependBackupAndTrim(&binding, backup, cleanup); err != nil {
			_ = s.clearOperation(operation.ID)
			return nil, err
		}
	}
	assignBinding(&state, agent, binding)
	switch agent {
	case domainprofile.AgentCodex:
		if err := s.refreshCodexStateAfterRestore(&state); err != nil {
			return nil, err
		}
	case domainprofile.AgentOpenCode:
		if err := s.refreshOpenCodeStateAfterRestore(&state); err != nil {
			return nil, err
		}
	}
	state.UpdatedAt = now

	operation.BackupID = backup.ID
	operation.ConfigPath = configPath
	operation.BackupPath = backup.BackupPath
	operation.Stage = operationStageConfigWritten
	operation.UpdatedAt = time.Now().UTC()
	if err := s.updateOperation(operation); err != nil {
		return nil, err
	}
	if err := s.saveState(state); err != nil {
		return nil, err
	}
	if err := s.clearOperation(operation.ID); err != nil {
		return nil, err
	}

	return &RestoreResult{
		Agent:      agent,
		ConfigPath: configPath,
		Backup:     backup,
	}, nil
}

func (s *ProfileService) removeCodexProfileLocked(relayName string) (*RestoreResult, error) {
	if s.codex == nil {
		return nil, &InvalidAgentError{Agent: string(domainprofile.AgentCodex)}
	}
	return s.removeAgentProfileLocked(
		domainprofile.AgentCodex,
		relayName,
		s.codex.RemoveProfile,
		s.refreshCodexStateAfterRestore,
	)
}

func (s *ProfileService) removeOpenCodeProfileLocked(relayName string) (*RestoreResult, error) {
	if s.openCode == nil {
		return nil, &InvalidAgentError{Agent: string(domainprofile.AgentOpenCode)}
	}
	return s.removeAgentProfileLocked(
		domainprofile.AgentOpenCode,
		relayName,
		s.openCode.RemoveProfile,
		s.refreshOpenCodeStateAfterRestore,
	)
}

// removeAgentProfileLocked runs the shared snapshot → RemoveProfile →
// state-refresh → journal-update sequence used by both
// removeCodexProfileLocked and removeOpenCodeProfileLocked.
//
// removeProfile and refreshState carry the agent-specific differences:
// the syncer-level RemoveProfile call and the
// refresh{Codex,OpenCode}StateAfterRestore step that rebuilds derived
// state from the agent's native config after the managed entry is gone.
func (s *ProfileService) removeAgentProfileLocked(
	agent domainprofile.Agent,
	relayName string,
	removeProfile func(string) (string, error),
	refreshState func(*domainprofile.State) error,
) (*RestoreResult, error) {
	relayName = domainprofile.NormalizeProfileName(relayName)
	state, err := s.loadEffectiveState()
	if err != nil {
		return nil, err
	}

	binding := currentBinding(state, agent)
	now := time.Now().UTC()
	backupID := nextBackupID(agent, binding.Backups, now)
	operation := newOperationRecord("clear", agent, relayName, now)
	if err := s.beginOperation(operation); err != nil {
		return nil, err
	}

	backup, err := s.snapshotCurrentConfig(agent, relayName, backupID, now)
	if err != nil {
		_ = s.clearOperation(operation.ID)
		return nil, err
	}

	configPath, err := removeProfile(relayName)
	if err != nil {
		_ = s.clearOperation(operation.ID)
		return nil, err
	}

	if cleanup := deleteBackupForAgent(agent, s); cleanup != nil {
		if err := prependBackupAndTrim(&binding, backup, cleanup); err != nil {
			_ = s.clearOperation(operation.ID)
			return nil, err
		}
	}
	binding.ConfigPath = configPath
	assignBinding(&state, agent, binding)
	if err := refreshState(&state); err != nil {
		_ = s.clearOperation(operation.ID)
		return nil, err
	}
	state.UpdatedAt = now

	operation.BackupID = backup.ID
	operation.ConfigPath = configPath
	operation.BackupPath = backup.BackupPath
	operation.Stage = operationStageConfigWritten
	operation.UpdatedAt = time.Now().UTC()
	if err := s.updateOperation(operation); err != nil {
		return nil, err
	}
	if err := s.saveState(state); err != nil {
		return nil, err
	}
	if err := s.clearOperation(operation.ID); err != nil {
		return nil, err
	}

	return &RestoreResult{
		Agent:      agent,
		ConfigPath: configPath,
		Backup:     backup,
	}, nil
}

func (s *ProfileService) syncCodexProfile(profile domainprofile.Profile, state *domainprofile.State, now time.Time) error {
	if s.codex == nil {
		return nil
	}
	return s.syncAgentProfileToState(domainprofile.AgentCodex, profile, state, now,
		state.Codex.SourceProfile,
		&state.Codex.Backups,
		func(path string) { state.Codex.ConfigPath = path },
	)
}

func (s *ProfileService) syncOpenCodeProfile(profile domainprofile.Profile, state *domainprofile.State, now time.Time) error {
	if s.openCode == nil {
		return nil
	}
	return s.syncAgentProfileToState(domainprofile.AgentOpenCode, profile, state, now,
		state.OpenCode.SourceProfile,
		&state.OpenCode.Backups,
		func(path string) { state.OpenCode.ConfigPath = path },
	)
}

// syncAgentProfileToState is the shared body of syncCodexProfile and
// syncOpenCodeProfile: snapshot the agent config, run its bespoke Sync,
// then attach the backup either to the active binding (if this is the
// currently bound profile) or to the agent-level history list (for
// inactive profiles that just got rewritten in place).
//
// The agent-specific bits — which SourceProfile field to compare, which
// Backups list to append to, where to write the new ConfigPath — are
// passed in explicitly so we don't reach back into the *State by
// switching on agent.
func (s *ProfileService) syncAgentProfileToState(
	agent domainprofile.Agent,
	profile domainprofile.Profile,
	state *domainprofile.State,
	now time.Time,
	sourceProfile string,
	backupsField *[]domainprofile.Backup,
	setConfigPath func(string),
) error {
	backupID := nextBackupID(agent, currentBinding(*state, agent).Backups, now)
	backup, cleanup, _, err := s.syncProfileToAgent(agent, profile, backupID, *state, false)
	if err != nil {
		return err
	}

	binding := currentBinding(*state, agent)
	if sourceProfile == profile.Name {
		setBindingApplied(&binding, profile.Name, backup.ConfigPath, now, backup.ID)
		if err := prependBackupAndTrim(&binding, backup, cleanup); err != nil {
			return err
		}
		assignBinding(state, agent, binding)
		return nil
	}
	binding.ConfigPath = backup.ConfigPath
	if err := prependBackupListAndTrim(backupsField, backup, cleanup); err != nil {
		return err
	}
	setConfigPath(backup.ConfigPath)
	return nil
}

func (s *ProfileService) syncProfileToBoundAgents(profile domainprofile.Profile, state *domainprofile.State, now time.Time) error {
	for _, agent := range currentAgents(profile.Name, *state) {
		if agent == domainprofile.AgentCodex {
			continue
		}
		backupID := nextBackupID(agent, currentBinding(*state, agent).Backups, now)
		backup, cleanup, _, err := s.syncProfileToAgent(agent, profile, backupID, *state, false)
		if err != nil {
			return err
		}

		binding := currentBinding(*state, agent)
		setBindingApplied(&binding, profile.Name, backup.ConfigPath, now, backup.ID)
		if err := prependBackupAndTrim(&binding, backup, cleanup); err != nil {
			return err
		}
		assignBinding(state, agent, binding)
		state.UpdatedAt = now
		if err := s.saveState(*state); err != nil {
			return err
		}
	}
	return nil
}

func (s *ProfileService) syncProfileToAgent(agent domainprofile.Agent, profile domainprofile.Profile, backupID string, state domainprofile.State, setCodexDefault bool) (domainprofile.Backup, func(string) error, string, error) {
	now := time.Now().UTC()

	switch agent {
	case domainprofile.AgentCodex:
		if s.codex == nil {
			return domainprofile.Backup{}, nil, "", &InvalidAgentError{Agent: string(agent)}
		}
		backup, err := s.snapshotCurrentConfig(agent, profile.Name, backupID, now)
		if err != nil {
			return domainprofile.Backup{}, nil, "", err
		}
		options := ports.CodexSyncOptions{}
		if setCodexDefault {
			options.DefaultProfileName = profile.Name
		} else if state.Codex.SourceProfile == profile.Name {
			options.DefaultProfileName = profile.Name
		}
		result, err := s.codex.Sync(profile, options)
		if err != nil {
			return domainprofile.Backup{}, nil, "", err
		}
		backup.ConfigPath = result.ConfigPath
		return backup, s.codex.DeleteBackup, result.ProfileName, nil
	case domainprofile.AgentClaude:
		if s.claude == nil {
			return domainprofile.Backup{}, nil, "", &InvalidAgentError{Agent: string(agent)}
		}
		backup, err := s.snapshotCurrentConfig(agent, profile.Name, backupID, now)
		if err != nil {
			return domainprofile.Backup{}, nil, "", err
		}
		result, err := s.claude.Sync(profile)
		if err != nil {
			return domainprofile.Backup{}, nil, "", err
		}
		backup.ConfigPath = result.ConfigPath
		return backup, s.claude.DeleteBackup, "", nil
	case domainprofile.AgentGemini:
		if s.gemini == nil {
			return domainprofile.Backup{}, nil, "", &InvalidAgentError{Agent: string(agent)}
		}
		backup, err := s.snapshotCurrentConfig(agent, profile.Name, backupID, now)
		if err != nil {
			return domainprofile.Backup{}, nil, "", err
		}
		result, err := s.gemini.Sync(profile)
		if err != nil {
			return domainprofile.Backup{}, nil, "", err
		}
		backup.ConfigPath = result.ConfigPath
		return backup, s.gemini.DeleteBackup, "", nil
	case domainprofile.AgentOpenCode:
		if s.openCode == nil {
			return domainprofile.Backup{}, nil, "", &InvalidAgentError{Agent: string(agent)}
		}
		backup, err := s.snapshotCurrentConfig(agent, profile.Name, backupID, now)
		if err != nil {
			return domainprofile.Backup{}, nil, "", err
		}
		binding := normalizeOpenCodeProfileBinding(profile.Name, &domainprofile.OpenCodeProfileBinding{
			ProviderFamily: profile.ProviderFamily,
			ModelID:        profile.ModelID,
			ModelName:      profile.ModelID,
		})
		options := ports.OpenCodeSyncOptions{
			ProviderFamily: binding.ProviderFamily,
			ModelID:        binding.ModelID,
			ModelName:      binding.ModelName,
			ProviderName:   profile.Name,
			SetAsCurrent:   setCodexDefault || state.OpenCode.SourceProfile == profile.Name,
		}
		result, err := s.openCode.Sync(profile, options)
		if err != nil {
			return domainprofile.Backup{}, nil, "", err
		}
		backup.ConfigPath = result.ConfigPath
		return backup, s.openCode.DeleteBackup, result.ProviderID + "/" + result.ModelID, nil
	default:
		return domainprofile.Backup{}, nil, "", &InvalidAgentError{Agent: string(agent)}
	}
}

func (s *ProfileService) snapshotCurrentConfig(agent domainprofile.Agent, profileName, backupID string, now time.Time) (domainprofile.Backup, error) {
	syncer := s.resolveAgentSyncer(agent)
	if syncer == nil {
		return domainprofile.Backup{}, &InvalidAgentError{Agent: string(agent)}
	}

	snapshot, err := syncer.Snapshot()
	if err != nil {
		return domainprofile.Backup{}, err
	}

	backup := domainprofile.Backup{
		ID:             backupID,
		AppliedProfile: profileName,
		ConfigPath:     snapshot.ConfigPath,
		CreatedAt:      now,
	}
	if !snapshot.Exists {
		backup.RestoreMode = domainprofile.RestoreModeRemoveCreatedFile
		return backup, nil
	}

	backup.RestoreMode = domainprofile.RestoreModeRestoreFile
	backup.BackupPath, err = syncer.CreateBackup(backupID, snapshot.Content)
	if err != nil {
		return domainprofile.Backup{}, err
	}
	return backup, nil
}

func deleteBackupForAgent(agent domainprofile.Agent, s *ProfileService) func(string) error {
	if syncer := s.resolveAgentSyncer(agent); syncer != nil {
		return syncer.DeleteBackup
	}
	return nil
}

func nextBackupID(agent domainprofile.Agent, existing []domainprofile.Backup, now time.Time) string {
	base := fmt.Sprintf("before-%s-sync-%s", agent, now.UTC().Format("20060102T150405Z"))
	candidate := base
	seq := 2
	for hasBackupID(existing, candidate) {
		candidate = fmt.Sprintf("%s-%d", base, seq)
		seq++
	}
	return candidate
}

func hasBackupID(existing []domainprofile.Backup, candidate string) bool {
	for _, backup := range existing {
		if backup.ID == candidate {
			return true
		}
	}
	return false
}
