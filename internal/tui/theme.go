package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Theme defines the color palette for the TUI
type Theme struct {
	BgPrimary   tcell.Color // Main background
	BgSecondary tcell.Color // Panel/box background
	BgHighlight tcell.Color // Selection highlight

	FgPrimary   tcell.Color // Main text
	FgSecondary tcell.Color // Secondary text
	FgMuted     tcell.Color // Disabled/hint text

	Accent  tcell.Color // Accent color (blue)
	Success tcell.Color // Success indicators (green)
	Warning tcell.Color // Warnings (yellow)
	Error   tcell.Color // Errors (red)

	Border      tcell.Color // Normal border
	BorderFocus tcell.Color // Focused border
}

// CurrentTheme is the active theme (Catppuccin Mocha style)
var CurrentTheme = &Theme{
	BgPrimary:   tcell.NewRGBColor(30, 30, 46),    // #1e1e2e
	BgSecondary: tcell.NewRGBColor(49, 50, 68),    // #313244
	BgHighlight: tcell.NewRGBColor(69, 71, 90),    // #45475a

	FgPrimary:   tcell.NewRGBColor(205, 214, 244), // #cdd6f4
	FgSecondary: tcell.NewRGBColor(166, 173, 200), // #a6adc8
	FgMuted:     tcell.NewRGBColor(108, 112, 134), // #6c7086

	Accent:  tcell.NewRGBColor(137, 180, 250), // #89b4fa (蓝)
	Success: tcell.NewRGBColor(166, 227, 161), // #a6e3a1 (绿)
	Warning: tcell.NewRGBColor(249, 226, 175), // #f9e2af (黄)
	Error:   tcell.NewRGBColor(243, 139, 168), // #f38ba8 (红)

	Border:      tcell.NewRGBColor(88, 91, 112),   // #585b70
	BorderFocus: tcell.NewRGBColor(137, 180, 250), // #89b4fa
}

// ApplyToApp configures the application with theme colors
func (t *Theme) ApplyToApp(app *tview.Application) {
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		screen.Fill(' ', tcell.StyleDefault.Background(t.BgPrimary))
		return false
	})
}

// ApplyToList applies theme styling to a List
func (t *Theme) ApplyToList(list *tview.List) {
	list.SetBackgroundColor(t.BgSecondary)
	list.SetMainTextColor(t.FgPrimary)
	list.SetSelectedBackgroundColor(t.BgHighlight)
	list.SetSelectedTextColor(t.Accent)
	list.SetBorderColor(t.Border)
	list.SetTitleColor(t.FgPrimary)
}

// ApplyToTable applies theme styling to a Table
func (t *Theme) ApplyToTable(table *tview.Table) {
	table.SetBackgroundColor(t.BgSecondary)
	table.SetBordersColor(t.Border)
	table.SetSelectedStyle(tcell.StyleDefault.Background(t.BgHighlight).Foreground(t.Accent))
	table.SetBorderColor(t.Border)
	table.SetTitleColor(t.FgPrimary)
}

// ApplyToTreeView applies theme styling to a TreeView
func (t *Theme) ApplyToTreeView(tree *tview.TreeView) {
	tree.SetBackgroundColor(t.BgSecondary)
	tree.SetBorderColor(t.Border)
	tree.SetTitleColor(t.FgPrimary)
	tree.SetGraphicsColor(t.FgMuted)
}

// ApplyToForm applies theme styling to a Form
func (t *Theme) ApplyToForm(form *tview.Form) {
	form.SetBackgroundColor(t.BgSecondary)
	form.SetFieldBackgroundColor(t.BgHighlight)
	form.SetFieldTextColor(t.FgPrimary)
	form.SetButtonBackgroundColor(t.Accent)
	form.SetButtonTextColor(t.BgPrimary)
	form.SetLabelColor(t.FgSecondary)
	form.SetBorderColor(t.Border)
	form.SetTitleColor(t.FgPrimary)
}

// ApplyToTextView applies theme styling to a TextView
func (t *Theme) ApplyToTextView(tv *tview.TextView) {
	tv.SetBackgroundColor(t.BgPrimary)
	tv.SetTextColor(t.FgSecondary)
}

// ApplyToFlex applies theme styling to a Flex
func (t *Theme) ApplyToFlex(flex *tview.Flex) {
	flex.SetBackgroundColor(t.BgSecondary)
	flex.SetBorderColor(t.Border)
	flex.SetTitleColor(t.FgPrimary)
}

// ApplyToPages applies theme styling to a Pages
func (t *Theme) ApplyToPages(pages *tview.Pages) {
	pages.SetBackgroundColor(t.BgPrimary)
}
