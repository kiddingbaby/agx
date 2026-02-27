package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kiddingbaby/agx/internal/usecase"
)

func (r *Root) handleLaunch(agentName string, args []string) int {
	dir, err := r.getwd()
	if err != nil {
		fmt.Fprintf(r.stderr, "Error: %v\n", err)
		return 1
	}

	var (
		profile       string
		keyIdentifier string
		passArgs      []string
	)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--profile", "-p":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --profile requires a value")
				return 1
			}
			profile = args[i+1]
			i++
		case "--key", "-k":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --key requires a value")
				return 1
			}
			keyIdentifier = args[i+1]
			i++
		default:
			passArgs = append(passArgs, args[i])
		}
	}

	extraArgs := ""
	if len(passArgs) > 0 {
		extraArgs = JoinArgs(passArgs)
	}

	err = r.launchSvc.LaunchWithOptions(agentName, dir, usecase.LaunchOptions{
		Profile:       profile,
		KeyIdentifier: keyIdentifier,
		ExtraArgs:     extraArgs,
	})
	if err == nil {
		return 0
	}

	if usecase.IsUnknownAgentError(err) {
		fmt.Fprintf(r.stderr, "Unknown agent: %s\n", agentName)
		fmt.Fprintln(r.stderr, "Available agents: claude-code (claude), codex-cli (codex), gemini-cli (gemini)")
		return 1
	}
	if usecase.IsNoActiveKeyError(err) {
		var noActive *usecase.NoActiveKeyError
		if errors.As(err, &noActive) {
			fmt.Fprintf(r.stderr, "Error: No active key for %s\n", noActive.Provider)
		} else {
			fmt.Fprintln(r.stderr, "Error: No active key")
		}
		fmt.Fprintln(r.stderr, "Use 'agx keys' to add and activate a key.")
		return 1
	}
	fmt.Fprintf(r.stderr, "Error: %v\n", err)
	return 1
}

func JoinArgs(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, EscapeArg(arg))
	}
	return strings.Join(parts, " ")
}

func EscapeArg(value string) string {
	needsEscape := false
	for _, r := range value {
		switch r {
		case '\'', '\\', '"', '$', '`', '\n', '\r', '\t', ' ', '!', '*', '?', '[', ']', '(', ')', '{', '}', '|', '&', ';', '<', '>':
			needsEscape = true
		}
	}
	if !needsEscape {
		return value
	}

	var b strings.Builder
	b.WriteString("$'")
	for _, r := range value {
		switch r {
		case '\'':
			b.WriteString("\\'")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString("'")
	return b.String()
}
