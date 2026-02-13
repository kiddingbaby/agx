package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kiddingbaby/agx/internal/key"
)

// KeyMgrView tracks which view is active
type KeyMgrView int

const (
	KeyMgrViewList KeyMgrView = iota
	KeyMgrViewForm
	KeyMgrViewConfirm
)

const formFieldCount = 5 // provider, name, baseURL, apiKey, tags

// KeyManagerModel is the Bubble Tea model for key management
type KeyManagerModel struct {
	store *key.Store

	view       KeyMgrView
	cursor     int
	keyRows    []keyRow // flat list: provider headers + key entries
	width      int
	height     int
	quitting   bool
	switchBack bool // signal to switch back to dashboard

	// Form state
	formProviderIdx int
	formName        textinput.Model
	formBaseURL     textinput.Model
	formKey         textinput.Model
	formTags        textinput.Model
	formFocus       int // 0=provider, 1=name, 2=baseURL, 3=apiKey, 4=tags

	// Confirm delete
	confirmIdx    int
	confirmCursor int // 0=cancel, 1=delete
}

// keyRow represents a selectable row in the key list
type keyRow struct {
	keyIdx   int // index into store.Keys, -1 for provider header
	provider key.Provider
	isHeader bool
	display  string
}

var providers = []key.Provider{key.ProviderClaude, key.ProviderOpenAI, key.ProviderGemini}
var providerNames = []string{"claude", "openai", "gemini"}

// baseURLPlaceholders maps provider index to a placeholder URL
var baseURLPlaceholders = []string{
	"https://api.anthropic.com",
	"https://api.openai.com/v1",
	"https://generativelanguage.googleapis.com/v1",
}

// NewKeyManagerModel creates a new key manager model
func NewKeyManagerModel(store *key.Store) KeyManagerModel {
	m := KeyManagerModel{
		store: store,
		view:  KeyMgrViewList,
	}
	m.buildKeyRows()
	return m
}

// ShouldSwitchBack returns true if user requested going back
func (m KeyManagerModel) ShouldSwitchBack() bool {
	return m.switchBack
}

func (m KeyManagerModel) Init() tea.Cmd {
	return nil
}

func (m KeyManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.view {
		case KeyMgrViewList:
			return m.updateList(msg)
		case KeyMgrViewForm:
			return m.updateForm(msg)
		case KeyMgrViewConfirm:
			return m.updateConfirm(msg)
		}
	}

	return m, nil
}

func (m KeyManagerModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.buildKeyRows()
	k := msg.String()

	switch k {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.switchBack = true
		return m, tea.Quit
	case "j", "down":
		m.moveDown()
	case "k", "up":
		m.moveUp()
	case "enter":
		m.activateSelected()
	case "a":
		providerIdx := m.currentProviderIdx()
		m.initForm(providerIdx)
		m.view = KeyMgrViewForm
		return m, textinput.Blink
	case "d":
		if m.cursor < len(m.keyRows) && m.keyRows[m.cursor].keyIdx >= 0 {
			m.confirmIdx = m.cursor
			m.confirmCursor = 0 // default to Cancel
			m.view = KeyMgrViewConfirm
		}
	}

	return m, nil
}

// buildKeyRows creates a flat list with provider headers and key entries.
// Provider headers are always present so navigation works even with no keys.
func (m *KeyManagerModel) buildKeyRows() {
	m.keyRows = nil
	for _, provider := range providers {
		// Add provider header row (always selectable)
		m.keyRows = append(m.keyRows, keyRow{
			keyIdx:   -1,
			provider: provider,
			isHeader: true,
			display:  strings.ToUpper(string(provider)),
		})
		// Add key rows for this provider
		for i, k := range m.store.Keys {
			if k.Provider == provider {
				active := "  "
				if k.Active {
					active = "* "
				}
				tagsStr := "-"
				if len(k.Tags) > 0 {
					tagsStr = strings.Join(k.Tags, ", ")
				}
				display := fmt.Sprintf("%s%-20s  %-20s  %s",
					active,
					truncate(k.Name, 20),
					truncate(tagsStr, 20),
					k.CreatedAt.Format("2006-01-02"))
				m.keyRows = append(m.keyRows, keyRow{keyIdx: i, provider: provider, display: display})
			}
		}
	}
	// Clamp cursor
	maxIdx := len(m.keyRows) - 1
	if m.cursor > maxIdx {
		m.cursor = maxIdx
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *KeyManagerModel) moveDown() {
	if m.cursor < len(m.keyRows)-1 {
		m.cursor++
	}
}

func (m *KeyManagerModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// currentProviderIdx returns the provider index for the row at cursor
func (m *KeyManagerModel) currentProviderIdx() int {
	if m.cursor < len(m.keyRows) {
		p := m.keyRows[m.cursor].provider
		for i, pn := range providerNames {
			if key.Provider(pn) == p {
				return i
			}
		}
	}
	return 0
}

func (m *KeyManagerModel) activateSelected() {
	if m.cursor >= len(m.keyRows) {
		return
	}
	row := m.keyRows[m.cursor]
	if row.keyIdx < 0 || row.keyIdx >= len(m.store.Keys) {
		return
	}
	k := m.store.Keys[row.keyIdx]
	m.store.Activate(k.ID)
}

func (m *KeyManagerModel) initForm(providerIdx int) {
	m.formProviderIdx = providerIdx
	m.formFocus = 1 // start on Name (provider already pre-selected)

	m.formName = textinput.New()
	m.formName.Placeholder = "my-key"
	m.formName.CharLimit = 30
	m.formName.Width = 30
	m.formName.Focus() // auto-focus name field

	m.formBaseURL = textinput.New()
	m.formBaseURL.Placeholder = baseURLPlaceholders[providerIdx]
	m.formBaseURL.CharLimit = 200
	m.formBaseURL.Width = 50

	m.formKey = textinput.New()
	m.formKey.Placeholder = "sk-..."
	m.formKey.EchoMode = textinput.EchoPassword
	m.formKey.EchoCharacter = '*'
	m.formKey.CharLimit = 200
	m.formKey.Width = 50

	m.formTags = textinput.New()
	m.formTags.Placeholder = "cache, code"
	m.formTags.CharLimit = 100
	m.formTags.Width = 30
}

func (m KeyManagerModel) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch k {
	case "esc":
		m.view = KeyMgrViewList
		return m, nil
	case "tab", "shift+tab":
		if k == "tab" {
			m.formFocus = (m.formFocus + 1) % formFieldCount
		} else {
			m.formFocus = (m.formFocus + formFieldCount - 1) % formFieldCount
		}
		m.updateFormFocus()
		return m, textinput.Blink
	case "enter":
		if m.formFocus == 0 {
			// Cycle provider on enter when focused on provider
			m.formProviderIdx = (m.formProviderIdx + 1) % len(providerNames)
			m.formBaseURL.Placeholder = baseURLPlaceholders[m.formProviderIdx]
			return m, nil
		}
		// Try to save
		m.saveForm()
		m.view = KeyMgrViewList
		return m, nil
	}

	// Forward to active input
	var cmd tea.Cmd
	switch m.formFocus {
	case 0:
		// Provider selection: h/l or left/right to cycle
		switch k {
		case "left", "h":
			m.formProviderIdx = (m.formProviderIdx + len(providerNames) - 1) % len(providerNames)
			m.formBaseURL.Placeholder = baseURLPlaceholders[m.formProviderIdx]
		case "right", "l":
			m.formProviderIdx = (m.formProviderIdx + 1) % len(providerNames)
			m.formBaseURL.Placeholder = baseURLPlaceholders[m.formProviderIdx]
		}
	case 1:
		m.formName, cmd = m.formName.Update(msg)
	case 2:
		m.formBaseURL, cmd = m.formBaseURL.Update(msg)
	case 3:
		m.formKey, cmd = m.formKey.Update(msg)
	case 4:
		m.formTags, cmd = m.formTags.Update(msg)
	}

	return m, cmd
}

func (m *KeyManagerModel) updateFormFocus() {
	m.formName.Blur()
	m.formBaseURL.Blur()
	m.formKey.Blur()
	m.formTags.Blur()

	switch m.formFocus {
	case 1:
		m.formName.Focus()
	case 2:
		m.formBaseURL.Focus()
	case 3:
		m.formKey.Focus()
	case 4:
		m.formTags.Focus()
	}
}

func (m *KeyManagerModel) saveForm() {
	name := m.formName.Value()
	apiKey := m.formKey.Value()
	baseURL := m.formBaseURL.Value()
	tagsStr := m.formTags.Value()

	if name == "" || apiKey == "" {
		return
	}

	var tags []string
	if tagsStr != "" {
		for _, t := range splitTags(tagsStr) {
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	provider := key.Provider(providerNames[m.formProviderIdx])
	m.store.Add(provider, name, apiKey, baseURL, tags)
}

func (m KeyManagerModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch k {
	case "esc":
		m.view = KeyMgrViewList
		return m, nil
	case "tab", "left", "right", "h", "l":
		m.confirmCursor = 1 - m.confirmCursor
	case "enter":
		if m.confirmCursor == 1 {
			// Delete
			if m.confirmIdx < len(m.keyRows) {
				row := m.keyRows[m.confirmIdx]
				if row.keyIdx >= 0 && row.keyIdx < len(m.store.Keys) {
					k := m.store.Keys[row.keyIdx]
					m.store.Delete(k.ID)
				}
			}
		}
		m.view = KeyMgrViewList
		return m, nil
	}

	return m, nil
}

func (m KeyManagerModel) View() string {
	if m.quitting {
		return ""
	}

	switch m.view {
	case KeyMgrViewForm:
		return m.viewForm()
	case KeyMgrViewConfirm:
		return m.viewConfirm()
	default:
		return m.viewList()
	}
}

func (m KeyManagerModel) viewList() string {
	var lines []string

	for i, row := range m.keyRows {
		if row.isHeader {
			// Add blank line before non-first provider headers
			if i > 0 {
				lines = append(lines, "")
			}
			header := WarningStyle.Render(row.display)
			separator := MutedStyle.Render("────────────────────────────────────────────────")

			if i == m.cursor {
				header = SelectedStyle.Render("> " + row.display)
			}
			lines = append(lines, header, separator)

			// Check if this provider has any keys
			hasKeys := false
			for j := i + 1; j < len(m.keyRows); j++ {
				if m.keyRows[j].isHeader {
					break
				}
				hasKeys = true
				break
			}
			if !hasKeys {
				lines = append(lines, MutedStyle.Render("  (no keys - press 'a' to add)"))
			}
		} else {
			style := NormalStyle
			if row.keyIdx >= 0 && row.keyIdx < len(m.store.Keys) && m.store.Keys[row.keyIdx].Active {
				style = SuccessStyle
			}
			line := "  " + style.Render(row.display)
			if i == m.cursor {
				line = SelectedStyle.Render("> " + row.display)
			}
			lines = append(lines, line)
		}
	}

	content := strings.Join(lines, "\n")

	title := " KEY MANAGER "
	panel := PanelFocusStyle.
		Width(m.width - 2).
		Height(m.height - 3).
		Render(TitleStyle.Render(title) + "\n\n" + content)

	bar := fmt.Sprintf(" %s │ %s Activate │ %s Add │ %s Delete │ %s Back",
		WarningStyle.Render("Keys"),
		SuccessStyle.Render("Enter"),
		SuccessStyle.Render("a"),
		SuccessStyle.Render("d"),
		SuccessStyle.Render("Esc"),
	)
	statusBar := StatusBarStyle.Width(m.width).Render(bar)

	return lipgloss.JoinVertical(lipgloss.Left, panel, statusBar)
}

func (m KeyManagerModel) viewForm() string {
	title := TitleStyle.Render(" ADD KEY ")

	providerDisplay := providerNames[m.formProviderIdx]
	if m.formFocus == 0 {
		providerDisplay = SelectedStyle.Render("< " + providerDisplay + " >")
	} else {
		providerDisplay = AccentStyle.Render(providerDisplay)
	}

	focusLabel := func(label string, idx int) string {
		if m.formFocus == idx {
			return SelectedStyle.Render(label)
		}
		return SecondaryStyle.Render(label)
	}

	form := fmt.Sprintf(`%s

  %s   %s

  %s       %s

  %s  %s
             %s

  %s   %s

  %s       %s

  %s`,
		title,
		focusLabel("Provider:", 0), providerDisplay,
		focusLabel("Name:", 1), m.formName.View(),
		focusLabel("Base URL:", 2), m.formBaseURL.View(),
		MutedStyle.Render("(optional, leave empty for default)"),
		focusLabel("API Key:", 3), m.formKey.View(),
		focusLabel("Tags:", 4), m.formTags.View(),
		MutedStyle.Render("Tab: next field │ Enter: save │ Esc: cancel"),
	)

	panel := PanelFocusStyle.
		Width(m.width - 2).
		Height(m.height - 1).
		Render(form)

	return panel
}

func (m KeyManagerModel) viewConfirm() string {
	name := "unknown"
	if m.confirmIdx < len(m.keyRows) {
		row := m.keyRows[m.confirmIdx]
		if row.keyIdx >= 0 && row.keyIdx < len(m.store.Keys) {
			name = m.store.Keys[row.keyIdx].Name
		}
	}

	cancelStyle := NormalStyle
	deleteStyle := NormalStyle
	if m.confirmCursor == 0 {
		cancelStyle = SelectedStyle
	} else {
		deleteStyle = lipgloss.NewStyle().Foreground(Error).Bold(true)
	}

	dialog := fmt.Sprintf(`
  %s

  Are you sure you want to delete %s?

  This action cannot be undone.

      %s       %s

  %s`,
		ErrorStyle.Bold(true).Render("DELETE KEY"),
		AccentStyle.Render(fmt.Sprintf("%q", name)),
		cancelStyle.Render("[ Cancel ]"),
		deleteStyle.Render("[ Delete ]"),
		MutedStyle.Render("Tab: switch │ Enter: confirm │ Esc: cancel"),
	)

	return PanelFocusStyle.
		Width(m.width - 2).
		Height(m.height - 1).
		Render(dialog)
}

// truncate truncates a string to max length with ellipsis
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func splitTags(s string) []string {
	var result []string
	var current string
	for _, c := range s {
		if c == ',' {
			result = append(result, strings.TrimSpace(current))
			current = ""
		} else {
			current += string(c)
		}
	}
	if trimmed := strings.TrimSpace(current); trimmed != "" {
		result = append(result, trimmed)
	}
	return result
}
