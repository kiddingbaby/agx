package usecase

import (
	"strings"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	"github.com/kiddingbaby/agx/internal/ports"
)

// KeyService encapsulates key-related use cases.
type KeyService struct {
	repo ports.KeyRepository
}

func NewKeyService(repo ports.KeyRepository) *KeyService {
	return &KeyService{repo: repo}
}

func (s *KeyService) List() []domainkey.Key {
	return s.repo.List()
}

func (s *KeyService) ListProfiles(provider domainkey.Provider) []domainkey.Profile {
	return s.repo.ListProfiles(provider)
}

func (s *KeyService) HasActive(provider domainkey.Provider, profile string) bool {
	return s.repo.HasActive(provider, domainkey.NormalizeProfileName(profile))
}

func (s *KeyService) GetActive(provider domainkey.Provider, profile string) (*domainkey.Key, error) {
	return s.repo.GetActive(provider, domainkey.NormalizeProfileName(profile))
}

func (s *KeyService) Add(provider domainkey.Provider, profile, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	return s.repo.Add(provider, domainkey.NormalizeProfileName(profile), name, apiKey, baseURL, tags)
}

func (s *KeyService) Update(id string, provider domainkey.Provider, profile, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	return s.repo.Update(id, provider, domainkey.NormalizeProfileName(profile), name, apiKey, baseURL, tags)
}

func (s *KeyService) Delete(id string) error {
	return s.repo.Delete(id)
}

func (s *KeyService) Activate(id string) error {
	return s.repo.Activate(id)
}

func (s *KeyService) Resolve(provider domainkey.Provider, profile, identifier string) (*domainkey.Key, error) {
	return s.repo.Resolve(provider, domainkey.NormalizeProfileName(profile), identifier)
}

func (s *KeyService) SetProfileStrategy(provider domainkey.Provider, profile string, strategy domainkey.RotationStrategy, fixedKey string) error {
	return s.repo.SetProfileStrategy(provider, domainkey.NormalizeProfileName(profile), strategy, fixedKey)
}

func (s *KeyService) FindByIdentifier(identifier string) (*domainkey.Key, error) {
	keys := s.repo.List()
	var prefixMatch *domainkey.Key
	for i := range keys {
		if keys[i].Name == identifier {
			return &keys[i], nil
		}
		if prefixMatch == nil && strings.HasPrefix(keys[i].ID, identifier) {
			prefixMatch = &keys[i]
		}
	}
	if prefixMatch != nil {
		return prefixMatch, nil
	}
	return nil, &KeyNotFoundError{Identifier: identifier}
}

func (s *KeyService) FindByIdentifierInScope(provider domainkey.Provider, profile, identifier string) (*domainkey.Key, error) {
	keys := s.repo.List()
	profile = domainkey.NormalizeProfileName(profile)

	var prefixMatch *domainkey.Key
	for i := range keys {
		if keys[i].Provider != provider {
			continue
		}
		if domainkey.NormalizeProfileName(keys[i].Profile) != profile {
			continue
		}
		if keys[i].Name == identifier {
			return &keys[i], nil
		}
		if prefixMatch == nil && strings.HasPrefix(keys[i].ID, identifier) {
			prefixMatch = &keys[i]
		}
	}
	if prefixMatch != nil {
		return prefixMatch, nil
	}
	return nil, &KeyNotFoundError{Identifier: identifier}
}

func (s *KeyService) ActivateByIdentifier(identifier string) (*domainkey.Key, error) {
	k, err := s.FindByIdentifier(identifier)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Activate(k.ID); err != nil {
		return nil, err
	}
	return k, nil
}

func (s *KeyService) DeleteByIdentifier(identifier string) (*domainkey.Key, error) {
	k, err := s.FindByIdentifier(identifier)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(k.ID); err != nil {
		return nil, err
	}
	return k, nil
}

func (s *KeyService) ActivateByIdentifierInScope(provider domainkey.Provider, profile, identifier string) (*domainkey.Key, error) {
	k, err := s.FindByIdentifierInScope(provider, profile, identifier)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Activate(k.ID); err != nil {
		return nil, err
	}
	return k, nil
}

func (s *KeyService) DeleteByIdentifierInScope(provider domainkey.Provider, profile, identifier string) (*domainkey.Key, error) {
	k, err := s.FindByIdentifierInScope(provider, profile, identifier)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(k.ID); err != nil {
		return nil, err
	}
	return k, nil
}
