package main

import (
	"crypto/rand"
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
	configDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	agxDir := filepath.Join(configDir, ".config", "agx")
	storePath := filepath.Join(agxDir, "keys.yaml")
	secretPath := filepath.Join(agxDir, "secret")

	// 1. 优先使用环境变量
	if secret := os.Getenv("AGX_SECRET"); secret != "" {
		if len(secret) != 32 {
			return nil, fmt.Errorf("AGX_SECRET must be exactly 32 bytes (got %d)", len(secret))
		}
		return key.NewStore(storePath, []byte(secret))
	}

	// 2. 尝试从文件读取
	if secretBytes, err := os.ReadFile(secretPath); err == nil && len(secretBytes) >= 32 {
		return key.NewStore(storePath, secretBytes[:32])
	}

	// 3. 检测迁移场景
	if _, err := os.Stat(storePath); err == nil {
		return nil, fmt.Errorf("found existing keys.yaml but no encryption secret\n" +
			"Migration: echo -n \"$AGX_SECRET\" > ~/.config/agx/secret")
	}

	// 4. 自动生成并保存
	if err := os.MkdirAll(agxDir, 0700); err != nil {
		return nil, fmt.Errorf("cannot create config directory: %w", err)
	}
	newSecret := make([]byte, 32)
	if _, err := rand.Read(newSecret); err != nil {
		return nil, fmt.Errorf("cannot generate secret: %w", err)
	}
	if err := os.WriteFile(secretPath, newSecret, 0600); err != nil {
		return nil, fmt.Errorf("cannot save secret: %w", err)
	}

	return key.NewStore(storePath, newSecret)
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
		handleKeysLs(store, args[1:])
	case "add":
		handleKeysAdd(store, args[1:])
	case "activate":
		handleKeysActivate(store, args[1:])
	case "delete", "rm":
		handleKeysDelete(store, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown keys subcommand: %s\n", args[0])
		printKeysUsage()
		os.Exit(1)
	}
}

func printKeysUsage() {
	fmt.Fprintln(os.Stderr, `Usage: agx keys [command]

Commands:
  ls [--provider P]              List all keys
  add --provider P --name N --key K [--base-url URL] [--tags T]  Add a new key
  activate <id|name>             Activate a key
  delete <id|name>               Delete a key

Without command: Open TUI Key Manager`)
}

// handleKeysLs lists all keys
func handleKeysLs(store *key.Store, args []string) {
	var providerFilter string
	for i := 0; i < len(args); i++ {
		if args[i] == "--provider" || args[i] == "-p" {
			if i+1 < len(args) {
				providerFilter = args[i+1]
				i++
			}
		}
	}

	keys := store.List()
	if len(keys) == 0 {
		fmt.Println("No keys configured. Use 'agx keys add' or TUI (agx keys) to add keys.")
		return
	}

	// Group by provider
	providers := []key.Provider{key.ProviderClaude, key.ProviderOpenAI, key.ProviderGemini}
	for _, provider := range providers {
		if providerFilter != "" && string(provider) != providerFilter {
			continue
		}

		fmt.Printf("\n%s:\n", strings.ToUpper(string(provider)))
		hasKeys := false
		for _, k := range keys {
			if k.Provider == provider {
				hasKeys = true
				active := " "
				if k.Active {
					active = "*"
				}
				fmt.Printf("  %s %s  (%s)\n", active, k.Name, k.ID[:8])
			}
		}
		if !hasKeys {
			fmt.Println("  (no keys)")
		}
	}
	fmt.Println()
}

// handleKeysAdd adds a new key
func handleKeysAdd(store *key.Store, args []string) {
	var provider, name, apiKey, baseURL, tagsStr string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider", "-p":
			if i+1 < len(args) {
				provider = args[i+1]
				i++
			}
		case "--name", "-n":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--key", "-k":
			if i+1 < len(args) {
				apiKey = args[i+1]
				i++
			}
		case "--base-url", "-b":
			if i+1 < len(args) {
				baseURL = args[i+1]
				i++
			}
		case "--tags", "-t":
			if i+1 < len(args) {
				tagsStr = args[i+1]
				i++
			}
		}
	}

	if provider == "" || name == "" || apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: --provider, --name, and --key are required")
		fmt.Fprintln(os.Stderr, "Usage: agx keys add --provider P --name N --key K [--base-url URL] [--tags T]")
		os.Exit(1)
	}

	// Validate provider
	validProviders := map[string]bool{"claude": true, "openai": true, "gemini": true}
	if !validProviders[provider] {
		fmt.Fprintf(os.Stderr, "Error: invalid provider '%s'. Valid: claude, openai, gemini\n", provider)
		os.Exit(1)
	}

	// Parse tags
	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	k, err := store.Add(key.Provider(provider), name, apiKey, baseURL, tags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Added key: %s (%s)\n", k.Name, k.ID[:8])
}

// handleKeysActivate activates a key
func handleKeysActivate(store *key.Store, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: key ID or name required")
		fmt.Fprintln(os.Stderr, "Usage: agx keys activate <id|name>")
		os.Exit(1)
	}

	identifier := args[0]
	k := findKeyByIdOrName(store, identifier)
	if k == nil {
		fmt.Fprintf(os.Stderr, "Error: key not found: %s\n", identifier)
		os.Exit(1)
	}

	if err := store.Activate(k.ID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Activated key: %s [%s]\n", k.Name, k.Provider)
}

// handleKeysDelete deletes a key
func handleKeysDelete(store *key.Store, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: key ID or name required")
		fmt.Fprintln(os.Stderr, "Usage: agx keys delete <id|name>")
		os.Exit(1)
	}

	identifier := args[0]
	k := findKeyByIdOrName(store, identifier)
	if k == nil {
		fmt.Fprintf(os.Stderr, "Error: key not found: %s\n", identifier)
		os.Exit(1)
	}

	if err := store.Delete(k.ID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deleted key: %s\n", k.Name)
}

// findKeyByIdOrName finds a key by ID (partial match) or name
func findKeyByIdOrName(store *key.Store, identifier string) *key.Key {
	keys := store.List()
	for i := range keys {
		k := &keys[i]
		// Match by ID prefix
		if strings.HasPrefix(k.ID, identifier) {
			return k
		}
		// Match by name (exact)
		if k.Name == identifier {
			return k
		}
	}
	return nil
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
		attached := ""
		if s.Attached {
			attached = " (attached)"
		}
		fmt.Printf("  %s  %d windows%s\n", s.Name, s.Windows, attached)
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

	cfg := buildSessionConfig(agent, activeKey, dir, command)

	if err := orch.Launch(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func buildSessionConfig(agent tui.Agent, activeKey *key.Key, dir, command string) session.SessionConfig {
	cfg := session.SessionConfig{
		Agent:   agent.Name,
		Dir:     dir,
		Command: command,
		EnvVars: map[string]string{
			agent.EnvVar: activeKey.APIKey,
		},
	}
	if activeKey.BaseURL != "" && agent.BaseURLEnvVar != "" {
		cfg.EnvVars[agent.BaseURLEnvVar] = activeKey.BaseURL
	}
	return cfg
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
