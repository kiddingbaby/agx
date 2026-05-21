package usecase

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func (s *ProfileService) ListTargets(agent domainprofile.Agent) ([]TargetResult, error) {
	if !agent.Valid() {
		return nil, &InvalidAgentError{Agent: string(agent)}
	}
	managed, err := s.ManagedAgent(agent)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(managed.Targets))
	for name := range managed.Targets {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]TargetResult, 0, len(names))
	for _, name := range names {
		out = append(out, TargetResult{Agent: agent, Name: name, Target: managed.Targets[name]})
	}
	return out, nil
}

func (s *ProfileService) Target(agent domainprofile.Agent, name string) (TargetResult, error) {
	if !agent.Valid() {
		return TargetResult{}, &InvalidAgentError{Agent: string(agent)}
	}
	name = normalizeTargetName(name)
	if err := validateTargetName(name); err != nil {
		return TargetResult{}, err
	}
	managed, err := s.ManagedAgent(agent)
	if err != nil {
		return TargetResult{}, err
	}
	target, ok := managed.Targets[name]
	if !ok {
		return TargetResult{}, &TargetNotFoundError{Agent: agent, Name: name}
	}
	return TargetResult{Agent: agent, Name: name, Target: target}, nil
}

func (s *ProfileService) CreateRelayTarget(agent domainprofile.Agent, name string, input RelayTargetInput) (TargetResult, error) {
	return s.upsertRelayTarget(agent, name, input, false)
}

func (s *ProfileService) EditRelayTarget(agent domainprofile.Agent, name string, input RelayTargetInput) (TargetResult, error) {
	return s.upsertRelayTarget(agent, name, input, true)
}

func (s *ProfileService) upsertRelayTarget(agent domainprofile.Agent, name string, input RelayTargetInput, edit bool) (TargetResult, error) {
	if !agent.Valid() {
		return TargetResult{}, &InvalidAgentError{Agent: string(agent)}
	}
	name = normalizeTargetName(name)
	if err := validateTargetName(name); err != nil {
		return TargetResult{}, err
	}
	baseURL := domainprofile.NormalizeBaseURL(input.BaseURL)
	if err := domainprofile.ValidateBaseURL(baseURL); err != nil {
		return TargetResult{}, err
	}
	if err := domainprofile.ValidateAPIKey(input.APIKey); err != nil {
		return TargetResult{}, err
	}
	providerFamily := input.ProviderFamily
	if providerFamily == "" {
		providerFamily = domainprofile.OpenCodeProviderFamilyOpenAICompatible
	}
	if !providerFamily.Valid() {
		return TargetResult{}, fmt.Errorf("provider family must be one of: %s, %s, %s", domainprofile.OpenCodeProviderFamilyOpenAICompatible, domainprofile.OpenCodeProviderFamilyAnthropic, domainprofile.OpenCodeProviderFamilyGemini)
	}
	if agent == domainprofile.AgentOpenCode {
		if err := domainprofile.ValidateOpenCodeModelID(input.ModelID); err != nil {
			return TargetResult{}, err
		}
	}

	return withMutationGuard(s,
		func(g *mutationGuard) error {
			if err := g.CaptureProfile(targetProfileName(agent, name)); err != nil {
				return err
			}
			return g.CaptureState()
		},
		func() (TargetResult, error) {
			return s.upsertRelayTargetLocked(agent, name, baseURL, providerFamily, input, edit)
		},
	)
}

func (s *ProfileService) upsertRelayTargetLocked(agent domainprofile.Agent, name, baseURL string, providerFamily domainprofile.OpenCodeProviderFamily, input RelayTargetInput, edit bool) (TargetResult, error) {
	state, err := s.loadStoredState()
	if err != nil {
		return TargetResult{}, err
	}
	managed := s.hydrateManagedAgentDefaults(agent, managedAgentState(state, agent))
	existing, exists := managed.Targets[name]
	if !edit && exists && existing.Kind != "" {
		return TargetResult{}, fmt.Errorf("%s target %s already exists", agent, name)
	}
	if edit && (!exists || existing.Kind != domainprofile.TargetKindRelay) {
		return TargetResult{}, &TargetNotFoundError{Agent: agent, Name: name}
	}

	now := time.Now().UTC()
	profileName := targetProfileName(agent, name)
	createdAt := now
	contextPath := s.managedTargetContextRoot(agent, name)
	backups := []domainprofile.ContextBackup(nil)
	configPath := ""
	if exists {
		createdAt = existing.CreatedAt
		if !createdAt.IsZero() {
			createdAt = existing.CreatedAt
		} else {
			createdAt = now
		}
		if existing.ContextPath != "" {
			contextPath = existing.ContextPath
		}
		backups = append([]domainprofile.ContextBackup(nil), existing.Backups...)
		configPath = existing.ConfigPath
	}
	if _, err := s.profiles.Upsert(domainprofile.Profile{
		Name:      profileName,
		BaseURL:   baseURL,
		APIKey:    input.APIKey,
		CreatedAt: createdAt,
		UpdatedAt: now,
	}); err != nil {
		return TargetResult{}, err
	}
	target := domainprofile.TargetState{
		Kind:        domainprofile.TargetKindRelay,
		ContextPath: contextPath,
		ConfigPath:  configPath,
		Relay: domainprofile.RelayTargetState{
			ProfileName:    profileName,
			BaseURL:        baseURL,
			ProviderFamily: providerFamily,
			ModelID:        strings.TrimSpace(input.ModelID),
			ModelName:      strings.TrimSpace(input.ModelName),
		},
		Backups:   backups,
		CreatedAt: createdAt,
		UpdatedAt: now,
	}
	managed.Targets[name] = target
	managed.UpdatedAt = now
	assignManagedAgentState(&state, agent, managed)
	state.UpdatedAt = now
	if err := s.saveState(state); err != nil {
		return TargetResult{}, err
	}
	return TargetResult{Agent: agent, Name: name, Target: target}, nil
}

func (s *ProfileService) UseTarget(agent domainprofile.Agent, name string) (TargetResult, error) {
	if !agent.Valid() {
		return TargetResult{}, &InvalidAgentError{Agent: string(agent)}
	}
	name = normalizeTargetName(name)
	if err := validateTargetName(name); err != nil {
		return TargetResult{}, err
	}

	return withMutationGuard(s,
		func(g *mutationGuard) error {
			if err := g.CaptureState(); err != nil {
				return err
			}
			return g.CaptureAgents(agent)
		},
		func() (TargetResult, error) { return s.useTargetLocked(agent, name) },
	)
}

func (s *ProfileService) useTargetLocked(agent domainprofile.Agent, name string) (TargetResult, error) {
	state, err := s.loadStoredState()
	if err != nil {
		return TargetResult{}, err
	}
	managed := s.hydrateManagedAgentDefaults(agent, managedAgentState(state, agent))
	target, ok := managed.Targets[name]
	if !ok {
		return TargetResult{}, &TargetNotFoundError{Agent: agent, Name: name}
	}
	now := time.Now().UTC()
	if target.Kind != domainprofile.TargetKindRelay {
		return TargetResult{}, fmt.Errorf("%s target %s has unsupported kind %s", agent, name, target.Kind)
	}
	if err := s.ensureManagedRuntimeReady(agent); err != nil {
		return TargetResult{}, err
	}
	profileName := target.Relay.ProfileName
	if strings.TrimSpace(profileName) == "" {
		profileName = targetProfileName(agent, name)
	}
	profile, err := s.profiles.Get(profileName)
	if err != nil {
		return TargetResult{}, err
	}
	if err := s.ensureManagedContextRoot(target.ContextPath); err != nil {
		return TargetResult{}, err
	}
	var binding *domainprofile.OpenCodeProfileBinding
	if agent == domainprofile.AgentOpenCode {
		binding = &domainprofile.OpenCodeProfileBinding{
			ProviderFamily: target.Relay.ProviderFamily,
			ModelID:        target.Relay.ModelID,
			ModelName:      target.Relay.ModelName,
		}
	}
	configPath, normalizedBinding, err := s.syncRelayContext(agent, target.ContextPath, *profile, binding)
	if err != nil {
		return TargetResult{}, err
	}
	target.ConfigPath = configPath
	target.Relay.BaseURL = profile.BaseURL
	if normalizedBinding != nil {
		target.Relay.ProviderFamily = normalizedBinding.ProviderFamily
		target.Relay.ModelID = normalizedBinding.ModelID
		target.Relay.ModelName = normalizedBinding.ModelName
	}
	target.UpdatedAt = now
	managed.Targets[name] = target
	managed.CurrentTarget = domainprofile.CurrentTarget{Kind: target.Kind, Name: name}
	managed.UpdatedAt = now
	assignManagedAgentState(&state, agent, managed)
	state.UpdatedAt = now
	if err := s.saveState(state); err != nil {
		return TargetResult{}, err
	}
	return TargetResult{Agent: agent, Name: name, Target: target}, nil
}

func (s *ProfileService) RemoveTarget(agent domainprofile.Agent, name string) (TargetResult, error) {
	if !agent.Valid() {
		return TargetResult{}, &InvalidAgentError{Agent: string(agent)}
	}
	name = normalizeTargetName(name)
	if err := validateTargetName(name); err != nil {
		return TargetResult{}, err
	}

	return withMutationGuard(s,
		func(g *mutationGuard) error {
			if err := g.CaptureProfile(targetProfileName(agent, name)); err != nil {
				return err
			}
			return g.CaptureState()
		},
		func() (TargetResult, error) { return s.removeTargetLocked(agent, name) },
	)
}

func (s *ProfileService) removeTargetLocked(agent domainprofile.Agent, name string) (TargetResult, error) {
	state, err := s.loadStoredState()
	if err != nil {
		return TargetResult{}, err
	}
	managed := s.hydrateManagedAgentDefaults(agent, managedAgentState(state, agent))
	target, ok := managed.Targets[name]
	if !ok {
		return TargetResult{}, &TargetNotFoundError{Agent: agent, Name: name}
	}
	if managed.CurrentTarget.Name == name {
		return TargetResult{}, fmt.Errorf("%s target %s is current; switch targets before removing it", agent, name)
	}
	delete(managed.Targets, name)
	managed.UpdatedAt = time.Now().UTC()
	assignManagedAgentState(&state, agent, managed)
	state.UpdatedAt = managed.UpdatedAt
	if err := s.saveState(state); err != nil {
		return TargetResult{}, err
	}
	if target.Kind == domainprofile.TargetKindRelay && strings.TrimSpace(target.Relay.ProfileName) != "" {
		_ = s.profiles.Delete(target.Relay.ProfileName)
	}
	if strings.TrimSpace(target.ContextPath) != "" {
		_ = os.RemoveAll(target.ContextPath)
	}
	return TargetResult{Agent: agent, Name: name, Target: target}, nil
}
