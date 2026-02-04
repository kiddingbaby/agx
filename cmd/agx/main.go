package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/kiddingbaby/agx/internal/key"
	"github.com/kiddingbaby/agx/internal/session"
	"github.com/kiddingbaby/agx/internal/tui"
	"github.com/rivo/tview"
)

func main() {
	var (
		agentFlag string
		dirFlag   string
		keyMgr    bool
	)

	flag.StringVar(&agentFlag, "agent", "", "Agent to use (claude-code, codex-cli, gemini-cli)")
	flag.StringVar(&dirFlag, "dir", "", "Working directory")
	flag.BoolVar(&keyMgr, "keys", false, "Open key manager")
	flag.Parse()

	// Initialize key store
	secret := os.Getenv("AGX_SECRET")
	if secret == "" {
		secret = "agx-default-secret-key-32bytes!" // Default for development
	}
	if len(secret) < 32 {
		secret = secret + "00000000000000000000000000000000"[:32-len(secret)]
	}

	configDir, _ := os.UserHomeDir()
	storePath := filepath.Join(configDir, ".config", "agx", "keys.yaml")

	store, err := key.NewStore(storePath, []byte(secret[:32]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing key store: %v\n", err)
		os.Exit(1)
	}

	// Initialize session orchestrator
	orch, err := session.NewOrchestrator()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create TUI application
	app := tview.NewApplication()

	// Pages for navigation
	pages := tview.NewPages()

	var selectedAgent tui.Agent
	var selectedDir string

	// Create components
	launcher := tui.NewLauncher()
	dirPicker := tui.NewDirPicker("")
	keyManager := tui.NewKeyManager(store, app)

	// Status bar
	status := tview.NewTextView().
		SetDynamicColors(true).
		SetText(" [yellow]AGX[white] | [green]↑↓/jk[white] Navigate | [green]Enter[white] Select | [green]Esc[white] Back | [green]K[white] Keys | [green]q[white] Quit")

	// Main layout
	mainLayout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true).
		AddItem(status, 1, 0, false)

	// Setup launcher
	launcher.SetOnSelect(func(agent tui.Agent) {
		selectedAgent = agent
		pages.SwitchToPage("dirpicker")
		app.SetFocus(dirPicker)
	})
	launcher.SetOnCancel(func() {
		app.Stop()
	})

	// Setup directory picker
	dirPicker.SetOnSelect(func(dir string) {
		selectedDir = dir
		launchSession(app, orch, store, selectedAgent, selectedDir)
	})
	dirPicker.SetOnCancel(func() {
		pages.SwitchToPage("launcher")
		app.SetFocus(launcher)
	})

	// Setup key manager
	keyManager.SetOnClose(func() {
		pages.SwitchToPage("launcher")
		app.SetFocus(launcher)
	})

	// Add pages
	pages.AddPage("launcher", launcher, true, true)
	pages.AddPage("dirpicker", dirPicker, true, false)
	pages.AddPage("keymgr", keyManager, true, false)

	// Global key handler
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && (event.Rune() == 'K' || event.Rune() == 'M') {
			pages.SwitchToPage("keymgr")
			app.SetFocus(keyManager)
			return nil
		}
		return event
	})

	// Handle command line flags
	if keyMgr {
		pages.SwitchToPage("keymgr")
		app.SetFocus(keyManager)
	}

	if err := app.SetRoot(mainLayout, true).EnableMouse(false).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func launchSession(app *tview.Application, orch *session.Orchestrator, store *key.Store, agent tui.Agent, dir string) {
	// Get active key for the agent's provider
	activeKey, err := store.GetActive(key.Provider(agent.Provider))
	if err != nil {
		// Show error modal
		modal := tview.NewModal().
			SetText(fmt.Sprintf("No active key for %s.\nPlease add and activate a key first.", agent.Provider)).
			AddButtons([]string{"OK"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				app.Stop()
			})
		app.SetRoot(modal, true)
		return
	}

	// Stop TUI before launching tmux
	app.Stop()

	// Launch session
	cfg := session.SessionConfig{
		Agent:   agent.Name,
		Dir:     dir,
		Command: agent.Command,
		EnvVars: map[string]string{
			agent.EnvVar: activeKey.APIKey,
		},
	}

	if err := orch.Launch(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error launching session: %v\n", err)
		os.Exit(1)
	}
}
