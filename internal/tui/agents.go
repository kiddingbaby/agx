package tui

// Agent represents an AI CLI tool
type Agent struct {
	Name          string
	Command       string
	EnvVar        string
	BaseURLEnvVar string
	Provider      string
}

// DefaultAgents returns the list of supported AI CLI tools
func DefaultAgents() []Agent {
	return []Agent{
		{Name: "claude-code", Command: "claude", EnvVar: "ANTHROPIC_API_KEY", BaseURLEnvVar: "ANTHROPIC_BASE_URL", Provider: "claude"},
		{Name: "codex-cli", Command: "codex", EnvVar: "OPENAI_API_KEY", BaseURLEnvVar: "OPENAI_API_BASE", Provider: "openai"},
		{Name: "gemini-cli", Command: "gemini", EnvVar: "GOOGLE_API_KEY", BaseURLEnvVar: "GEMINI_BASE_URL", Provider: "gemini"},
	}
}
