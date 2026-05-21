package ports

import domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"

type OpenCodeManagedProvider struct {
	ID     string
	Name   string
	Family domainprofile.OpenCodeProviderFamily
	Model  string
}

type OpenCodeConfigStatus struct {
	ConfigPath           string
	DefaultModel         string
	ManagedProvidersByID map[string]OpenCodeManagedProvider
}

type OpenCodeSyncResult struct {
	ProviderID string
	ModelID    string
	ConfigPath string
}

type OpenCodeSyncOptions struct {
	ProviderFamily domainprofile.OpenCodeProviderFamily
	ModelID        string
	ModelName      string
	ProviderName   string
	SetAsCurrent   bool
}

type OpenCodeSyncer interface {
	AgentSyncer
	Status() (*OpenCodeConfigStatus, error)
	Sync(profile domainprofile.Profile, options OpenCodeSyncOptions) (*OpenCodeSyncResult, error)
	ClearDefaultModel() (string, error)
	RemoveProfile(name string) (string, error)
}
