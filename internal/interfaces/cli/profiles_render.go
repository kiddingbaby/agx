package cli

import (
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

func relayBindings(name string, state domainprofile.State) []bindingView {
	bindings := make([]bindingView, 0, 3)
	name = domainprofile.NormalizeProfileName(name)

	if binding := bindingForAgent(state, domainprofile.AgentCodex); binding.SourceProfile == name {
		bindings = append(bindings, toBindingView(domainprofile.AgentCodex, binding))
	}
	for _, agent := range []domainprofile.Agent{domainprofile.AgentClaude, domainprofile.AgentGemini} {
		binding := bindingForAgent(state, agent)
		if binding.SourceProfile != name {
			continue
		}
		bindings = append(bindings, toBindingView(agent, binding))
	}
	return bindings
}

func relayAgents(name string, state domainprofile.State) []domainprofile.Agent {
	agents := make([]domainprofile.Agent, 0, 3)
	name = domainprofile.NormalizeProfileName(name)
	if bindingForAgent(state, domainprofile.AgentCodex).SourceProfile == name {
		agents = append(agents, domainprofile.AgentCodex)
	}
	for _, agent := range []domainprofile.Agent{domainprofile.AgentClaude, domainprofile.AgentGemini} {
		if bindingForAgent(state, agent).SourceProfile == name {
			agents = append(agents, agent)
		}
	}
	return agents
}

func bindingForAgent(state domainprofile.State, agent domainprofile.Agent) domainprofile.AgentBinding {
	switch agent {
	case domainprofile.AgentCodex:
		return state.Codex.AgentBinding()
	case domainprofile.AgentClaude:
		return state.Claude
	case domainprofile.AgentGemini:
		return state.Gemini
	default:
		return domainprofile.AgentBinding{}
	}
}

func codexProfileBinding(state domainprofile.State, name string) domainprofile.CodexProfileBinding {
	if len(state.CodexProfiles) == 0 {
		return domainprofile.CodexProfileBinding{}
	}
	return state.CodexProfiles[domainprofile.NormalizeProfileName(name)]
}

func renderAgents(agents []domainprofile.Agent) string {
	if len(agents) == 0 {
		return "-"
	}
	return strings.Join(agentsToStrings(agents), ",")
}

func agentsToStrings(agents []domainprofile.Agent) []string {
	items := make([]string, 0, len(agents))
	for _, agent := range agents {
		items = append(items, string(agent))
	}
	return items
}

const timeFormat = "2006-01-02T15:04:05Z07:00"
