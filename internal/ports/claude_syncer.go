package ports

import domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"

type ClaudeSyncResult struct {
	ConfigPath string
}

type ClaudeSyncer interface {
	AgentSyncer
	Sync(profile domainprofile.Profile) (*ClaudeSyncResult, error)
}
