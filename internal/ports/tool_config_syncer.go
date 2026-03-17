package ports

import (
	domainagent "github.com/kiddingbaby/agx/internal/domain/agent"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
)

// ToolConfigSyncer writes resolved credentials/provider config into each CLI's live config files.
type ToolConfigSyncer interface {
	Apply(agent domainagent.Agent, key domainkey.Key, target domainprovider.Target) error
}
