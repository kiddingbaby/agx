package ports

import domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"

// ProviderConfigRepository persists reusable provider targets and active family bindings.
type ProviderConfigRepository interface {
	ListTargets() []domainprovider.Target
	GetTarget(name string) (*domainprovider.Target, error)
	UpsertTarget(target domainprovider.Target) (*domainprovider.Target, error)
	DeleteTarget(name string) error
	ListBindings() []domainprovider.Binding
	GetBinding(family domainprovider.Family) (*domainprovider.Binding, error)
	SetBinding(family domainprovider.Family, target string) (*domainprovider.Binding, error)

	// GetCurrentSite returns the persisted current site/target name (if any).
	// This is used as the default scope for key operations so users don't need to pass --site each time.
	GetCurrentSite() string
	// SetCurrentSite persists the current site/target name. Passing an empty string clears it.
	SetCurrentSite(targetName string) error
}
