package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/kiddingbaby/agx/internal/usecase"
	"github.com/spf13/cobra"
)

const internalAPIKeyCommand = "__api-key"

type Root struct {
	profiles *usecase.ProfileService
	build    BuildInfo
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
	isTTY    func() bool
	native   nativeRuntime
}

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func (info BuildInfo) normalized() BuildInfo {
	if strings.TrimSpace(info.Version) == "" {
		info.Version = "dev"
	}
	if strings.TrimSpace(info.Commit) == "" {
		info.Commit = "unknown"
	}
	if strings.TrimSpace(info.Date) == "" {
		info.Date = "unknown"
	}
	return info
}

func New(profiles *usecase.ProfileService, build BuildInfo) *Root {
	root := &Root{
		profiles: profiles,
		build:    build.normalized(),
		stdin:    os.Stdin,
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		native:   execNativeRuntime{},
	}
	root.isTTY = func() bool {
		return isTerminalReader(root.stdin)
	}
	return root
}

func (r *Root) Execute(args []string) int {
	if len(args) == 0 {
		r.printHelp()
		return 0
	}

	switch args[0] {
	case "-h", "--help":
		if len(args) == 1 {
			r.printHelp()
			return 0
		}
	case "--version":
		r.printVersion()
		return 0
	}

	return r.executeCobra(args)
}

func (r *Root) executeCobra(args []string) int {
	cmd := r.newCobraRoot()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		var exitErr exitCodeError
		if errors.As(err, &exitErr) {
			return exitErr.code
		}
		fmt.Fprintln(r.stderr, err)
		return 1
	}
	return 0
}

func (r *Root) cobraHelpFunc(cmd *cobra.Command, _ []string) {
	if cmd.Name() == "agx" {
		r.printHelp()
		return
	}
	_ = cmd.Usage()
}

func (r *Root) printHelp() {
	r.printOnboardingBannerIfEmpty()
	fmt.Fprintln(r.stdout, "AGX - Local Multi-Agent Runtime Manager")
	fmt.Fprintln(r.stdout)
	fmt.Fprintln(r.stdout, "Usage:")
	fmt.Fprintln(r.stdout, "  agx add <profile> --base-url URL --api-key KEY [--model MODEL]")
	fmt.Fprintln(r.stdout, "  agx edit <profile> [--name NEW_NAME] [--base-url URL] [--api-key KEY] [--model MODEL]")
	fmt.Fprintln(r.stdout, "  agx rm <profile>")
	fmt.Fprintln(r.stdout, "  agx ls")
	fmt.Fprintln(r.stdout, "  agx show <profile>")
	fmt.Fprintln(r.stdout, "  agx use <profile>")
	fmt.Fprintln(r.stdout, "  agx current")
	fmt.Fprintln(r.stdout, "  agx detach <agent> <profile>")
	fmt.Fprintln(r.stdout, "  agx run <agent> [profile] [-- native args...]")
	fmt.Fprintln(r.stdout, "  agx restore <agent>")
	fmt.Fprintln(r.stdout, "  agx backup <agent>")
	fmt.Fprintln(r.stdout, "  agx doctor")
	fmt.Fprintln(r.stdout, "  agx version")
	fmt.Fprintln(r.stdout, "  agx completion {bash|zsh|fish|powershell}")
	fmt.Fprintln(r.stdout)
	fmt.Fprintln(r.stdout, "Examples:")
	fmt.Fprintln(r.stdout, "  agx add work --base-url https://relay.example/v1 --api-key sk-live")
	fmt.Fprintln(r.stdout, "  agx use work")
	fmt.Fprintln(r.stdout, "  agx run codex")
	fmt.Fprintln(r.stdout, "  agx run claude work -- --print 'say hi'")
	fmt.Fprintln(r.stdout, "  agx restore codex")
	fmt.Fprintln(r.stdout)
	fmt.Fprintln(r.stdout, "Agents: codex, claude, gemini, opencode")
}

// printOnboardingBannerIfEmpty prints a short "you have no profiles, start
// here" banner above the standard help when the profile store is empty.
// First-run UX: avoid leaving newcomers staring at a generic usage list.
func (r *Root) printOnboardingBannerIfEmpty() {
	if r.profiles == nil {
		return
	}
	summaries, _, err := r.profiles.ListManagedProfiles()
	if err != nil || len(summaries) > 0 {
		return
	}
	fmt.Fprintln(r.stdout, "Welcome to agx. You have no profiles yet. Start with:")
	fmt.Fprintln(r.stdout, "  agx add <name> --base-url <url> --api-key <key>")
	fmt.Fprintln(r.stdout, "  agx use <name>")
	fmt.Fprintln(r.stdout, "  agx run codex")
	fmt.Fprintln(r.stdout)
}

func (r *Root) printVersion() {
	fmt.Fprintf(r.stdout, "agx %s\n", r.build.Version)
	fmt.Fprintf(r.stdout, "commit=%s\n", r.build.Commit)
	fmt.Fprintf(r.stdout, "date=%s\n", r.build.Date)
	fmt.Fprintf(r.stdout, "go=%s\n", runtime.Version())
	fmt.Fprintf(r.stdout, "os=%s\n", runtime.GOOS)
	fmt.Fprintf(r.stdout, "arch=%s\n", runtime.GOARCH)
}
