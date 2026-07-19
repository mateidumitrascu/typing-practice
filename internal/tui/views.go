package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (a *App) View() string {
	if a.width == 0 {
		return ""
	}
	var label, body, hints string
	switch a.screen {
	case screenSetup:
		label, body, hints = "setup", a.viewSetup(), "enter connect · ctrl+c quit"
	case screenLoading:
		label, body, hints = "", a.spin.View()+" "+a.s.Subtext.Render("loading…"), ""
	case screenLogin:
		label, body, hints = "sign in", a.viewLogin(), "tab switch field · enter submit · ctrl+c quit"
	case screenMenu:
		label, body, hints = "menu", a.viewMenu(), "↑/↓ move · enter select · q quit"
	case screenLetterPick:
		label, body, hints = "letter focus", a.viewLetterPick(), "press a letter · esc back"
	case screenTyping:
		label = a.mode
		if a.mode == "letter" {
			label = "letter · " + string(a.letter)
		}
		if a.result != nil {
			body, hints = a.viewResult(), "enter next set · esc menu"
		} else {
			body, hints = a.viewTyping(), "tab new set · esc menu"
		}
	case screenStats:
		label, body, hints = "stats", a.viewStats(), "esc back"
	case screenThemes:
		label, body, hints = "themes", a.viewThemes(), "↑/↓ preview · enter save · esc back"
	}
	return a.frame(label, body, hints)
}

func (a *App) frame(label, body, hints string) string {
	left := a.s.Accent.Render("●") + a.s.Text.Bold(true).Render(" typepractice")
	right := a.s.Subtext.Render(label)
	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gap < 1 {
		gap = 1
	}
	header := "  " + left + strings.Repeat(" ", gap) + right
	mid := lipgloss.Place(a.width, max(a.height-3, 1), lipgloss.Center, lipgloss.Center, body)
	footer := lipgloss.PlaceHorizontal(a.width, lipgloss.Center, a.s.Hint.Render(hints))
	return header + "\n" + mid + "\n" + footer
}

func (a *App) viewSetup() string {
	var b strings.Builder
	b.WriteString(a.s.Title.Render("server") + "\n\n")
	b.WriteString(a.setupInput.View())
	if a.busy {
		b.WriteString("\n\n" + a.spin.View() + " " + a.s.Subtext.Render("connecting…"))
	} else if a.setupErr != "" {
		b.WriteString("\n\n" + a.s.Error.Render(a.setupErr))
	}
	return a.s.Panel.Width(46).Render(b.String())
}

func (a *App) viewLogin() string {
	var b strings.Builder
	b.WriteString(a.s.Title.Render("sign in") + "\n\n")
	b.WriteString(a.userInput.View() + "\n")
	b.WriteString(a.passInput.View())
	if a.busy {
		b.WriteString("\n\n" + a.spin.View() + " " + a.s.Subtext.Render("signing in…"))
	} else if a.loginErr != "" {
		b.WriteString("\n\n" + a.s.Error.Render(a.loginErr))
	}
	return a.s.Panel.Width(40).Render(b.String())
}

func (a *App) viewMenu() string {
	var b strings.Builder
	if a.menuErr != "" {
		b.WriteString(a.s.Error.Render(a.menuErr) + "\n\n")
	}
	for i, it := range menuItems {
		if i == a.menuIdx {
			b.WriteString(a.s.Selected.Render("▸ " + it.label))
		} else {
			b.WriteString(a.s.Text.Render("  " + it.label))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n" + a.s.Subtext.Render(menuItems[a.menuIdx].desc))
	return b.String()
}

func (a *App) viewLetterPick() string {
	if a.busy {
		return a.spin.View() + " " + a.s.Subtext.Render("building set…")
	}
	var rows []string
	for _, row := range []string{"abcdefghijklm", "nopqrstuvwxyz"} {
		var b strings.Builder
		for i, r := range row {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(a.s.Text.Render(string(r)))
		}
		rows = append(rows, b.String())
	}
	title := a.s.Title.Render("pick a letter")
	return title + "\n\n" + strings.Join(rows, "\n\n")
}

func (a *App) viewTyping() string {
	if a.busy || a.eng == nil {
		return a.spin.View() + " " + a.s.Subtext.Render("dealing words…")
	}
	maxW := min(60, max(a.width-10, 20))

	var status string
	switch a.mode {
	case "free":
		status = a.s.Subtext.Render(fmt.Sprintf("%d words", a.freeWords+a.eng.Index()))
	default:
		status = a.s.Subtext.Render(fmt.Sprintf("%d/%d", a.eng.Index(), len(a.eng.Words())))
		if a.eng.Started() {
			status += a.s.Subtext.Render(" · ") + a.s.Accent.Render(fmt.Sprintf("%.0f wpm", a.eng.LiveWPM()))
		}
	}
	return status + "\n\n" + a.renderText(maxW)
}

func (a *App) renderText(maxW int) string {
	words := a.eng.Words()
	idx := a.eng.Index()
	type piece struct {
		str string
		w   int
	}
	pieces := make([]piece, len(words))
	for i, w := range words {
		var b strings.Builder
		n := max(len(w.Target), len(w.Typed))
		caret := -1
		if i == idx {
			caret = len(w.Typed)
		}
		width := n
		for j := range n {
			var ch rune
			var st lipgloss.Style
			switch {
			case j < len(w.Typed) && j < len(w.Target):
				ch = w.Target[j]
				if w.Typed[j] == w.Target[j] {
					st = a.s.Correct
				} else {
					st = a.s.Wrong
				}
			case j < len(w.Typed):
				ch = w.Typed[j]
				st = a.s.Wrong
			default:
				ch = w.Target[j]
				st = a.s.Untyped
			}
			if j == caret {
				st = a.s.Caret
			}
			b.WriteString(st.Render(string(ch)))
		}
		if caret >= n {
			b.WriteString(a.s.Caret.Render(" "))
			width++
		}
		pieces[i] = piece{b.String(), width}
	}

	var lines []string
	var cur strings.Builder
	curW := 0
	for _, p := range pieces {
		if curW > 0 && curW+1+p.w > maxW {
			lines = append(lines, cur.String())
			cur.Reset()
			curW = 0
		}
		if curW > 0 {
			cur.WriteString(" ")
			curW++
		}
		cur.WriteString(p.str)
		curW += p.w
	}
	lines = append(lines, cur.String())
	return strings.Join(lines, "\n")
}

func (a *App) viewResult() string {
	r := a.result
	big := a.s.Accent.Render(bigDigits(fmt.Sprintf("%.1f", r.GrossWPM)))

	detail := strings.Join([]string{
		a.s.Text.Render(fmt.Sprintf("%.1f net", r.NetWPM)),
		a.s.Text.Render(fmt.Sprintf("%.1f%% accuracy", r.Accuracy)),
		a.s.Subtext.Render(fmt.Sprintf("%d words", r.WordCount)),
		a.s.Subtext.Render(fmt.Sprintf("%.1fs", r.Duration.Seconds())),
	}, a.s.Subtext.Render(" · "))

	var save string
	switch a.save {
	case saveInFlight:
		save = a.s.Subtext.Render("saving…")
	case saveDone:
		save = a.s.Success.Render("✓ saved")
	case saveFailed:
		save = a.s.Error.Render("save failed: " + a.saveErr)
	}

	parts := []string{big, a.s.Subtext.Render("wpm"), "", detail}
	if save != "" {
		parts = append(parts, "", save)
	}
	return lipgloss.JoinVertical(lipgloss.Center, parts...)
}

func (a *App) viewStats() string {
	if a.busy || a.stats == nil {
		return a.spin.View() + " " + a.s.Subtext.Render("loading stats…")
	}
	st := a.stats
	if st.TotalTests == 0 {
		return a.s.Subtext.Render("no tests yet — run a speed test first")
	}

	summary := strings.Join([]string{
		a.s.Accent.Render(fmt.Sprintf("best %.1f wpm", st.BestWPM)),
		a.s.Text.Render(fmt.Sprintf("net %.1f", st.BestNetWPM)),
		a.s.Text.Render(fmt.Sprintf("avg₁₀ %.1f", st.AvgWPM10)),
		a.s.Text.Render(fmt.Sprintf("acc₁₀ %.1f%%", st.AvgAcc10)),
		a.s.Subtext.Render(fmt.Sprintf("%d tests", st.TotalTests)),
	}, a.s.Subtext.Render("  ·  "))

	var chart string
	if len(st.Series) >= 2 {
		values := make([]float64, len(st.Series))
		for i, r := range st.Series {
			values[i] = r.WPM
		}
		w := min(56, max(a.width-24, 24))
		chart = chartWithAxis(values, w, 8, a.s, func(v float64) string {
			return fmt.Sprintf("%3.0f", v)
		})
	} else {
		chart = a.s.Subtext.Render("chart appears after a couple of tests")
	}

	var recent []string
	recent = append(recent, a.s.Subtext.Render(fmt.Sprintf("%-12s %-10s %8s %8s", "when", "mode", "wpm", "acc")))
	n := len(st.Series)
	for i := n - 1; i >= max(0, n-6); i-- {
		r := st.Series[i]
		mode := r.Mode
		if r.Letter != "" {
			mode += " · " + r.Letter
		}
		when := r.CreatedAt
		if len(when) >= 16 {
			when = when[5:16]
		}
		recent = append(recent, fmt.Sprintf("%s %s %s %s",
			a.s.Subtext.Render(fmt.Sprintf("%-12s", when)),
			a.s.Text.Render(fmt.Sprintf("%-10s", mode)),
			a.s.Accent.Render(fmt.Sprintf("%8.1f", r.WPM)),
			a.s.Text.Render(fmt.Sprintf("%7.1f%%", r.Accuracy))))
	}

	return lipgloss.JoinVertical(lipgloss.Center,
		summary, "", chart, "", strings.Join(recent, "\n"))
}

func (a *App) viewThemes() string {
	var b strings.Builder
	b.WriteString(a.s.Title.Render("themes") + "\n\n")
	for i, t := range a.themes {
		cursor := "  "
		name := a.s.Text.Render(fmt.Sprintf("%-8s", t.Label))
		if i == a.themeIdx {
			cursor = a.s.Selected.Render("▸ ")
			name = a.s.Selected.Render(fmt.Sprintf("%-8s", t.Label))
		}
		sw := swatch(t.Colors.Accent) + swatch(t.Colors.Text) + swatch(t.Colors.Subtext) +
			swatch(t.Colors.Error) + swatch(t.Colors.Success)
		mark := "  "
		if i == a.themeSavedIdx {
			mark = a.s.Subtext.Render(" ✓")
		}
		b.WriteString(cursor + name + " " + sw + mark + "\n")
	}
	if a.themeNote != "" {
		b.WriteString("\n" + a.s.Subtext.Render(a.themeNote))
	}
	return b.String()
}

func swatch(hex string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render("██")
}
