package typing

import "testing"

func typeStr(e *Engine, s string) {
	for _, r := range s {
		if r == ' ' {
			e.Space()
		} else {
			e.Type(r)
		}
	}
}

func TestPerfectRun(t *testing.T) {
	e := New([]string{"cat", "dog"})
	typeStr(e, "cat dog")
	if !e.Done() {
		t.Fatal("should be done after typing last word exactly")
	}
	r := e.Result()
	if r.UncorrectedErrors != 0 || r.Keystrokes != 7 || r.CorrectKeystrokes != 7 {
		t.Errorf("perfect run: %+v", r)
	}
	if r.CharsTyped != 7 || r.Accuracy != 100 {
		t.Errorf("chars=%d acc=%v, want 7 and 100", r.CharsTyped, r.Accuracy)
	}
}

func TestWrongCharCorrected(t *testing.T) {
	e := New([]string{"cat"})
	e.Type('c')
	e.Type('x')
	e.Backspace()
	e.Type('a')
	e.Type('t')
	if !e.Done() {
		t.Fatal("should finish on exact last word")
	}
	r := e.Result()
	if r.UncorrectedErrors != 0 {
		t.Errorf("corrected error still counted: %+v", r)
	}
	if r.Keystrokes != 4 || r.CorrectKeystrokes != 3 {
		t.Errorf("keystrokes=%d correct=%d, want 4/3", r.Keystrokes, r.CorrectKeystrokes)
	}
	if r.Accuracy != 75 {
		t.Errorf("accuracy=%v, want 75", r.Accuracy)
	}
}

func TestSkippedWordCountsErrors(t *testing.T) {
	e := New([]string{"house", "cat"})
	typeStr(e, "ho cat")
	if !e.Done() {
		t.Fatal("should be done")
	}
	if got := e.UncorrectedErrors(); got != 3 {
		t.Errorf("skipped 3 chars of 'house', got %d errors", got)
	}
}

func TestExtraCharsCountAndCap(t *testing.T) {
	e := New([]string{"hi", "yo"})
	typeStr(e, "hixx yo")
	if got := e.UncorrectedErrors(); got != 2 {
		t.Errorf("2 extra chars, got %d errors", got)
	}
	e2 := New([]string{"hi"})
	for range 50 {
		e2.Type('x')
	}
	if got := len(e2.Words()[0].Typed); got > len("hi")+maxExtra {
		t.Errorf("typed buffer grew to %d, cap is %d", got, len("hi")+maxExtra)
	}
}

func TestSpaceIgnoredOnEmptyWord(t *testing.T) {
	e := New([]string{"cat", "dog"})
	e.Type('c')
	e.Type('a')
	e.Type('t')
	e.Space()
	e.Space()
	if e.Index() != 1 {
		t.Errorf("double space advanced twice: idx=%d", e.Index())
	}
}

func TestSpaceOnLastWordFinishes(t *testing.T) {
	e := New([]string{"cat"})
	typeStr(e, "ca ")
	if !e.Done() {
		t.Fatal("space on last word should finish the set")
	}
	if got := e.UncorrectedErrors(); got != 1 {
		t.Errorf("missing char, got %d errors", got)
	}
}

func TestNoTimerBeforeFirstKey(t *testing.T) {
	e := New([]string{"cat"})
	if e.Started() || e.Elapsed() != 0 || e.LiveWPM() != 0 {
		t.Error("engine should be idle before first keystroke")
	}
}
