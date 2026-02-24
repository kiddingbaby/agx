package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/kiddingbaby/agx/internal/usecase"
)

type DashboardRunner func(*usecase.KeyService, *usecase.SessionService, *usecase.LaunchService)

type KeyManagerRunner func(*usecase.KeyService)

type Handlers struct {
	RunDashboard  DashboardRunner
	RunKeyManager KeyManagerRunner
}

type Root struct {
	keySvc     *usecase.KeyService
	sessionSvc *usecase.SessionService
	launchSvc  *usecase.LaunchService

	runDashboard  DashboardRunner
	runKeyManager KeyManagerRunner

	stdout io.Writer
	stderr io.Writer
	getwd  func() (string, error)
}

func New(
	keySvc *usecase.KeyService,
	sessionSvc *usecase.SessionService,
	launchSvc *usecase.LaunchService,
	handlers Handlers,
) *Root {
	return &Root{
		keySvc:        keySvc,
		sessionSvc:    sessionSvc,
		launchSvc:     launchSvc,
		runDashboard:  handlers.RunDashboard,
		runKeyManager: handlers.RunKeyManager,
		stdout:        os.Stdout,
		stderr:        os.Stderr,
		getwd:         os.Getwd,
	}
}

func (r *Root) Execute(args []string) int {
	if len(args) == 0 {
		if r.runDashboard != nil {
			r.runDashboard(r.keySvc, r.sessionSvc, r.launchSvc)
		}
		return 0
	}

	switch args[0] {
	case "keys":
		return r.handleKeys(args[1:])
	case "ls":
		return r.handleList()
	case "attach", "a":
		if len(args) < 2 {
			fmt.Fprintln(r.stderr, "Usage: agx attach <session-name>")
			return 1
		}
		return r.handleAttach(args[1])
	case "kill":
		if len(args) < 2 {
			fmt.Fprintln(r.stderr, "Usage: agx kill <session-name>")
			return 1
		}
		return r.handleKill(args[1])
	case "help", "-h", "--help":
		r.printHelp()
		return 0
	default:
		return r.handleLaunch(args[0], args[1:])
	}
}

func (r *Root) printHelp() {
	fmt.Fprintln(r.stdout, `AGX - AI CLI Session Orchestrator

Usage:
  agx                     Open Session Dashboard (TUI)
  agx <agent> [args...]   Launch agent in current directory
  agx keys                Open Key Manager (TUI)
  agx ls                  List active AI sessions
  agx attach <name>       Attach to session (alias: a)
  agx kill <name>         Kill a session

Key Management:
  agx keys ls [--provider P]              List all keys
  agx keys add --provider P --name N --key K [--base-url URL] [--tags T]
  agx keys activate <id|name>             Activate a key
  agx keys delete <id|name>               Delete a key

Agents:
  claude-code (claude)    Claude Code CLI
  codex-cli (codex)       OpenAI Codex CLI
  gemini-cli (gemini)     Google Gemini CLI

Examples:
  agx claude              Launch claude-code in current directory
  agx claude -c           Launch with -c flag passed through
  agx keys add --provider claude --name mykey --key sk-xxx
  agx keys activate mykey`)
}
