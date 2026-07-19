package words

import (
	"strings"
	"testing"
)

func TestSetLengthInRange(t *testing.T) {
	g := NewGenerator()
	for range 1000 {
		n := g.SetLength()
		if n < DefaultSetMin || n > DefaultSetMax {
			t.Fatalf("SetLength() = %d, want %d..%d", n, DefaultSetMin, DefaultSetMax)
		}
	}
	g.SetLengthRange(10, 12)
	for range 100 {
		if n := g.SetLength(); n < 10 || n > 12 {
			t.Fatalf("SetLength() = %d after SetLengthRange(10, 12)", n)
		}
	}
	g.SetLengthRange(0, -1) // invalid, must keep previous range
	if n := g.SetLength(); n < 10 || n > 12 {
		t.Fatalf("invalid range was applied, got length %d", n)
	}
}

func TestSetWordsValid(t *testing.T) {
	g := NewGenerator()
	for range 50 {
		set := g.Set()
		if len(set) < DefaultSetMin || len(set) > DefaultSetMax {
			t.Fatalf("set length %d out of range", len(set))
		}
		for i, w := range set {
			if len(w) < minLen || len(w) > maxLen {
				t.Fatalf("word %q has invalid length", w)
			}
			if w != strings.ToLower(w) {
				t.Fatalf("word %q not lowercase", w)
			}
			if i > 0 && w == set[i-1] {
				t.Fatalf("immediate repeat %q at %d", w, i)
			}
		}
	}
}

func TestLetterSetIsLetterHeavy(t *testing.T) {
	g := NewGenerator()
	for _, letter := range []rune{'e', 'r', 'q', 'z', 'j'} {
		containing, total := 0, 0
		for range 100 {
			set, err := g.LetterSet(letter)
			if err != nil {
				t.Fatalf("LetterSet(%q): %v", letter, err)
			}
			for _, w := range set {
				total++
				if strings.ContainsRune(w, letter) {
					containing++
				}
			}
		}
		frac := float64(containing) / float64(total)
		if frac < 0.7 {
			t.Errorf("letter %q: only %.0f%% of words contain it, want >= 70%%", letter, frac*100)
		}
	}
}

func TestLetterSetInvalid(t *testing.T) {
	g := NewGenerator()
	if _, err := g.LetterSet('1'); err == nil {
		t.Error("expected error for non-letter")
	}
}
