package usecase

import (
	"errors"
	"strings"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

func (s *ProfileService) State() (domainprofile.State, error) {
	state, err := s.loadEffectiveState()
	if err != nil {
		return domainprofile.State{}, err
	}
	return state, nil
}

func (s *ProfileService) loadStoredState() (domainprofile.State, error) {
	state, err := s.state.Load()
	if err != nil {
		return domainprofile.State{}, err
	}
	normalizeStateRegistries(&state)
	return cloneState(state), nil
}

func (s *ProfileService) loadResolvedState() (domainprofile.State, error) {
	state, err := s.loadStoredState()
	if err != nil {
		return domainprofile.State{}, err
	}
	if s.codex == nil {
	} else {
		status, err := s.codex.Status()
		if err != nil {
			return domainprofile.State{}, err
		}
		applyResolvedCodexStatus(&state, status)
	}
	if s.openCode != nil {
		status, err := s.openCode.Status()
		if err != nil {
			return domainprofile.State{}, err
		}
		applyResolvedOpenCodeStatus(&state, status)
	}
	return state, nil
}

func (s *ProfileService) loadEffectiveState() (domainprofile.State, error) {
	state, err := s.loadResolvedState()
	if err != nil {
		return domainprofile.State{}, err
	}
	if !s.hasRuntimeManagedAgents() {
		return state, nil
	}

	profiles, err := s.List()
	if err != nil {
		return domainprofile.State{}, err
	}
	s.enrichRuntimeState(&state, profiles)
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
	return s.syncProfileAfterMutationWithState(profile, state, syncCodexRegistry)
}

func (s *ProfileService) syncProfileAfterMutationWithState(profile domainprofile.Profile, state domainprofile.State, syncCodexRegistry bool) error {
	now := time.Now().UTC()
	if syncCodexRegistry && isCodexProfileTracked(state, profile.Name) {
		if err := s.syncCodexProfile(profile, &state, now); err != nil {
			return err
		}
	}
	if isOpenCodeProfileTracked(state, profile.Name) {
		if err := s.syncOpenCodeProfile(profile, &state, now); err != nil {
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
	for _, agent := range currentAgents(name, state) {
		add(agent)
	}
	if syncCodexRegistry && isCodexProfileTracked(state, name) {
		add(domainprofile.AgentCodex)
	}
	if isOpenCodeProfileTracked(state, name) {
		add(domainprofile.AgentOpenCode)
	}
	return agents
}

func isCodexProfileTracked(state domainprofile.State, name string) bool {
	name = domainprofile.NormalizeProfileName(name)
	if state.Codex.SourceProfile == name {
		return true
	}
	return isManagedTargetTracked(state, domainprofile.AgentCodex, name)
}

func isAgentProfileTracked(state domainprofile.State, agent domainprofile.Agent, name string) bool {
	name = domainprofile.NormalizeProfileName(name)
	if name == "" {
		return false
	}
	if currentBinding(state, agent).SourceProfile == name {
		return true
	}
	binding := agentProfileBinding(state, agent, name)
	return binding.Status != "" && binding.Status.Valid()
}

func isOpenCodeProfileTracked(state domainprofile.State, name string) bool {
	name = domainprofile.NormalizeProfileName(name)
	if state.OpenCode.SourceProfile == name {
		return true
	}
	return isManagedTargetTracked(state, domainprofile.AgentOpenCode, name)
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

func (s *ProfileService) clearOpenCodeSourceProfileIfMatches(name string) error {
	state, err := s.loadStoredState()
	if err != nil {
		return err
	}
	if state.OpenCode.SourceProfile != name {
		return nil
	}
	clearBindingView(&state.OpenCode.BindingView)
	state.UpdatedAt = time.Now().UTC()
	return s.saveState(state)
}

func (s *ProfileService) boundAgentsForProfile(name string) ([]domainprofile.Agent, error) {
	state, err := s.State()
	if err != nil {
		return nil, err
	}
	return boundAgents(name, state), nil
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
	if state.Codex.SourceProfile == name {
		clearBindingView(&state.Codex.BindingView)
	}
	state.UpdatedAt = time.Now().UTC()
	return s.saveState(state)
}

func (s *ProfileService) removeOpenCodeProfileArtifacts(name string) error {
	if s.openCode == nil {
		return nil
	}
	if _, err := s.openCode.RemoveProfile(name); err != nil {
		return err
	}

	state, err := s.loadStoredState()
	if err != nil {
		return err
	}
	if state.OpenCode.SourceProfile == name {
		clearBindingView(&state.OpenCode.BindingView)
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

	clearBindingView(&state.Codex.BindingView)
	if status == nil {
		return nil
	}
	state.Codex.ConfigPath = status.ConfigPath
	if status.DefaultProfileName == "" {
		return nil
	}

	state.Codex.SourceProfile = status.DefaultProfileName
	state.Codex.Status = domainprofile.BindingStatusApplied
	return nil
}

func (s *ProfileService) refreshOpenCodeStateAfterRestore(state *domainprofile.State) error {
	if s.openCode == nil {
		return nil
	}

	status, err := s.openCode.Status()
	if err != nil {
		return err
	}

	clearBindingView(&state.OpenCode.BindingView)
	if status == nil {
		return nil
	}
	state.OpenCode.ConfigPath = status.ConfigPath
	if status.DefaultModel == "" {
		return nil
	}

	providerID, modelID := splitOpenCodeModelRef(status.DefaultModel)
	if providerID == "" || modelID == "" {
		return nil
	}
	profileName := openCodeProfileNameFromProviderID(providerID)
	state.OpenCode.SourceProfile = profileName
	state.OpenCode.Status = domainprofile.BindingStatusApplied
	return nil
}

func normalizeStateRegistries(state *domainprofile.State) {
	if state == nil {
		return
	}
	normalizeManagedAgentRegistries(state)
}

func applyResolvedCodexStatus(state *domainprofile.State, status *ports.CodexConfigStatus) {
	if status == nil {
		return
	}

	clearBindingView(&state.Codex.BindingView)
	state.Codex.ConfigPath = status.ConfigPath
	if status.DefaultProfileName == "" {
		return
	}

	state.Codex.SourceProfile = status.DefaultProfileName
	state.Codex.Status = domainprofile.BindingStatusApplied
}

func applyResolvedOpenCodeStatus(state *domainprofile.State, status *ports.OpenCodeConfigStatus) {
	if status == nil {
		return
	}

	clearBindingView(&state.OpenCode.BindingView)
	state.OpenCode.ConfigPath = status.ConfigPath
	if status.DefaultModel == "" {
		return
	}

	providerID, modelID := splitOpenCodeModelRef(status.DefaultModel)
	if providerID == "" || modelID == "" {
		return
	}
	state.OpenCode.SourceProfile = openCodeProfileNameFromProviderID(providerID)
	state.OpenCode.Status = domainprofile.BindingStatusApplied
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

	// Run the cleanups before committing the in-memory slice trim. If
	// cleanup fails midway we surface all failures via errors.Join instead
	// of returning early with the metadata already truncated — that earlier
	// behaviour orphaned both the slice entries and the on-disk files.
	trimmed := append([]domainprofile.Backup(nil), (*backups)[backupHistoryLimit:]...)
	var cleanupErrs []error
	for _, item := range trimmed {
		if err := cleanup(item.BackupPath); err != nil {
			cleanupErrs = append(cleanupErrs, err)
		}
	}
	if len(cleanupErrs) > 0 {
		return errors.Join(cleanupErrs...)
	}
	*backups = (*backups)[:backupHistoryLimit]
	return nil
}

func agentProfileBindings(state domainprofile.State, agent domainprofile.Agent) map[string]domainprofile.AgentProfileBinding {
	_ = state
	_ = agent
	return map[string]domainprofile.AgentProfileBinding{}
}

func agentProfileBinding(state domainprofile.State, agent domainprofile.Agent, name string) domainprofile.AgentProfileBinding {
	_ = state
	_ = agent
	_ = name
	return domainprofile.AgentProfileBinding{}
}

func updateAgentProfileBinding(state *domainprofile.State, agent domainprofile.Agent, name string, update func(*domainprofile.AgentProfileBinding)) {
	_ = state
	_ = agent
	_ = name
	_ = update
}

func deleteAgentProfileBinding(state *domainprofile.State, agent domainprofile.Agent, name string) {
	_ = state
	_ = agent
	_ = name
}

func currentAgents(name string, state domainprofile.State) []domainprofile.Agent {
	agents := make([]domainprofile.Agent, 0, 3)
	name = domainprofile.NormalizeProfileName(name)
	if currentBinding(state, domainprofile.AgentCodex).SourceProfile == name {
		agents = append(agents, domainprofile.AgentCodex)
	}
	for _, agent := range []domainprofile.Agent{domainprofile.AgentClaude, domainprofile.AgentGemini, domainprofile.AgentOpenCode} {
		if currentBinding(state, agent).SourceProfile == name {
			agents = append(agents, agent)
		}
	}
	return agents
}

func boundAgents(name string, state domainprofile.State) []domainprofile.Agent {
	agents := make([]domainprofile.Agent, 0, 3)
	name = domainprofile.NormalizeProfileName(name)
	if isCodexProfileTracked(state, name) || isManagedTargetTracked(state, domainprofile.AgentCodex, name) {
		agents = append(agents, domainprofile.AgentCodex)
	}
	for _, agent := range []domainprofile.Agent{domainprofile.AgentClaude, domainprofile.AgentGemini} {
		if isAgentProfileTracked(state, agent, name) || isManagedTargetTracked(state, agent, name) {
			agents = append(agents, agent)
		}
	}
	if isOpenCodeProfileTracked(state, name) || isManagedTargetTracked(state, domainprofile.AgentOpenCode, name) {
		agents = append(agents, domainprofile.AgentOpenCode)
	}
	return agents
}

func isManagedTargetTracked(state domainprofile.State, agent domainprofile.Agent, name string) bool {
	name = domainprofile.NormalizeProfileName(name)
	if name == "" {
		return false
	}
	managed, ok := state.ManagedAgents[agent]
	if !ok || managed.Targets == nil {
		return false
	}
	_, ok = managed.Targets[name]
	return ok
}

func renameStateProfileReferences(state *domainprofile.State, oldName, newName string) {
	oldName = domainprofile.NormalizeProfileName(oldName)
	newName = domainprofile.NormalizeProfileName(newName)
	if oldName == "" || newName == "" || oldName == newName {
		return
	}

	if domainprofile.NormalizeProfileName(state.Codex.SourceProfile) == oldName {
		state.Codex.SourceProfile = newName
	}
	if domainprofile.NormalizeProfileName(state.Claude.SourceProfile) == oldName {
		state.Claude.SourceProfile = newName
	}
	if domainprofile.NormalizeProfileName(state.Gemini.SourceProfile) == oldName {
		state.Gemini.SourceProfile = newName
	}
	if domainprofile.NormalizeProfileName(state.OpenCode.SourceProfile) == oldName {
		state.OpenCode.SourceProfile = newName
	}
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

func currentBinding(state domainprofile.State, agent domainprofile.Agent) domainprofile.AgentBinding {
	switch agent {
	case domainprofile.AgentCodex:
		return state.Codex.AgentBinding()
	case domainprofile.AgentClaude:
		return state.Claude
	case domainprofile.AgentGemini:
		return state.Gemini
	case domainprofile.AgentOpenCode:
		return state.OpenCode.AgentBinding()
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
	case domainprofile.AgentOpenCode:
		state.OpenCode.SetAgentBinding(binding)
	}
}

func openCodeProfileNameFromProviderID(providerID string) string {
	providerID = strings.TrimSpace(providerID)
	if !strings.HasPrefix(providerID, "agx-") {
		return ""
	}
	name := strings.TrimPrefix(providerID, "agx-")
	// agx writes one provider per family with id `agx-<name>-<family>`. Trim
	// the family suffix so doctor / state inference still recovers the
	// underlying profile name. Legacy single-provider format (`agx-<name>`)
	// has no suffix and falls through unchanged.
	for _, family := range domainprofile.OpenCodeManagedFamilies() {
		if trimmed, ok := strings.CutSuffix(name, "-"+string(family)); ok {
			name = trimmed
			break
		}
	}
	return domainprofile.NormalizeProfileName(name)
}

func splitOpenCodeModelRef(ref string) (string, string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ""
	}
	providerID, modelID, ok := strings.Cut(ref, "/")
	if !ok {
		return "", ""
	}
	return strings.TrimSpace(providerID), strings.TrimSpace(modelID)
}
