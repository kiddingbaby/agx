package agent

// Agent represents an AI CLI tool and its runtime requirements.
type Agent struct {
	Name           string
	Command        string
	Provider       string
	EnvVar         string
	EnvVars        []string
	BaseURLEnvVar  string
	BaseURLEnvVars []string
}
