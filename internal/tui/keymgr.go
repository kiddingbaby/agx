package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	"github.com/kiddingbaby/agx/internal/usecase"
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
	keyService *usecase.KeyService
	keys       []domainkey.Key

	view             KeyMgrView
	cursor           int
	selectedProvider int
	keyRows          []keyRow // keys for current provider
	showDetails      bool
	width            int
	height           int
	quitting         bool
	switchBack       bool   // signal to switch back to dashboard
	errMsg           string // error message to display in status bar
	pendingG         bool
	filterMode       bool
	filter           textinput.Model

	// Form state
	formMode           string // add | edit
	formEditingKey     string
	formEditingProfile string
	formProviderIdx    int
	formName           textinput.Model
	formBaseURL        textinput.Model
	formKey            textinput.Model
	formTags           textinput.Model
	formFocus          int // 0=provider, 1=name, 2=baseURL, 3=apiKey, 4=tags
	formInsertMode     bool

	// Confirm delete
	confirmIdx    int
	confirmCursor int // 0=cancel, 1=delete
}

// keyRow represents a selectable row in the key list
type keyRow struct {
	keyID   string
	display string
}

var providers = []domainkey.Provider{domainkey.ProviderClaude, domainkey.ProviderOpenAI, domainkey.ProviderGemini}
var providerNames = []string{"claude", "openai", "gemini"}

// baseURLPlaceholders maps provider index to a placeholder URL
var baseURLPlaceholders = []string{
	"https://api.anthropic.com",
	"https://api.openai.com/v1",
	"https://generativelanguage.googleapis.com/v1",
}

// NewKeyManagerModel creates a new key manager model
func NewKeyManagerModel(keySvc *usecase.KeyService) KeyManagerModel {
	filter := textinput.New()
	filter.Placeholder = "filter by name/tag/provider"
	filter.CharLimit = 100
	filter.Width = 40
	m := KeyManagerModel{
		keyService:       keySvc,
		view:             KeyMgrViewList,
		selectedProvider: 0,
		filter:           filter,
	}
	m.refreshKeys()
	m.buildKeyRows()
	return m
}

func (m *KeyManagerModel) refreshKeys() {
	m.keys = m.keyService.List()
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
		m.buildKeyRows()
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
	m.errMsg = "" // clear error on any key press
	k := msg.String()

	if m.filterMode {
		switch k {
		case "esc":
			m.filterMode = false
			m.filter.Blur()
			return m, nil
		case "enter":
			m.filterMode = false
			m.filter.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.buildKeyRows()
		return m, cmd
	}

	switch k {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.switchBack = true
		return m, tea.Quit
	case "h", "left":
		m.selectProvider(m.selectedProvider - 1)
	case "l", "right":
		m.selectProvider(m.selectedProvider + 1)
	case "1", "2", "3":
		idx := int(k[0] - '1')
		if idx >= 0 && idx < len(providers) {
			m.selectProvider(idx)
		}
	case "j", "down":
		m.moveDown()
	case "k", "up":
		m.moveUp()
	case "g":
		if m.pendingG {
			m.cursor = 0
			m.pendingG = false
		} else {
			m.pendingG = true
		}
	case "G":
		if len(m.keyRows) > 0 {
			m.cursor = len(m.keyRows) - 1
		}
	case "/":
		m.filterMode = true
		m.filter.Focus()
		return m, textinput.Blink
	case "enter":
		m.activateSelected()
	case "a":
		m.initForm(m.selectedProvider)
		m.view = KeyMgrViewForm
		return m, textinput.Blink
	case "i":
		if len(m.keyRows) > 0 {
			m.showDetails = !m.showDetails
		}
	case "e":
		if m.cursor < len(m.keyRows) {
			m.initEditForm(m.keyRows[m.cursor].keyID)
			m.view = KeyMgrViewForm
			return m, textinput.Blink
		}
	case "d":
		if m.cursor < len(m.keyRows) {
			m.confirmIdx = m.cursor
			m.confirmCursor = 0 // default to Cancel
			m.view = KeyMgrViewConfirm
		}
	default:
		m.pendingG = false
	}

	return m, nil
}

// buildKeyRows rebuilds key rows for the currently selected provider.
func (m *KeyManagerModel) buildKeyRows() {
	// Dynamic column widths: active(2) + name + gap(2) + tags + gap(2) + date(10) + padding(6)
	nameW, tagsW := m.colWidths()

	m.keyRows = nil
	if m.selectedProvider < 0 || m.selectedProvider >= len(providers) {
		m.selectedProvider = 0
	}
	provider := providers[m.selectedProvider]
	for _, k := range m.keys {
		if k.Provider != provider || !m.matchesFilter(k) {
			continue
		}
		active := "  "
		if k.Active {
			active = "* "
		}
		tagsStr := "-"
		if len(k.Tags) > 0 {
			tagsStr = strings.Join(k.Tags, ", ")
		}
		display := fmt.Sprintf("%s%-*s  %-*s  %s",
			active,
			nameW, truncate(k.Name, nameW),
			tagsW, truncate(tagsStr, tagsW),
			displayDate(k).Format("2006-01-02"))
		m.keyRows = append(m.keyRows, keyRow{keyID: k.ID, display: display})
	}

	// Clamp cursor
	maxIdx := len(m.keyRows) - 1
	if m.cursor > maxIdx {
		m.cursor = maxIdx
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if len(m.keyRows) == 0 {
		m.showDetails = false
	}
}

func (m *KeyManagerModel) matchesFilter(k domainkey.Key) bool {
	q := strings.TrimSpace(strings.ToLower(m.filter.Value()))
	if q == "" {
		return true
	}
	if strings.Contains(strings.ToLower(k.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(string(k.Provider)), q) {
		return true
	}
	for _, t := range k.Tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

// colWidths returns dynamic name and tags column widths based on terminal width.
func (m *KeyManagerModel) colWidths() (nameW, tagsW int) {
	w := m.width
	if w < 40 {
		w = 80 // default before WindowSizeMsg
	}
	// Reserve: active(2) + gap(2) + gap(2) + date(10) + panel border/cursor(6) = 22
	avail := w - 22
	if avail < 20 {
		avail = 20
	}
	nameW = avail * 2 / 5
	tagsW = avail - nameW
	if nameW < 10 {
		nameW = 10
	}
	if tagsW < 10 {
		tagsW = 10
	}
	return
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

func (m *KeyManagerModel) selectProvider(idx int) {
	if idx < 0 {
		idx = len(providers) - 1
	}
	if idx >= len(providers) {
		idx = 0
	}
	if m.selectedProvider == idx {
		return
	}
	m.selectedProvider = idx
	m.cursor = 0
	m.buildKeyRows()
}

func (m *KeyManagerModel) activateSelected() {
	if m.cursor >= len(m.keyRows) {
		return
	}
	row := m.keyRows[m.cursor]
	if row.keyID == "" {
		return
	}
	if err := m.keyService.Activate(row.keyID); err != nil {
		m.errMsg = fmt.Sprintf("Activate failed: %v", err)
		return
	}
	m.errMsg = ""
	m.refreshKeys()
	m.buildKeyRows()
}

func (m *KeyManagerModel) initForm(providerIdx int) {
	m.formMode = "add"
	m.formEditingKey = ""
	m.formEditingProfile = domainkey.DefaultProfile
	m.formProviderIdx = providerIdx
	m.formFocus = 1 // start on Name (provider already pre-selected)
	m.formInsertMode = false

	m.formName = textinput.New()
	m.formName.Placeholder = "my-key"
	m.formName.CharLimit = 30
	m.formName.Width = 30

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
	m.updateFormFocus()
}

func (m *KeyManagerModel) initEditForm(keyID string) {
	k, ok := m.getKeyByID(keyID)
	if !ok {
		return
	}
	providerIdx := 0
	for i, pn := range providerNames {
		if domainkey.Provider(pn) == k.Provider {
			providerIdx = i
			break
		}
	}

	m.initForm(providerIdx)
	m.formMode = "edit"
	m.formEditingKey = k.ID
	m.formEditingProfile = domainkey.NormalizeProfileName(k.Profile)
	m.formName.SetValue(k.Name)
	m.formBaseURL.SetValue(k.BaseURL)
	m.formKey.SetValue("")
	m.formKey.Placeholder = "(leave empty to keep unchanged)"
	m.formTags.SetValue(strings.Join(k.Tags, ", "))
}

func (m KeyManagerModel) getKeyByID(id string) (domainkey.Key, bool) {
	for i := range m.keys {
		if m.keys[i].ID == id {
			return m.keys[i], true
		}
	}
	return domainkey.Key{}, false
}

func (m KeyManagerModel) selectedKey() (domainkey.Key, bool) {
	if m.cursor < 0 || m.cursor >= len(m.keyRows) {
		return domainkey.Key{}, false
	}
	return m.getKeyByID(m.keyRows[m.cursor].keyID)
}

func (m KeyManagerModel) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.errMsg = "" // clear error on any key press
	k := msg.String()

	if m.formInsertMode {
		switch k {
		case "esc":
			m.formInsertMode = false
			m.updateFormFocus()
			return m, nil
		case "enter":
			m.saveForm()
			if m.errMsg != "" {
				return m, nil
			}
			m.view = KeyMgrViewList
			return m, nil
		case "ctrl+u":
			m.clearCurrentFormField()
			return m, nil
		}

		// Forward to active input in INSERT mode.
		var cmd tea.Cmd
		switch m.formFocus {
		case 0:
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

	// NORMAL mode: vim-style navigation.
	switch k {
	case "esc":
		m.view = KeyMgrViewList
		return m, nil
	case "i":
		m.formInsertMode = true
		m.updateFormFocus()
		return m, textinput.Blink
	case "x":
		m.clearCurrentFormField()
		return m, nil
	case "j", "down":
		if m.formFocus < formFieldCount-1 {
			m.formFocus++
			m.updateFormFocus()
		}
		return m, nil
	case "k", "up":
		if m.formFocus > 0 {
			m.formFocus--
			m.updateFormFocus()
		}
		return m, nil
	case "h", "left":
		if m.formFocus == 0 {
			m.formProviderIdx = (m.formProviderIdx + len(providerNames) - 1) % len(providerNames)
			m.formBaseURL.Placeholder = baseURLPlaceholders[m.formProviderIdx]
		}
		return m, nil
	case "l", "right":
		if m.formFocus == 0 {
			m.formProviderIdx = (m.formProviderIdx + 1) % len(providerNames)
			m.formBaseURL.Placeholder = baseURLPlaceholders[m.formProviderIdx]
		}
		return m, nil
	case "enter":
		m.saveForm()
		if m.errMsg != "" {
			return m, nil
		}
		m.view = KeyMgrViewList
		return m, nil
	}

	return m, nil
}

func (m *KeyManagerModel) clearCurrentFormField() {
	switch m.formFocus {
	case 1:
		m.formName.SetValue("")
		m.formName.CursorStart()
	case 2:
		m.formBaseURL.SetValue("")
		m.formBaseURL.CursorStart()
	case 3:
		m.formKey.SetValue("")
		m.formKey.CursorStart()
	case 4:
		m.formTags.SetValue("")
		m.formTags.CursorStart()
	}
}

func (m *KeyManagerModel) updateFormFocus() {
	m.formName.Blur()
	m.formBaseURL.Blur()
	m.formKey.Blur()
	m.formTags.Blur()
	if !m.formInsertMode {
		return
	}

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

	if name == "" {
		m.errMsg = "Name is required"
		return
	}
	if m.formMode == "add" && apiKey == "" {
		m.errMsg = "API Key is required"
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

	provider := domainkey.Provider(providerNames[m.formProviderIdx])
	if m.formMode == "edit" {
		if _, err := m.keyService.Update(m.formEditingKey, provider, m.formEditingProfile, name, apiKey, baseURL, tags); err != nil {
			m.errMsg = fmt.Sprintf("Save failed: %v", err)
			return
		}
	} else {
		if _, err := m.keyService.Add(provider, domainkey.DefaultProfile, name, apiKey, baseURL, tags); err != nil {
			m.errMsg = fmt.Sprintf("Save failed: %v", err)
			return
		}
	}
	m.errMsg = ""
	m.refreshKeys()
	m.buildKeyRows()
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
				if row.keyID != "" {
					if err := m.keyService.Delete(row.keyID); err != nil {
						m.errMsg = fmt.Sprintf("Delete failed: %v", err)
						return m, nil
					}
					m.refreshKeys()
					m.buildKeyRows()
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

	if len(m.keyRows) == 0 {
		lines = append(lines, MutedStyle.Render("  (no keys - press 'a' to add)"))
	} else {
		for i, row := range m.keyRows {
			style := NormalStyle
			if k, ok := m.getKeyByID(row.keyID); ok && k.Active {
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
	filterLine := ""
	if m.filterMode || strings.TrimSpace(m.filter.Value()) != "" {
		filterLine = "Filter: " + m.filter.View() + "\n\n"
	}
	detailBlock := ""
	if m.showDetails {
		detailBlock = "\n\n" + m.viewSelectedKeyDetails()
	}
	providerTabs := m.renderProviderTabsWithSelected(m.selectedProvider)

	title := " KEY MANAGER "
	panel := PanelFocusStyle.
		Width(m.width - 2).
		Height(m.height - 3).
		Render(TitleStyle.Render(title) + "\n\n" + providerTabs + "\n\n" + filterLine + content + detailBlock)

	bar := fmt.Sprintf(" %s │ %s Activate │ %s Add │ %s Edit │ %s Delete │ %s Details │ %s/%s Provider │ %s Filter │ %s/%s Nav │ %s Back",
		WarningStyle.Render("Keys"),
		SuccessStyle.Render("Enter"),
		SuccessStyle.Render("a"),
		SuccessStyle.Render("e"),
		SuccessStyle.Render("d"),
		SuccessStyle.Render("i"),
		SuccessStyle.Render("h"),
		SuccessStyle.Render("l"),
		SuccessStyle.Render("/"),
		SuccessStyle.Render("gg"),
		SuccessStyle.Render("G"),
		SuccessStyle.Render("Esc"),
	)
	if m.errMsg != "" {
		bar = " " + ErrorStyle.Render(m.errMsg)
	}
	statusBar := StatusBarStyle.Width(m.width).Render(bar)

	return lipgloss.JoinVertical(lipgloss.Left, panel, statusBar)
}

func (m KeyManagerModel) renderProviderTabsWithSelected(selected int) string {
	base := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Border).
		Foreground(FgSecondary)
	active := base.Copy().
		BorderForeground(Accent).
		Foreground(Accent).
		Bold(true)

	tabs := make([]string, 0, len(providers))
	for i, provider := range providers {
		label := strings.ToUpper(string(provider))
		if i == selected {
			tabs = append(tabs, active.Render(label))
			continue
		}
		tabs = append(tabs, base.Render(label))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

func (m KeyManagerModel) viewSelectedKeyDetails() string {
	k, ok := m.selectedKey()
	if !ok {
		return MutedStyle.Render("Details: select a key to view.")
	}

	tags := "-"
	if len(k.Tags) > 0 {
		tags = strings.Join(k.Tags, ", ")
	}
	baseURL := k.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "(default)"
	}
	profile := domainkey.NormalizeProfileName(k.Profile)
	active := "no"
	if k.Active {
		active = "yes"
	}

	return strings.Join([]string{
		SectionHeaderStyle.Render("DETAILS"),
		fmt.Sprintf("  name: %s", AccentStyle.Render(k.Name)),
		fmt.Sprintf("  id: %s", MutedStyle.Render(k.ID)),
		fmt.Sprintf("  provider: %s", strings.ToUpper(string(k.Provider))),
		fmt.Sprintf("  profile: %s", profile),
		fmt.Sprintf("  base URL: %s", baseURL),
		fmt.Sprintf("  tags: %s", tags),
		fmt.Sprintf("  active: %s", active),
		fmt.Sprintf("  updated: %s", displayDate(k).Format("2006-01-02 15:04:05")),
		MutedStyle.Render("  API key is hidden for security"),
	}, "\n")
}

func (m KeyManagerModel) viewForm() string {
	titleText := " ADD KEY "
	if m.formMode == "edit" {
		titleText = " EDIT KEY "
	}
	modeText := "NORMAL"
	modeStyle := WarningStyle
	if m.formInsertMode {
		modeText = "INSERT"
		modeStyle = SuccessStyle
	}
	title := TitleStyle.Render(titleText) + "  " + modeStyle.Render("["+modeText+"]")

	fieldLine := func(idx int, label, value string) string {
		prefix := "  "
		labelStyle := SecondaryStyle
		valueStyle := NormalStyle
		if m.formFocus == idx {
			prefix = SelectedStyle.Render("> ")
			labelStyle = SelectedStyle
			if m.formInsertMode {
				valueStyle = SuccessStyle
			} else {
				valueStyle = AccentStyle
			}
		}
		return fmt.Sprintf("%s%s %s", prefix, labelStyle.Render(label), valueStyle.Render(value))
	}

	providerDisplay := strings.ToUpper(providerNames[m.formProviderIdx])
	if m.formFocus == 0 {
		providerDisplay = "< " + providerDisplay + " >"
	}

	lines := []string{
		SectionHeaderStyle.Render("FORM"),
		fieldLine(0, "Provider:", providerDisplay),
		fieldLine(1, "Name:", m.formName.View()),
		fieldLine(2, "Base URL:", m.formBaseURL.View()),
		MutedStyle.Render("    (optional, leave empty for default)"),
		fieldLine(3, "API Key:", m.formKey.View()),
		fieldLine(4, "Tags:", m.formTags.View()),
	}
	if m.errMsg != "" {
		lines = append(lines, "", "  "+ErrorStyle.Render(m.errMsg))
	}

	content := title + "\n\n" + m.renderProviderTabsWithSelected(m.formProviderIdx) + "\n\n" + strings.Join(lines, "\n")
	panel := PanelFocusStyle.
		Width(m.width - 2).
		Height(m.height - 3).
		Render(content)

	bar := fmt.Sprintf(" %s │ %s Mode │ %s/%s Field │ %s/%s Provider │ %s Insert │ %s Clear │ %s Save │ %s Back",
		WarningStyle.Render("Edit"),
		WarningStyle.Render("NORMAL"),
		SuccessStyle.Render("j"),
		SuccessStyle.Render("k"),
		SuccessStyle.Render("h"),
		SuccessStyle.Render("l"),
		SuccessStyle.Render("i"),
		SuccessStyle.Render("x"),
		SuccessStyle.Render("Enter"),
		SuccessStyle.Render("Esc"),
	)
	if m.formInsertMode {
		bar = fmt.Sprintf(" %s │ %s Mode │ %s Input │ %s Clear │ %s Save │ %s Normal",
			WarningStyle.Render("Edit"),
			SuccessStyle.Render("INSERT"),
			SuccessStyle.Render("Type"),
			SuccessStyle.Render("Ctrl+u"),
			SuccessStyle.Render("Enter"),
			SuccessStyle.Render("Esc"),
		)
	}
	if m.errMsg != "" {
		bar = " " + ErrorStyle.Render(m.errMsg)
	}
	statusBar := StatusBarStyle.Width(m.width).Render(bar)

	return lipgloss.JoinVertical(lipgloss.Left, panel, statusBar)
}

func (m KeyManagerModel) viewConfirm() string {
	name := "unknown"
	selectedProvider := m.selectedProvider
	if m.confirmIdx < len(m.keyRows) {
		row := m.keyRows[m.confirmIdx]
		if row.keyID != "" {
			if k, ok := m.getKeyByID(row.keyID); ok {
				name = k.Name
				for i, p := range providers {
					if p == k.Provider {
						selectedProvider = i
						break
					}
				}
			}
		}
	}

	cancelStyle := NormalStyle
	deleteStyle := NormalStyle
	if m.confirmCursor == 0 {
		cancelStyle = SelectedStyle
	} else {
		deleteStyle = lipgloss.NewStyle().Foreground(Error).Bold(true)
	}

	dialog := fmt.Sprintf(`%s

  Are you sure you want to delete %s?

  This action cannot be undone.

      %s       %s`,
		ErrorStyle.Bold(true).Render("DELETE KEY"),
		AccentStyle.Render(fmt.Sprintf("%q", name)),
		cancelStyle.Render("[ Cancel ]"),
		deleteStyle.Render("[ Delete ]"),
	)
	title := " KEY MANAGER "
	content := TitleStyle.Render(title) + "\n\n" + m.renderProviderTabsWithSelected(selectedProvider) + "\n\n" + dialog
	panel := PanelFocusStyle.
		Width(m.width - 2).
		Height(m.height - 3).
		Render(content)

	bar := fmt.Sprintf(" %s │ %s/%s Switch │ %s Confirm │ %s Cancel",
		WarningStyle.Render("Confirm"),
		SuccessStyle.Render("h"),
		SuccessStyle.Render("l"),
		SuccessStyle.Render("Enter"),
		SuccessStyle.Render("Esc"),
	)
	statusBar := StatusBarStyle.Width(m.width).Render(bar)

	return lipgloss.JoinVertical(lipgloss.Left, panel, statusBar)
}

// truncate truncates a string to max rune length with ellipsis
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func splitTags(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		if t := strings.TrimSpace(part); t != "" {
			result = append(result, t)
		}
	}
	return result
}

func displayDate(k domainkey.Key) time.Time {
	if !k.UpdatedAt.IsZero() {
		return k.UpdatedAt
	}
	return k.CreatedAt
}
