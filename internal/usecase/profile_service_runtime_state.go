package usecase

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

type runtimeStateIssue struct {
	Agent   domainprofile.Agent
	Code    string
	Message string
	Action  string
}

type runtimeBindingResolution struct {
	Relay            string
	ConfigPath       string
	PreserveExisting bool
}

func (s *ProfileService) enrichRuntimeState(state *domainprofile.State, profiles []domainprofile.Profile) []runtimeStateIssue {
	if state == nil {
		return nil
	}

	var issues []runtimeStateIssue
	if s.claude != nil {
		issues = append(issues, s.applyRuntimeAgentBinding(state, profiles, domainprofile.AgentClaude)...)
	}
	if s.gemini != nil {
		issues = append(issues, s.applyRuntimeAgentBinding(state, profiles, domainprofile.AgentGemini)...)
	}
	return issues
}

func (s *ProfileService) hasRuntimeManagedAgents() bool {
	return s.claude != nil || s.gemini != nil
}

func (s *ProfileService) applyRuntimeAgentBinding(state *domainprofile.State, profiles []domainprofile.Profile, agent domainprofile.Agent) []runtimeStateIssue {
	binding := currentBinding(*state, agent)
	resolution, issues := s.detectRuntimeAgentRelay(agent, profiles, binding.SourceProfile)

	switch {
	case resolution.PreserveExisting:
		assignBinding(state, agent, binding)
		return issues
	case resolution.Relay == "":
		clearResolvedBinding(&binding)
	default:
		relayChanged := domainprofile.NormalizeProfileName(binding.SourceProfile) != domainprofile.NormalizeProfileName(resolution.Relay)
		staleMetadata := relayChanged || binding.Status != domainprofile.BindingStatusApplied

		binding.SourceProfile = resolution.Relay
		binding.Status = domainprofile.BindingStatusApplied
		if strings.TrimSpace(resolution.ConfigPath) != "" {
			binding.ConfigPath = resolution.ConfigPath
		}
		if staleMetadata {
			binding.LastAppliedAt = time.Time{}
			binding.LastBackupID = ""
		}
	}

	assignBinding(state, agent, binding)
	return issues
}

func (s *ProfileService) detectRuntimeAgentRelay(agent domainprofile.Agent, profiles []domainprofile.Profile, current string) (runtimeBindingResolution, []runtimeStateIssue) {
	switch agent {
	case domainprofile.AgentClaude:
		snapshot, err := s.claude.Snapshot()
		if err != nil {
			return runtimeBindingResolution{Relay: current, PreserveExisting: true}, []runtimeStateIssue{runtimeSnapshotErrorIssue(agent, err)}
		}
		if snapshot == nil || !snapshot.Exists {
			if current != "" {
				return runtimeBindingResolution{}, []runtimeStateIssue{{
					Agent:   agent,
					Code:    "runtime_binding_missing",
					Message: fmt.Sprintf("%s config is missing but state still points to relay %s", agent, current),
					Action:  runtimeRestoreAction(agent),
				}}
			}
			return runtimeBindingResolution{}, nil
		}

		helperRelay, baseURL, err := parseClaudeBindingSnapshot(snapshot.Content)
		if err != nil {
			return runtimeBindingResolution{Relay: current, PreserveExisting: true}, []runtimeStateIssue{runtimeSnapshotErrorIssue(agent, err)}
		}
		relay, outcome := chooseClaudeRelay(profiles, helperRelay, baseURL, current)
		switch outcome {
		case claudeBindingResolved:
			return runtimeBindingResolution{Relay: relay, ConfigPath: snapshot.ConfigPath}, nil
		case claudeBindingConflict:
			return runtimeBindingResolution{Relay: current, PreserveExisting: true}, []runtimeStateIssue{{
				Agent:   agent,
				Code:    "runtime_binding_conflict",
				Message: fmt.Sprintf("%s config has conflicting relay markers", agent),
				Action:  runtimeRestoreAction(agent),
			}}
		case claudeBindingStaleHelper:
			return runtimeBindingResolution{Relay: current, PreserveExisting: true}, []runtimeStateIssue{{
				Agent:   agent,
				Code:    "runtime_binding_conflict",
				Message: fmt.Sprintf("%s config references a relay that no longer exists", agent),
				Action:  runtimeRestoreAction(agent),
			}}
		case claudeBindingIncomplete:
			return runtimeBindingResolution{}, []runtimeStateIssue{{
				Agent:   agent,
				Code:    "runtime_binding_incomplete",
				Message: fmt.Sprintf("%s config has apiKeyHelper but is missing ANTHROPIC_BASE_URL", agent),
				Action:  runtimeRestoreAction(agent),
			}}
		default:
			return runtimeBindingResolution{}, nil
		}
	case domainprofile.AgentGemini:
		// gemini's credential and base URL are injected into the process
		// environment at launch (see interfaces/cli/native_runtime.go) and
		// never persisted to disk, so the on-disk settings.json carries only
		// auth-selection + user-added MCP/extensions. We can't cross-check
		// the binding against the relay list — settings.json existence is
		// all we can validate. Trust state.yaml for the rest.
		snapshot, err := s.gemini.Snapshot()
		if err != nil {
			return runtimeBindingResolution{Relay: current, PreserveExisting: true}, []runtimeStateIssue{runtimeSnapshotErrorIssue(agent, err)}
		}
		if snapshot == nil || !snapshot.Exists {
			if current != "" {
				return runtimeBindingResolution{}, []runtimeStateIssue{{
					Agent:   agent,
					Code:    "runtime_binding_missing",
					Message: fmt.Sprintf("%s config is missing but state still points to relay %s", agent, current),
					Action:  runtimeRestoreAction(agent),
				}}
			}
			return runtimeBindingResolution{}, nil
		}
		return runtimeBindingResolution{Relay: current, ConfigPath: snapshot.ConfigPath, PreserveExisting: true}, nil
	default:
		return runtimeBindingResolution{Relay: current, PreserveExisting: true}, nil
	}
}

func runtimeSnapshotErrorIssue(agent domainprofile.Agent, err error) runtimeStateIssue {
	return runtimeStateIssue{
		Agent:   agent,
		Code:    "runtime_config_unreadable",
		Message: fmt.Sprintf("%s config could not be read: %v", agent, err),
		Action:  runtimeRestoreAction(agent),
	}
}

func runtimeRestoreAction(agent domainprofile.Agent) string {
	return fmt.Sprintf("run `agx restore %s` and rerun `agx doctor`", agent)
}

func parseClaudeBindingSnapshot(content []byte) (string, string, error) {
	if len(content) == 0 {
		return "", "", nil
	}

	var settings map[string]any
	if err := json.Unmarshal(content, &settings); err != nil {
		return "", "", fmt.Errorf("parse claude settings: %w", err)
	}

	var helperRelay string
	if helper, ok := settings["apiKeyHelper"].(string); ok {
		helperRelay = extractRelayNameFromHelper(helper)
	}

	var baseURL string
	if rawEnv, ok := settings["env"].(map[string]any); ok {
		if value, ok := rawEnv["ANTHROPIC_BASE_URL"].(string); ok {
			baseURL = domainprofile.NormalizeBaseURL(value)
		}
	}

	return helperRelay, baseURL, nil
}

func extractRelayNameFromHelper(helper string) string {
	fields := strings.Fields(helper)
	for i := 0; i < len(fields)-1; i++ {
		if fields[i] == "__api-key" {
			return domainprofile.NormalizeProfileName(fields[i+1])
		}
	}
	return ""
}

type claudeBindingOutcome string

const (
	claudeBindingNone        claudeBindingOutcome = "none"
	claudeBindingResolved    claudeBindingOutcome = "resolved"
	claudeBindingConflict    claudeBindingOutcome = "conflict"
	claudeBindingStaleHelper claudeBindingOutcome = "stale_helper"
	claudeBindingIncomplete  claudeBindingOutcome = "incomplete"
)

func chooseClaudeRelay(profiles []domainprofile.Profile, helperRelay, baseURL, current string) (string, claudeBindingOutcome) {
	helperRelay = domainprofile.NormalizeProfileName(helperRelay)
	baseURL = domainprofile.NormalizeBaseURL(baseURL)
	current = domainprofile.NormalizeProfileName(current)

	helperMatch := ""
	if helperRelay != "" {
		for _, profile := range profiles {
			if profile.Name == helperRelay {
				helperMatch = helperRelay
				break
			}
		}
		if helperMatch == "" {
			return current, claudeBindingStaleHelper
		}
	}

	baseMatches := matchingProfilesByBaseURL(profiles, baseURL)
	if helperMatch != "" && baseURL != "" {
		if profileMatchesAgentBaseURL(profiles, helperMatch, domainprofile.AgentClaude, baseURL) {
			return helperMatch, claudeBindingResolved
		}
		return current, claudeBindingConflict
	}
	if helperMatch != "" {
		if baseURL == "" {
			return "", claudeBindingIncomplete
		}
		return helperMatch, claudeBindingResolved
	}
	if baseURL == "" {
		return "", claudeBindingNone
	}

	switch len(baseMatches) {
	case 0:
		return "", claudeBindingNone
	case 1:
		return baseMatches[0], claudeBindingResolved
	default:
		if current != "" && containsProfile(baseMatches, current) {
			return current, claudeBindingConflict
		}
		return "", claudeBindingConflict
	}
}

func matchingProfilesByBaseURL(profiles []domainprofile.Profile, baseURL string) []string {
	if baseURL == "" {
		return nil
	}
	var matches []string
	for _, profile := range profiles {
		if profileBaseURLMatchesAgent(profile, domainprofile.AgentClaude, baseURL) {
			matches = append(matches, profile.Name)
		}
	}
	return matches
}

func profileMatchesAgentBaseURL(profiles []domainprofile.Profile, name string, agent domainprofile.Agent, baseURL string) bool {
	name = domainprofile.NormalizeProfileName(name)
	for _, profile := range profiles {
		if profile.Name == name {
			return profileBaseURLMatchesAgent(profile, agent, baseURL)
		}
	}
	return false
}

func profileBaseURLMatchesAgent(profile domainprofile.Profile, agent domainprofile.Agent, baseURL string) bool {
	baseURL = domainprofile.NormalizeBaseURL(baseURL)
	return baseURL != "" && (domainprofile.NormalizeBaseURL(profile.BaseURL) == baseURL || domainprofile.AgentBaseURL(agent, profile.BaseURL) == baseURL)
}

func containsProfile(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
