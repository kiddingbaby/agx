package cli

import (
	"fmt"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/kiddingbaby/agx/internal/usecase"
)

func (r *Root) handleRestore(args []string) int {
	if r.profiles == nil {
		fmt.Fprintln(r.stderr, "Error: relay service is unavailable")
		return 1
	}
	if hasHelpToken(args) {
		fmt.Fprintln(r.stdout, "Usage: agx restore --agent codex|claude|gemini [--to BACKUP_ID] [-o json]")
		return 0
	}

	agent, backupID, asJSON, ok := parseRestoreArgs(r, args, "Usage: agx restore --agent codex|claude|gemini [--to BACKUP_ID] [-o json]")
	if !ok {
		return 1
	}

	result, err := r.profiles.Restore(agent, backupID)
	if err != nil {
		r.printUserError(err)
		return 1
	}
	return r.writeRestoreResult(result, asJSON, false)
}

func (r *Root) writeBindingsResult(result *usecase.BindingsResult, asJSON bool) int {
	if asJSON {
		type bindingChangeView struct {
			Agent         domainprofile.Agent `json:"agent"`
			Action        string              `json:"action"`
			Binding       *bindingView        `json:"binding,omitempty"`
			Backup        backupView          `json:"backup"`
			CodexProfile  string              `json:"codex_profile,omitempty"`
		}
		changes := make([]bindingChangeView, 0, len(result.Changed))
		for _, change := range result.Changed {
			var binding *bindingView
			if change.Binding != nil {
				view := toBindingView(change.Agent, *change.Binding)
				binding = &view
			}
			changes = append(changes, bindingChangeView{
				Agent:        change.Agent,
				Action:       change.Action,
				Binding:      binding,
				Backup:       toBackupView(change.Backup),
				CodexProfile: change.CodexProfile,
			})
		}
		return r.writeJSON(struct {
			Relay   profileView        `json:"relay"`
			Changes []bindingChangeView `json:"changes"`
		}{
			Relay:   toProfileView(*result.Relay),
			Changes: changes,
		})
	}

	fmt.Fprintf(r.stdout, "Updated relay bindings: %s\n", result.Relay.Name)
	for _, change := range result.Changed {
		fmt.Fprintf(r.stdout, "  %s %s backup_id=%s restore_mode=%s", change.Action, change.Agent, change.Backup.ID, change.Backup.RestoreMode)
		if change.Binding != nil && change.Binding.SourceProfile != "" {
			fmt.Fprintf(r.stdout, " relay=%s", change.Binding.SourceProfile)
		}
		fmt.Fprintln(r.stdout)
	}
	return 0
}

func (r *Root) writeRestoreResult(result *usecase.RestoreResult, asJSON, cleared bool) int {
	if asJSON {
		return r.writeJSON(struct {
			Agent      domainprofile.Agent `json:"agent"`
			ConfigPath string              `json:"config_path"`
			Backup     backupView          `json:"backup"`
		}{
			Agent:      result.Agent,
			ConfigPath: result.ConfigPath,
			Backup:     toBackupView(result.Backup),
		})
	}

	if cleared {
		fmt.Fprintf(r.stdout, "Unbound agent: %s\n", result.Agent)
	} else {
		fmt.Fprintf(r.stdout, "Restored agent: %s\n", result.Agent)
	}
	fmt.Fprintf(r.stdout, "  config_path=%s\n", result.ConfigPath)
	fmt.Fprintf(r.stdout, "  backup_id=%s\n", result.Backup.ID)
	fmt.Fprintf(r.stdout, "  restore_mode=%s\n", result.Backup.RestoreMode)
	if result.Backup.BackupPath != "" {
		fmt.Fprintf(r.stdout, "  backup_path=%s\n", result.Backup.BackupPath)
	}
	return 0
}
