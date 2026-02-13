package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kiddingbaby/agx/internal/key"
	"github.com/kiddingbaby/agx/internal/session"
	"github.com/kiddingbaby/agx/internal/tui"
)

// runTUI runs the main TUI Dashboard
func runTUI(store *key.Store, orch *session.Orchestrator) {
	for {
		callbacks := tui.DashboardCallbacks{
			OnAttach: func(sessionName string) {
				// Will be called before tea.Quit, stored for post-quit action
			},
			OnLaunch: func(agent tui.Agent) {
				// Will be called before tea.Quit, stored for post-quit action
			},
			OnKill: func(sessionName string) {
				_ = orch.KillSession(sessionName)
			},
		}

		model := tui.NewDashboardModel(orch, store, callbacks)

		// Track post-quit actions via closures
		var postAction func()

		callbacks.OnAttach = func(sessionName string) {
			postAction = func() {
				if err := orch.Attach(sessionName); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
			}
		}
		callbacks.OnLaunch = func(agent tui.Agent) {
			postAction = func() {
				launchAgentFromTUI(orch, store, agent)
			}
		}

		// Recreate model with updated callbacks
		model = tui.NewDashboardModel(orch, store, callbacks)

		p := tea.NewProgram(model, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		dm := finalModel.(tui.DashboardModel)

		// Check if we should switch to key manager
		if dm.ShouldSwitchToKeys() {
			runKeyManagerTUI(store)
			continue // Return to dashboard after key manager
		}

		// Execute post-quit action (attach or launch)
		if postAction != nil {
			postAction()
		}
		return
	}
}

// runKeyManagerTUI runs the key manager TUI
func runKeyManagerTUI(store *key.Store) {
	model := tui.NewKeyManagerModel(store)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// launchAgentFromTUI launches an agent from the TUI
func launchAgentFromTUI(orch *session.Orchestrator, store *key.Store, agent tui.Agent) {
	activeKey, err := store.GetActive(key.Provider(agent.Provider))
	if err != nil {
		fmt.Fprintf(os.Stderr, "No active key for %s. Use 'agx keys' to add one.\n", agent.Provider)
		os.Exit(1)
	}

	dir := tui.GetCwd()

	cfg := session.SessionConfig{
		Agent:   agent.Name,
		Dir:     dir,
		Command: agent.Command,
		EnvVars: map[string]string{
			agent.EnvVar: activeKey.APIKey,
		},
	}

	// Inject Base URL if set
	if activeKey.BaseURL != "" && agent.BaseURLEnvVar != "" {
		cfg.EnvVars[agent.BaseURLEnvVar] = activeKey.BaseURL
	}

	if err := orch.Launch(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error launching session: %v\n", err)
		os.Exit(1)
	}
}
