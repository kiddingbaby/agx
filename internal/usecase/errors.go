package usecase

import (
	"errors"
	"fmt"
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

var ErrProfileNotFound error = &domainprofile.NotFoundError{}

func IsProfileNotFoundError(err error) bool {
	var target *domainprofile.NotFoundError
	return errors.As(err, &target)
}

type InvalidAgentError struct {
	Agent string
}

type ConflictingAgentChangesError struct {
	Agents []domainprofile.Agent
}

type AgentNotBoundToRelayError struct {
	Agent domainprofile.Agent
	Relay string
}

type ProfileAlreadyExistsError struct {
	Name string
}

type DuplicateRelayConfigError struct {
	Name         string
	ExistingName string
}

func (e *ProfileAlreadyExistsError) Error() string {
	if e.Name == "" {
		return "relay already exists"
	}
	return fmt.Sprintf("relay already exists: %s", e.Name)
}

func (e *DuplicateRelayConfigError) Error() string {
	switch {
	case strings.TrimSpace(e.Name) != "" && strings.TrimSpace(e.ExistingName) != "":
		return fmt.Sprintf("cannot add relay %s: base_url and api_key already match existing relay %s", e.Name, e.ExistingName)
	case strings.TrimSpace(e.ExistingName) != "":
		return fmt.Sprintf("base_url and api_key already match existing relay %s", e.ExistingName)
	default:
		return "base_url and api_key already match an existing relay"
	}
}

func (e *InvalidAgentError) Error() string {
	if strings.TrimSpace(e.Agent) == "" {
		return "invalid agent"
	}
	return fmt.Sprintf("invalid agent: %s", e.Agent)
}

type ProfileInUseError struct {
	Name   string
	Agents []domainprofile.Agent
}

func (e *ProfileInUseError) Error() string {
	agents := make([]string, 0, len(e.Agents))
	for _, agent := range e.Agents {
		if agent == "" {
			continue
		}
		agents = append(agents, string(agent))
	}
	if len(agents) == 0 {
		return fmt.Sprintf("relay is currently bound: %s", e.Name)
	}
	if len(agents) == 1 {
		return fmt.Sprintf("relay %s is currently bound to %s; unbind %s or bind it to another relay before removing it", e.Name, agents[0], agents[0])
	}
	return fmt.Sprintf("relay %s is currently bound to %s; unbind them or bind them to other relays before removing it", e.Name, strings.Join(agents, ", "))
}

type NoBackupError struct {
	Agent domainprofile.Agent
}

func (e *NoBackupError) Error() string {
	if e.Agent == "" {
		return "no backup available"
	}
	return fmt.Sprintf("no %s backup available", e.Agent)
}

type BackupNotFoundError struct {
	ID string
}

type NoCurrentTargetError struct {
	Agent domainprofile.Agent
}

func (e *BackupNotFoundError) Error() string {
	if strings.TrimSpace(e.ID) == "" {
		return "backup not found"
	}
	return fmt.Sprintf("backup not found: %s", e.ID)
}

func (e *NoCurrentTargetError) Error() string {
	if e.Agent == "" {
		return "no current target selected"
	}
	return fmt.Sprintf("no current target selected for %s", e.Agent)
}

func (e *ConflictingAgentChangesError) Error() string {
	if len(e.Agents) == 0 {
		return "bind and unbind contain overlapping agents"
	}
	items := make([]string, 0, len(e.Agents))
	for _, agent := range e.Agents {
		if agent != "" {
			items = append(items, string(agent))
		}
	}
	if len(items) == 0 {
		return "bind and unbind contain overlapping agents"
	}
	return fmt.Sprintf("bind and unbind contain overlapping agents: %s", strings.Join(items, ", "))
}

func (e *AgentNotBoundToRelayError) Error() string {
	if strings.TrimSpace(e.Relay) == "" {
		return fmt.Sprintf("%s is not bound to this relay", e.Agent)
	}
	return fmt.Sprintf("%s is not currently bound to relay %s", e.Agent, e.Relay)
}

type ManagedRuntimeUnavailableError struct {
	Agent  domainprofile.Agent
	Reason string
}

func (e *ManagedRuntimeUnavailableError) Error() string {
	if e.Agent == "" {
		if strings.TrimSpace(e.Reason) != "" {
			return fmt.Sprintf("managed runtime is unavailable: %s", e.Reason)
		}
		return "managed runtime is unavailable"
	}
	if strings.TrimSpace(e.Reason) != "" {
		return fmt.Sprintf("%s managed runtime is unavailable: %s", e.Agent, e.Reason)
	}
	return fmt.Sprintf("%s managed runtime is unavailable", e.Agent)
}

type TargetNotFoundError struct {
	Agent domainprofile.Agent
	Name  string
}

func (e *TargetNotFoundError) Error() string {
	switch {
	case e.Agent != "" && strings.TrimSpace(e.Name) != "":
		return fmt.Sprintf("%s target %s not found", e.Agent, e.Name)
	case strings.TrimSpace(e.Name) != "":
		return fmt.Sprintf("target %s not found", e.Name)
	default:
		return "target not found"
	}
}
