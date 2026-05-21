package usecase

import (
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func (s *ProfileService) applyRelayBindingChanges(relayName string, bindAgents, unbindAgents []domainprofile.Agent) (*BindingsResult, error) {
	relayName = domainprofile.NormalizeProfileName(relayName)
	if _, err := s.Get(relayName); err != nil {
		return nil, err
	}

	bindAgents = normalizeAgents(bindAgents)
	unbindAgents = normalizeAgents(unbindAgents)
	if overlap := overlappingAgents(bindAgents, unbindAgents); len(overlap) > 0 {
		return nil, &ConflictingAgentChangesError{Agents: overlap}
	}

	stateBefore, err := s.loadStoredState()
	if err != nil {
		return nil, err
	}
	stateSnapshot, err := s.loadEffectiveState()
	if err != nil {
		return nil, err
	}

	for _, agent := range unbindAgents {
		if !agentBoundToRelay(stateSnapshot, agent, relayName) {
			return nil, &AgentNotBoundToRelayError{Agent: agent, Relay: relayName}
		}
	}

	results := make([]BindingChangeResult, 0, len(bindAgents)+len(unbindAgents))
	for _, agent := range bindAgents {
		previousRelay := domainprofile.NormalizeProfileName(currentBinding(stateSnapshot, agent).SourceProfile)
		if previousRelay == relayName {
			previousRelay = ""
		}
		result, err := s.agentBindLocked(agent, relayName)
		if err != nil {
			rollbackErr := s.rollbackBindingChanges(stateBefore, results)
			if rollbackErr != nil {
				return nil, rollbackErr
			}
			return nil, err
		}
		results = append(results, BindingChangeResult{
			Agent:         agent,
			Action:        "bind",
			PreviousRelay: previousRelay,
			Binding:       result.Binding,
			Backup:        result.Backup,
			CodexProfile:  result.CodexProfile,
		})
	}

	for _, agent := range unbindAgents {
		result, err := s.unbindRelayFromAgentLocked(agent, relayName)
		if err != nil {
			rollbackErr := s.rollbackBindingChanges(stateBefore, results)
			if rollbackErr != nil {
				return nil, rollbackErr
			}
			return nil, err
		}
		results = append(results, BindingChangeResult{
			Agent:  agent,
			Action: "unbind",
			Backup: result.Backup,
		})
	}

	relay, err := s.Get(relayName)
	if err != nil {
		return nil, err
	}
	return &BindingsResult{
		Relay:   relay,
		Changed: results,
	}, nil
}

func agentBoundToRelay(state domainprofile.State, agent domainprofile.Agent, relayName string) bool {
	relayName = domainprofile.NormalizeProfileName(relayName)
	if agent == domainprofile.AgentCodex {
		return isCodexProfileTracked(state, relayName)
	}
	if domainprofile.NormalizeProfileName(currentBinding(state, agent).SourceProfile) == relayName {
		return true
	}
	binding := agentProfileBinding(state, agent, relayName)
	return binding.Status != "" && binding.Status.Valid()
}

func (s *ProfileService) unbindRelayFromAgentLocked(agent domainprofile.Agent, relayName string) (*RestoreResult, error) {
	if agent == domainprofile.AgentCodex {
		return s.removeCodexProfileLocked(relayName)
	}
	state, err := s.loadEffectiveState()
	if err != nil {
		return nil, err
	}

	current := currentBinding(state, agent)
	if domainprofile.NormalizeProfileName(current.SourceProfile) == domainprofile.NormalizeProfileName(relayName) {
		if agent == domainprofile.AgentOpenCode {
			return s.removeOpenCodeProfileLocked(relayName)
		}
		result, err := s.clearLocked(agent)
		if err != nil {
			return nil, err
		}
		stateAfter, err := s.loadStoredState()
		if err != nil {
			return nil, err
		}
		deleteAgentProfileBinding(&stateAfter, agent, relayName)
		stateAfter.UpdatedAt = time.Now().UTC()
		if err := s.saveState(stateAfter); err != nil {
			return nil, err
		}
		return result, nil
	}

	binding := agentProfileBinding(state, agent, relayName)
	if binding.Status == "" {
		return nil, &AgentNotBoundToRelayError{Agent: agent, Relay: relayName}
	}

	now := time.Now().UTC()
	backupID := nextBackupID(agent, current.Backups, now)
	operation := newOperationRecord("unbind", agent, relayName, now)
	if err := s.beginOperation(operation); err != nil {
		return nil, err
	}

	backup, err := s.snapshotCurrentConfig(agent, current.SourceProfile, backupID, now)
	if err != nil {
		_ = s.clearOperation(operation.ID)
		return nil, err
	}

	deleteAgentProfileBinding(&state, agent, relayName)
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

	return &RestoreResult{
		Agent:      agent,
		ConfigPath: backup.ConfigPath,
		Backup:     backup,
	}, nil
}

func (s *ProfileService) rollbackBindingChanges(state domainprofile.State, applied []BindingChangeResult) error {
	for i := len(applied) - 1; i >= 0; i-- {
		change := applied[i]
		switch change.Action {
		case "bind", "unbind":
			if err := s.restoreBindingChange(change); err != nil {
				return err
			}
		}
	}
	return s.saveState(cloneState(state))
}

func (s *ProfileService) restoreBindingChange(change BindingChangeResult) error {
	switch change.Agent {
	case domainprofile.AgentCodex:
		if s.codex == nil {
			return &InvalidAgentError{Agent: string(change.Agent)}
		}
		_, err := restoreBackupForAgent(change.Backup, s.codex)
		return err
	case domainprofile.AgentClaude:
		if s.claude == nil {
			return &InvalidAgentError{Agent: string(change.Agent)}
		}
		_, err := restoreBackupForAgent(change.Backup, s.claude)
		return err
	case domainprofile.AgentGemini:
		if s.gemini == nil {
			return &InvalidAgentError{Agent: string(change.Agent)}
		}
		_, err := restoreBackupForAgent(change.Backup, s.gemini)
		return err
	case domainprofile.AgentOpenCode:
		if s.openCode == nil {
			return &InvalidAgentError{Agent: string(change.Agent)}
		}
		_, err := restoreBackupForAgent(change.Backup, s.openCode)
		return err
	default:
		return &InvalidAgentError{Agent: string(change.Agent)}
	}
}

func normalizeAgents(agents []domainprofile.Agent) []domainprofile.Agent {
	if len(agents) == 0 {
		return nil
	}
	seen := make(map[domainprofile.Agent]struct{}, len(agents))
	out := make([]domainprofile.Agent, 0, len(agents))
	for _, agent := range agents {
		if !agent.Valid() {
			continue
		}
		if _, ok := seen[agent]; ok {
			continue
		}
		seen[agent] = struct{}{}
		out = append(out, agent)
	}
	return out
}

func overlappingAgents(a, b []domainprofile.Agent) []domainprofile.Agent {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	seen := make(map[domainprofile.Agent]struct{}, len(a))
	for _, agent := range a {
		seen[agent] = struct{}{}
	}
	var out []domainprofile.Agent
	for _, agent := range b {
		if _, ok := seen[agent]; ok {
			out = append(out, agent)
			delete(seen, agent)
		}
	}
	return out
}

func cloneState(state domainprofile.State) domainprofile.State {
	copyState := state
	copyState.Codex.Backups = append([]domainprofile.Backup(nil), state.Codex.Backups...)
	copyState.Claude.Backups = append([]domainprofile.Backup(nil), state.Claude.Backups...)
	copyState.Gemini.Backups = append([]domainprofile.Backup(nil), state.Gemini.Backups...)
	copyState.OpenCode.Backups = append([]domainprofile.Backup(nil), state.OpenCode.Backups...)
	copyState.ManagedAgents = cloneManagedAgents(state.ManagedAgents)
	return copyState
}

func cloneManagedAgents(src map[domainprofile.Agent]domainprofile.ManagedAgentState) map[domainprofile.Agent]domainprofile.ManagedAgentState {
	if src == nil {
		return nil
	}
	dst := make(map[domainprofile.Agent]domainprofile.ManagedAgentState, len(src))
	for agent, managed := range src {
		dst[agent] = cloneManagedAgentState(managed)
	}
	return dst
}

func cloneManagedAgentState(src domainprofile.ManagedAgentState) domainprofile.ManagedAgentState {
	dst := src
	if src.Targets != nil {
		dst.Targets = make(map[string]domainprofile.TargetState, len(src.Targets))
		for name, target := range src.Targets {
			dst.Targets[name] = cloneTargetState(target)
		}
	}
	return dst
}

func cloneTargetState(src domainprofile.TargetState) domainprofile.TargetState {
	dst := src
	dst.Backups = cloneContextBackups(src.Backups)
	return dst
}
