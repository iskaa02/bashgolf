package game

import (
	"container/heap"
	"errors"
	"slices"
	"strings"

	"github.com/iskaa02/bashgolf/internal/readline"
)

// Solution is a key sequence that solves a level.
type Solution struct {
	Keys []string // key names; a chord is one entry like "ctrl+x ctrl+x"
	Cost int      // keypresses, counting both halves of a chord
}

// typedText returns the text of a 'quoted' solution key, which stands for
// typing that text one character at a time.
func typedText(key string) (string, bool) {
	if len(key) >= 3 && key[0] == '\'' && key[len(key)-1] == '\'' {
		return key[1 : len(key)-1], true
	}
	return "", false
}

// KeyCost is how many keypresses a solver key takes: 2 for a chord like
// "ctrl+x ctrl+x", one per character for 'typed text', otherwise 1
// (including typing a single character such as a space).
func KeyCost(key string) int {
	if text, ok := typedText(key); ok {
		return len([]rune(text))
	}
	if len([]rune(key)) == 1 {
		return 1
	}
	return strings.Count(key, " ") + 1
}

// Cost adds up the keypresses in a key sequence.
func Cost(keys []string) int {
	n := 0
	for _, k := range keys {
		n += KeyCost(k)
	}
	return n
}

// Press feeds a solver key (a chord, a key name, a single character or
// 'typed text') into the editor.
func Press(ed *readline.Editor, key string) readline.Result {
	if len([]rune(key)) == 1 {
		return ed.Feed(readline.Key{Name: key, Text: key})
	}
	var r readline.Result
	if text, ok := typedText(key); ok {
		for _, c := range text {
			r = ed.Feed(readline.Key{Name: string(c), Text: string(c)})
		}
		return r
	}
	for _, part := range strings.Split(key, " ") {
		r = ed.Feed(readline.Key{Name: part})
	}
	return r
}

// SolveOptions tunes Solve.
type SolveOptions struct {
	// MaxStates bounds the search; Solve gives up beyond it.
	MaxStates int
	// Weight 0 is an exhaustive uniform-cost search: any answer is optimal.
	// Above 0 it is a weighted A* steered by how different the text still
	// is from the target: far faster, but the answer may not be optimal.
	Weight float64
	// MaxCost skips anything costlier, so an exhaustive search can prove
	// that no solution costs MaxCost or less. 0 means no limit.
	MaxCost int
}

type node struct {
	ed     *readline.Editor
	parent *node
	key    string
	g      int     // cost so far
	f      float64 // priority
	seq    int     // insertion order, for stable ties
}

type queue []*node

func (q queue) Len() int { return len(q) }
func (q queue) Less(i, j int) bool {
	if q[i].f != q[j].f {
		return q[i].f < q[j].f
	}
	return q[i].seq < q[j].seq
}
func (q queue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x any)   { *q = append(*q, x.(*node)) }
func (q *queue) Pop() any {
	old := *q
	n := old[len(old)-1]
	*q = old[:len(old)-1]
	return n
}

var (
	// ErrNoSolution means the search finished: nothing reaches the target
	// within MaxCost (or at all).
	ErrNoSolution = errors.New("no solution")
	// ErrGaveUp means the search hit MaxStates before finishing.
	ErrGaveUp = errors.New("search too large")
)

// Solve searches for keys that turn start's text into target, pressing only
// the given keys.
func Solve(start *readline.Editor, target string, keys []string, opt SolveOptions) (Solution, error) {
	// Texts much longer than both ends are never on a shortest path in
	// practice, and cutting them off keeps the search small.
	maxLen := max(len([]rune(start.Text())), len([]rune(target))) + 2
	wholeRing := slices.Contains(keys, "alt+y")
	mark := slices.Contains(keys, "ctrl+x ctrl+x")
	fingerprint := func(ed *readline.Editor) string { return ed.Fingerprint(wholeRing, mark) }
	goal := []rune(target)
	keepUndo := slices.Contains(keys, "ctrl+_") || slices.Contains(keys, "alt+r")
	priority := func(ed *readline.Editor, g int) float64 {
		if opt.Weight == 0 {
			return float64(g)
		}
		return float64(g) + opt.Weight*float64(diffHunks([]rune(ed.Text()), goal))
	}

	best := map[string]int{fingerprint(start): 0}
	q := &queue{{ed: start, f: priority(start, 0)}}
	seq := 0
	for q.Len() > 0 {
		n := heap.Pop(q).(*node)
		if n.ed.TextIs(goal) {
			return n.solution(), nil
		}
		if g, seen := best[fingerprint(n.ed)]; seen && g < n.g {
			continue // a cheaper route to this state was found later
		}
		for _, k := range keys {
			g := n.g + KeyCost(k)
			if opt.MaxCost > 0 && g > opt.MaxCost {
				continue
			}
			ed := n.ed.Clone()
			if r := Press(ed, k); r.Unbound || r.Event == readline.EventDing {
				continue
			}
			if ed.Len() > maxLen {
				continue
			}
			if !keepUndo {
				ed.ForgetUndo()
			}
			fp := fingerprint(ed)
			if old, seen := best[fp]; seen && old <= g {
				continue
			}
			best[fp] = g
			if len(best) > opt.MaxStates {
				return Solution{}, ErrGaveUp
			}
			seq++
			heap.Push(q, &node{ed: ed, parent: n, key: k, g: g, f: priority(ed, g), seq: seq})
		}
	}
	return Solution{}, ErrNoSolution
}

func (n *node) solution() Solution {
	var keys []string
	for m := n; m.parent != nil; m = m.parent {
		keys = append(keys, m.key)
	}
	slices.Reverse(keys)
	return Solution{Keys: keys, Cost: n.g}
}

// diffHunks counts the separate places where a and b differ, using a
// longest-common-subsequence alignment. It is a rough estimate of how many
// more edits are needed.
func diffHunks(a, b []rune) int {
	if slices.Equal(a, b) {
		return 0
	}
	// lcs[i][j] is the LCS length of a[i:] and b[j:].
	lcs := make([][]int32, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int32, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	hunks, inHunk := 0, false
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			i, j, inHunk = i+1, j+1, false
			continue
		case j == len(b) || (i < len(a) && lcs[i+1][j] >= lcs[i][j+1]):
			i++
		default:
			j++
		}
		if !inHunk {
			hunks++
			inHunk = true
		}
	}
	return hunks
}
