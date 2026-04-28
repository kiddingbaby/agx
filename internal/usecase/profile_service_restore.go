package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func (s *ProfileService) Restore(agent domainprofile.Agent, backupID string) (result *RestoreResult, err error) {
	unlock, err := s.lockMutations()
	if err != nil {
		return nil, err
	}
	defer unlock()

	if !agent.Valid() {
		return nil, &InvalidAgentError{Agent: string(agent)}
	}

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

	state, err := s.state.Load()
	if err != nil {
		return nil, err
	}
	binding := currentBinding(state, agent)
	if len(binding.Backups) == 0 {
		return nil, &NoBackupError{Agent: agent}
	}

	selected, err := selectBackup(binding.Backups, backupID)
	if err != nil {
		return nil, err
	}
	operation := newOperationRecord("restore", agent, binding.SourceProfile, time.Now().UTC())
	operation.BackupID = selected.ID
	operation.ConfigPath = selected.ConfigPath
	operation.BackupPath = selected.BackupPath
	if err := s.beginOperation(operation); err != nil {
		return nil, err
	}

	var configPath string
	switch agent {
	case domainprofile.AgentCodex:
		if s.codex == nil {
			return nil, &InvalidAgentError{Agent: string(agent)}
		}
		configPath, err = restoreBackupForAgent(selected, s.codex)
	case domainprofile.AgentClaude:
		if s.claude == nil {
			return nil, &InvalidAgentError{Agent: string(agent)}
		}
		configPath, err = restoreBackupForAgent(selected, s.claude)
	case domainprofile.AgentGemini:
		if s.gemini == nil {
			return nil, &InvalidAgentError{Agent: string(agent)}
		}
		configPath, err = restoreBackupForAgent(selected, s.gemini)
	default:
		err = &InvalidAgentError{Agent: string(agent)}
	}
	if err != nil {
		_ = s.clearOperation(operation.ID)
		return nil, err
	}
	operation.ConfigPath = configPath
	operation.Stage = operationStageConfigWritten
	operation.UpdatedAt = time.Now().UTC()
	if err := s.updateOperation(operation); err != nil {
		return nil, err
	}

	clearResolvedBinding(&binding)
	assignBinding(&state, agent, binding)
	if agent == domainprofile.AgentCodex {
		if err := s.refreshCodexStateAfterRestore(&state); err != nil {
			return nil, err
		}
	}
	state.UpdatedAt = time.Now().UTC()
	if _, err := s.state.Save(state); err != nil {
		return nil, err
	}
	if err := s.clearOperation(operation.ID); err != nil {
		return nil, err
	}

	return &RestoreResult{
		Agent:      agent,
		ConfigPath: configPath,
		Backup:     selected,
	}, nil
}

func restoreBackupForAgent[T interface {
	Restore(string) (string, error)
	RemoveConfig() (string, error)
}](backup domainprofile.Backup, syncer T) (string, error) {
	switch backup.RestoreMode {
	case domainprofile.RestoreModeRestoreFile:
		return syncer.Restore(backup.BackupPath)
	case domainprofile.RestoreModeRemoveCreatedFile:
		return syncer.RemoveConfig()
	default:
		return "", fmt.Errorf("unsupported restore mode: %s", backup.RestoreMode)
	}
}

func (s *ProfileService) BackupList(agent domainprofile.Agent) ([]domainprofile.Backup, error) {
	if !agent.Valid() {
		return nil, &InvalidAgentError{Agent: string(agent)}
	}
	state, err := s.state.Load()
	if err != nil {
		return nil, err
	}
	backups := append([]domainprofile.Backup(nil), currentBinding(state, agent).Backups...)
	return backups, nil
}

func selectBackup(backups []domainprofile.Backup, backupID string) (domainprofile.Backup, error) {
	if strings.TrimSpace(backupID) == "" {
		return backups[0], nil
	}
	for _, backup := range backups {
		if backup.ID == backupID {
			return backup, nil
		}
	}
	return domainprofile.Backup{}, &BackupNotFoundError{ID: backupID}
}
