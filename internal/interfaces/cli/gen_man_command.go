package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newGenManCommand exposes a hidden helper that writes one roff(7) man page
// per subcommand into the requested directory. The output is consumed by
// `task docs:man` and is also what goreleaser packages into the Homebrew
// formula so that `man agx` works after `brew install`.
//
// Header date is pinned to the Unix epoch so regenerated man pages diff
// only when the CLI itself changes. Source string is kept generic
// ("agx") rather than the build version so dev rebuilds don't churn the
// committed man pages either.
func (r *Root) newGenManCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "__gen-man <output-dir>",
		Short:         "Generate man pages (internal helper)",
		Hidden:        true,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: agx __gen-man <output-dir>")
			}
			outDir := args[0]
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}
			pinned := time.Unix(0, 0).UTC()
			header := &doc.GenManHeader{
				Title:   "AGX",
				Section: "1",
				Source:  "agx",
				Manual:  "agx Manual",
				Date:    &pinned,
			}
			root := cmd.Root()
			if err := doc.GenManTree(root, header, outDir); err != nil {
				return err
			}
			fmt.Fprintf(r.stdout, "wrote man pages to %s\n", filepath.Clean(outDir))
			return nil
		},
	}
	return cmd
}
