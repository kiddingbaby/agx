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
	profile = domainkey.NormalizeProfileName(profile)
	if strings.TrimSpace(identifier) == "" {
		return s.repo.Resolve(provider, profile, "")
	}

	matched, err := s.FindByIdentifierInScope(provider, profile, identifier)
	if err != nil {
		return nil, err
	}
	return s.repo.Resolve(provider, profile, matched.ID)
}

func (s *KeyService) SetProfileStrategy(provider domainkey.Provider, profile string, strategy domainkey.RotationStrategy, fixedKey string) error {
	return s.repo.SetProfileStrategy(provider, domainkey.NormalizeProfileName(profile), strategy, fixedKey)
}

func (s *KeyService) FindByIdentifier(identifier string) (*domainkey.Key, error) {
	return findKeyByIdentifier(s.repo.List(), identifier)
}

func (s *KeyService) FindByIdentifierInScope(provider domainkey.Provider, profile, identifier string) (*domainkey.Key, error) {
	keys := s.repo.List()
	profile = domainkey.NormalizeProfileName(profile)

	filtered := make([]domainkey.Key, 0, len(keys))
	for i := range keys {
		if keys[i].Provider != provider {
			continue
		}
		if domainkey.NormalizeProfileName(keys[i].Profile) != profile {
			continue
		}
		filtered = append(filtered, keys[i])
	}
	return findKeyByIdentifier(filtered, identifier)
}

func findKeyByIdentifier(keys []domainkey.Key, identifier string) (*domainkey.Key, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, &KeyNotFoundError{}
	}

	var exactMatch *domainkey.Key
	for i := range keys {
		if keys[i].Name != identifier {
			continue
		}
		if exactMatch != nil {
			return nil, &AmbiguousKeyIdentifierError{Identifier: identifier}
		}
		exactMatch = &keys[i]
	}
	if exactMatch != nil {
		return exactMatch, nil
	}

	var prefixMatch *domainkey.Key
	for i := range keys {
		if !strings.HasPrefix(keys[i].ID, identifier) {
			continue
		}
		if prefixMatch != nil {
			return nil, &AmbiguousKeyIdentifierError{Identifier: identifier}
		}
		prefixMatch = &keys[i]
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
