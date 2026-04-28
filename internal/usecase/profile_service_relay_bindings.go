package usecase

import domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"

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
	stateSnapshot := cloneState(stateBefore)

	for _, agent := range unbindAgents {
		if currentBinding(stateSnapshot, agent).SourceProfile != relayName {
			return nil, &AgentNotBoundToRelayError{Agent: agent, Relay: relayName}
		}
	}

	results := make([]BindingChangeResult, 0, len(bindAgents)+len(unbindAgents))
	for _, agent := range bindAgents {
		result, err := s.agentSetLocked(agent, relayName)
		if err != nil {
			rollbackErr := s.rollbackBindingChanges(stateBefore, results)
			if rollbackErr != nil {
				return nil, rollbackErr
			}
			return nil, err
		}
		binding := result.Binding
		results = append(results, BindingChangeResult{
			Agent:        agent,
			Action:       "bind",
			Binding:      &binding,
			Backup:       result.Backup,
			CodexProfile: result.CodexProfileName,
		})
	}

	for _, agent := range unbindAgents {
		result, err := s.clearLocked(agent)
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
	if state.CodexProfiles != nil {
		copyState.CodexProfiles = make(map[string]domainprofile.CodexProfileBinding, len(state.CodexProfiles))
		for key, value := range state.CodexProfiles {
			copyState.CodexProfiles[key] = value
		}
	}
	copyState.Claude.Backups = append([]domainprofile.Backup(nil), state.Claude.Backups...)
	copyState.Gemini.Backups = append([]domainprofile.Backup(nil), state.Gemini.Backups...)
	return copyState
}
