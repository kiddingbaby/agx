package cli

import (
	"encoding/json"
	"fmt"
)

func (r *Root) handleUndo(args []string) int {
	if r.switchSvc == nil {
		fmt.Fprintln(r.stderr, "Error: undo requires switch service")
		return 1
	}

	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx undo [-o json]")
		return 0
	}

	asJSON := false
	if len(args) != 0 {
		if len(args) != 2 || args[0] != "-o" {
			fmt.Fprintln(r.stderr, "Usage: agx undo [-o json]")
			return 1
		}
		if args[1] != "json" {
			fmt.Fprintf(r.stderr, "Error: unsupported output format: %s\n", args[1])
			return 1
		}
		asJSON = true
	}

	result, err := r.switchSvc.UndoLatest()
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		fmt.Fprintln(r.stderr, "Tip: undo is only available after at least one successful `agx use <site>`.")
		return 1
	}

	if asJSON {
		payload := struct {
			Status string `json:"status"`
			Data   any    `json:"data"`
		}{
			Status: "ok",
			Data:   result,
		}
		if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
			fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(r.stdout, "Undo restored: id=%s restored=%d deleted=%d\n", result.ID, len(result.Restored), len(result.Deleted))
	for _, p := range result.Restored {
		fmt.Fprintf(r.stdout, "  restored: %s\n", p)
	}
	for _, p := range result.Deleted {
		fmt.Fprintf(r.stdout, "  deleted:  %s\n", p)
	}
	return 0
}
