package tui

// Agent represents an AI CLI tool
type Agent struct {
	Name     string
	Command  string
	EnvVar   string
	Provider string
}

// DefaultAgents returns the list of supported AI CLI tools
func DefaultAgents() []Agent {
	return []Agent{
		{Name: "claude-code", Command: "claude", EnvVar: "ANTHROPIC_API_KEY", Provider: "claude"},
		{Name: "codex-cli", Command: "codex", EnvVar: "OPENAI_API_KEY", Provider: "openai"},
		{Name: "gemini-cli", Command: "gemini", EnvVar: "GOOGLE_API_KEY", Provider: "gemini"},
	}
}
