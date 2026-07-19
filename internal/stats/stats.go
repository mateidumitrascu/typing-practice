// Package stats is the single source of truth for typing metrics.
// All clients must use these formulas so results are comparable.
package stats

import "time"

// GrossWPM = (chars typed / 5) / minutes.
func GrossWPM(charsTyped int, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	return float64(charsTyped) / 5 / elapsed.Minutes()
}

// NetWPM penalizes uncorrected errors: gross − errors/minute, floored at 0.
func NetWPM(charsTyped, uncorrectedErrors int, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	net := GrossWPM(charsTyped, elapsed) - float64(uncorrectedErrors)/elapsed.Minutes()
	if net < 0 {
		return 0
	}
	return net
}

// Accuracy returns percent of correct keystrokes (0–100).
func Accuracy(correctKeystrokes, totalKeystrokes int) float64 {
	if totalKeystrokes <= 0 {
		return 0
	}
	return 100 * float64(correctKeystrokes) / float64(totalKeystrokes)
}
