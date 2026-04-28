package usecase

import (
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func (s *ProfileService) State() (domainprofile.State, error) {
	state, err := s.loadResolvedState()
	if err != nil {
		return domainprofile.State{}, err
	}
	return state, nil
}

func (s *ProfileService) loadStoredState() (domainprofile.State, error) {
	return s.state.Load()
}

func (s *ProfileService) loadResolvedState() (domainprofile.State, error) {
	state, err := s.loadStoredState()
	if err != nil {
		return domainprofile.State{}, err
	}
	if s.codex == nil {
		return state, nil
	}

	status, err := s.codex.Status()
	if err != nil {
		return domainprofile.State{}, err
	}

	applyResolvedCodexStatus(&state, status)
	return state, nil
}

func (s *ProfileService) saveState(state domainprofile.State) error {
	_, err := s.state.Save(state)
	return err
}

func (s *ProfileService) syncProfileAfterMutation(profile domainprofile.Profile, syncCodexRegistry bool) error {
	state, err := s.loadStoredState()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if syncCodexRegistry && isCodexProfileTracked(state, profile.Name) {
		if err := s.syncCodexProfile(profile, &state, now); err != nil {
			return err
		}
	}
	if err := s.syncProfileToBoundAgents(profile, &state, now); err != nil {
		return err
	}
	state.UpdatedAt = now
	return s.saveState(state)
}

func affectedAgentsForProfileMutation(name string, state domainprofile.State, syncCodexRegistry bool, bindAgents, unbindAgents []domainprofile.Agent) []domainprofile.Agent {
	seen := map[domainprofile.Agent]struct{}{}
	var agents []domainprofile.Agent
	add := func(agent domainprofile.Agent) {
		if !agent.Valid() {
			return
		}
		if _, ok := seen[agent]; ok {
			return
		}
		seen[agent] = struct{}{}
		agents = append(agents, agent)
	}

	for _, agent := range bindAgents {
		add(agent)
	}
	for _, agent := range unbindAgents {
		add(agent)
	}
	for _, agent := range boundAgents(name, state) {
		add(agent)
	}
	if syncCodexRegistry && isCodexProfileTracked(state, name) {
		add(domainprofile.AgentCodex)
	}
	return agents
}

func isCodexProfileTracked(state domainprofile.State, name string) bool {
	name = domainprofile.NormalizeProfileName(name)
	if state.Codex.SourceProfile == name {
		return true
	}
	if len(state.CodexProfiles) == 0 {
		return false
	}
	_, ok := state.CodexProfiles[name]
	return ok
}

func (s *ProfileService) clearCodexSourceProfileIfMatches(name string) error {
	state, err := s.loadStoredState()
	if err != nil {
		return err
	}
	if state.Codex.SourceProfile != name {
		return nil
	}
	clearBindingView(&state.Codex.BindingView)
	state.UpdatedAt = time.Now().UTC()
	return s.saveState(state)
}

func (s *ProfileService) boundAgentsForProfile(name string) ([]domainprofile.Agent, error) {
	state, err := s.loadResolvedState()
	if err != nil {
		return nil, err
	}

	agents := make([]domainprofile.Agent, 0, 3)
	if state.Codex.SourceProfile == name {
		agents = append(agents, domainprofile.AgentCodex)
	}
	for _, agent := range []domainprofile.Agent{domainprofile.AgentClaude, domainprofile.AgentGemini} {
		if currentBinding(state, agent).SourceProfile == name {
			agents = append(agents, agent)
		}
	}
	return agents, nil
}

func (s *ProfileService) removeCodexProfileArtifacts(name string) error {
	if s.codex == nil {
		return nil
	}
	if _, err := s.codex.RemoveProfile(name); err != nil {
		return err
	}

	state, err := s.loadStoredState()
	if err != nil {
		return err
	}
	deleteCodexProfileBinding(&state, name)
	if state.Codex.SourceProfile == name {
		clearBindingView(&state.Codex.BindingView)
	}
	state.UpdatedAt = time.Now().UTC()
	return s.saveState(state)
}

func (s *ProfileService) refreshCodexStateAfterRestore(state *domainprofile.State) error {
	if s.codex == nil {
		return nil
	}

	status, err := s.codex.Status()
	if err != nil {
		return err
	}

	state.CodexProfiles = resolveCodexProfiles(nil, status)
	clearBindingView(&state.Codex.BindingView)
	if status == nil {
		return nil
	}
	state.Codex.ConfigPath = status.ConfigPath
	if status.DefaultProfileName == "" {
		return nil
	}

	binding := codexProfileBinding(*state, status.DefaultProfileName)
	state.Codex.SourceProfile = status.DefaultProfileName
	state.Codex.Status = binding.Status
	state.Codex.ConfigPath = binding.ConfigPath
	return nil
}

func ensureCodexProfiles(bindings map[string]domainprofile.CodexProfileBinding) map[string]domainprofile.CodexProfileBinding {
	if bindings == nil {
		return map[string]domainprofile.CodexProfileBinding{}
	}
	return bindings
}

func resolveCodexProfiles(stored map[string]domainprofile.CodexProfileBinding, status *ports.CodexConfigStatus) map[string]domainprofile.CodexProfileBinding {
	resolved := map[string]domainprofile.CodexProfileBinding{}
	for name, binding := range stored {
		resolved[domainprofile.NormalizeProfileName(name)] = binding
	}
	if status == nil {
		return resolved
	}
	for name := range status.ManagedProfilesByID {
		name = domainprofile.NormalizeProfileName(name)
		binding := resolved[name]
		binding.ConfigPath = status.ConfigPath
		if binding.Status == "" {
			binding.Status = domainprofile.BindingStatusApplied
		}
		resolved[name] = binding
	}
	for name, binding := range resolved {
		if binding.ConfigPath == "" {
			binding.ConfigPath = status.ConfigPath
		}
		if _, ok := status.ManagedProfilesByID[name]; !ok {
			delete(resolved, name)
		}
	}
	return resolved
}

func applyResolvedCodexStatus(state *domainprofile.State, status *ports.CodexConfigStatus) {
	if status == nil {
		return
	}

	state.CodexProfiles = resolveCodexProfiles(state.CodexProfiles, status)
	clearBindingView(&state.Codex.BindingView)
	state.Codex.ConfigPath = status.ConfigPath
	if status.DefaultProfileName == "" {
		return
	}

	binding := codexProfileBinding(*state, status.DefaultProfileName)
	state.Codex.SourceProfile = status.DefaultProfileName
	state.Codex.Status = binding.Status
	state.Codex.LastAppliedAt = binding.LastAppliedAt
	state.Codex.LastBackupID = binding.LastBackupID
	if binding.ConfigPath != "" {
		state.Codex.ConfigPath = binding.ConfigPath
	}
}

func prependBackupAndTrim(binding *domainprofile.AgentBinding, backup domainprofile.Backup, cleanup func(string) error) error {
	binding.LastBackupID = backup.ID
	return prependBackupListAndTrim(&binding.Backups, backup, cleanup)
}

func prependBackupListAndTrim(backups *[]domainprofile.Backup, backup domainprofile.Backup, cleanup func(string) error) error {
	*backups = append([]domainprofile.Backup{backup}, (*backups)...)
	if len(*backups) <= backupHistoryLimit {
		return nil
	}

	trimmed := (*backups)[backupHistoryLimit:]
	*backups = (*backups)[:backupHistoryLimit]
	for _, item := range trimmed {
		if err := cleanup(item.BackupPath); err != nil {
			return err
		}
	}
	return nil
}

func assignCodexProfileApplied(state *domainprofile.State, name, configPath string, appliedAt time.Time, backupID string) {
	updateCodexProfileBinding(state, name, func(binding *domainprofile.CodexProfileBinding) {
		binding.Status = domainprofile.BindingStatusApplied
		binding.ConfigPath = configPath
		binding.LastAppliedAt = appliedAt
		binding.LastBackupID = backupID
	})
}

func updateCodexProfileBinding(state *domainprofile.State, name string, update func(*domainprofile.CodexProfileBinding)) {
	name = domainprofile.NormalizeProfileName(name)
	if name == "" {
		return
	}

	state.CodexProfiles = ensureCodexProfiles(state.CodexProfiles)
	binding := state.CodexProfiles[name]
	update(&binding)
	state.CodexProfiles[name] = binding
}

func deleteCodexProfileBinding(state *domainprofile.State, name string) {
	if len(state.CodexProfiles) == 0 {
		return
	}
	delete(state.CodexProfiles, domainprofile.NormalizeProfileName(name))
}

func setBindingApplied(binding *domainprofile.AgentBinding, profileName, configPath string, appliedAt time.Time, backupID string) {
	binding.SourceProfile = profileName
	binding.Status = domainprofile.BindingStatusApplied
	binding.ConfigPath = configPath
	binding.LastAppliedAt = appliedAt
	binding.LastBackupID = backupID
}

func clearResolvedBinding(binding *domainprofile.AgentBinding) {
	binding.SourceProfile = ""
	binding.Status = ""
	binding.ConfigPath = ""
	binding.LastAppliedAt = time.Time{}
	binding.LastBackupID = ""
}

func clearBindingView(binding *domainprofile.BindingView) {
	binding.SourceProfile = ""
	binding.Status = ""
	binding.ConfigPath = ""
	binding.LastAppliedAt = time.Time{}
	binding.LastBackupID = ""
}

func codexBindingView(state *domainprofile.State) *domainprofile.BindingView {
	return &state.Codex.BindingView
}

func codexProfileBinding(state domainprofile.State, name string) domainprofile.CodexProfileBinding {
	if len(state.CodexProfiles) == 0 {
		return domainprofile.CodexProfileBinding{}
	}
	return state.CodexProfiles[domainprofile.NormalizeProfileName(name)]
}

func currentBinding(state domainprofile.State, agent domainprofile.Agent) domainprofile.AgentBinding {
	switch agent {
	case domainprofile.AgentCodex:
		return state.Codex.AgentBinding()
	case domainprofile.AgentClaude:
		return state.Claude
	case domainprofile.AgentGemini:
		return state.Gemini
	default:
		return domainprofile.AgentBinding{}
	}
}

func assignBinding(state *domainprofile.State, agent domainprofile.Agent, binding domainprofile.AgentBinding) {
	switch agent {
	case domainprofile.AgentCodex:
		state.Codex.SetAgentBinding(binding)
	case domainprofile.AgentClaude:
		state.Claude = binding
	case domainprofile.AgentGemini:
		state.Gemini = binding
	}
}
