package usecase

import (
	"os"
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
		activeKey *domainkey.Key
		err       error
	)
	if strings.TrimSpace(opts.KeyIdentifier) != "" {
		activeKey, err = s.keyService.Resolve(provider, profile, opts.KeyIdentifier)
		if err != nil {
			return domainsession.SessionConfig{}, &NoActiveKeyError{Provider: agent.Provider}
		}
	} else {
		_, envAPIKey := firstEnvValue(envNames(agent))
		if envAPIKey != "" {
			activeKey = &domainkey.Key{
				Provider: provider,
				Profile:  profile,
				APIKey:   envAPIKey,
			}
		} else {
			activeKey, err = s.keyService.Resolve(provider, profile, "")
			if err != nil {
				return domainsession.SessionConfig{}, &NoActiveKeyError{Provider: agent.Provider}
			}
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
	if envBaseURL != "" {
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
