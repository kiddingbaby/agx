package key

import "time"

// Provider represents an API key provider.
type Provider string

const (
	ProviderOpenAI Provider = "openai"
	ProviderClaude Provider = "claude"
	ProviderGemini Provider = "gemini"
)

// Key represents an API key entry.
type Key struct {
	ID        string    `yaml:"id"`
	Provider  Provider  `yaml:"provider"`
	Name      string    `yaml:"name"`
	APIKey    string    `yaml:"api_key"`
	BaseURL   string    `yaml:"base_url,omitempty"`
	Tags      []string  `yaml:"tags,omitempty"`
	Active    bool      `yaml:"active"`
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at,omitempty"`
}
