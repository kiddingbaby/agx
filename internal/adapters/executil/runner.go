package executil

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/kiddingbaby/agx/internal/ports"
)

var _ ports.CommandRunner = (*Runner)(nil)

// Runner executes external commands using os/exec.
type Runner struct{}

func NewRunner() *Runner {
	return &Runner{}
}

func (r *Runner) Run(name string, args []string, env map[string]string) ports.CommandResult {
	cmd := exec.Command(name, args...)
	cmd.Env = mergeEnv(os.Environ(), env)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = 1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		}
	}

	return ports.CommandResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: exitCode,
		Err:      err,
	}
}

func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(overrides))
	seen := map[string]struct{}{}
	for _, kv := range base {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			out = append(out, kv)
			continue
		}
		if _, has := overrides[k]; has {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, kv)
	}
	for k, v := range overrides {
		if _, has := seen[k]; has {
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}
