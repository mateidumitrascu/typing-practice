package stats

import (
	"testing"
	"time"
)

func TestGrossWPM(t *testing.T) {
	if got := GrossWPM(250, time.Minute); got != 50 {
		t.Errorf("GrossWPM(250, 1m) = %v, want 50", got)
	}
	if got := GrossWPM(250, 30*time.Second); got != 100 {
		t.Errorf("GrossWPM(250, 30s) = %v, want 100", got)
	}
	if got := GrossWPM(100, 0); got != 0 {
		t.Errorf("GrossWPM with zero duration = %v, want 0", got)
	}
}

func TestNetWPM(t *testing.T) {
	if got := NetWPM(250, 5, time.Minute); got != 45 {
		t.Errorf("NetWPM(250, 5, 1m) = %v, want 45", got)
	}
	if got := NetWPM(50, 100, time.Minute); got != 0 {
		t.Errorf("NetWPM floored = %v, want 0", got)
	}
}

func TestAccuracy(t *testing.T) {
	if got := Accuracy(90, 100); got != 90 {
		t.Errorf("Accuracy(90, 100) = %v, want 90", got)
	}
	if got := Accuracy(0, 0); got != 0 {
		t.Errorf("Accuracy(0, 0) = %v, want 0", got)
	}
}
