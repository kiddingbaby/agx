package tui

import domainagent "github.com/kiddingbaby/agx/internal/domain/agent"

// Agent is re-exported from domain layer for backward compatibility in TUI.
type Agent = domainagent.Agent

// DefaultAgents returns the list of supported AI CLI tools
func DefaultAgents() []Agent {
	return domainagent.DefaultAgents()
}
