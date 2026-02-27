package cli

import (
	"encoding/json"
	"fmt"

	domainsession "github.com/kiddingbaby/agx/internal/domain/session"
)

type sessionsJSONResponse struct {
	Sessions []sessionJSONItem `json:"sessions"`
}

type sessionJSONItem struct {
	Name     string `json:"name"`
	Windows  int    `json:"windows"`
	Attached bool   `json:"attached"`
}

func (r *Root) handleList(args []string) int {
	asJSON := false
	for _, arg := range args {
		switch arg {
		case "--json":
			asJSON = true
		default:
			fmt.Fprintln(r.stderr, "Usage: agx ls [--json]")
			return 1
		}
	}

	sessions, err := r.sessionSvc.List()
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	if asJSON {
		payload := sessionsJSONResponse{
			Sessions: make([]sessionJSONItem, 0, len(sessions)),
		}
		for _, s := range sessions {
			payload.Sessions = append(payload.Sessions, sessionJSONItem{
				Name:     s.Name,
				Windows:  s.Windows,
				Attached: s.Attached,
			})
		}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	if len(sessions) == 0 {
		fmt.Fprintln(r.stdout, "No active AI sessions.")
		return 0
	}

	fmt.Fprintln(r.stdout, "Active AI sessions:")
	for _, s := range sessions {
		attached := ""
		if s.Attached {
			attached = " (attached)"
		}
		fmt.Fprintf(r.stdout, "  %s  %d windows%s\n", s.Name, s.Windows, attached)
	}
	return 0
}

func (r *Root) handleAttach(name string) int {
	if err := r.sessionSvc.Attach(name); err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func (r *Root) handleKill(name string) int {
	if err := r.sessionSvc.Kill(name); err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(r.stdout, "Killed session: %s\n", domainsession.NormalizeSessionName(name))
	return 0
}
