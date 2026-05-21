package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (r *Root) newDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "doctor",
		Short:         "Check AGX runtime health",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, err := r.preflight(cmd, args, 0)
			if err != nil {
				return err
			}
			report, err := r.profiles.Doctor()
			if err != nil {
				return r.reportError(err)
			}
			if asJSON {
				return r.emitJSON(report)
			}
			if report.OK {
				fmt.Fprintln(r.stdout, "ok")
				return nil
			}
			fmt.Fprintln(r.stdout, "issues:")
			for _, issue := range report.Issues {
				fmt.Fprintf(r.stdout, "- %s: %s\n", issue.Code, issue.Message)
				if issue.Action != "" {
					fmt.Fprintf(r.stdout, "  action: %s\n", issue.Action)
				}
			}
			return exitCodeError{code: 1}
		},
	}
	r.addJSONOutputFlag(cmd)
	return cmd
}
