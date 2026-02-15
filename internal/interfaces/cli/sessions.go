package cli

import (
	"fmt"

	domainsession "github.com/kiddingbaby/agx/internal/domain/session"
)

func (r *Root) handleList() int {
	sessions, err := r.sessionSvc.List()
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
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
