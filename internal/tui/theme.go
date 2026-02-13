package tui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha color palette
var (
	BgPrimary   = lipgloss.Color("#1e1e2e")
	BgSecondary = lipgloss.Color("#313244")
	BgHighlight = lipgloss.Color("#45475a")

	FgPrimary   = lipgloss.Color("#cdd6f4")
	FgSecondary = lipgloss.Color("#a6adc8")
	FgMuted     = lipgloss.Color("#6c7086")

	Accent  = lipgloss.Color("#89b4fa") // blue
	Success = lipgloss.Color("#a6e3a1") // green
	Warning = lipgloss.Color("#f9e2af") // yellow
	Error   = lipgloss.Color("#f38ba8") // red

	Border      = lipgloss.Color("#585b70")
	BorderFocus = lipgloss.Color("#89b4fa")
)

// Shared styles
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Warning)

	SectionHeaderStyle = lipgloss.NewStyle().
				Foreground(FgMuted).
				Bold(true)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true)

	NormalStyle = lipgloss.NewStyle().
			Foreground(FgPrimary)

	MutedStyle = lipgloss.NewStyle().
			Foreground(FgMuted)

	AccentStyle = lipgloss.NewStyle().
			Foreground(Accent)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error)

	SecondaryStyle = lipgloss.NewStyle().
			Foreground(FgSecondary)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(FgSecondary).
			Background(BgSecondary)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Border)

	PanelFocusStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderFocus)
)
