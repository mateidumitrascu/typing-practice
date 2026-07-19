package tui

import (
	"math"
	"strings"
)

// brailleDots[y][x] is the unicode braille bit for a dot at row y (0=top)
// and column x within a 2x4 cell.
var brailleDots = [4][2]rune{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

// brailleChart renders values as a line chart, width x height in terminal
// cells (each cell holds 2x4 braille dots). Returns one string per row.
func brailleChart(values []float64, width, height int) []string {
	if len(values) < 2 || width < 2 || height < 1 {
		return nil
	}
	lo, hi := values[0], values[0]
	for _, v := range values {
		lo = math.Min(lo, v)
		hi = math.Max(hi, v)
	}
	if hi-lo < 1e-9 {
		hi = lo + 1
	}

	cols, rows := width*2, height*4
	grid := make([][]rune, height)
	for i := range grid {
		grid[i] = make([]rune, width)
		for j := range grid[i] {
			grid[i][j] = 0x2800
		}
	}
	set := func(x, y int) {
		if x < 0 || x >= cols || y < 0 || y >= rows {
			return
		}
		grid[y/4][x/2] |= brailleDots[y%4][x%2]
	}
	yFor := func(v float64) int {
		norm := (v - lo) / (hi - lo)
		return (rows - 1) - int(math.Round(norm*float64(rows-1)))
	}

	prevY := yFor(values[0])
	for x := range cols {
		t := float64(x) * float64(len(values)-1) / float64(cols-1)
		i := int(t)
		frac := t - float64(i)
		v := values[i]
		if i+1 < len(values) {
			v = values[i]*(1-frac) + values[i+1]*frac
		}
		y := yFor(v)
		// connect vertical gaps so the line reads as continuous
		step := 1
		if y < prevY {
			step = -1
		}
		for yy := prevY; yy != y; yy += step {
			set(x, yy)
		}
		set(x, y)
		prevY = y
	}

	out := make([]string, height)
	for i, row := range grid {
		out[i] = string(row)
	}
	return out
}

func chartWithAxis(values []float64, width, height int, s Styles, format func(float64) string) string {
	rows := brailleChart(values, width, height)
	if rows == nil {
		return s.Subtext.Render("not enough data yet — complete a few tests")
	}
	lo, hi := values[0], values[0]
	for _, v := range values {
		lo = math.Min(lo, v)
		hi = math.Max(hi, v)
	}
	labels := make([]string, height)
	labels[0] = format(hi)
	labels[height-1] = format(lo)
	labelW := max(len(labels[0]), len(labels[height-1]))

	var b strings.Builder
	for i, row := range rows {
		b.WriteString(s.Subtext.Render(pad(labels[i], labelW)))
		b.WriteString(" ")
		b.WriteString(s.Accent.Render(row))
		if i < len(rows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func pad(s string, w int) string {
	for len(s) < w {
		s = " " + s
	}
	return s
}
