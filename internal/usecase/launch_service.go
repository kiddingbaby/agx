package usecase

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	domainagent "github.com/kiddingbaby/agx/internal/domain/agent"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainsession "github.com/kiddingbaby/agx/internal/domain/session"
	"github.com/kiddingbaby/agx/internal/ports"
)

// LaunchService encapsulates agent launch orchestration.
type LaunchService struct {
	keyService *KeyService
	runtime    ports.SessionRuntime
}

type LaunchOptions struct {
	Profile       string
	KeyIdentifier string
	ExtraArgs     string
}

func NewLaunchService(keyService *KeyService, runtime ports.SessionRuntime) *LaunchService {
	return &LaunchService{
		keyService: keyService,
		runtime:    runtime,
	}
}

func (s *LaunchService) Launch(agentName, dir, extraArgs string) error {
	return s.LaunchWithOptions(agentName, dir, LaunchOptions{ExtraArgs: extraArgs})
}

func (s *LaunchService) LaunchWithOptions(agentName, dir string, opts LaunchOptions) error {
	cfg, err := s.BuildSessionConfigWithOptions(agentName, dir, opts)
	if err != nil {
		return err
	}
	return WrapRuntimeError("launch", s.runtime.Launch(cfg))
}

func (s *LaunchService) BuildSessionConfig(agentName, dir, extraArgs string) (domainsession.SessionConfig, error) {
	return s.BuildSessionConfigWithOptions(agentName, dir, LaunchOptions{ExtraArgs: extraArgs})
}

func (s *LaunchService) BuildSessionConfigWithOptions(agentName, dir string, opts LaunchOptions) (domainsession.SessionConfig, error) {
	agent, ok := domainagent.Find(agentName)
	if !ok {
		return domainsession.SessionConfig{}, &UnknownAgentError{Name: agentName}
	}

	provider := domainkey.Provider(agent.Provider)
	profile := domainkey.NormalizeProfileName(opts.Profile)

	var (
		activeKey  *domainkey.Key
		err        error
		keyFromEnv bool
	)
	if strings.TrimSpace(opts.KeyIdentifier) != "" {
		activeKey, err = s.keyService.Resolve(provider, profile, opts.KeyIdentifier)
		if err != nil {
			return domainsession.SessionConfig{}, err
		}
	} else {
		// Prefer agx-managed key selection strategy. Only fallback to env
		// when provider/profile has no key in the key store.
		activeKey, err = s.keyService.Resolve(provider, profile, "")
		if err != nil {
			_, envAPIKey := firstEnvValue(envNames(agent))
			if envAPIKey == "" {
				return domainsession.SessionConfig{}, &NoActiveKeyError{Provider: agent.Provider}
			}
			activeKey = &domainkey.Key{
				Provider: provider,
				Profile:  profile,
				APIKey:   envAPIKey,
			}
			keyFromEnv = true
		}
	}

	command := agent.Command
	if strings.TrimSpace(opts.ExtraArgs) != "" {
		command = command + " " + opts.ExtraArgs
	}

	apiEnvNames := envNames(agent)
	baseURLEnvNames := baseURLEnvNames(agent)

	baseURL := activeKey.BaseURL
	_, envBaseURL := firstEnvValue(baseURLEnvNames)
	// Keep launch strategy deterministic: store-selected key base URL wins.
	// Env base URL is only fallback, or primary when key came from env fallback.
	if keyFromEnv && envBaseURL != "" {
		baseURL = envBaseURL
	} else if !keyFromEnv && baseURL == "" && envBaseURL != "" {
		baseURL = envBaseURL
	}

	cfg := domainsession.SessionConfig{
		Agent:   domainsession.SessionName(agent.Name),
		Dir:     dir,
		Command: command,
		EnvVars: map[string]string{},
	}
	for _, envName := range apiEnvNames {
		cfg.EnvVars[envName] = activeKey.APIKey
	}
	if baseURL != "" {
		for _, envName := range baseURLEnvNames {
			cfg.EnvVars[envName] = baseURL
		}
	}
	return cfg, nil
}

func envNames(agent domainagent.Agent) []string {
	if agent.Name == "codex-cli" {
		return []string{resolveCodexEnvKey()}
	}
	if len(agent.EnvVars) > 0 {
		return uniqStrings(agent.EnvVars)
	}
	if agent.EnvVar == "" {
		return nil
	}
	return []string{agent.EnvVar}
}

func baseURLEnvNames(agent domainagent.Agent) []string {
	if len(agent.BaseURLEnvVars) > 0 {
		return uniqStrings(agent.BaseURLEnvVars)
	}
	if agent.BaseURLEnvVar == "" {
		return nil
	}
	return []string{agent.BaseURLEnvVar}
}

func firstEnvValue(names []string) (string, string) {
	for _, name := range names {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return name, v
		}
	}
	return "", ""
}

func uniqStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

const defaultCodexEnvKey = "OPENAI_API_KEY"

// resolveCodexEnvKey resolves env_key from ~/.codex/config.toml selected provider.
// Falls back to OPENAI_API_KEY when config is missing/invalid.
func resolveCodexEnvKey() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return defaultCodexEnvKey
	}
	envKey, err := parseCodexEnvKey(filepath.Join(home, ".codex", "config.toml"))
	if err != nil || strings.TrimSpace(envKey) == "" {
		return defaultCodexEnvKey
	}
	return envKey
}

func parseCodexEnvKey(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var (
		selectedProvider string
		currentProvider  string
		envKeys          = map[string]string{}
	)

	lines := strings.Split(string(raw), "\n")
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}

		if strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]") {
			section := strings.TrimSpace(l[1 : len(l)-1])
			currentProvider = ""
			const prefix = "model_providers."
			if strings.HasPrefix(section, prefix) {
				name := strings.TrimSpace(strings.TrimPrefix(section, prefix))
				currentProvider = strings.Trim(name, "\"")
			}
			continue
		}

		if currentProvider == "" {
			if v, ok := parseTomlStringAssign(l, "model_provider"); ok {
				selectedProvider = v
			}
			continue
		}

		if v, ok := parseTomlStringAssign(l, "env_key"); ok {
			envKeys[currentProvider] = v
		}
	}

	if selectedProvider == "" {
		return "", nil
	}
	return strings.TrimSpace(envKeys[selectedProvider]), nil
}

func parseTomlStringAssign(line, key string) (string, bool) {
	if !strings.HasPrefix(line, key) {
		return "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, key))
	if !strings.HasPrefix(rest, "=") {
		return "", false
	}
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "="))
	if !strings.HasPrefix(rest, "\"") {
		return "", false
	}

	escaped := false
	for i := 1; i < len(rest); i++ {
		switch rest[i] {
		case '\\':
			if !escaped {
				escaped = true
				continue
			}
		case '"':
			if !escaped {
				unquoted, err := strconv.Unquote(rest[:i+1])
				if err != nil {
					return "", false
				}
				return unquoted, true
			}
		}
		escaped = false
	}
	return "", false
}
