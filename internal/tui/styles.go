package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/mateidumitrascu/typepractice/internal/theme"
)

type Styles struct {
	Theme theme.Theme

	Text    lipgloss.Style
	Subtext lipgloss.Style
	Accent  lipgloss.Style
	Error   lipgloss.Style
	Success lipgloss.Style

	// typing screen
	Untyped lipgloss.Style
	Correct lipgloss.Style
	Wrong   lipgloss.Style
	Caret   lipgloss.Style

	Title    lipgloss.Style
	Selected lipgloss.Style
	Panel    lipgloss.Style
	Hint     lipgloss.Style
}

func NewStyles(t theme.Theme) Styles {
	c := t.Colors
	text := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Text))
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Subtext))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Accent))
	errS := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Error))
	return Styles{
		Theme:   t,
		Text:    text,
		Subtext: sub,
		Accent:  accent,
		Error:   errS,
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color(c.Success)),

		Untyped: sub,
		Correct: text,
		Wrong:   errS.Underline(true),
		Caret:   lipgloss.NewStyle().Foreground(lipgloss.Color(c.Bg)).Background(lipgloss.Color(c.Accent)),

		Title:    accent.Bold(true),
		Selected: accent.Bold(true),
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(c.Surface)).
			Padding(1, 3),
		Hint: sub,
	}
}
