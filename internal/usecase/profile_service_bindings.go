package usecase

import (
	"errors"
	"fmt"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func (s *ProfileService) AgentSet(agent domainprofile.Agent, name string) (result *AgentSetResult, err error) {
	unlock, err := s.lockMutations()
	if err != nil {
		return nil, err
	}
	defer unlock()

	guard := newMutationGuard(s)
	if err := guard.CaptureState(); err != nil {
		return nil, err
	}
	if err := guard.CaptureAgents(agent); err != nil {
		return nil, err
	}
	defer func() {
		if err == nil {
			guard.Commit()
			return
		}
		if rollbackErr := guard.Rollback(); rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
		}
	}()

	return s.agentSetLocked(agent, name)
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
	if agent == domainprofile.AgentCodex {
		assignCodexProfileApplied(&state, profile.Name, backup.ConfigPath, now, backup.ID)
	}
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

func (s *ProfileService) Clear(agent domainprofile.Agent) (result *RestoreResult, err error) {
	unlock, err := s.lockMutations()
	if err != nil {
		return nil, err
	}
	defer unlock()

	guard := newMutationGuard(s)
	if err := guard.CaptureState(); err != nil {
		return nil, err
	}
	if err := guard.CaptureAgents(agent); err != nil {
		return nil, err
	}
	defer func() {
		if err == nil {
			guard.Commit()
			return
		}
		if rollbackErr := guard.Rollback(); rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
		}
	}()

	return s.clearLocked(agent)
}

func (s *ProfileService) clearLocked(agent domainprofile.Agent) (*RestoreResult, error) {
	if !agent.Valid() {
		return nil, &InvalidAgentError{Agent: string(agent)}
	}

	state, err := s.loadStoredState()
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
	if agent == domainprofile.AgentCodex {
		if err := s.refreshCodexStateAfterRestore(&state); err != nil {
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

func (s *ProfileService) syncCodexProfile(profile domainprofile.Profile, state *domainprofile.State, now time.Time) error {
	if s.codex == nil {
		return nil
	}

	backupID := nextBackupID(domainprofile.AgentCodex, currentBinding(*state, domainprofile.AgentCodex).Backups, now)
	backup, cleanup, _, err := s.syncProfileToAgent(domainprofile.AgentCodex, profile, backupID, *state, false)
	if err != nil {
		return err
	}

	binding := currentBinding(*state, domainprofile.AgentCodex)
	if state.Codex.SourceProfile == profile.Name {
		setBindingApplied(&binding, profile.Name, backup.ConfigPath, now, backup.ID)
		if err := prependBackupAndTrim(&binding, backup, cleanup); err != nil {
			return err
		}
		assignBinding(state, domainprofile.AgentCodex, binding)
	} else {
		binding.ConfigPath = backup.ConfigPath
		if err := prependBackupListAndTrim(&state.Codex.Backups, backup, cleanup); err != nil {
			return err
		}
		state.Codex.ConfigPath = backup.ConfigPath
	}
	assignCodexProfileApplied(state, profile.Name, backup.ConfigPath, now, backup.ID)
	return nil
}

func (s *ProfileService) syncProfileToBoundAgents(profile domainprofile.Profile, state *domainprofile.State, now time.Time) error {
	for _, agent := range boundAgents(profile.Name, *state) {
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
	default:
		return domainprofile.Backup{}, nil, "", &InvalidAgentError{Agent: string(agent)}
	}
}

func (s *ProfileService) snapshotCurrentConfig(agent domainprofile.Agent, profileName, backupID string, now time.Time) (domainprofile.Backup, error) {
	var snapshot *ports.AgentConfigSnapshot
	var err error

	switch agent {
	case domainprofile.AgentCodex:
		if s.codex == nil {
			return domainprofile.Backup{}, &InvalidAgentError{Agent: string(agent)}
		}
		snapshot, err = s.codex.Snapshot()
	case domainprofile.AgentClaude:
		if s.claude == nil {
			return domainprofile.Backup{}, &InvalidAgentError{Agent: string(agent)}
		}
		snapshot, err = s.claude.Snapshot()
	case domainprofile.AgentGemini:
		if s.gemini == nil {
			return domainprofile.Backup{}, &InvalidAgentError{Agent: string(agent)}
		}
		snapshot, err = s.gemini.Snapshot()
	default:
		return domainprofile.Backup{}, &InvalidAgentError{Agent: string(agent)}
	}
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
	switch agent {
	case domainprofile.AgentCodex:
		backup.BackupPath, err = s.codex.CreateBackup(backupID, snapshot.Content)
	case domainprofile.AgentClaude:
		backup.BackupPath, err = s.claude.CreateBackup(backupID, snapshot.Content)
	case domainprofile.AgentGemini:
		backup.BackupPath, err = s.gemini.CreateBackup(backupID, snapshot.Content)
	}
	if err != nil {
		return domainprofile.Backup{}, err
	}
	return backup, nil
}

func deleteBackupForAgent(agent domainprofile.Agent, s *ProfileService) func(string) error {
	switch agent {
	case domainprofile.AgentCodex:
		if s.codex != nil {
			return s.codex.DeleteBackup
		}
	case domainprofile.AgentClaude:
		if s.claude != nil {
			return s.claude.DeleteBackup
		}
	case domainprofile.AgentGemini:
		if s.gemini != nil {
			return s.gemini.DeleteBackup
		}
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

func boundAgents(name string, state domainprofile.State) []domainprofile.Agent {
	agents := make([]domainprofile.Agent, 0, 3)
	name = domainprofile.NormalizeProfileName(name)
	if state.Codex.SourceProfile == name {
		agents = append(agents, domainprofile.AgentCodex)
	}
	for _, agent := range []domainprofile.Agent{domainprofile.AgentClaude, domainprofile.AgentGemini} {
		if currentBinding(state, agent).SourceProfile == name {
			agents = append(agents, agent)
		}
	}
	return agents
}
