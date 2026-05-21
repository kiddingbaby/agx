package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"github.com/spf13/cobra"
)

const cobraOutputFlagName = "output"
const timeFormat = "2006-01-02T15:04:05Z07:00"

var (
	agentUsageList  = domainprofile.SupportedAgents()
	agentUsageHuman = strings.Join(agentsToStrings(agentUsageList), ", ")
)

var errInteractiveCanceled = errors.New("interactive input canceled")

type contextBackupView struct {
	ID         string                   `json:"id"`
	TargetKind domainprofile.TargetKind `json:"target_kind"`
	TargetName string                   `json:"target_name"`
	Path       string                   `json:"path,omitempty"`
	CreatedAt  string                   `json:"created_at,omitempty"`
}

func (r *Root) newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "version",
		Short:         "Print version information",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				_ = cmd.Usage()
				return exitCodeError{code: 1}
			}
			r.printVersion()
			return nil
		},
	}
}

func (r *Root) newInternalAPIKeyCommand() *cobra.Command {
	return &cobra.Command{
		Use:                internalAPIKeyCommand + " <profile>",
		Hidden:             true,
		DisableFlagParsing: true,
		SilenceErrors:      true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := r.runInternalAPIKey(args); code != 0 {
				return exitCodeError{code: code}
			}
			return nil
		},
	}
}

func (r *Root) addJSONOutputFlag(cmd *cobra.Command) {
	cmd.Flags().StringP(cobraOutputFlagName, "o", "", "machine-readable output format")
}

func (r *Root) commandJSONOutput(cmd *cobra.Command) (bool, bool) {
	value, err := cmd.Flags().GetString(cobraOutputFlagName)
	if err != nil {
		r.printInvalidJSONOutputError()
		return false, false
	}
	if value == "" {
		return false, true
	}
	if value != "json" {
		r.printInvalidJSONOutputError()
		return false, false
	}
	return true, true
}

func (r *Root) writeJSON(payload any) int {
	if err := json.NewEncoder(r.stdout).Encode(payload); err != nil {
		fmt.Fprintf(r.stderr, "Error: failed to encode JSON output: %v\n", err)
		return 1
	}
	return 0
}

func (r *Root) printInvalidJSONOutputError() {
	fmt.Fprintln(r.stderr, "Error: -o requires value json")
}

func (r *Root) usageError(cmd *cobra.Command) error {
	_ = cmd.Usage()
	return exitCodeError{code: 1}
}

func (r *Root) reportError(err error) error {
	r.printUserError(err)
	return exitCodeError{code: 1}
}

func (r *Root) emitJSON(payload any) error {
	return exitCodeError{code: r.writeJSON(payload)}
}

// preflight performs the cobra-command setup that almost every profile command
// repeats: parse the -o/--output flag and validate arg count. wantArgs of -1
// disables the arg-count check; wantArgs of 1 additionally requires the single
// arg to be non-empty after trimming.
func (r *Root) preflight(cmd *cobra.Command, args []string, wantArgs int) (bool, error) {
	asJSON, ok := r.commandJSONOutput(cmd)
	if !ok {
		return false, exitCodeError{code: 1}
	}
	if wantArgs >= 0 {
		if len(args) != wantArgs {
			return false, r.usageError(cmd)
		}
		if wantArgs == 1 && strings.TrimSpace(args[0]) == "" {
			return false, r.usageError(cmd)
		}
	}
	return asJSON, nil
}

func (r *Root) nativeError(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitCodeError{code: exitErr.ExitCode()}
	}
	return err
}

func (r *Root) runAgentNative(agent domainprofile.Agent, args []string) error {
	_, contextPath, err := r.profiles.CurrentTargetContext(agent)
	if err != nil {
		r.printUserError(err)
		return err
	}
	return r.native.Run(agent, contextPath, args, r.stdin, r.stdout, r.stderr)
}

func (r *Root) runInternalAPIKey(args []string) int {
	if r.profiles == nil {
		fmt.Fprintln(r.stderr, "Error: profile service is unavailable")
		return 1
	}
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(r.stderr, "Error: internal api key helper requires exactly one profile name")
		return 1
	}

	apiKey, err := r.profiles.APIKey(args[0])
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintln(r.stdout, apiKey)
	return 0
}

func toContextBackupView(backup domainprofile.ContextBackup) contextBackupView {
	view := contextBackupView{
		ID:         backup.ID,
		TargetKind: backup.TargetKind,
		TargetName: backup.TargetName,
		Path:       backup.Path,
	}
	if !backup.CreatedAt.IsZero() {
		view.CreatedAt = backup.CreatedAt.Format(timeFormat)
	}
	return view
}

func isTerminalReader(reader io.Reader) bool {
	_, ok := reader.(interface{ Fd() uintptr })
	return ok
}

func agentsToStrings(agents []domainprofile.Agent) []string {
	out := make([]string, 0, len(agents))
	for _, agent := range agents {
		out = append(out, string(agent))
	}
	return out
}
