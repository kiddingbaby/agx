package key

import "time"

// Provider represents an API key provider.
type Provider string

const (
	ProviderOpenAI Provider = "openai"
	ProviderClaude Provider = "claude"
	ProviderGemini Provider = "gemini"
)

// RotationStrategy defines how a profile selects key at runtime.
type RotationStrategy string

const (
	StrategyFixed      RotationStrategy = "fixed"
	StrategyRoundRobin RotationStrategy = "round_robin"
	StrategyRandom     RotationStrategy = "random"
)

const DefaultProfile = "default"

// Profile holds provider/profile level selection strategy and state.
type Profile struct {
	Provider  Provider         `yaml:"provider"`
	Name      string           `yaml:"name"`
	Strategy  RotationStrategy `yaml:"strategy,omitempty"`
	FixedKey  string           `yaml:"fixed_key,omitempty"`
	NextIndex int              `yaml:"next_index,omitempty"`
	UpdatedAt time.Time        `yaml:"updated_at,omitempty"`
}

// Key represents an API key entry.
type Key struct {
	ID        string    `yaml:"id"`
	Provider  Provider  `yaml:"provider"`
	Profile   string    `yaml:"profile,omitempty"`
	Name      string    `yaml:"name"`
	APIKey    string    `yaml:"api_key"`
	BaseURL   string    `yaml:"base_url,omitempty"`
	Tags      []string  `yaml:"tags,omitempty"`
	Active    bool      `yaml:"active"`
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at,omitempty"`
}
