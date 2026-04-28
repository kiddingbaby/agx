package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/usecase"
)

func parseJSONOnlyArgs(r *Root, args []string, usage string) (bool, bool) {
	asJSON := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) || args[i+1] != "json" {
				fmt.Fprintln(r.stderr, "Error: -o requires value json")
				return false, false
			}
			asJSON = true
			i++
		default:
			fmt.Fprintln(r.stderr, usage)
			return false, false
		}
	}
	return asJSON, true
}

func parseNameWithJSON(r *Root, args []string, usage string) (string, bool, bool) {
	var name string
	asJSON := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) || args[i+1] != "json" {
				fmt.Fprintln(r.stderr, "Error: -o requires value json")
				return "", false, false
			}
			asJSON = true
			i++
		default:
			if strings.HasPrefix(args[i], "-") || name != "" {
				fmt.Fprintln(r.stderr, usage)
				return "", false, false
			}
			name = args[i]
		}
	}
	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(r.stderr, usage)
		return "", false, false
	}
	return name, asJSON, true
}

func parseAgentOnly(r *Root, args []string, usage string) (domainprofile.Agent, bool, bool) {
	asJSON := false
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) || args[i+1] != "json" {
				fmt.Fprintln(r.stderr, "Error: -o requires value json")
				return "", false, false
			}
			asJSON = true
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintln(r.stderr, usage)
				return "", false, false
			}
			positional = append(positional, args[i])
		}
	}
	if len(positional) != 1 {
		fmt.Fprintln(r.stderr, usage)
		return "", false, false
	}
	agent, ok := parseAgentValue(r, positional[0])
	if !ok {
		return "", false, false
	}
	return agent, asJSON, true
}

func parseAgentOnlyFlagRequired(r *Root, args []string, usage string) (domainprofile.Agent, bool, bool) {
	agent, asJSON, hasAgent, ok := parseOptionalAgentFlag(r, args, usage)
	if !ok {
		return "", false, false
	}
	if !hasAgent {
		fmt.Fprintln(r.stderr, usage)
		return "", false, false
	}
	return agent, asJSON, true
}

func parseOptionalAgentFlag(r *Root, args []string, usage string) (domainprofile.Agent, bool, bool, bool) {
	var (
		agent  domainprofile.Agent
		asJSON bool
	)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) || args[i+1] != "json" {
				fmt.Fprintln(r.stderr, "Error: -o requires value json")
				return "", false, false, false
			}
			asJSON = true
			i++
		case "--agent":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, usage)
				return "", false, false, false
			}
			if agent != "" {
				fmt.Fprintln(r.stderr, usage)
				return "", false, false, false
			}
			var ok bool
			agent, ok = parseAgentValue(r, args[i+1])
			if !ok {
				return "", false, false, false
			}
			i++
		default:
			fmt.Fprintln(r.stderr, usage)
			return "", false, false, false
		}
	}
	return agent, asJSON, agent != "", true
}

func parseRestoreArgs(r *Root, args []string, usage string) (domainprofile.Agent, string, bool, bool) {
	var (
		agent    domainprofile.Agent
		backupID string
		asJSON   bool
	)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) || args[i+1] != "json" {
				fmt.Fprintln(r.stderr, "Error: -o requires value json")
				return "", "", false, false
			}
			asJSON = true
			i++
		case "--to":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
				fmt.Fprintln(r.stderr, usage)
				return "", "", false, false
			}
			backupID = args[i+1]
			i++
		case "--agent":
			if i+1 >= len(args) || agent != "" {
				fmt.Fprintln(r.stderr, usage)
				return "", "", false, false
			}
			var ok bool
			agent, ok = parseAgentValue(r, args[i+1])
			if !ok {
				return "", "", false, false
			}
			i++
		default:
			if strings.HasPrefix(args[i], "-") || agent != "" {
				fmt.Fprintln(r.stderr, usage)
				return "", "", false, false
			}
			fmt.Fprintln(r.stderr, usage)
			return "", "", false, false
		}
	}
	if agent == "" {
		fmt.Fprintln(r.stderr, usage)
		return "", "", false, false
	}
	return agent, backupID, asJSON, true
}

func (r *Root) writeJSON(payload any) int {
	if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
		fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
		return 1
	}
	return 0
}

func parseAgentValue(r *Root, value string) (domainprofile.Agent, bool) {
	agent, ok := domainprofile.ParseAgent(value)
	if !ok {
		r.printUserError(&usecase.InvalidAgentError{Agent: value})
		return "", false
	}
	return agent, true
}

func parseAgentList(r *Root, raw string) ([]domainprofile.Agent, bool) {
	if strings.TrimSpace(raw) == "" {
		fmt.Fprintln(r.stderr, "Error: agent list cannot be empty")
		return nil, false
	}
	parts := strings.Split(raw, ",")
	agents := make([]domainprofile.Agent, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			fmt.Fprintln(r.stderr, "Error: agent list cannot contain empty values")
			return nil, false
		}
		agent, ok := parseAgentValue(r, part)
		if !ok {
			return nil, false
		}
		agents = append(agents, agent)
	}
	return normalizeParsedAgents(agents), true
}

func normalizeParsedAgents(agents []domainprofile.Agent) []domainprofile.Agent {
	if len(agents) == 0 {
		return nil
	}
	seen := make(map[domainprofile.Agent]struct{}, len(agents))
	out := make([]domainprofile.Agent, 0, len(agents))
	for _, agent := range agents {
		if !agent.Valid() {
			continue
		}
		if _, ok := seen[agent]; ok {
			continue
		}
		seen[agent] = struct{}{}
		out = append(out, agent)
	}
	slices.Sort(out)
	return out
}
