package cli

import (
	"fmt"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/spf13/cobra"
)

func (r *Root) newBackupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "backup <agent>",
		Short:         "Snapshot the current managed target's context directory",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 1)
			if err != nil {
				return err
			}
			agent, ok := domainprofile.ParseAgent(args[0])
			if !ok || agent == domainprofile.AgentOpenCode {
				return r.reportError(fmt.Errorf("agent must be one of: codex, claude, gemini"))
			}
			backup, err := r.profiles.BackupManagedTarget(agent, "", "")
			if err != nil {
				return r.reportError(err)
			}
			view := toContextBackupView(backup)
			if asJSON {
				return r.emitJSON(struct {
					Agent  domainprofile.Agent `json:"agent"`
					Backup contextBackupView   `json:"backup"`
				}{Agent: agent, Backup: view})
			}
			fmt.Fprintf(r.stdout, "backed up %s %s\n", agent, backup.ID)
			return nil
		},
	}
	r.addJSONOutputFlag(cmd)
	return cmd
}

func (r *Root) newRestoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "restore <agent>",
		Short:         "Restore the latest managed snapshot for the current target",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 1)
			if err != nil {
				return err
			}
			agent, ok := domainprofile.ParseAgent(args[0])
			if !ok || agent == domainprofile.AgentOpenCode {
				return r.reportError(fmt.Errorf("agent must be one of: codex, claude, gemini"))
			}
			backup, err := r.profiles.RestoreCurrentTarget(agent)
			if err != nil {
				return r.reportError(err)
			}
			view := toContextBackupView(backup)
			if asJSON {
				return r.emitJSON(struct {
					Agent  domainprofile.Agent `json:"agent"`
					Backup contextBackupView   `json:"backup"`
				}{Agent: agent, Backup: view})
			}
			fmt.Fprintf(r.stdout, "restored %s %s\n", agent, backup.ID)
			return nil
		},
	}
	r.addJSONOutputFlag(cmd)
	return cmd
}
