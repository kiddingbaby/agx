package usecase

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

type UpdateManagedProfileInput struct {
	Name           *string
	Kind           *domainprofile.ProfileKind
	BaseURL        *string
	APIKey         *string
	ModelID        *string
	CodexWireAPI   *domainprofile.CodexWireAPI
	ProviderFamily *domainprofile.OpenCodeProviderFamily
}

// ManagedProfileSummary pairs a profile with the agents that currently
// reference it through managed targets or native config bindings.
type ManagedProfileSummary struct {
	Profile     domainprofile.Profile
	BoundAgents []domainprofile.Agent
}

func (s *ProfileService) ListManagedProfiles() ([]ManagedProfileSummary, string, error) {
	profiles, err := s.profiles.List()
	if err != nil {
		return nil, "", err
	}
	state, err := s.loadStoredState()
	if err != nil {
		return nil, "", err
	}
	filtered := make([]ManagedProfileSummary, 0, len(profiles))
	for _, profile := range profiles {
		if isReservedManagedProfileName(profile.Name) {
			continue
		}
		filtered = append(filtered, ManagedProfileSummary{
			Profile:     profile,
			BoundAgents: boundAgents(profile.Name, state),
		})
	}
	return filtered, domainprofile.NormalizeProfileName(state.CurrentProfile), nil
}

func (s *ProfileService) ListAllProfiles() ([]ManagedProfileSummary, string, error) {
	profiles, err := s.profiles.List()
	if err != nil {
		return nil, "", err
	}
	state, err := s.loadStoredState()
	if err != nil {
		return nil, "", err
	}
	summaries := make([]ManagedProfileSummary, 0, len(profiles))
	for _, profile := range profiles {
		summaries = append(summaries, ManagedProfileSummary{
			Profile:     profile,
			BoundAgents: boundAgents(profile.Name, state),
		})
	}
	return summaries, domainprofile.NormalizeProfileName(state.CurrentProfile), nil
}

func (s *ProfileService) ManagedProfile(name string) (*domainprofile.Profile, error) {
	state, err := s.loadStoredState()
	if err != nil {
		return nil, err
	}
	name = domainprofile.NormalizeProfileName(name)
	if isReservedManagedProfileName(name) {
		return nil, &domainprofile.NotFoundError{Name: name}
	}
	if _, ok := referencedDerivedProfiles(state)[name]; ok {
		return nil, &domainprofile.NotFoundError{Name: name}
	}
	return s.Get(name)
}

func (s *ProfileService) CurrentManagedProfile() (*domainprofile.Profile, error) {
	state, err := s.loadStoredState()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(state.CurrentProfile) == "" {
		return nil, nil
	}
	current := domainprofile.NormalizeProfileName(state.CurrentProfile)
	if isReservedManagedProfileName(current) {
		return nil, nil
	}
	if _, ok := referencedDerivedProfiles(state)[current]; ok {
		return nil, nil
	}
	return s.Get(current)
}

func (s *ProfileService) AddManagedProfile(name string, profile domainprofile.Profile) (*domainprofile.Profile, error) {
	name = domainprofile.NormalizeProfileName(name)
	if err := domainprofile.ValidateProfileName(name); err != nil {
		return nil, err
	}
	if isReservedManagedProfileName(name) {
		return nil, fmt.Errorf("profile name %s is reserved for internal managed targets", name)
	}
	return withMutationGuard(s,
		func(g *mutationGuard) error {
			if err := g.CaptureProfile(name); err != nil {
				return err
			}
			return g.captureDerivedProfilesFor(name)
		},
		func() (*domainprofile.Profile, error) {
			if _, err := s.profiles.Get(name); err == nil {
				return nil, &ProfileAlreadyExistsError{Name: name}
			} else if !IsProfileNotFoundError(err) {
				return nil, err
			}
			now := time.Now().UTC()
			profile = domainprofile.NormalizeProfile(profile)
			profile.Name = name
			profile.CreatedAt = now
			profile.UpdatedAt = now
			saved, err := s.profiles.Upsert(profile)
			if err != nil {
				return nil, err
			}
			return saved, nil
		},
	)
}

func (s *ProfileService) EditManagedProfile(name string, input UpdateManagedProfileInput) (result *EditManagedProfileResult, err error) {
	name = domainprofile.NormalizeProfileName(name)
	current, err := s.ManagedProfile(name)
	if err != nil {
		return nil, err
	}
	next := domainprofile.NormalizeProfile(*current)
	nextName := current.Name
	nameChanged := false
	if input.Name != nil {
		nextName = domainprofile.NormalizeProfileName(*input.Name)
		if err = domainprofile.ValidateProfileName(nextName); err != nil {
			return nil, err
		}
		if isReservedManagedProfileName(nextName) {
			return nil, fmt.Errorf("profile name %s is reserved for internal managed targets", nextName)
		}
		if nextName != current.Name {
			if _, lookupErr := s.profiles.Get(nextName); lookupErr == nil {
				return nil, &ProfileAlreadyExistsError{Name: nextName}
			} else if !IsProfileNotFoundError(lookupErr) {
				return nil, lookupErr
			}
			nameChanged = true
		}
	}
	if input.Kind != nil {
		next.Kind = (*input.Kind).Normalized()
	}
	if input.BaseURL != nil {
		next.BaseURL = *input.BaseURL
	}
	if input.APIKey != nil {
		next.APIKey = *input.APIKey
	}
	if input.ModelID != nil {
		next.ModelID = *input.ModelID
	}
	if input.CodexWireAPI != nil {
		next.CodexWireAPI = *input.CodexWireAPI
	}
	if input.ProviderFamily != nil {
		next.ProviderFamily = *input.ProviderFamily
	}
	next.UpdatedAt = time.Now().UTC()
	next.Name = nextName

	unlock, err := s.lockMutations()
	if err != nil {
		return nil, err
	}
	defer unlock()

	state, err := s.loadStoredState()
	if err != nil {
		return nil, err
	}
	guard := newMutationGuard(s)
	if err = guard.CaptureProfile(current.Name); err != nil {
		return nil, err
	}
	if err = guard.captureDerivedProfilesFor(current.Name); err != nil {
		return nil, err
	}
	if nameChanged {
		if err = guard.CaptureProfile(next.Name); err != nil {
			return nil, err
		}
		if err = guard.captureDerivedProfilesFor(next.Name); err != nil {
			return nil, err
		}
	}
	if err = guard.CaptureState(); err != nil {
		return nil, err
	}
	affectedAgents := managedAgentsForProfileRename(current.Name, state)
	if err = guard.CaptureAgents(affectedAgents...); err != nil {
		return nil, err
	}
	defer finishMutationGuard(guard, &err)

	if !nameChanged {
		var saved *domainprofile.Profile
		saved, err = s.profiles.Upsert(next)
		if err != nil {
			return nil, err
		}
		var resynced []TargetSyncOutcome
		var failed []TargetSyncFailure
		resynced, failed, err = s.resyncManagedTargetsForProfileLocked(*saved)
		if err != nil {
			return nil, err
		}
		return &EditManagedProfileResult{Profile: saved, ResyncedTargets: resynced, FailedTargets: failed}, nil
	}

	now := time.Now().UTC()
	next.UpdatedAt = now
	saved, renameErr := s.renameProfile(current.Name, next, state, now)
	if renameErr != nil {
		err = renameErr
		return nil, err
	}
	if err = s.renameManagedProfileTargets(&state, current.Name, *saved, now); err != nil {
		return nil, err
	}
	if domainprofile.NormalizeProfileName(state.CurrentProfile) == current.Name {
		state.CurrentProfile = saved.Name
	}
	state.UpdatedAt = now
	if err = s.saveState(state); err != nil {
		return nil, err
	}
	return &EditManagedProfileResult{Profile: saved}, nil
}

func (s *ProfileService) resyncManagedTargetsForProfileLocked(profile domainprofile.Profile) ([]TargetSyncOutcome, []TargetSyncFailure, error) {
	if profile.Kind != domainprofile.ProfileKindRelay {
		return nil, nil, nil
	}
	profileName := strings.TrimSpace(profile.Name)
	if profileName == "" {
		return nil, nil, nil
	}

	state, err := s.loadStoredState()
	if err != nil {
		return nil, nil, err
	}

	resynced := make([]TargetSyncOutcome, 0)
	failed := make([]TargetSyncFailure, 0)
	now := time.Now().UTC()
	dirty := false

	for _, agent := range domainprofile.SupportedAgents() {
		managed, ok := state.ManagedAgents[agent]
		if !ok || managed.Targets == nil {
			continue
		}
		target, ok := managed.Targets[profileName]
		if !ok || target.Kind != domainprofile.TargetKindRelay {
			continue
		}

		newTarget, syncErr := s.resyncOneRelayTarget(agent, profileName, target, profile, now)
		if syncErr != nil {
			failed = append(failed, TargetSyncFailure{Agent: agent, TargetName: profileName, Error: syncErr.Error()})
			continue
		}
		managed.Targets[profileName] = newTarget
		managed.UpdatedAt = now
		state.ManagedAgents[agent] = managed
		dirty = true
		resynced = append(resynced, TargetSyncOutcome{Agent: agent, TargetName: profileName, ConfigPath: newTarget.ConfigPath})
	}

	if !dirty {
		return resynced, failed, nil
	}
	state.UpdatedAt = now
	if err := s.saveState(state); err != nil {
		return resynced, failed, err
	}
	return resynced, failed, nil
}

func (s *ProfileService) resyncOneRelayTarget(agent domainprofile.Agent, profileName string, target domainprofile.TargetState, profile domainprofile.Profile, now time.Time) (domainprofile.TargetState, error) {
	apiKey, err := domainprofile.ResolveCredential(profile)
	if err != nil {
		return target, err
	}
	if err := s.ensureManagedRuntimeReady(agent); err != nil {
		return target, err
	}
	contextPath := target.ContextPath
	if strings.TrimSpace(contextPath) == "" {
		contextPath = s.managedTargetContextRoot(agent, profileName)
	}
	if err := s.ensureManagedContextRoot(contextPath); err != nil {
		return target, err
	}
	derivedName := targetProfileName(agent, profileName)
	createdAt := target.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	derivedProfile := domainprofile.Profile{
		Name:           derivedName,
		Kind:           domainprofile.ProfileKindRelay,
		BaseURL:        profile.BaseURL,
		APIKey:         apiKey,
		ModelID:        profile.ModelID,
		CodexWireAPI:   profile.CodexWireAPI,
		ProviderFamily: profile.ProviderFamily,
		CreatedAt:      createdAt,
		UpdatedAt:      now,
	}
	if _, err := s.profiles.Upsert(derivedProfile); err != nil {
		return target, err
	}
	var binding *domainprofile.OpenCodeProfileBinding
	if agent == domainprofile.AgentOpenCode {
		binding = &domainprofile.OpenCodeProfileBinding{
			ProviderFamily: profile.ProviderFamily,
			ModelID:        profile.ModelID,
			ModelName:      profile.ModelID,
		}
	}
	configPath, normalizedBinding, err := s.syncRelayContext(agent, contextPath, derivedProfile, binding)
	if err != nil {
		return target, err
	}
	providerFamily := profile.ProviderFamily
	modelID := strings.TrimSpace(profile.ModelID)
	modelName := modelID
	if normalizedBinding != nil {
		providerFamily = normalizedBinding.ProviderFamily
		modelID = normalizedBinding.ModelID
		modelName = normalizedBinding.ModelName
	} else if providerFamily == "" {
		providerFamily = domainprofile.OpenCodeProviderFamilyOpenAICompatible
	}
	next := target
	next.ContextPath = contextPath
	next.ConfigPath = configPath
	next.UpdatedAt = now
	next.Relay = domainprofile.RelayTargetState{
		ProfileName:    derivedName,
		BaseURL:        profile.BaseURL,
		ProviderFamily: providerFamily,
		ModelID:        modelID,
		ModelName:      modelName,
	}
	return next, nil
}

func (s *ProfileService) RemoveManagedProfile(name string) (*domainprofile.Profile, error) {
	name = domainprofile.NormalizeProfileName(name)
	profile, err := s.ManagedProfile(name)
	if err != nil {
		return nil, err
	}

	return withMutationGuard(s,
		func(g *mutationGuard) error {
			if err := g.CaptureProfile(name); err != nil {
				return err
			}
			if err := g.captureDerivedProfilesFor(name); err != nil {
				return err
			}
			if err := g.CaptureState(); err != nil {
				return err
			}
			return g.CaptureAgents(domainprofile.SupportedAgents()...)
		},
		func() (*domainprofile.Profile, error) {
			return s.removeManagedProfileLocked(name, profile)
		},
	)
}

func (s *ProfileService) removeManagedProfileLocked(name string, profile *domainprofile.Profile) (*domainprofile.Profile, error) {
	state, err := s.loadStoredState()
	if err != nil {
		return nil, err
	}
	if domainprofile.NormalizeProfileName(state.CurrentProfile) == name {
		return nil, fmt.Errorf("profile %s is current; switch profiles before removing it", name)
	}

	for _, agent := range domainprofile.SupportedAgents() {
		managed := s.hydrateManagedAgentDefaults(agent, managedAgentState(state, agent))
		target, ok := managed.Targets[name]
		if !ok {
			continue
		}
		delete(managed.Targets, name)
		if managed.CurrentTarget.Name == name {
			managed.CurrentTarget = domainprofile.CurrentTarget{}
		}
		managed.UpdatedAt = time.Now().UTC()
		assignManagedAgentState(&state, agent, managed)
		if strings.TrimSpace(target.ContextPath) != "" {
			_ = os.RemoveAll(target.ContextPath)
		}
		if strings.TrimSpace(target.Relay.ProfileName) != "" {
			_ = s.profiles.Delete(target.Relay.ProfileName)
		}
	}
	state.UpdatedAt = time.Now().UTC()
	if err := s.saveState(state); err != nil {
		return nil, err
	}
	if err := s.profiles.Delete(name); err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *ProfileService) UseManagedProfile(name string) (*domainprofile.Profile, error) {
	name = domainprofile.NormalizeProfileName(name)
	profile, err := s.ManagedProfile(name)
	if err != nil {
		return nil, err
	}
	return withMutationGuard(s,
		func(g *mutationGuard) error { return g.CaptureState() },
		func() (*domainprofile.Profile, error) {
			state, err := s.loadStoredState()
			if err != nil {
				return nil, err
			}
			state.CurrentProfile = name
			state.UpdatedAt = time.Now().UTC()
			if err := s.saveState(state); err != nil {
				return nil, err
			}
			return profile, nil
		},
	)
}

func (s *ProfileService) ActivateManagedProfile(agent domainprofile.Agent, name string) (TargetResult, error) {
	profileValue, err := s.validateManagedActivation(agent, name)
	if err != nil {
		return TargetResult{}, err
	}

	return withMutationGuard(s,
		func(g *mutationGuard) error {
			if err := g.captureDerivedProfilesFor(profileValue.Name); err != nil {
				return err
			}
			if err := g.CaptureState(); err != nil {
				return err
			}
			return g.CaptureAgents(agent)
		},
		func() (TargetResult, error) {
			return s.activateManagedProfileLocked(agent, profileValue)
		},
	)
}

func (s *ProfileService) activateManagedProfileLocked(agent domainprofile.Agent, profileValue domainprofile.Profile) (TargetResult, error) {
	state, err := s.loadStoredState()
	if err != nil {
		return TargetResult{}, err
	}
	managed := s.hydrateManagedAgentDefaults(agent, managedAgentState(state, agent))

	now := time.Now().UTC()
	target, contextPath := s.prepareManagedTarget(agent, profileValue, managed, now)

	if err := s.ensureManagedContextReady(agent, contextPath); err != nil {
		return TargetResult{}, err
	}
	target, err = s.syncManagedRelayTarget(agent, profileValue, target, contextPath, now)
	if err != nil {
		return TargetResult{}, err
	}
	return s.recordManagedActivation(agent, profileValue.Name, target, managed, &state, now)
}

func (s *ProfileService) validateManagedActivation(agent domainprofile.Agent, name string) (domainprofile.Profile, error) {
	if !agent.Valid() {
		return domainprofile.Profile{}, &InvalidAgentError{Agent: string(agent)}
	}
	profile, err := s.ManagedProfile(name)
	if err != nil {
		return domainprofile.Profile{}, err
	}
	normalized := domainprofile.NormalizeProfile(*profile)
	if normalized.Kind != domainprofile.ProfileKindRelay {
		return domainprofile.Profile{}, fmt.Errorf("profile kind must be %s", domainprofile.ProfileKindRelay)
	}
	return normalized, nil
}

// prepareManagedTarget seeds the TargetState we'll fill in: it carries
// over existing context/config paths and backups when the profile has
// been activated before, and wipes a stale non-relay context root if the
// previous target kind doesn't match.
func (s *ProfileService) prepareManagedTarget(
	agent domainprofile.Agent,
	profile domainprofile.Profile,
	managed domainprofile.ManagedAgentState,
	now time.Time,
) (domainprofile.TargetState, string) {
	existing, exists := managed.Targets[profile.Name]
	contextPath := s.managedTargetContextRoot(agent, profile.Name)
	configPath := ""
	createdAt := now
	var backups []domainprofile.ContextBackup
	if exists {
		if existing.ContextPath != "" {
			contextPath = existing.ContextPath
		}
		configPath = existing.ConfigPath
		if !existing.CreatedAt.IsZero() {
			createdAt = existing.CreatedAt
		}
		backups = cloneContextBackups(existing.Backups)
	}

	target := domainprofile.TargetState{
		ContextPath: contextPath,
		ConfigPath:  configPath,
		Backups:     backups,
		CreatedAt:   createdAt,
		UpdatedAt:   now,
	}
	if exists && existing.Kind != "" && existing.Kind != domainprofile.TargetKindRelay && strings.TrimSpace(contextPath) != "" {
		_ = os.RemoveAll(contextPath)
		target.ConfigPath = ""
	}
	return target, contextPath
}

func (s *ProfileService) ensureManagedContextReady(agent domainprofile.Agent, contextPath string) error {
	if err := s.ensureManagedRuntimeReady(agent); err != nil {
		return err
	}
	return s.ensureManagedContextRoot(contextPath)
}

// syncManagedRelayTarget upserts the derived profile that backs this
// managed target, asks the agent-specific syncer to materialize it
// inside the per-target context root, then fills in the relay metadata
// the caller needs to persist.
func (s *ProfileService) syncManagedRelayTarget(
	agent domainprofile.Agent,
	profile domainprofile.Profile,
	target domainprofile.TargetState,
	contextPath string,
	now time.Time,
) (domainprofile.TargetState, error) {
	apiKey, err := domainprofile.ResolveCredential(profile)
	if err != nil {
		return domainprofile.TargetState{}, err
	}
	derivedName := targetProfileName(agent, profile.Name)
	derivedProfile := domainprofile.Profile{
		Name:           derivedName,
		Kind:           domainprofile.ProfileKindRelay,
		BaseURL:        profile.BaseURL,
		APIKey:         apiKey,
		ModelID:        profile.ModelID,
		ProviderFamily: profile.ProviderFamily,
		CreatedAt:      target.CreatedAt,
		UpdatedAt:      now,
	}
	if _, err := s.profiles.Upsert(derivedProfile); err != nil {
		return domainprofile.TargetState{}, err
	}
	var binding *domainprofile.OpenCodeProfileBinding
	if agent == domainprofile.AgentOpenCode {
		binding = &domainprofile.OpenCodeProfileBinding{
			ProviderFamily: profile.ProviderFamily,
			ModelID:        profile.ModelID,
			ModelName:      profile.ModelID,
		}
	}
	configPath, normalizedBinding, err := s.syncRelayContext(agent, contextPath, derivedProfile, binding)
	if err != nil {
		return domainprofile.TargetState{}, err
	}
	providerFamily := profile.ProviderFamily
	modelID := strings.TrimSpace(profile.ModelID)
	modelName := modelID
	if normalizedBinding != nil {
		providerFamily = normalizedBinding.ProviderFamily
		modelID = normalizedBinding.ModelID
		modelName = normalizedBinding.ModelName
	} else if providerFamily == "" {
		providerFamily = domainprofile.OpenCodeProviderFamilyOpenAICompatible
	}
	target.Kind = domainprofile.TargetKindRelay
	target.ConfigPath = configPath
	target.Relay = domainprofile.RelayTargetState{
		ProfileName:    derivedName,
		BaseURL:        profile.BaseURL,
		ProviderFamily: providerFamily,
		ModelID:        modelID,
		ModelName:      modelName,
	}
	return target, nil
}

func (s *ProfileService) recordManagedActivation(
	agent domainprofile.Agent,
	profileName string,
	target domainprofile.TargetState,
	managed domainprofile.ManagedAgentState,
	state *domainprofile.State,
	now time.Time,
) (TargetResult, error) {
	managed.Targets[profileName] = target
	managed.CurrentTarget = domainprofile.CurrentTarget{Kind: target.Kind, Name: profileName}
	managed.UpdatedAt = now
	assignManagedAgentState(state, agent, managed)
	state.UpdatedAt = now
	if err := s.saveState(*state); err != nil {
		return TargetResult{}, err
	}
	return TargetResult{Agent: agent, Name: profileName, Target: target}, nil
}

func referencedDerivedProfiles(state domainprofile.State) map[string]struct{} {
	out := make(map[string]struct{})
	for _, managed := range state.ManagedAgents {
		for _, target := range managed.Targets {
			if target.Kind != domainprofile.TargetKindRelay {
				continue
			}
			name := domainprofile.NormalizeProfileName(target.Relay.ProfileName)
			if name == "" || !isReservedManagedProfileName(name) {
				continue
			}
			out[name] = struct{}{}
		}
	}
	return out
}

func managedAgentsForProfileRename(name string, state domainprofile.State) []domainprofile.Agent {
	name = domainprofile.NormalizeProfileName(name)
	if name == "" {
		return nil
	}
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
	for agent, managed := range state.ManagedAgents {
		if _, ok := managed.Targets[name]; !ok {
			continue
		}
		add(agent)
	}
	// renameProfile also touches every agent that has the old name as its
	// native binding (Codex/Claude/Gemini/OpenCode). Without capturing those
	// agent configs the mutationGuard cannot roll back a half-applied
	// rename that already rewrote ~/.codex/config.toml or
	// ~/.config/opencode/opencode.json before a later step failed.
	for _, agent := range currentAgents(name, state) {
		add(agent)
	}
	if isCodexProfileTracked(state, name) {
		add(domainprofile.AgentCodex)
	}
	if isOpenCodeProfileTracked(state, name) {
		add(domainprofile.AgentOpenCode)
	}
	return agents
}

func (s *ProfileService) renameManagedProfileTargets(state *domainprofile.State, oldName string, profile domainprofile.Profile, now time.Time) (err error) {
	if state == nil {
		return nil
	}
	oldName = domainprofile.NormalizeProfileName(oldName)
	profile = domainprofile.NormalizeProfile(profile)
	newName := profile.Name
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}
	// Track every directory rename we apply so we can undo them if any later
	// step (Upsert, syncRelayContext, …) fails. The outer mutationGuard
	// captures profile + state + native agent config, but **not** the
	// per-target context directory tree; without this rollback, a half-way
	// failure would leave the dir at newPath while state/profile have been
	// restored to oldName, and the next `agx use <oldName>` would silently
	// recreate an empty dir at oldPath — losing the user's accumulated
	// context for that target.
	type renameRecord struct {
		oldPath string
		newPath string
	}
	var renames []renameRecord
	defer func() {
		if err == nil {
			return
		}
		// Undo renames in reverse order: best-effort, errors are aggregated
		// into the returned err so the caller (mutationGuard) still sees a
		// consistent failure.
		var undoErrs []error
		for i := len(renames) - 1; i >= 0; i-- {
			r := renames[i]
			if undoErr := moveManagedTargetContext(r.newPath, r.oldPath); undoErr != nil {
				undoErrs = append(undoErrs, undoErr)
			}
		}
		if len(undoErrs) > 0 {
			err = errors.Join(append([]error{err}, undoErrs...)...)
		}
	}()
	for agent, managed := range state.ManagedAgents {
		managed.Targets = ensureTargets(managed.Targets)
		target, ok := managed.Targets[oldName]
		if ok {
			delete(managed.Targets, oldName)
			if strings.TrimSpace(target.ContextPath) != "" {
				nextContextPath := s.managedTargetContextRoot(agent, newName)
				if err = moveManagedTargetContext(target.ContextPath, nextContextPath); err != nil {
					return err
				}
				renames = append(renames, renameRecord{oldPath: target.ContextPath, newPath: nextContextPath})
				target.ContextPath = nextContextPath
				target.ConfigPath = managedContextConfigPath(agent, nextContextPath)
			}
			if target.Kind == domainprofile.TargetKindRelay {
				if derived := domainprofile.NormalizeProfileName(target.Relay.ProfileName); derived != "" {
					if err = s.profiles.Delete(derived); err != nil && !IsProfileNotFoundError(err) {
						return err
					}
					err = nil
				}
				apiKey, credErr := domainprofile.ResolveCredential(profile)
				if credErr != nil {
					err = credErr
					return err
				}
				if err = s.ensureManagedRuntimeReady(agent); err != nil {
					return err
				}
				if err = s.ensureManagedContextRoot(target.ContextPath); err != nil {
					return err
				}
				derivedName := targetProfileName(agent, newName)
				derivedProfile := domainprofile.Profile{
					Name:           derivedName,
					Kind:           domainprofile.ProfileKindRelay,
					BaseURL:        profile.BaseURL,
					APIKey:         apiKey,
					ModelID:        profile.ModelID,
					ProviderFamily: profile.ProviderFamily,
					CreatedAt:      target.CreatedAt,
					UpdatedAt:      now,
				}
				if derivedProfile.CreatedAt.IsZero() {
					derivedProfile.CreatedAt = now
				}
				if _, err = s.profiles.Upsert(derivedProfile); err != nil {
					return err
				}
				var binding *domainprofile.OpenCodeProfileBinding
				if agent == domainprofile.AgentOpenCode {
					binding = &domainprofile.OpenCodeProfileBinding{
						ProviderFamily: target.Relay.ProviderFamily,
						ModelID:        target.Relay.ModelID,
						ModelName:      target.Relay.ModelName,
					}
				}
				configPath, normalizedBinding, syncErr := s.syncRelayContext(agent, target.ContextPath, derivedProfile, binding)
				if syncErr != nil {
					err = syncErr
					return err
				}
				providerFamily := profile.ProviderFamily
				modelID := strings.TrimSpace(profile.ModelID)
				modelName := modelID
				if normalizedBinding != nil {
					providerFamily = normalizedBinding.ProviderFamily
					modelID = normalizedBinding.ModelID
					modelName = normalizedBinding.ModelName
				} else if providerFamily == "" {
					providerFamily = domainprofile.OpenCodeProviderFamilyOpenAICompatible
				}
				target.ConfigPath = configPath
				target.Relay = domainprofile.RelayTargetState{
					ProfileName:    derivedName,
					BaseURL:        profile.BaseURL,
					ProviderFamily: providerFamily,
					ModelID:        modelID,
					ModelName:      modelName,
				}
			}
			target.UpdatedAt = now
			managed.Targets[newName] = target
		}
		if managed.CurrentTarget.Name == oldName {
			managed.CurrentTarget.Name = newName
		}
		managed.UpdatedAt = now
		state.ManagedAgents[agent] = managed
	}
	return nil
}

func moveManagedTargetContext(oldPath, newPath string) error {
	oldPath = strings.TrimSpace(oldPath)
	newPath = strings.TrimSpace(newPath)
	if oldPath == "" || newPath == "" || oldPath == newPath {
		return nil
	}
	info, err := os.Stat(oldPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o700); err != nil {
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}
	return nil
}

func isReservedManagedProfileName(name string) bool {
	_, _, ok := parseDerivedProfileName(name)
	return ok
}

// ParseDerivedProfileName splits a derived profile name like "codex.tmp"
// into ("codex", "tmp", true). Returns ok=false for user profile names.
func ParseDerivedProfileName(name string) (domainprofile.Agent, string, bool) {
	return parseDerivedProfileName(name)
}

func parseDerivedProfileName(name string) (domainprofile.Agent, string, bool) {
	name = domainprofile.NormalizeProfileName(name)
	if strings.TrimSpace(name) == "" {
		return "", "", false
	}
	for _, agent := range domainprofile.SupportedAgents() {
		prefix := string(agent) + "."
		if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
			return agent, name[len(prefix):], true
		}
	}
	return "", "", false
}
