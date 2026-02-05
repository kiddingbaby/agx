package main

import (
	"fmt"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/kiddingbaby/agx/internal/key"
	"github.com/kiddingbaby/agx/internal/session"
	"github.com/kiddingbaby/agx/internal/tui"
	"github.com/rivo/tview"
)

// runTUI runs the main TUI Dashboard
func runTUI(store *key.Store, orch *session.Orchestrator) {
	app := tview.NewApplication()
	tui.CurrentTheme.ApplyToApp(app)

	pages := tview.NewPages()

	// Key manager
	keyManager := tui.NewKeyManager(store, app)

	// Dashboard callbacks
	callbacks := tui.DashboardCallbacks{
		OnAttach: func(sessionName string) {
			app.Stop()
			if err := orch.Attach(sessionName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
		OnLaunch: func(agent tui.Agent) {
			launchAgentFromDashboard(app, orch, store, agent)
		},
		OnKill: func(sessionName string) {
			_ = orch.KillSession(sessionName)
		},
		OnKeys: func() {
			pages.SwitchToPage("keymgr")
			app.SetFocus(keyManager)
		},
		OnQuit: func() {
			app.Stop()
		},
	}

	// Dashboard
	dashboard := tui.NewDashboard(orch, store, app, callbacks)

	// Setup key manager
	keyManager.SetOnClose(func() {
		pages.SwitchToPage("dashboard")
		app.SetFocus(dashboard)
	})

	// Add pages
	pages.AddPage("dashboard", dashboard, true, true)
	pages.AddPage("keymgr", keyManager, true, false)

	// Status bar
	status := tview.NewTextView().
		SetDynamicColors(true).
		SetText(" [#f9e2af]AGX[#cdd6f4] | [#a6e3a1]Enter[#cdd6f4] Attach | [#a6e3a1]1-3[#cdd6f4] Launch | [#a6e3a1]d[#cdd6f4] Kill | [#a6e3a1]K[#cdd6f4] Keys | [#a6e3a1]Tab[#cdd6f4] Switch | [#a6e3a1]q[#cdd6f4] Quit")
	tui.CurrentTheme.ApplyToTextView(status)

	// Main layout
	mainLayout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true).
		AddItem(status, 1, 0, false)

	// Global key handler for K to open key manager
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == 'K' {
			name, _ := pages.GetFrontPage()
			if name == "dashboard" {
				pages.SwitchToPage("keymgr")
				app.SetFocus(keyManager)
				return nil
			}
		}
		return event
	})

	if err := app.SetRoot(mainLayout, true).EnableMouse(false).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runKeyManagerTUI runs just the key manager TUI
func runKeyManagerTUI(store *key.Store) {
	app := tview.NewApplication()
	tui.CurrentTheme.ApplyToApp(app)

	keyManager := tui.NewKeyManager(store, app)
	keyManager.SetOnClose(func() {
		app.Stop()
	})

	// Status bar
	status := tview.NewTextView().
		SetDynamicColors(true).
		SetText(" [#f9e2af]Key Manager[#cdd6f4] | [#a6e3a1]↑↓/jk[#cdd6f4] Navigate | [#a6e3a1]Enter[#cdd6f4] Activate | [#a6e3a1]a[#cdd6f4] Add | [#a6e3a1]d[#cdd6f4] Delete | [#a6e3a1]Esc[#cdd6f4] Quit")
	tui.CurrentTheme.ApplyToTextView(status)

	mainLayout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(keyManager, 0, 1, true).
		AddItem(status, 1, 0, false)

	if err := app.SetRoot(mainLayout, true).EnableMouse(false).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// launchAgentFromDashboard launches an agent from the dashboard
func launchAgentFromDashboard(app *tview.Application, orch *session.Orchestrator, store *key.Store, agent tui.Agent) {
	// Get active key for the agent's provider
	activeKey, err := store.GetActive(key.Provider(agent.Provider))
	if err != nil {
		// Show error in status (modal has complex focus issues)
		// Just print error and continue - user can press K to add keys
		fmt.Fprintf(os.Stderr, "No active key for %s. Use 'agx keys' to add one.\n", agent.Provider)
		return
	}

	// Stop TUI before launching tmux
	app.Stop()

	// Get current directory
	dir := tui.GetCwd()

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
