package usecase

import (
	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/ports"
)

// resolveAgentSyncer returns the agent-specific syncer registered for the
// given agent as a generic ports.AgentSyncer. Returns nil if the agent has
// no syncer wired up (e.g. tests that don't need that agent) or if the
// agent identifier is unknown. Callers that need agent-specific operations
// (Sync, Status, RemoveProfile) should branch on the concrete syncer field
// instead.
func (s *ProfileService) resolveAgentSyncer(agent domainprofile.Agent) ports.AgentSyncer {
	switch agent {
	case domainprofile.AgentCodex:
		if s.codex == nil {
			return nil
		}
		return s.codex
	case domainprofile.AgentClaude:
		if s.claude == nil {
			return nil
		}
		return s.claude
	case domainprofile.AgentGemini:
		if s.gemini == nil {
			return nil
		}
		return s.gemini
	case domainprofile.AgentOpenCode:
		if s.openCode == nil {
			return nil
		}
		return s.openCode
	default:
		return nil
	}
}
