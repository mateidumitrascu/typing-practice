package tui

import "strings"

// block-glyph digit font for the results screen
var digitFont = map[rune][5]string{
	'0': {"█████", "█   █", "█   █", "█   █", "█████"},
	'1': {"   ██", "   ██", "   ██", "   ██", "   ██"},
	'2': {"█████", "    █", "█████", "█    ", "█████"},
	'3': {"█████", "    █", " ████", "    █", "█████"},
	'4': {"█   █", "█   █", "█████", "    █", "    █"},
	'5': {"█████", "█    ", "█████", "    █", "█████"},
	'6': {"█████", "█    ", "█████", "█   █", "█████"},
	'7': {"█████", "    █", "   █ ", "  █  ", "  █  "},
	'8': {"█████", "█   █", "█████", "█   █", "█████"},
	'9': {"█████", "█   █", "█████", "    █", "█████"},
	'.': {"  ", "  ", "  ", "  ", "██"},
}

func bigDigits(s string) string {
	var rows [5]strings.Builder
	for i, r := range s {
		glyph, ok := digitFont[r]
		if !ok {
			continue
		}
		for row := range 5 {
			if i > 0 {
				rows[row].WriteString(" ")
			}
			rows[row].WriteString(glyph[row])
		}
	}
	out := make([]string, 5)
	for i := range 5 {
		out[i] = rows[i].String()
	}
	return strings.Join(out, "\n")
}
