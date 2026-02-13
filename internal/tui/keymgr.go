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

// KeyManagerModel is the Bubble Tea model for key management
type KeyManagerModel struct {
	store *key.Store

	view       KeyMgrView
	cursor     int
	keyRows    []keyRow // flat list of selectable key entries
	width      int
	height     int
	quitting   bool
	switchBack bool // signal to switch back to dashboard

	// Form state
	formProviderIdx int
	formName        textinput.Model
	formKey         textinput.Model
	formTags        textinput.Model
	formFocus       int // 0=provider, 1=name, 2=key, 3=tags

	// Confirm delete
	confirmIdx    int
	confirmCursor int // 0=cancel, 1=delete
}

// keyRow represents a selectable row in the key list
type keyRow struct {
	keyIdx   int    // index into store.Keys, -1 for non-selectable
	provider key.Provider
	display  string
}

var providers = []key.Provider{key.ProviderClaude, key.ProviderOpenAI, key.ProviderGemini}
var providerNames = []string{"claude", "openai", "gemini"}

// NewKeyManagerModel creates a new key manager model
func NewKeyManagerModel(store *key.Store) KeyManagerModel {
	return KeyManagerModel{
		store: store,
		view:  KeyMgrViewList,
	}
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
		m.initForm()
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

func (m *KeyManagerModel) buildKeyRows() {
	m.keyRows = nil
	for _, provider := range providers {
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
	if m.cursor >= len(m.keyRows) {
		m.cursor = len(m.keyRows) - 1
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

func (m *KeyManagerModel) initForm() {
	m.formProviderIdx = 0
	m.formFocus = 0

	m.formName = textinput.New()
	m.formName.Placeholder = "my-key"
	m.formName.CharLimit = 30
	m.formName.Width = 30

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
			m.formFocus = (m.formFocus + 1) % 4
		} else {
			m.formFocus = (m.formFocus + 3) % 4
		}
		m.updateFormFocus()
		return m, nil
	case "enter":
		if m.formFocus == 0 {
			// Cycle provider on enter
			m.formProviderIdx = (m.formProviderIdx + 1) % len(providerNames)
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
		// Provider selection: left/right to cycle
		if k == "left" || k == "h" {
			m.formProviderIdx = (m.formProviderIdx + len(providerNames) - 1) % len(providerNames)
		} else if k == "right" || k == "l" {
			m.formProviderIdx = (m.formProviderIdx + 1) % len(providerNames)
		}
	case 1:
		m.formName, cmd = m.formName.Update(msg)
	case 2:
		m.formKey, cmd = m.formKey.Update(msg)
	case 3:
		m.formTags, cmd = m.formTags.Update(msg)
	}

	return m, cmd
}

func (m *KeyManagerModel) updateFormFocus() {
	m.formName.Blur()
	m.formKey.Blur()
	m.formTags.Blur()

	switch m.formFocus {
	case 1:
		m.formName.Focus()
	case 2:
		m.formKey.Focus()
	case 3:
		m.formTags.Focus()
	}
}

func (m *KeyManagerModel) saveForm() {
	name := m.formName.Value()
	apiKey := m.formKey.Value()
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
	m.store.Add(provider, name, apiKey, tags)
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
	var sections []string

	for _, provider := range providers {
		header := WarningStyle.Render(strings.ToUpper(string(provider)))
		separator := MutedStyle.Render("────────────────────────────────────────────────")

		var rows []string
		rows = append(rows, header)
		rows = append(rows, separator)

		hasKeys := false
		rowIdx := 0
		for _, kr := range m.keyRows {
			if kr.provider == provider {
				hasKeys = true
				style := NormalStyle
				if kr.keyIdx >= 0 && kr.keyIdx < len(m.store.Keys) && m.store.Keys[kr.keyIdx].Active {
					style = SuccessStyle
				}

				line := style.Render(kr.display)
				if rowIdx == m.cursor {
					// Find actual index in keyRows
					for ki, krow := range m.keyRows {
						if krow.keyIdx == kr.keyIdx && krow.provider == kr.provider {
							if ki == m.cursor {
								line = SelectedStyle.Render("> " + kr.display)
							}
							break
						}
					}
				}
				rows = append(rows, "  "+line)
				rowIdx++
			}
		}

		if !hasKeys {
			rows = append(rows, MutedStyle.Render("  (no keys - press 'a' to add)"))
		}

		sections = append(sections, strings.Join(rows, "\n"))
	}

	content := strings.Join(sections, "\n\n")

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

  %s   %s

  %s   %s

  %s   %s

  %s`,
		title,
		focusLabel("Provider:", 0), providerDisplay,
		focusLabel("Name:", 1), m.formName.View(),
		focusLabel("API Key:", 2), m.formKey.View(),
		focusLabel("Tags:", 3), m.formTags.View(),
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
