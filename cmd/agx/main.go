package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kiddingbaby/agx/internal/key"
	"github.com/kiddingbaby/agx/internal/session"
	"github.com/kiddingbaby/agx/internal/tui"
)

// Agent aliases for quick access
var agentAliases = map[string]string{
	"claude": "claude-code",
	"codex":  "codex-cli",
	"gemini": "gemini-cli",
}

func main() {
	// Initialize key store
	store, err := initKeyStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Initialize session orchestrator
	orch, err := session.NewOrchestrator()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Parse command
	args := os.Args[1:]
	if len(args) == 0 {
		// agx → TUI Dashboard (placeholder: launches launcher for now)
		runTUI(store, orch)
		return
	}

	cmd := args[0]
	switch cmd {
	case "keys":
		handleKeys(store, args[1:])
	case "ls":
		handleList(orch)
	case "attach", "a":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: agx attach <session-name>")
			os.Exit(1)
		}
		handleAttach(orch, args[1])
	case "kill":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: agx kill <session-name>")
			os.Exit(1)
		}
		handleKill(orch, args[1])
	case "help", "-h", "--help":
		printHelp()
	default:
		// Assume it's an agent name with optional args
		handleLaunch(store, orch, cmd, args[1:])
	}
}

func initKeyStore() (*key.Store, error) {
	secret := os.Getenv("AGX_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("Error: AGX_SECRET environment variable is required (32 bytes)\nGenerate one with: openssl rand -base64 32 | head -c 32")
	}
	if len(secret) != 32 {
		return nil, fmt.Errorf("Error: AGX_SECRET must be exactly 32 bytes (got %d)", len(secret))
	}

	configDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	storePath := filepath.Join(configDir, ".config", "agx", "keys.yaml")

	return key.NewStore(storePath, []byte(secret[:32]))
}

// handleKeys handles `agx keys [sub]` commands
func handleKeys(store *key.Store, args []string) {
	if len(args) == 0 {
		// agx keys → TUI Key Manager
		runKeyManagerTUI(store)
		return
	}

	switch args[0] {
	case "ls":
		handleKeysLs(store)
	default:
		fmt.Fprintf(os.Stderr, "Unknown keys subcommand: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Usage: agx keys [ls]")
		os.Exit(1)
	}
}

// handleKeysLs lists all keys
func handleKeysLs(store *key.Store) {
	keys := store.List()
	if len(keys) == 0 {
		fmt.Println("No keys configured. Use TUI (agx keys) to add keys.")
		return
	}

	fmt.Println("Keys:")
	for _, k := range keys {
		active := " "
		if k.Active {
			active = "*"
		}
		fmt.Printf("  %s [%s] %s (%s)\n", active, k.Provider, k.Name, k.ID[:8])
	}
}

// handleList handles `agx ls` command
func handleList(orch *session.Orchestrator) {
	sessions, err := orch.ListSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Println("No active AI sessions.")
		return
	}

	fmt.Println("Active AI sessions:")
	for _, s := range sessions {
		fmt.Printf("  %s\n", s)
	}
}

// handleAttach handles `agx attach <name>` command
func handleAttach(orch *session.Orchestrator, name string) {
	sessionName := normalizeSessionName(name)
	if err := orch.Attach(sessionName); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// handleKill handles `agx kill <name>` command
func handleKill(orch *session.Orchestrator, name string) {
	sessionName := normalizeSessionName(name)
	if err := orch.KillSession(sessionName); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Killed session: %s\n", sessionName)
}

// handleLaunch handles `agx <agent> [args...]` command
func handleLaunch(store *key.Store, orch *session.Orchestrator, agentName string, args []string) {
	// Resolve alias
	if resolved, ok := agentAliases[agentName]; ok {
		agentName = resolved
	}

	// Find agent
	agent, ok := findAgent(agentName)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown agent: %s\n", agentName)
		fmt.Fprintln(os.Stderr, "Available agents: claude-code (claude), codex-cli (codex), gemini-cli (gemini)")
		os.Exit(1)
	}

	// Get active key
	activeKey, err := store.GetActive(key.Provider(agent.Provider))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: No active key for %s\n", agent.Provider)
		fmt.Fprintln(os.Stderr, "Use 'agx keys' to add and activate a key.")
		os.Exit(1)
	}

	// Get current directory
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Build command with args
	command := agent.Command
	if len(args) > 0 {
		command = command + " " + joinArgs(args)
	}

	// Launch session
	cfg := session.SessionConfig{
		Agent:   agent.Name,
		Dir:     dir,
		Command: command,
		EnvVars: map[string]string{
			agent.EnvVar: activeKey.APIKey,
		},
	}

	if err := orch.Launch(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// findAgent finds an agent by name
func findAgent(name string) (tui.Agent, bool) {
	for _, a := range tui.DefaultAgents() {
		if a.Name == name {
			return a, true
		}
	}
	return tui.Agent{}, false
}

// normalizeSessionName adds "ai-" prefix if not present
func normalizeSessionName(name string) string {
	if strings.HasPrefix(name, "ai-") {
		return name
	}
	return "ai-" + name
}

// joinArgs joins args with proper shell escaping using $'...' syntax
func joinArgs(args []string) string {
	var parts []string
	for _, arg := range args {
		parts = append(parts, escapeArg(arg))
	}
	return strings.Join(parts, " ")
}

// escapeArg uses $'...' syntax for complete escaping
func escapeArg(value string) string {
	// If no special chars, return as-is
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

func printHelp() {
	fmt.Println(`AGX - AI CLI Session Orchestrator

Usage:
  agx                     Open Session Dashboard (TUI)
  agx <agent> [args...]   Launch agent in current directory
  agx keys                Open Key Manager (TUI)
  agx keys ls             List all keys
  agx ls                  List active AI sessions
  agx attach <name>       Attach to session (alias: a)
  agx kill <name>         Kill a session

Agents:
  claude-code (claude)    Claude Code CLI
  codex-cli (codex)       OpenAI Codex CLI
  gemini-cli (gemini)     Google Gemini CLI

Examples:
  agx claude              Launch claude-code in current directory
  agx claude -c           Launch with -c flag passed through
  agx ls                  List all AI sessions
  agx attach claude       Attach to ai-claude session
  agx kill claude         Kill ai-claude session`)
}
