package usecase

import (
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

func NewLaunchService(keyService *KeyService, runtime ports.SessionRuntime) *LaunchService {
	return &LaunchService{
		keyService: keyService,
		runtime:    runtime,
	}
}

func (s *LaunchService) Launch(agentName, dir, extraArgs string) error {
	cfg, err := s.BuildSessionConfig(agentName, dir, extraArgs)
	if err != nil {
		return err
	}
	return WrapRuntimeError("launch", s.runtime.Launch(cfg))
}

func (s *LaunchService) BuildSessionConfig(agentName, dir, extraArgs string) (domainsession.SessionConfig, error) {
	agent, ok := domainagent.Find(agentName)
	if !ok {
		return domainsession.SessionConfig{}, &UnknownAgentError{Name: agentName}
	}

	provider := domainkey.Provider(agent.Provider)
	if !s.keyService.HasActive(provider) {
		return domainsession.SessionConfig{}, &NoActiveKeyError{Provider: agent.Provider}
	}
	activeKey, err := s.keyService.GetActive(provider)
	if err != nil {
		return domainsession.SessionConfig{}, err
	}

	command := agent.Command
	if strings.TrimSpace(extraArgs) != "" {
		command = command + " " + extraArgs
	}

	cfg := domainsession.SessionConfig{
		Agent:   domainsession.SessionName(agent.Name),
		Dir:     dir,
		Command: command,
		EnvVars: map[string]string{
			agent.EnvVar: activeKey.APIKey,
		},
	}
	if activeKey.BaseURL != "" && agent.BaseURLEnvVar != "" {
		cfg.EnvVars[agent.BaseURLEnvVar] = activeKey.BaseURL
	}
	return cfg, nil
}
