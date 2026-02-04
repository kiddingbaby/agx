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

// runTUI runs the main TUI (launcher + dirpicker flow)
// TODO: Replace with Dashboard in P2-7
func runTUI(store *key.Store, orch *session.Orchestrator) {
	app := tview.NewApplication()
	tui.CurrentTheme.ApplyToApp(app)

	pages := tview.NewPages()

	var selectedAgent tui.Agent

	// Create components
	launcher := tui.NewLauncher()
	dirPicker := tui.NewDirPicker("")
	keyManager := tui.NewKeyManager(store, app)

	// Status bar
	status := tview.NewTextView().
		SetDynamicColors(true).
		SetText(" [#f9e2af]AGX[#cdd6f4] | [#a6e3a1]↑↓/jk[#cdd6f4] Navigate | [#a6e3a1]Enter[#cdd6f4] Select | [#a6e3a1]Esc[#cdd6f4] Back | [#a6e3a1]K[#cdd6f4] Keys | [#a6e3a1]q[#cdd6f4] Quit")
	tui.CurrentTheme.ApplyToTextView(status)

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
		launchSessionFromTUI(app, orch, store, selectedAgent, dir)
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

// launchSessionFromTUI launches a session from the TUI
func launchSessionFromTUI(app *tview.Application, orch *session.Orchestrator, store *key.Store, agent tui.Agent, dir string) {
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
