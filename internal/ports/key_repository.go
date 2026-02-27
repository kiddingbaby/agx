package ports

import domainkey "github.com/kiddingbaby/agx/internal/domain/key"

// KeyRepository defines key persistence operations for use cases.
type KeyRepository interface {
	List() []domainkey.Key
	Add(provider domainkey.Provider, profile, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error)
	Update(id string, provider domainkey.Provider, profile, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error)
	Delete(id string) error
	Activate(id string) error
	GetActive(provider domainkey.Provider, profile string) (*domainkey.Key, error)
	HasActive(provider domainkey.Provider, profile string) bool
	Resolve(provider domainkey.Provider, profile, identifier string) (*domainkey.Key, error)
	ListProfiles(provider domainkey.Provider) []domainkey.Profile
	SetProfileStrategy(provider domainkey.Provider, profile string, strategy domainkey.RotationStrategy, fixedKey string) error
}
