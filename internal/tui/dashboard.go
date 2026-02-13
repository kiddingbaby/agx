package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	ks "github.com/kiddingbaby/agx/internal/key"
	"github.com/kiddingbaby/agx/internal/session"
)

// Focus tracks which panel has focus
type Focus int

const (
	FocusSessions Focus = iota
	FocusAgents
)

// DashboardCallbacks holds callback functions for dashboard actions
type DashboardCallbacks struct {
	OnAttach func(sessionName string)
	OnLaunch func(agent Agent)
	OnKill   func(sessionName string)
	OnKeys   func()
}

// sessionsMsg carries the result of listing sessions
type sessionsMsg struct {
	sessions []session.SessionInfo
	err      error
}

// DashboardModel is the Bubble Tea model for the session dashboard
type DashboardModel struct {
	orch      *session.Orchestrator
	store     *ks.Store
	callbacks DashboardCallbacks

	sessions  []session.SessionInfo
	agents    []Agent
	focus     Focus
	cursor    int // cursor in the focused list
	loading   bool
	err       error
	spinner   spinner.Model
	width     int
	height    int
	quitting  bool
	switchKey bool // signal to switch to key manager
}

// NewDashboardModel creates a new dashboard model
func NewDashboardModel(orch *session.Orchestrator, store *ks.Store, cb DashboardCallbacks) DashboardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(Accent)

	return DashboardModel{
		orch:      orch,
		store:     store,
		callbacks: cb,
		agents:    DefaultAgents(),
		spinner:   s,
		loading:   true,
	}
}

// ShouldSwitchToKeys returns true if user requested key manager
func (m DashboardModel) ShouldSwitchToKeys() bool {
	return m.switchKey
}

func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchSessions())
}

func (m DashboardModel) fetchSessions() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.orch.ListSessions()
		return sessionsMsg{sessions: sessions, err: err}
	}
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sessionsMsg:
		m.loading = false
		m.sessions = msg.sessions
		m.err = msg.err
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m DashboardModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// Global keys
	switch k {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "K":
		m.switchKey = true
		return m, tea.Quit
	}

	// Number keys for quick launch (always available)
	if len(k) == 1 && k[0] >= '1' && k[0] <= '3' {
		idx := int(k[0] - '1')
		if idx < len(m.agents) && m.callbacks.OnLaunch != nil {
			m.callbacks.OnLaunch(m.agents[idx])
			return m, tea.Quit
		}
		return m, nil
	}

	switch m.focus {
	case FocusSessions:
		return m.handleSessionKeys(k)
	case FocusAgents:
		return m.handleAgentKeys(k)
	}

	return m, nil
}

func (m DashboardModel) handleSessionKeys(k string) (tea.Model, tea.Cmd) {
	maxIdx := len(m.sessions) - 1

	switch k {
	case "j", "down":
		if m.cursor < maxIdx {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "tab", "n":
		m.focus = FocusAgents
		m.cursor = 0
	case "enter":
		if m.cursor < len(m.sessions) && m.callbacks.OnAttach != nil {
			m.callbacks.OnAttach(m.sessions[m.cursor].Name)
			return m, tea.Quit
		}
	case "d":
		if m.cursor < len(m.sessions) && m.callbacks.OnKill != nil {
			name := m.sessions[m.cursor].Name
			m.callbacks.OnKill(name)
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetchSessions())
		}
	case "r":
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.fetchSessions())
	}

	return m, nil
}

func (m DashboardModel) handleAgentKeys(k string) (tea.Model, tea.Cmd) {
	maxIdx := len(m.agents) - 1

	switch k {
	case "j", "down":
		if m.cursor < maxIdx {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "tab", "esc":
		m.focus = FocusSessions
		m.cursor = 0
	case "enter":
		if m.cursor < len(m.agents) && m.callbacks.OnLaunch != nil {
			m.callbacks.OnLaunch(m.agents[m.cursor])
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m DashboardModel) View() string {
	if m.quitting {
		return ""
	}

	// Calculate available height (subtract status bar)
	availHeight := m.height - 3

	// Session panel
	sessionPanel := m.renderSessionPanel(availHeight)

	// Agent panel
	agentPanel := m.renderAgentPanel()

	// Status bar
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left,
		sessionPanel,
		agentPanel,
		statusBar,
	)
}

func (m DashboardModel) renderSessionPanel(maxHeight int) string {
	var content string

	if m.loading {
		content = fmt.Sprintf("  %s Loading sessions...", m.spinner.View())
	} else if m.err != nil {
		content = ErrorStyle.Render("  Error loading sessions")
	} else if len(m.sessions) == 0 {
		content = MutedStyle.Render("  No active sessions") + "\n" +
			MutedStyle.Render("  Press 1-3 or Tab to start a new agent")
	} else {
		// Table header
		header := fmt.Sprintf("  %-20s  %-12s  %s",
			WarningStyle.Render("Session"),
			WarningStyle.Render("Windows"),
			WarningStyle.Render("Status"))
		content = header

		for i, s := range m.sessions {
			winStr := fmt.Sprintf("%d window", s.Windows)
			if s.Windows != 1 {
				winStr += "s"
			}
			status := ""
			if s.Attached {
				status = "attached"
			}

			line := fmt.Sprintf("  %-20s  %-12s  %s",
				AccentStyle.Render(s.Name),
				SecondaryStyle.Render(winStr),
				SuccessStyle.Render(status))

			if m.focus == FocusSessions && i == m.cursor {
				line = SelectedStyle.Render("> ") + fmt.Sprintf("%-20s  %-12s  %s",
					SelectedStyle.Render(s.Name),
					SecondaryStyle.Render(winStr),
					SuccessStyle.Render(status))
			}
			content += "\n" + line
		}
	}

	title := " ACTIVE SESSIONS "
	style := PanelStyle
	if m.focus == FocusSessions {
		style = PanelFocusStyle
	}

	panelHeight := maxHeight - len(m.agents) - 4
	if panelHeight < 5 {
		panelHeight = 5
	}

	return style.
		Width(m.width - 2).
		Height(panelHeight).
		Render(TitleStyle.Render(title) + "\n" + content)
}

func (m DashboardModel) renderAgentPanel() string {
	var lines []string

	for i, agent := range m.agents {
		hasKey := m.hasActiveKey(agent.Provider)
		keyStatus := SuccessStyle.Render("✓")
		if !hasKey {
			keyStatus = ErrorStyle.Render("✗ (no key)")
		}

		label := fmt.Sprintf("[%d] %-14s %s", i+1, agent.Name, keyStatus)

		if m.focus == FocusAgents && i == m.cursor {
			label = SelectedStyle.Render(fmt.Sprintf("> [%d] %-14s", i+1, agent.Name)) + " " + keyStatus
		}

		lines = append(lines, "  "+label)
	}

	content := strings.Join(lines, "\n")

	title := " QUICK START "
	style := PanelStyle
	if m.focus == FocusAgents {
		style = PanelFocusStyle
	}

	return style.
		Width(m.width - 2).
		Render(TitleStyle.Render(title) + "\n" + content)
}

func (m DashboardModel) renderStatusBar() string {
	bar := fmt.Sprintf(" %s │ %s Attach │ %s Launch │ %s Kill │ %s Keys │ %s Switch │ %s Quit",
		WarningStyle.Render("AGX"),
		SuccessStyle.Render("Enter"),
		SuccessStyle.Render("1-3"),
		SuccessStyle.Render("d"),
		SuccessStyle.Render("K"),
		SuccessStyle.Render("Tab"),
		SuccessStyle.Render("q"),
	)

	return StatusBarStyle.Width(m.width).Render(bar)
}

func (m DashboardModel) hasActiveKey(provider string) bool {
	_, err := m.store.GetActive(ks.Provider(provider))
	return err == nil
}

// GetCwd returns the current working directory
func GetCwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

// Ensure unused imports are used (table, key are used in keymgr)
var (
	_ = table.New
	_ = key.NewBinding
)
