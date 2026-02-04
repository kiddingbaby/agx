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
	store    *key.Store
	table    *tview.Table
	form     *tview.Form
	pages    *tview.Pages
	onClose  func()
	app      *tview.Application
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

	headers := []string{"", "Provider", "Name", "Tags", "Created"}
	for i, h := range headers {
		km.table.SetCell(0, i, tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false))
	}

	for i, k := range km.store.Keys {
		active := " "
		if k.Active {
			active = "*"
		}
		km.table.SetCell(i+1, 0, tview.NewTableCell(active).SetTextColor(tcell.ColorGreen))
		km.table.SetCell(i+1, 1, tview.NewTableCell(string(k.Provider)))
		km.table.SetCell(i+1, 2, tview.NewTableCell(k.Name))
		km.table.SetCell(i+1, 3, tview.NewTableCell(fmt.Sprintf("%v", k.Tags)))
		km.table.SetCell(i+1, 4, tview.NewTableCell(k.CreatedAt.Format("2006-01-02")))
	}
}

func (km *KeyManager) showAddForm() {
	km.form = tview.NewForm()
	km.form.SetTitle(" Add Key ").SetBorder(true)

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
	if row <= 0 || row > len(km.store.Keys) || len(km.store.Keys) == 0 {
		return
	}
	k := km.store.Keys[row-1]
	km.store.Delete(k.ID)
	km.refreshTable()
}

func (km *KeyManager) activateSelected() {
	row, _ := km.table.GetSelection()
	if row <= 0 || row > len(km.store.Keys) || len(km.store.Keys) == 0 {
		return
	}
	k := km.store.Keys[row-1]
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
