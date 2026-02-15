package ports

import domainkey "github.com/kiddingbaby/agx/internal/domain/key"

// KeyRepository defines key persistence operations for use cases.
type KeyRepository interface {
	List() []domainkey.Key
	Add(provider domainkey.Provider, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error)
	Update(id string, provider domainkey.Provider, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error)
	Delete(id string) error
	Activate(id string) error
	GetActive(provider domainkey.Provider) (*domainkey.Key, error)
	HasActive(provider domainkey.Provider) bool
}
