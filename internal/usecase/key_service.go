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

func (s *KeyService) HasActive(provider domainkey.Provider) bool {
	return s.repo.HasActive(provider)
}

func (s *KeyService) GetActive(provider domainkey.Provider) (*domainkey.Key, error) {
	return s.repo.GetActive(provider)
}

func (s *KeyService) Add(provider domainkey.Provider, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	return s.repo.Add(provider, name, apiKey, baseURL, tags)
}

func (s *KeyService) Update(id string, provider domainkey.Provider, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	return s.repo.Update(id, provider, name, apiKey, baseURL, tags)
}

func (s *KeyService) Delete(id string) error {
	return s.repo.Delete(id)
}

func (s *KeyService) Activate(id string) error {
	return s.repo.Activate(id)
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
