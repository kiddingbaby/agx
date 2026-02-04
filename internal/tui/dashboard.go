package tui

import (
	"fmt"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/kiddingbaby/agx/internal/key"
	"github.com/kiddingbaby/agx/internal/session"
	"github.com/rivo/tview"
)

// DashboardCallbacks holds the callback functions for dashboard actions
type DashboardCallbacks struct {
	OnAttach func(sessionName string)
	OnLaunch func(agent Agent)
	OnKill   func(sessionName string)
	OnKeys   func()
	OnQuit   func()
}

// Dashboard is the main session management TUI
type Dashboard struct {
	*tview.Flex
	orch         *session.Orchestrator
	store        *key.Store
	app          *tview.Application
	sessionTable *tview.Table
	agentList    *tview.List
	callbacks    DashboardCallbacks
	sessions     []session.SessionInfo
	focusOnAgent bool // true = focus on agent list, false = focus on session table
}

// NewDashboard creates a new session dashboard
func NewDashboard(orch *session.Orchestrator, store *key.Store, app *tview.Application, cb DashboardCallbacks) *Dashboard {
	d := &Dashboard{
		Flex:      tview.NewFlex(),
		orch:      orch,
		store:     store,
		app:       app,
		callbacks: cb,
	}

	// Session table (upper section)
	d.sessionTable = tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)
	d.sessionTable.SetTitle(" ACTIVE SESSIONS ").SetBorder(true)
	CurrentTheme.ApplyToTable(d.sessionTable)

	// Agent list (lower section)
	d.agentList = tview.NewList()
	d.agentList.SetTitle(" QUICK START ").SetBorder(true)
	d.agentList.ShowSecondaryText(false)
	CurrentTheme.ApplyToList(d.agentList)

	// Populate agents
	agents := DefaultAgents()
	for i, agent := range agents {
		idx := i
		hasKey := d.hasActiveKey(agent.Provider)
		keyStatus := "[#a6e3a1]✓[-]"
		if !hasKey {
			keyStatus = "[#f38ba8]✗ (no key)[-]"
		}
		label := fmt.Sprintf("[%d] %s  %s", i+1, agent.Name, keyStatus)
		d.agentList.AddItem(label, "", rune('1'+idx), func() {
			if cb.OnLaunch != nil {
				cb.OnLaunch(agents[idx])
			}
		})
	}

	// Layout: session table on top, agent list on bottom
	d.SetDirection(tview.FlexRow)
	d.AddItem(d.sessionTable, 0, 1, true)
	d.AddItem(d.agentList, len(agents)+2, 0, false)

	// Refresh sessions
	d.refreshSessions()

	// Key handlers for session table
	d.sessionTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			d.attachSelected()
			return nil
		case tcell.KeyTab:
			d.focusOnAgent = true
			d.app.SetFocus(d.agentList)
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'd':
				d.killSelected()
				return nil
			case 'n':
				d.focusOnAgent = true
				d.app.SetFocus(d.agentList)
				return nil
			case 'K':
				if cb.OnKeys != nil {
					cb.OnKeys()
				}
				return nil
			case 'q':
				if cb.OnQuit != nil {
					cb.OnQuit()
				}
				return nil
			case '1', '2', '3':
				idx := int(event.Rune() - '1')
				if idx < d.agentList.GetItemCount() {
					agents := DefaultAgents()
					if idx < len(agents) && cb.OnLaunch != nil {
						cb.OnLaunch(agents[idx])
					}
				}
				return nil
			}
		}
		return event
	})

	// Key handlers for agent list
	d.agentList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyEscape:
			d.focusOnAgent = false
			d.app.SetFocus(d.sessionTable)
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'K':
				if cb.OnKeys != nil {
					cb.OnKeys()
				}
				return nil
			case 'q':
				if cb.OnQuit != nil {
					cb.OnQuit()
				}
				return nil
			}
		}
		return event
	})

	return d
}

// Focus delegates focus to the appropriate child
func (d *Dashboard) Focus(delegate func(p tview.Primitive)) {
	if d.focusOnAgent {
		delegate(d.agentList)
	} else {
		delegate(d.sessionTable)
	}
}

func (d *Dashboard) refreshSessions() {
	d.sessionTable.Clear()

	sessions, err := d.orch.ListSessions()
	if err != nil {
		d.sessionTable.SetCell(0, 0, tview.NewTableCell("Error loading sessions").
			SetTextColor(CurrentTheme.Error).
			SetSelectable(false))
		return
	}

	d.sessions = sessions

	if len(sessions) == 0 {
		d.sessionTable.SetCell(0, 0, tview.NewTableCell("No active sessions").
			SetTextColor(CurrentTheme.FgMuted).
			SetSelectable(false))
		d.sessionTable.SetCell(1, 0, tview.NewTableCell("Press 1-3 or Tab to start a new agent").
			SetTextColor(CurrentTheme.FgMuted).
			SetSelectable(false))
		return
	}

	// Headers
	headers := []string{"Session", "Windows", "Status"}
	for i, h := range headers {
		d.sessionTable.SetCell(0, i, tview.NewTableCell(h).
			SetTextColor(CurrentTheme.Warning).
			SetSelectable(false).
			SetExpansion(1))
	}

	for i, s := range sessions {
		d.sessionTable.SetCell(i+1, 0, tview.NewTableCell(s.Name).
			SetTextColor(CurrentTheme.Accent))

		// Windows
		winStr := fmt.Sprintf("%d window", s.Windows)
		if s.Windows != 1 {
			winStr += "s"
		}
		d.sessionTable.SetCell(i+1, 1, tview.NewTableCell(winStr).
			SetTextColor(CurrentTheme.FgSecondary))

		// Status
		status := ""
		if s.Attached {
			status = "attached"
		}
		d.sessionTable.SetCell(i+1, 2, tview.NewTableCell(status).
			SetTextColor(CurrentTheme.Success))
	}

	d.sessionTable.Select(1, 0)
}

func (d *Dashboard) attachSelected() {
	row, _ := d.sessionTable.GetSelection()
	if row <= 0 || row > len(d.sessions) || len(d.sessions) == 0 {
		return
	}
	s := d.sessions[row-1]
	if d.callbacks.OnAttach != nil {
		d.callbacks.OnAttach(s.Name)
	}
}

func (d *Dashboard) killSelected() {
	row, _ := d.sessionTable.GetSelection()
	if row <= 0 || row > len(d.sessions) || len(d.sessions) == 0 {
		return
	}
	s := d.sessions[row-1]
	if d.callbacks.OnKill != nil {
		d.callbacks.OnKill(s.Name)
	}
	d.refreshSessions()
}

func (d *Dashboard) hasActiveKey(provider string) bool {
	_, err := d.store.GetActive(key.Provider(provider))
	return err == nil
}

// GetCwd returns the current working directory (for launching agents)
func GetCwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
