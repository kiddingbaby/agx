package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/kiddingbaby/agx/internal/usecase"
)

type Root struct {
	keySvc      *usecase.KeyService
	providerSvc *usecase.ProviderService
	switchSvc   *usecase.SwitchService
	envSyncSvc  *usecase.EnvSyncService

	stdout io.Writer
	stderr io.Writer
	getwd  func() (string, error)
}

func New(
	keySvc *usecase.KeyService,
	providerSvc *usecase.ProviderService,
	switchSvc *usecase.SwitchService,
	envSyncSvc *usecase.EnvSyncService,
) *Root {
	return &Root{
		keySvc:      keySvc,
		providerSvc: providerSvc,
		switchSvc:   switchSvc,
		envSyncSvc:  envSyncSvc,
		stdout:      os.Stdout,
		stderr:      os.Stderr,
		getwd:       os.Getwd,
	}
}

func (r *Root) Execute(args []string) int {
	if len(args) == 0 {
		return r.handleStatus(nil)
	}

	switch args[0] {
	case "use":
		return r.handleUse(args[1:])
	case "undo":
		return r.handleUndo(args[1:])
	case "init":
		return r.handleInit(args[1:])
	case "sync":
		return r.handleSync(args[1:])
	case "apply":
		return r.handleApply(args[1:])
	case "import":
		return r.handleImport(args[1:])
	case "status":
		return r.handleStatus(args[1:])
	case "get":
		return r.handleGet(args[1:])
	case "describe":
		return r.handleDescribe(args[1:])
	case "create":
		return r.handleCreate(args[1:])
	case "patch":
		return r.handlePatch(args[1:])
	case "delete":
		return r.handleDelete(args[1:])
	case "help", "-h", "--help":
		r.printHelp()
		return 0
	case "site", "add", "k", "ls", "config", "keys", "provider", "providers":
		fmt.Fprintf(r.stderr, "Error: command removed: %s\n", args[0])
		fmt.Fprintln(r.stderr, "Tip: run `agx help` for the new command layout.")
		return 1
	default:
		fmt.Fprintf(r.stderr, "Error: unknown command: %s\n", args[0])
		fmt.Fprintln(r.stderr, "Tip: did you mean `agx use <site>`?")
		return 1
	}
}

func (r *Root) printHelp() {
	fmt.Fprint(r.stdout, `AGX - AI CLI Config Switcher

Usage:
  agx                                 Show status (bindings + current site)
  agx status [-o json]                Show status
  agx use <site> [--agents codex,claude,gemini|all] [--key KEY | -l TAGS] [--dry-run] [-o json] Switch site + sync native CLI configs
  agx undo [-o json]                  Undo last switch (restore previous configs)

Resources:
  agx get sites [-o json]
  agx describe site <site> [-o json]
  agx create site <site> ...
  agx patch site <site> ...
  agx delete site <site> [-o json]

  agx get keys [--site <site>] [-A] [-l TAGS] [-o json]
  agx describe key <key> [--site <site>] [-o json]
  agx create key [name] [--site <site>] (--stdin | --api-key ... | --api-key-env ... | --api-key-file ...) [--activate] [--tags TAGS] [-o json]
  agx patch key <key> [--site <site>] [--name NEW] [--tags TAGS] [--activate] [-o json]
  agx delete key <key> [--site <site>] [-o json]

Config:
  agx init                              Generate a starter config template (~/.config/agx/agx.yml)
  agx apply [PATH|DIR] [-o json]        Apply config into AGX stores (keys/targets/bindings/profiles)
  agx sync [skills|system-prompt|mcp]   Sync global agent assets (skills/system prompts/MCP)
  agx import claude [--site <site>]     Import current Claude credentials from native config into AGX

Notes:
  - Official sites: openai | claude | gemini
  - Multi-protocol sites: if <site>-codex/<site>-claude/<site>-gemini exist, use --agents to sync multiple together (or select a subset).
  - Tip: for NewAPI/new-api, run: agx create site <name> --template newapi --agents codex,claude (skips generating unsupported endpoints).
  - After agx use <site>, key commands default to that site (no need for --site).
  - Machine output: use -o json.
`)
}
