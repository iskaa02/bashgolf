package game

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"termgame/internal/readline"
)

var packIDs = []string{"killring", "casefix", "history"}

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
		allowed := append(p.SolverKeys(i), "enter")
		ed := l.Editor()
		typed := ""
		solved := false
		for n, k := range l.Solution {
			text, isTyped := typedText(k)
			if len([]rune(k)) == 1 {
				text, isTyped = k, true
			}
			switch {
			case isTyped && l.NoTyping:
				t.Errorf("solution types %q on a no-typing level", k)
			case !isTyped && !slices.Contains(allowed, k):
				t.Errorf("solution uses %q, which the pack hasn't taught yet", k)
			}
			typed += text
			r := Press(ed, k)
			if r.Unbound {
				t.Errorf("solution key %q is unbound", k)
			}
			if l.Mode == ModeRun && r.Event == readline.EventAccept {
				ran, ok, err := l.Run(ed)
				if err != nil || !ok || n != len(l.Solution)-1 {
					t.Errorf("after key %d the solution runs %q (err %v), want to finish by running %q", n+1, ran, err, l.Target)
				}
				solved = ok
			}
		}
		switch l.Mode {
		case ModeEdit:
			if ed.Text() != l.Target {
				t.Errorf("solution gives %q, want %q", ed.Text(), l.Target)
			}
		case ModeRun:
			if !solved {
				t.Errorf("solution never runs %q", l.Target)
			}
		default:
			t.Errorf("unknown mode %q", l.Mode)
		}
		if c := Cost(l.Solution); c != l.Par {
			t.Errorf("solution costs %d keys but par is %d", c, l.Par)
		}
		for _, k := range l.New {
			if KeyInfo[k] == "" {
				t.Errorf("no KeyInfo for new key %q", k)
			}
			if pat, ok := typedPattern[k]; ok {
				if !regexp.MustCompile(pat).MatchString(typed) {
					t.Errorf("solution never types %s, which this level teaches", k)
				}
			} else if !slices.Contains(l.Solution, k) {
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
		if l.Mode == ModeRun {
			t.Logf("par %d set by hand (run levels are too big to search)", l.Par)
			return
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
		if l.Mode == ModeRun {
			keys := suggestRun(p, i)
			t.Logf("recall %d  %s", Cost(keys), quoteKeys(keys))
			return
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
		t.Logf("%s %d  %s", kind, sol.Cost, quoteKeys(sol.Keys))
	})
}

func quoteKeys(keys []string) string {
	quoted := make([]string, len(keys))
	for i, k := range keys {
		quoted[i] = `"` + k + `"`
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// suggestRun tries the usual ways to recall a command (only those the pack
// has taught by level i) and returns the cheapest that runs the target:
// Ctrl+P n times, Ctrl+R with a substring and repeats, !prefix, !n and !-n.
// It doesn't try editing, so levels that change the command need a
// hand-made solution.
func suggestRun(p *Pack, i int) []string {
	l := p.Levels[i]
	taught := map[string]bool{}
	for _, k := range p.SolverKeys(i) {
		taught[k] = true
	}
	for _, lv := range p.Levels[:i+1] {
		for _, k := range lv.New {
			taught[k] = true
		}
	}
	// !n needs the command's number, which players only know when the
	// mission tells them, so it's only tried on the level that teaches it.
	numbers := slices.Contains(l.New, "!n")
	var cands [][]string
	for n := 1; n <= len(l.History); n++ {
		keys := slices.Repeat([]string{"ctrl+p"}, n)
		cands = append(cands, append(keys, "enter"))
		if numbers {
			cands = append(cands, []string{fmt.Sprintf("'!%d'", len(l.History)-n+1), "enter"})
			cands = append(cands, []string{fmt.Sprintf("'!-%d'", n), "enter"})
		}
	}
	target := []rune(l.Target)
	for a := 0; a < len(target); a++ {
		for b := a + 1; b <= min(len(target), a+6); b++ {
			sub := string(target[a:b])
			if strings.ContainsAny(sub, "'") {
				continue
			}
			for repeat := 0; repeat <= 3; repeat++ {
				keys := []string{"ctrl+r", "'" + sub + "'"}
				keys = append(keys, slices.Repeat([]string{"ctrl+r"}, repeat)...)
				cands = append(cands, append(keys, "enter"))
			}
			if a == 0 {
				cands = append(cands, []string{"'!" + sub + "'", "enter"})
			}
		}
	}
	uses := func(keys []string) bool {
		for _, k := range keys {
			switch text, typed := typedText(k); {
			case typed && strings.HasPrefix(text, "!") && len(text) > 1 && text[1] >= '0' && text[1] <= '9':
				if !taught["!n"] {
					return false
				}
			case typed && strings.HasPrefix(text, "!-"):
				if !taught["!n"] {
					return false
				}
			case typed && strings.HasPrefix(text, "!"):
				if !taught["!string"] {
					return false
				}
			case !typed && k != "enter" && !taught[k]:
				return false
			}
		}
		return true
	}
	var best []string
	for _, keys := range cands {
		if best != nil && Cost(keys) >= Cost(best) || !uses(keys) {
			continue
		}
		ed := l.Editor()
		for _, k := range keys {
			if Press(ed, k).Event == readline.EventAccept {
				if _, ok, _ := l.Run(ed); ok {
					best = keys
				}
				break
			}
		}
	}
	return best
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
