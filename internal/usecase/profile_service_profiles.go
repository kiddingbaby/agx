package usecase

import (
	"errors"
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

	guard := newMutationGuard(s)
	if err := guard.CaptureProfile(name); err != nil {
		return nil, err
	}
	if err := guard.CaptureState(); err != nil {
		return nil, err
	}
	if err := guard.CaptureAgents(input.Bind...); err != nil {
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

	now := time.Now().UTC()
	profile := domainprofile.Profile{
		Name:      name,
		BaseURL:   domainprofile.NormalizeBaseURL(input.BaseURL),
		APIKey:    strings.TrimSpace(input.APIKey),
		CreatedAt: now,
		UpdatedAt: now,
	}
	saved, err := s.saveProfile(profile, true)
	if err != nil {
		return nil, err
	}
	if err := s.syncProfileAfterMutation(*saved, false); err != nil {
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
	needsSync := baseURLChanged || apiKeyChanged
	needsBindingChanges := len(input.Bind) > 0 || len(input.Unbind) > 0
	if !needsSync && !needsBindingChanges {
		return &EditProfileResult{Relay: profile}, nil
	}

	guard := newMutationGuard(s)
	if err := guard.CaptureProfile(updated.Name); err != nil {
		return nil, err
	}
	if err := guard.CaptureState(); err != nil {
		return nil, err
	}
	if err := guard.CaptureAgents(affectedAgentsForProfileMutation(updated.Name, *guard.stateBefore, needsSync, input.Bind, input.Unbind)...); err != nil {
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

	if needsSync {
		updated.UpdatedAt = time.Now().UTC()

		saved, err := s.saveProfile(updated, false)
		if err != nil {
			return nil, err
		}
		updated = *saved
		if err := s.syncProfileAfterMutation(updated, true); err != nil {
			return nil, err
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

func (s *ProfileService) Remove(name string) (profile *domainprofile.Profile, err error) {
	unlock, err := s.lockMutations()
	if err != nil {
		return nil, err
	}
	defer unlock()

	profile, err = s.Get(name)
	if err != nil {
		return nil, err
	}

	guard := newMutationGuard(s)
	if err := guard.CaptureProfile(profile.Name); err != nil {
		return nil, err
	}
	if err := guard.CaptureState(); err != nil {
		return nil, err
	}
	if err := guard.CaptureAgents(domainprofile.AgentCodex); err != nil {
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
