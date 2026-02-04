package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/kiddingbaby/agx/internal/key"
	"github.com/rivo/tview"
)

// KeyManager is the key management UI component
type KeyManager struct {
	*tview.Flex
	store      *key.Store
	table      *tview.Table
	form       *tview.Form
	pages      *tview.Pages
	onClose    func()
	app        *tview.Application
	keyIndexes []int // maps table row to store.Keys index (-1 for non-key rows)
}

// Focus delegates focus to the internal table (fixes focus issue)
func (km *KeyManager) Focus(delegate func(p tview.Primitive)) {
	delegate(km.table)
}

// NewKeyManager creates a new key manager UI
func NewKeyManager(store *key.Store, app *tview.Application) *KeyManager {
	km := &KeyManager{
		Flex:  tview.NewFlex(),
		store: store,
		app:   app,
	}

	km.table = tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)
	km.table.SetTitle(" Keys [a]dd [d]elete [Enter]activate [Esc]close ").SetBorder(true)

	// Apply theme
	CurrentTheme.ApplyToTable(km.table)
	CurrentTheme.ApplyToFlex(km.Flex)

	km.pages = tview.NewPages()
	km.pages.AddPage("table", km.table, true, true)

	km.SetDirection(tview.FlexRow)
	km.AddItem(km.pages, 0, 1, true)

	km.refreshTable()

	km.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			if km.onClose != nil {
				km.onClose()
			}
			return nil
		case tcell.KeyEnter:
			km.activateSelected()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'a':
				km.showAddForm()
				return nil
			case 'd':
				km.deleteSelected()
				return nil
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'q':
				if km.onClose != nil {
					km.onClose()
				}
				return nil
			}
		}
		return event
	})

	return km
}

func (km *KeyManager) refreshTable() {
	km.table.Clear()
	km.keyIndexes = nil

	// Group keys by provider
	providers := []key.Provider{key.ProviderClaude, key.ProviderOpenAI, key.ProviderGemini}
	providerNames := map[key.Provider]string{
		key.ProviderClaude: "CLAUDE",
		key.ProviderOpenAI: "OPENAI",
		key.ProviderGemini: "GEMINI",
	}

	row := 0
	firstSelectableRow := -1

	for _, provider := range providers {
		// Provider header
		km.table.SetCell(row, 0, tview.NewTableCell(providerNames[provider]).
			SetTextColor(CurrentTheme.Warning).
			SetSelectable(false).
			SetExpansion(1))
		km.keyIndexes = append(km.keyIndexes, -1) // header row
		row++

		// Separator line
		km.table.SetCell(row, 0, tview.NewTableCell("────────────────────────────────────────────────").
			SetTextColor(CurrentTheme.FgMuted).
			SetSelectable(false))
		km.keyIndexes = append(km.keyIndexes, -1) // separator row
		row++

		// Find keys for this provider
		hasKeys := false
		for i, k := range km.store.Keys {
			if k.Provider == provider {
				hasKeys = true
				active := "  "
				if k.Active {
					active = "* "
				}

				// Format tags
				tagsStr := "-"
				if len(k.Tags) > 0 {
					tagsStr = fmt.Sprintf("%v", k.Tags)
					if len(tagsStr) > 2 {
						tagsStr = tagsStr[1 : len(tagsStr)-1] // remove brackets
					}
				}

				// Build row: active marker + name + tags + date
				cellText := fmt.Sprintf("%s%-20s  %-20s  %s",
					active,
					truncate(k.Name, 20),
					truncate(tagsStr, 20),
					k.CreatedAt.Format("2006-01-02"))

				color := CurrentTheme.FgPrimary
				if k.Active {
					color = CurrentTheme.Success
				}

				km.table.SetCell(row, 0, tview.NewTableCell(cellText).
					SetTextColor(color).
					SetSelectable(true))
				km.keyIndexes = append(km.keyIndexes, i)

				if firstSelectableRow < 0 {
					firstSelectableRow = row
				}
				row++
			}
		}

		// Empty provider hint
		if !hasKeys {
			km.table.SetCell(row, 0, tview.NewTableCell("  (no keys - press 'a' to add)").
				SetTextColor(CurrentTheme.FgMuted).
				SetSelectable(false))
			km.keyIndexes = append(km.keyIndexes, -1)
			row++
		}

		// Blank line between providers
		km.table.SetCell(row, 0, tview.NewTableCell("").
			SetSelectable(false))
		km.keyIndexes = append(km.keyIndexes, -1)
		row++
	}

	// Select first selectable row
	if firstSelectableRow >= 0 {
		km.table.Select(firstSelectableRow, 0)
	}
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

func (km *KeyManager) showAddForm() {
	km.form = tview.NewForm()
	km.form.SetTitle(" Add Key ").SetBorder(true)
	CurrentTheme.ApplyToForm(km.form)

	providers := []string{"claude", "openai", "gemini"}
	var selectedProvider string

	km.form.AddDropDown("Provider", providers, 0, func(option string, index int) {
		selectedProvider = option
	})
	selectedProvider = providers[0]

	km.form.AddInputField("Name", "", 30, nil, nil)
	km.form.AddPasswordField("API Key", "", 50, '*', nil)
	km.form.AddInputField("Tags (comma separated)", "", 30, nil, nil)

	km.form.AddButton("Save", func() {
		name := km.form.GetFormItemByLabel("Name").(*tview.InputField).GetText()
		apiKey := km.form.GetFormItemByLabel("API Key").(*tview.InputField).GetText()
		tagsStr := km.form.GetFormItemByLabel("Tags (comma separated)").(*tview.InputField).GetText()

		var tags []string
		if tagsStr != "" {
			for _, t := range splitTags(tagsStr) {
				if t != "" {
					tags = append(tags, t)
				}
			}
		}

		if name != "" && apiKey != "" {
			km.store.Add(key.Provider(selectedProvider), name, apiKey, tags)
			km.refreshTable()
		}
		km.pages.SwitchToPage("table")
		km.app.SetFocus(km.table)
	})

	km.form.AddButton("Cancel", func() {
		km.pages.SwitchToPage("table")
		km.app.SetFocus(km.table)
	})

	km.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			km.pages.SwitchToPage("table")
			km.app.SetFocus(km.table)
			return nil
		}
		return event
	})

	km.pages.AddPage("form", km.form, true, true)
	km.pages.SwitchToPage("form")
	km.app.SetFocus(km.form)
}

func (km *KeyManager) deleteSelected() {
	row, _ := km.table.GetSelection()
	if row < 0 || row >= len(km.keyIndexes) {
		return
	}
	keyIdx := km.keyIndexes[row]
	if keyIdx < 0 || keyIdx >= len(km.store.Keys) {
		return
	}
	k := km.store.Keys[keyIdx]
	km.store.Delete(k.ID)
	km.refreshTable()
}

func (km *KeyManager) activateSelected() {
	row, _ := km.table.GetSelection()
	if row < 0 || row >= len(km.keyIndexes) {
		return
	}
	keyIdx := km.keyIndexes[row]
	if keyIdx < 0 || keyIdx >= len(km.store.Keys) {
		return
	}
	k := km.store.Keys[keyIdx]
	km.store.Activate(k.ID)
	km.refreshTable()
}

// SetOnClose sets the callback when the manager is closed
func (km *KeyManager) SetOnClose(fn func()) *KeyManager {
	km.onClose = fn
	return km
}

func splitTags(s string) []string {
	var result []string
	var current string
	for _, c := range s {
		if c == ',' {
			result = append(result, current)
			current = ""
		} else if c != ' ' {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
