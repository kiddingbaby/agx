package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Agent represents an AI CLI tool
type Agent struct {
	Name    string
	Command string
	EnvVar  string
	Provider string
}

// DefaultAgents returns the list of supported AI CLI tools
func DefaultAgents() []Agent {
	return []Agent{
		{Name: "claude-code", Command: "claude", EnvVar: "ANTHROPIC_API_KEY", Provider: "claude"},
		{Name: "codex-cli", Command: "codex", EnvVar: "OPENAI_API_KEY", Provider: "openai"},
		{Name: "gemini-cli", Command: "gemini", EnvVar: "GOOGLE_API_KEY", Provider: "gemini"},
	}
}

// Launcher is the agent selection component
type Launcher struct {
	*tview.List
	agents   []Agent
	onSelect func(Agent)
	onCancel func()
}

// NewLauncher creates a new agent launcher
func NewLauncher() *Launcher {
	l := &Launcher{
		List:   tview.NewList(),
		agents: DefaultAgents(),
	}

	l.SetTitle(" Select Agent ").SetBorder(true)
	l.ShowSecondaryText(false)
	CurrentTheme.ApplyToList(l.List)

	for i, agent := range l.agents {
		idx := i
		l.AddItem(agent.Name, "", rune('1'+i), func() {
			if l.onSelect != nil {
				l.onSelect(l.agents[idx])
			}
		})
	}

	l.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			if l.onCancel != nil {
				l.onCancel()
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				curr := l.GetCurrentItem()
				if curr < l.GetItemCount()-1 {
					l.SetCurrentItem(curr + 1)
				}
				return nil
			case 'k':
				curr := l.GetCurrentItem()
				if curr > 0 {
					l.SetCurrentItem(curr - 1)
				}
				return nil
			case 'q':
				if l.onCancel != nil {
					l.onCancel()
				}
				return nil
			}
		}
		return event
	})

	return l
}

// SetOnSelect sets the callback when an agent is selected
func (l *Launcher) SetOnSelect(fn func(Agent)) *Launcher {
	l.onSelect = fn
	return l
}

// SetOnCancel sets the callback when selection is cancelled
func (l *Launcher) SetOnCancel(fn func()) *Launcher {
	l.onCancel = fn
	return l
}
