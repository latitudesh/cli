package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Main colors
	PrimaryColor   = lipgloss.Color("#00D9FF")
	SecondaryColor = lipgloss.Color("#7C3AED")
	SuccessColor   = lipgloss.Color("#10B981")
	ErrorColor     = lipgloss.Color("#EF4444")
	WarningColor   = lipgloss.Color("#F59E0B")
	MutedColor     = lipgloss.Color("#6B7280")

	// Text styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)

	// Border styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(1, 2)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Background(lipgloss.Color("#1F2937"))

	// Input styles
	FocusedStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)

	BlurredStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	CursorStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor)

	// Help text style
	HelpStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true).
			MarginTop(1)
)

// Header renders a consistent header
func Header(title, subtitle string) string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		TitleStyle.Render(title),
		SubtitleStyle.Render(subtitle),
	)
}
