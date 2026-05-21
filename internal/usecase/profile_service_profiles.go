package usecase

import (
	"strings"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func (s *ProfileService) Add(name string, input AddProfileInput) (result *AddProfileResult, err error) {
	unlock, err := s.lockMutations()
	if err != nil {
		return nil, err
	}
	defer unlock()

	name = domainprofile.NormalizeProfileName(name)
	if err := domainprofile.ValidateProfileName(name); err != nil {
		return nil, err
	}
	if _, err := s.profiles.Get(name); err == nil {
		return nil, &ProfileAlreadyExistsError{Name: name}
	} else if !IsProfileNotFoundError(err) {
		return nil, err
	}

	normalizedBaseURL := domainprofile.NormalizeBaseURL(input.BaseURL)
	normalizedAPIKey := strings.TrimSpace(input.APIKey)
	existing, err := s.findProfileByConfig(normalizedBaseURL, normalizedAPIKey)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.Name != name {
		return nil, &DuplicateRelayConfigError{
			Name:         name,
			ExistingName: existing.Name,
		}
	}

	guard := newMutationGuard(s)
	if err := guard.CaptureProfile(name); err != nil {
		return nil, err
	}
	if err := guard.CaptureState(); err != nil {
		return nil, err
	}
	if err := guard.CaptureAgents(append(append([]domainprofile.Agent(nil), input.Bind...), domainprofile.AgentOpenCode)...); err != nil {
		return nil, err
	}
	defer finishMutationGuard(guard, &err)

	now := time.Now().UTC()
	profile := domainprofile.Profile{
		Name:      name,
		BaseURL:   normalizedBaseURL,
		APIKey:    normalizedAPIKey,
		CreatedAt: now,
		UpdatedAt: now,
	}
	saved, err := s.saveProfile(profile, true)
	if err != nil {
		return nil, err
	}
	state, err := s.loadStoredState()
	if err != nil {
		return nil, err
	}
	if err := s.syncProfileAfterMutationWithState(*saved, state, false); err != nil {
		return nil, err
	}
	result = &AddProfileResult{Relay: saved}
	if len(input.Bind) > 0 {
		bindings, err := s.applyRelayBindingChanges(saved.Name, input.Bind, nil)
		if err != nil {
			return nil, err
		}
		result.Bindings = bindings
	}
	return result, nil
}

func (s *ProfileService) findProfileByConfig(baseURL, apiKey string) (*domainprofile.Profile, error) {
	profiles, err := s.profiles.List()
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if domainprofile.NormalizeBaseURL(profile.BaseURL) != baseURL {
			continue
		}
		if strings.TrimSpace(profile.APIKey) != apiKey {
			continue
		}
		matched := profile
		return &matched, nil
	}
	return nil, nil
}

func (s *ProfileService) Edit(name string, input EditProfileInput) (result *EditProfileResult, err error) {
	unlock, err := s.lockMutations()
	if err != nil {
		return nil, err
	}
	defer unlock()

	profile, err := s.Get(name)
	if err != nil {
		return nil, err
	}

	updated := *profile
	nameChanged := false
	if input.Name != nil {
		next := domainprofile.NormalizeProfileName(*input.Name)
		if err := domainprofile.ValidateProfileName(next); err != nil {
			return nil, err
		}
		if next != profile.Name {
			if _, err := s.profiles.Get(next); err == nil {
				return nil, &ProfileAlreadyExistsError{Name: next}
			} else if !IsProfileNotFoundError(err) {
				return nil, err
			}
			updated.Name = next
			nameChanged = true
		}
	}
	baseURLChanged := false
	apiKeyChanged := false
	if input.BaseURL != nil {
		next := domainprofile.NormalizeBaseURL(*input.BaseURL)
		baseURLChanged = next != profile.BaseURL
		updated.BaseURL = next
	}
	if input.APIKey != nil {
		next := strings.TrimSpace(*input.APIKey)
		apiKeyChanged = next != profile.APIKey
		updated.APIKey = next
	}
	needsSync := nameChanged || baseURLChanged || apiKeyChanged
	if input.OpenCode != nil {
		needsSync = true
	}
	needsBindingChanges := len(input.Bind) > 0 || len(input.Unbind) > 0
	if !needsSync && !needsBindingChanges {
		return &EditProfileResult{Relay: profile}, nil
	}

	effectiveState, err := s.loadEffectiveState()
	if err != nil {
		return nil, err
	}

	guard := newMutationGuard(s)
	if err := guard.CaptureProfile(profile.Name); err != nil {
		return nil, err
	}
	if nameChanged {
		if err := guard.CaptureProfile(updated.Name); err != nil {
			return nil, err
		}
	}
	if err := guard.CaptureState(); err != nil {
		return nil, err
	}
	affectedAgents := affectedAgentsForProfileMutation(profile.Name, effectiveState, needsSync, input.Bind, input.Unbind)
	if input.OpenCode != nil || containsAgent(input.Bind, domainprofile.AgentOpenCode) || containsAgent(input.Unbind, domainprofile.AgentOpenCode) {
		affectedAgents = append(affectedAgents, domainprofile.AgentOpenCode)
	}
	if err := guard.CaptureAgents(affectedAgents...); err != nil {
		return nil, err
	}
	defer finishMutationGuard(guard, &err)

	if needsSync {
		now := time.Now().UTC()
		updated.UpdatedAt = now

		if nameChanged {
			saved, err := s.renameProfile(profile.Name, updated, effectiveState, now)
			if err != nil {
				return nil, err
			}
			updated = *saved
		} else {
			saved, err := s.saveProfile(updated, false)
			if err != nil {
				return nil, err
			}
			updated = *saved
			if err := s.syncProfileAfterMutationWithState(updated, effectiveState, true); err != nil {
				return nil, err
			}
		}
	}
	result = &EditProfileResult{}
	if needsBindingChanges {
		bindings, err := s.applyRelayBindingChanges(updated.Name, input.Bind, input.Unbind)
		if err != nil {
			return nil, err
		}
		result.Bindings = bindings
	}
	result.Relay, err = s.Get(updated.Name)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func containsAgent(agents []domainprofile.Agent, target domainprofile.Agent) bool {
	for _, agent := range agents {
		if agent == target {
			return true
		}
	}
	return false
}

func normalizeOpenCodeProfileBinding(name string, binding *domainprofile.OpenCodeProfileBinding) domainprofile.OpenCodeProfileBinding {
	out := domainprofile.OpenCodeProfileBinding{
		ProviderID: domainprofile.OpenCodeProviderID(name),
	}
	if binding != nil {
		out = *binding
		if out.ProviderID == "" {
			out.ProviderID = domainprofile.OpenCodeProviderID(name)
		}
	}
	if out.ProviderID == "" {
		out.ProviderID = domainprofile.OpenCodeProviderID(name)
	}
	if out.ProviderFamily == "" {
		out.ProviderFamily = domainprofile.OpenCodeProviderFamilyOpenAICompatible
	}
	if strings.TrimSpace(out.ModelID) == "" {
		out.ModelID = name
	}
	if strings.TrimSpace(out.ModelName) == "" {
		out.ModelName = out.ModelID
	}
	return out
}

func (s *ProfileService) renameProfile(oldName string, profile domainprofile.Profile, stateBefore domainprofile.State, now time.Time) (*domainprofile.Profile, error) {
	oldName = domainprofile.NormalizeProfileName(oldName)
	profile.Name = domainprofile.NormalizeProfileName(profile.Name)
	if oldName == "" || profile.Name == "" || oldName == profile.Name {
		return s.saveProfile(profile, false)
	}

	saved, err := s.saveProfile(profile, false)
	if err != nil {
		return nil, err
	}

	state := cloneState(stateBefore)
	if err := s.syncRenamedProfile(*saved, oldName, &state, now); err != nil {
		return nil, err
	}
	renameStateProfileReferences(&state, oldName, saved.Name)
	if err := s.removeCodexProfileAfterRename(oldName, stateBefore); err != nil {
		return nil, err
	}
	if err := s.removeOpenCodeProfileAfterRename(oldName, stateBefore); err != nil {
		return nil, err
	}
	if err := s.profiles.Delete(oldName); err != nil {
		return nil, err
	}
	state.UpdatedAt = now
	if err := s.saveState(state); err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *ProfileService) syncRenamedProfile(profile domainprofile.Profile, oldName string, state *domainprofile.State, now time.Time) error {
	oldName = domainprofile.NormalizeProfileName(oldName)
	if oldName == "" {
		return nil
	}

	wasCodexTracked := isCodexProfileTracked(*state, oldName)
	wasCodexDefault := domainprofile.NormalizeProfileName(state.Codex.SourceProfile) == oldName
	bound := boundAgents(oldName, *state)

	if wasCodexTracked {
		backupID := nextBackupID(domainprofile.AgentCodex, currentBinding(*state, domainprofile.AgentCodex).Backups, now)
		backup, cleanup, _, err := s.syncProfileToAgent(domainprofile.AgentCodex, profile, backupID, *state, wasCodexDefault)
		if err != nil {
			return err
		}

		binding := currentBinding(*state, domainprofile.AgentCodex)
		if wasCodexDefault {
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
	}

	for _, agent := range bound {
		if agent == domainprofile.AgentCodex {
			continue
		}
		current := currentBinding(*state, agent)
		if current.SourceProfile == oldName {
			backupID := nextBackupID(agent, current.Backups, now)
			backup, cleanup, _, err := s.syncProfileToAgent(agent, profile, backupID, *state, false)
			if err != nil {
				return err
			}

			binding := current
			setBindingApplied(&binding, profile.Name, backup.ConfigPath, now, backup.ID)
			if err := prependBackupAndTrim(&binding, backup, cleanup); err != nil {
				return err
			}
			assignBinding(state, agent, binding)
			continue
		}

		binding := agentProfileBinding(*state, agent, oldName)
		if binding.Status == "" {
			continue
		}
		deleteAgentProfileBinding(state, agent, oldName)
		updateAgentProfileBinding(state, agent, profile.Name, func(next *domainprofile.AgentProfileBinding) {
			*next = binding
			next.Status = domainprofile.BindingStatusBound
			next.ConfigPath = binding.ConfigPath
		})
	}

	return nil
}

func (s *ProfileService) removeCodexProfileAfterRename(oldName string, stateBefore domainprofile.State) error {
	if s.codex == nil || !isCodexProfileTracked(stateBefore, oldName) {
		return nil
	}
	_, err := s.codex.RemoveProfile(oldName)
	return err
}

func (s *ProfileService) removeOpenCodeProfileAfterRename(oldName string, stateBefore domainprofile.State) error {
	if s.openCode == nil || !isOpenCodeProfileTracked(stateBefore, oldName) {
		return nil
	}
	_, err := s.openCode.RemoveProfile(oldName)
	return err
}

func (s *ProfileService) Remove(name string) (*domainprofile.Profile, error) {
	var profile *domainprofile.Profile
	return withMutationGuard(s,
		func(g *mutationGuard) error {
			p, err := s.Get(name)
			if err != nil {
				return err
			}
			profile = p
			if err := g.CaptureProfile(profile.Name); err != nil {
				return err
			}
			if err := g.CaptureState(); err != nil {
				return err
			}
			return g.CaptureAgents(domainprofile.AgentCodex, domainprofile.AgentOpenCode)
		},
		func() (*domainprofile.Profile, error) { return s.removeLocked(profile) },
	)
}

func (s *ProfileService) removeLocked(profile *domainprofile.Profile) (*domainprofile.Profile, error) {
	inUseBy, err := s.boundAgentsForProfile(profile.Name)
	if err != nil {
		return nil, err
	}
	if len(inUseBy) > 0 {
		return nil, &ProfileInUseError{
			Name:   profile.Name,
			Agents: inUseBy,
		}
	}
	if err := s.profiles.Delete(profile.Name); err != nil {
		return nil, err
	}
	if err := s.removeCodexProfileArtifacts(profile.Name); err != nil {
		return nil, err
	}
	if err := s.clearCodexSourceProfileIfMatches(profile.Name); err != nil {
		return nil, err
	}
	if err := s.removeOpenCodeProfileArtifacts(profile.Name); err != nil {
		return nil, err
	}
	if err := s.clearOpenCodeSourceProfileIfMatches(profile.Name); err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *ProfileService) saveProfile(profile domainprofile.Profile, created bool) (*domainprofile.Profile, error) {
	if err := domainprofile.ValidateProfileName(profile.Name); err != nil {
		return nil, err
	}
	if err := domainprofile.ValidateBaseURL(profile.BaseURL); err != nil {
		return nil, err
	}
	if err := domainprofile.ValidateAPIKey(profile.APIKey); err != nil {
		return nil, err
	}
	if created && profile.CreatedAt.IsZero() {
		profile.CreatedAt = time.Now().UTC()
	}
	if profile.UpdatedAt.IsZero() {
		profile.UpdatedAt = time.Now().UTC()
	}

	return s.profiles.Upsert(profile)
}
