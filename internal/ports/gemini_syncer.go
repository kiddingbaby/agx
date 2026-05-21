package ports

import domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"

type GeminiSyncResult struct {
	ConfigPath string
}

type GeminiSyncer interface {
	AgentSyncer
	Sync(profile domainprofile.Profile) (*GeminiSyncResult, error)
}
