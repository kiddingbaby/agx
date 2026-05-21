package usecase

import (
	"fmt"
	"strings"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func (s *ProfileService) Restore(agent domainprofile.Agent, backupID string) (*RestoreResult, error) {
	if !agent.Valid() {
		return nil, &InvalidAgentError{Agent: string(agent)}
	}
	return withMutationGuard(s,
		func(g *mutationGuard) error {
			if err := g.CaptureState(); err != nil {
				return err
			}
			return g.CaptureAgents(agent)
		},
		func() (*RestoreResult, error) { return s.restoreLocked(agent, backupID) },
	)
}

func (s *ProfileService) restoreLocked(agent domainprofile.Agent, backupID string) (*RestoreResult, error) {
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
	if err := s.takeoverStaleOperation(); err != nil {
		return nil, err
	}
	if err := s.beginOperation(operation); err != nil {
		return nil, err
	}

	var configPath string
	syncer := s.resolveAgentSyncer(agent)
	if syncer == nil {
		_ = s.clearOperation(operation.ID)
		return nil, &InvalidAgentError{Agent: string(agent)}
	}
	configPath, err = restoreBackupForAgent(selected, syncer)
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
