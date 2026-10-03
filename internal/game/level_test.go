package game

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"termgame/internal/readline"
)

var packIDs = []string{"killring", "casefix"}

func forEachLevel(t *testing.T, fn func(t *testing.T, p *Pack, i int, l Level)) {
	for _, id := range packIDs {
		p, err := LoadPack(id)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for i, l := range p.Levels {
			if seen[l.ID] {
				t.Errorf("%s: duplicate level id %q", id, l.ID)
			}
			seen[l.ID] = true
			t.Run(id+"/"+l.ID, func(t *testing.T) { fn(t, p, i, l) })
		}
	}
}

// TestLevelSolutions checks each level's stored solution really solves it,
// matches the par, and uses the shortcuts the level claims to teach.
func TestLevelSolutions(t *testing.T) {
	forEachLevel(t, func(t *testing.T, p *Pack, i int, l Level) {
		if l.Editor().Text() == l.Target {
			t.Fatal("already solved at the start")
		}
		allowed := p.SolverKeys(i)
		ed := l.Editor()
		for _, k := range l.Solution {
			if !slices.Contains(allowed, k) && !(KeyCost(k) == 1 && len([]rune(k)) == 1 && !l.NoTyping) {
				t.Errorf("solution uses %q, which the pack hasn't taught yet", k)
			}
			if r := Press(ed, k); r.Unbound {
				t.Errorf("solution key %q is unbound", k)
			}
		}
		if ed.Text() != l.Target {
			t.Errorf("solution gives %q, want %q", ed.Text(), l.Target)
		}
		if c := Cost(l.Solution); c != l.Par {
			t.Errorf("solution costs %d keys but par is %d", c, l.Par)
		}
		for _, k := range l.New {
			if KeyInfo[k] == "" {
				t.Errorf("no KeyInfo for new key %q", k)
			}
			if !slices.Contains(l.Solution, k) {
				t.Errorf("solution never uses %q, the key this level teaches", k)
			}
		}
	})
}

// TestParsAreTight searches exhaustively for anything cheaper than par.
// Big levels may be out of reach; those are logged as unverified.
func TestParsAreTight(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	forEachLevel(t, func(t *testing.T, p *Pack, i int, l Level) {
		if l.Par <= 1 {
			return // the start is never solved already, so 1 is unbeatable
		}
		sol, err := Solve(l.Editor(), l.Target, p.SolverKeys(i), SolveOptions{MaxStates: 1_500_000, MaxCost: l.Par - 1})
		switch {
		case err == nil:
			t.Errorf("par %d can be beaten in %d: %s", l.Par, sol.Cost, strings.Join(sol.Keys, " · "))
		case errors.Is(err, ErrGaveUp):
			t.Logf("par %d unverified: search too large", l.Par)
		}
	})
}

// TestSuggest is an authoring aid: TERMGAME_SUGGEST=1 go test -run Suggest -v
// prints a solution for every level: optimal if the exhaustive search
// finishes, otherwise the best quick guess.
func TestSuggest(t *testing.T) {
	if os.Getenv("TERMGAME_SUGGEST") == "" {
		t.Skip("set TERMGAME_SUGGEST=1")
	}
	only := os.Getenv("TERMGAME_LEVEL")
	forEachLevel(t, func(t *testing.T, p *Pack, i int, l Level) {
		if only != "" && only != l.ID {
			t.Skip()
		}
		kind := "optimal"
		sol, err := Solve(l.Editor(), l.Target, p.SolverKeys(i), SolveOptions{MaxStates: 1_500_000})
		for _, w := range []float64{2, 4, 8} {
			if !errors.Is(err, ErrGaveUp) {
				break
			}
			kind = fmt.Sprintf("guess (weight %v)", w)
			sol, err = Solve(l.Editor(), l.Target, p.SolverKeys(i), SolveOptions{MaxStates: 1_500_000, Weight: w})
		}
		if err != nil {
			t.Logf("%v", err)
			return
		}
		quoted := make([]string, len(sol.Keys))
		for i, k := range sol.Keys {
			quoted[i] = `"` + k + `"`
		}
		t.Logf("%s %d  [%s]", kind, sol.Cost, strings.Join(quoted, ", "))
	})
}

func TestSolveFindsOptimal(t *testing.T) {
	// Ctrl+W twice (2 keys) beats Backspace five times.
	ed := readline.New("ls -la x", 8)
	sol, err := Solve(ed, "ls ", []string{"ctrl+h", "ctrl+w"}, SolveOptions{MaxStates: 1000})
	if err != nil || sol.Cost != 2 {
		t.Fatalf("got %+v err=%v, want cost 2", sol, err)
	}
}

func TestSolveCountsChordsAsTwoKeys(t *testing.T) {
	ed := readline.New("ab", 2)
	sol, err := Solve(ed, "b", []string{"ctrl+x ctrl+x", "ctrl+d"}, SolveOptions{MaxStates: 1000})
	if err != nil || sol.Cost != 3 {
		t.Fatalf("got %+v err=%v, want cost 3", sol, err)
	}
}

func TestSolveMaxCost(t *testing.T) {
	ed := readline.New("abc", 3)
	_, err := Solve(ed, "", []string{"ctrl+h"}, SolveOptions{MaxStates: 1000, MaxCost: 2})
	if !errors.Is(err, ErrNoSolution) {
		t.Fatalf("err = %v, want ErrNoSolution", err)
	}
}

func TestDiffHunks(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"abc", "abc", 0},
		{"abXc", "abc", 1},
		{"Xabc", "abcY", 2},
		{"cp b a", "cp a b", 2},
	} {
		if got := diffHunks([]rune(c.a), []rune(c.b)); got != c.want {
			t.Errorf("diffHunks(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestStars(t *testing.T) {
	for _, c := range []struct{ keys, par, want int }{
		{3, 4, 3}, {4, 4, 3}, {5, 4, 2}, {7, 4, 2}, {8, 4, 1},
	} {
		if got := Stars(c.keys, c.par); got != c.want {
			t.Errorf("Stars(%d, %d) = %d, want %d", c.keys, c.par, got, c.want)
		}
	}
}
