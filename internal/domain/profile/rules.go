package profile

import "strings"

var supportedAgents = []Agent{
	AgentCodex,
	AgentClaude,
	AgentGemini,
}

func SupportedAgents() []Agent {
	out := make([]Agent, len(supportedAgents))
	copy(out, supportedAgents)
	return out
}

func ParseAgent(raw string) (Agent, bool) {
	candidate := Agent(strings.TrimSpace(strings.ToLower(raw)))
	for _, agent := range supportedAgents {
		if agent == candidate {
			return agent, true
		}
	}
	return "", false
}

func (a Agent) Valid() bool {
	_, ok := ParseAgent(string(a))
	return ok
}

func (s BindingStatus) Valid() bool {
	switch s {
	case "", BindingStatusApplied:
		return true
	default:
		return false
	}
}

func (m RestoreMode) Valid() bool {
	switch m {
	case RestoreModeRestoreFile, RestoreModeRemoveCreatedFile:
		return true
	default:
		return false
	}
}

func NormalizeProfileName(raw string) string {
	return strings.TrimSpace(strings.ToLower(raw))
}
