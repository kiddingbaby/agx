package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kiddingbaby/agx/internal/usecase"
)

const internalAPIKeyCommand = "__api-key"

var supportedCommands = []string{
	"add",
	"edit",
	"ls",
	"show",
	"restore",
	"backup",
	"doctor",
	"rm",
	"version",
}

type Root struct {
	profiles *usecase.ProfileService
	build    BuildInfo
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
	isTTY    func() bool
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
	case "add":
		return r.handleAdd(args[1:])
	case "edit":
		return r.handleEdit(args[1:])
	case "ls":
		return r.handleList(args[1:])
	case "show":
		return r.handleShow(args[1:])
	case "restore":
		return r.handleRestore(args[1:])
	case "backup":
		return r.handleBackup(args[1:])
	case "doctor":
		return r.handleDoctor(args[1:])
	case "rm":
		return r.handleRemove(args[1:])
	case "version", "--version":
		r.printVersion()
		return 0
	case internalAPIKeyCommand:
		return r.handleInternalAPIKey(args[1:])
	case "help", "-h", "--help":
		r.printHelp()
		return 0
	default:
		fmt.Fprintf(r.stderr, "Error: unknown command: %s\n", args[0])
		fmt.Fprintf(r.stderr, "Supported commands: %s\n", strings.Join(supportedCommands, ", "))
		return 1
	}
}

func (r *Root) printHelp() {
	fmt.Fprint(r.stdout, `AGX - Relay Manager

Usage:
  agx                                         Show this help
  agx add <relay> --base-url URL --api-key KEY [--bind codex,claude,gemini] [-o json]
  agx edit <relay> [--base-url URL] [--api-key KEY] [--bind codex,claude,gemini] [--unbind codex,claude,gemini] [-o json]
  agx ls [--agent codex|claude|gemini] [-o json]
  agx show <relay> [-o json]
  agx restore --agent codex|claude|gemini [--to BACKUP_ID] [-o json]
  agx backup ls --agent codex|claude|gemini [-o json]
  agx doctor [-o json]
  agx rm <relay> [-o json]
  agx version

Notes:
  - A relay stores only name, base_url, and api_key.
  - add creates the relay only; agents change only when --bind/--unbind is used.
  - edit auto-syncs any agent currently bound to this relay after base_url/api_key changes.
  - --bind and --unbind accept comma-separated agent lists and may be repeated.
  - In a terminal, agx add prompts for missing values automatically.
  - In a terminal, agx edit <relay> shows current values and prompts when no edit flags are passed.
  - Use -o json on add/edit/ls/show/restore/backup ls/doctor/rm for machine-readable output.
`)
}

func (r *Root) printVersion() {
	fmt.Fprintf(r.stdout, "agx %s\n", r.build.Version)
	fmt.Fprintf(r.stdout, "commit=%s\n", r.build.Commit)
	fmt.Fprintf(r.stdout, "date=%s\n", r.build.Date)
}
