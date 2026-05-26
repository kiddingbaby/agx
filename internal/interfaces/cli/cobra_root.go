package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

type exitCodeError struct {
	code int
}

func (e exitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.code)
}

func (r *Root) newCobraRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "agx",
		Short: "Local multi-agent launcher: one relay profile for codex, claude, gemini, opencode",
		Long: `agx is a single-user CLI that manages OpenAI-compatible relay profiles
(base_url + api_key) and launches each AI coding agent inside an isolated
managed context. Every mutation is captured and rollback-safe, so 'agx
backup' / 'agx restore' / 'agx doctor' can always recover from a half-
applied change.`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetIn(r.stdin)
	root.SetOut(r.stdout)
	root.SetErr(r.stderr)
	root.SetHelpFunc(r.cobraHelpFunc)
	// Keep cobra's auto-generated `completion` subcommand. It produces
	// portable bash/zsh/fish/powershell completion scripts and is the
	// expected way to register shell completion for any cobra CLI.

	root.AddCommand(
		r.newProfileAddCommand(),
		r.newProfileEditCommand(),
		r.newProfileRemoveCommand(),
		r.newProfileListCommand(),
		r.newProfileShowCommand(),
		r.newProfileUseCommand(),
		r.newProfileCurrentCommand(),
		r.newDetachCommand(),
		r.newRunCommand(),
		r.newLauncherCommand("codex"),
		r.newLauncherCommand("claude"),
		r.newLauncherCommand("gemini"),
		r.newLauncherCommand("opencode"),
		r.newRestoreCommand(),
		r.newBackupCommand(),
		r.newDoctorCommand(),
		r.newVersionCommand(),
		r.newGenManCommand(),
		r.newInternalAPIKeyCommand(),
		r.newMcpCommand(),
	)
	return root
}
