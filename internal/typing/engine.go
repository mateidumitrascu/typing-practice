// Package typing implements the word-by-word typing engine: keystroke
// accounting, error tracking and final metrics. It is UI-agnostic.
package typing

import (
	"time"

	"github.com/mateidumitrascu/typepractice/internal/stats"
)

// maxExtra caps stray characters typed beyond a word's length so a runaway
// word can't grow unbounded on screen.
const maxExtra = 8

type Word struct {
	Target []rune
	Typed  []rune
}

type Engine struct {
	words   []Word
	idx     int
	started time.Time
	running bool
	done    bool

	keystrokes  int
	correctKeys int
	spaces      int
}

func New(words []string) *Engine {
	e := &Engine{words: make([]Word, len(words))}
	for i, w := range words {
		e.words[i] = Word{Target: []rune(w)}
	}
	return e
}

func (e *Engine) Words() []Word { return e.words }
func (e *Engine) Index() int    { return e.idx }
func (e *Engine) Done() bool    { return e.done }
func (e *Engine) Started() bool { return e.running }

func (e *Engine) Elapsed() time.Duration {
	if !e.running {
		return 0
	}
	return time.Since(e.started)
}

func (e *Engine) Type(r rune) {
	if e.done {
		return
	}
	if !e.running {
		e.running = true
		e.started = time.Now()
	}
	w := &e.words[e.idx]
	pos := len(w.Typed)
	e.keystrokes++
	if pos < len(w.Target) && w.Target[pos] == r {
		e.correctKeys++
	}
	if pos < len(w.Target)+maxExtra {
		w.Typed = append(w.Typed, r)
	}
	if e.idx == len(e.words)-1 && string(w.Typed) == string(w.Target) {
		e.done = true
	}
}

func (e *Engine) Space() {
	if e.done || !e.running {
		return
	}
	w := &e.words[e.idx]
	if len(w.Typed) == 0 {
		return
	}
	e.keystrokes++
	e.spaces++
	if string(w.Typed) == string(w.Target) {
		e.correctKeys++
	}
	if e.idx == len(e.words)-1 {
		e.done = true
		return
	}
	e.idx++
}

func (e *Engine) Backspace() {
	if e.done {
		return
	}
	w := &e.words[e.idx]
	if len(w.Typed) > 0 {
		w.Typed = w.Typed[:len(w.Typed)-1]
	}
}

// CharsTyped counts every rune currently in the buffer plus committed spaces.
func (e *Engine) CharsTyped() int {
	n := e.spaces
	for i := 0; i <= e.idx && i < len(e.words); i++ {
		n += len(e.words[i].Typed)
	}
	return n
}

func (e *Engine) LiveWPM() float64 {
	return stats.GrossWPM(e.CharsTyped(), e.Elapsed())
}

// UncorrectedErrors compares each reached word against its target:
// mismatches, missing chars and extras all count.
func (e *Engine) UncorrectedErrors() int {
	errs := 0
	for i := 0; i <= e.idx && i < len(e.words); i++ {
		w := e.words[i]
		for j := range max(len(w.Target), len(w.Typed)) {
			if j >= len(w.Target) || j >= len(w.Typed) || w.Target[j] != w.Typed[j] {
				errs++
			}
		}
	}
	return errs
}

type Result struct {
	WordCount         int
	CharsTyped        int
	Keystrokes        int
	CorrectKeystrokes int
	UncorrectedErrors int
	Duration          time.Duration
	GrossWPM          float64
	NetWPM            float64
	Accuracy          float64
}

func (e *Engine) Result() Result {
	d := e.Elapsed()
	chars := e.CharsTyped()
	errs := e.UncorrectedErrors()
	return Result{
		WordCount:         len(e.words),
		CharsTyped:        chars,
		Keystrokes:        e.keystrokes,
		CorrectKeystrokes: e.correctKeys,
		UncorrectedErrors: errs,
		Duration:          d,
		GrossWPM:          stats.GrossWPM(chars, d),
		NetWPM:            stats.NetWPM(chars, errs, d),
		Accuracy:          stats.Accuracy(e.correctKeys, e.keystrokes),
	}
}
