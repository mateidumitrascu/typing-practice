package words

import (
	_ "embed"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
)

//go:embed data/english-10k.txt
var corpus string

const (
	SetMin = 30
	SetMax = 45

	minLen       = 3
	maxLen       = 12
	letterMinLen = 4
	commonPool   = 2000
	// share of words in a letter-focus set drawn from the letter-heavy pool;
	// the rest are common words so the text still reads naturally
	letterRatio = 0.8
)

type Generator struct {
	common []string
	pools  map[rune]*pool
}

// pool holds words containing a letter, with cumulative density weights
// (count(letter)/len(word)) for weighted sampling.
type pool struct {
	words []string
	cum   []float64
}

func NewGenerator() *Generator {
	g := &Generator{pools: make(map[rune]*pool)}
	rank := 0
	for w := range strings.FieldsSeq(corpus) {
		w = strings.ToLower(strings.TrimSpace(w))
		rank++
		n := len(w)
		if n < minLen || n > maxLen {
			continue
		}
		if rank <= commonPool {
			g.common = append(g.common, w)
		}
		if n < letterMinLen {
			continue
		}
		for _, r := range "abcdefghijklmnopqrstuvwxyz" {
			c := strings.Count(w, string(r))
			if c == 0 {
				continue
			}
			p := g.pools[r]
			if p == nil {
				p = &pool{}
				g.pools[r] = p
			}
			prev := 0.0
			if len(p.cum) > 0 {
				prev = p.cum[len(p.cum)-1]
			}
			p.words = append(p.words, w)
			p.cum = append(p.cum, prev+float64(c)/float64(n))
		}
	}
	return g
}

func SetLength() int {
	return SetMin + rand.IntN(SetMax-SetMin+1)
}

func (g *Generator) Set() []string {
	n := SetLength()
	out := make([]string, 0, n)
	prev := ""
	for len(out) < n {
		w := g.common[rand.IntN(len(g.common))]
		if w == prev {
			continue
		}
		out = append(out, w)
		prev = w
	}
	return out
}

func (g *Generator) LetterSet(letter rune) ([]string, error) {
	p, ok := g.pools[letter]
	if !ok {
		return nil, fmt.Errorf("no words available for letter %q", letter)
	}
	n := SetLength()
	out := make([]string, 0, n)
	prev := ""
	for len(out) < n {
		var w string
		if rand.Float64() < letterRatio {
			w = p.pick()
		} else {
			w = g.common[rand.IntN(len(g.common))]
		}
		if w == prev {
			continue
		}
		out = append(out, w)
		prev = w
	}
	return out, nil
}

func (p *pool) pick() string {
	total := p.cum[len(p.cum)-1]
	r := rand.Float64() * total
	i := sort.SearchFloat64s(p.cum, r)
	if i >= len(p.words) {
		i = len(p.words) - 1
	}
	return p.words[i]
}
