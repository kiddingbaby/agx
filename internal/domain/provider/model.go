package provider

import "time"

// Family identifies the runtime-facing provider family used by an agent.
type Family string

const (
	FamilyOpenAI Family = "openai"
	FamilyClaude Family = "claude"
	FamilyGemini Family = "gemini"
)

// Kind identifies the upstream protocol/runtime shape.
type Kind string

const (
	KindOpenAI           Kind = "openai"
	KindOpenAICompatible Kind = "openai-compatible"
	KindClaude           Kind = "claude"
	KindGemini           Kind = "gemini"
)

// AccessMode describes whether AGX talks to the official endpoint or a third-party gateway.
type AccessMode string

const (
	AccessOfficial   AccessMode = "official"
	AccessThirdParty AccessMode = "third_party"
)

// AuthMode describes how a target authenticates.
type AuthMode string

const (
	AuthAPIKey AuthMode = "apikey"
)

// WireAPI controls which upstream API shape is used by codex-cli (OpenAI family).
type WireAPI string

const (
	WireAPIResponses       WireAPI = "responses"
	WireAPIChatCompletions WireAPI = "chat_completions"
)

// Target is a reusable provider endpoint configuration.
type Target struct {
	Name               string            `yaml:"name"`
	Family             Family            `yaml:"family"`
	Kind               Kind              `yaml:"kind"`
	Access             AccessMode        `yaml:"access"`
	Auth               AuthMode          `yaml:"auth"`
	BaseURL            string            `yaml:"base-url,omitempty"`
	Model              string            `yaml:"model,omitempty"`
	Env                map[string]string `yaml:"env,omitempty"`
	WireAPI            WireAPI           `yaml:"wire-api,omitempty"`
	RequiresOpenAIAuth *bool             `yaml:"requires-openai-auth,omitempty"`
	CreatedAt          time.Time         `yaml:"created-at,omitempty"`
	UpdatedAt          time.Time         `yaml:"updated-at,omitempty"`
}

// Binding points a provider family to the active target.
type Binding struct {
	Family    Family    `yaml:"family"`
	Target    string    `yaml:"target"`
	UpdatedAt time.Time `yaml:"updated-at,omitempty"`
}
