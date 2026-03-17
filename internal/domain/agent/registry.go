package agent

import "sort"

var defaultAgents = []Agent{
	{
		Name:           "claude-code",
		Command:        "claude",
		Provider:       "claude",
		EnvVar:         "ANTHROPIC_API_KEY",
		EnvVars:        []string{"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"},
		BaseURLEnvVar:  "ANTHROPIC_BASE_URL",
		BaseURLEnvVars: []string{"ANTHROPIC_BASE_URL"},
	},
	{
		Name:           "codex-cli",
		Command:        "codex",
		Provider:       "openai",
		EnvVar:         "OPENAI_API_KEY",
		EnvVars:        []string{"OPENAI_API_KEY"},
		BaseURLEnvVar:  "OPENAI_BASE_URL",
		BaseURLEnvVars: []string{"OPENAI_BASE_URL", "OPENAI_API_BASE"},
	},
	{
		Name:           "gemini-cli",
		Command:        "gemini",
		Provider:       "gemini",
		EnvVar:         "GEMINI_API_KEY",
		EnvVars:        []string{"GEMINI_API_KEY", "GOOGLE_GEMINI_API_KEY", "GOOGLE_API_KEY"},
		BaseURLEnvVar:  "GOOGLE_GEMINI_BASE_URL",
		BaseURLEnvVars: []string{"GOOGLE_GEMINI_BASE_URL", "GEMINI_BASE_URL"},
	},
}

var aliases = map[string]string{
	"claude": "claude-code",
	"codex":  "codex-cli",
	"gemini": "gemini-cli",
}

// DefaultAgents returns a copy of built-in agents.
func DefaultAgents() []Agent {
	result := make([]Agent, len(defaultAgents))
	copy(result, defaultAgents)
	return result
}

// Find resolves alias and returns the matching agent.
func Find(name string) (Agent, bool) {
	resolved := ResolveAlias(name)
	for _, a := range defaultAgents {
		if a.Name == resolved {
			return a, true
		}
	}
	return Agent{}, false
}

// ResolveAlias maps shorthand names to canonical agent names.
func ResolveAlias(name string) string {
	if resolved, ok := aliases[name]; ok {
		return resolved
	}
	return name
}

// AliasMap returns a copy of alias definitions.
func AliasMap() map[string]string {
	result := make(map[string]string, len(aliases))
	for k, v := range aliases {
		result[k] = v
	}
	return result
}

// CanonicalNames returns sorted canonical names.
func CanonicalNames() []string {
	result := make([]string, 0, len(defaultAgents))
	for _, a := range defaultAgents {
		result = append(result, a.Name)
	}
	sort.Strings(result)
	return result
}
