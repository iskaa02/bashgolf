package game

import (
	"math/rand/v2"
	"testing"

	"termgame/internal/readline"
)

func TestDashLinesFit(t *testing.T) {
	for _, l := range dashLines {
		if n := len([]rune(l)); n > dashMaxLine {
			t.Errorf("%d runes (max %d): %s", n, dashMaxLine, l)
		}
	}
}

func TestMoveParsOnSimpleLine(t *testing.T) {
	ed := readline.New("git commit -m", 13) // cursor at the end
	pars := movePars(ed)
	for pos, want := range map[int]int{
		13: 0, // already there
		0:  1, // Ctrl+A
		12: 1, // Alt+B to the "m" of -m
		11: 2, // Alt+B, Ctrl+B
		4:  2, // Alt+B twice
	} {
		if got := pars[pos]; got != want {
			t.Errorf("par to %d = %d, want %d", pos, got, want)
		}
	}
	if len(pars) != 14 {
		t.Errorf("reachable positions = %d, want all 14", len(pars))
	}
}

// TestMoveParsMatchesEditor checks the fast integer version of movement
// against a search that drives the real editor.
func TestMoveParsMatchesEditor(t *testing.T) {
	for _, line := range dashLines[:4] {
		for _, start := range []int{0, 7, len([]rune(line))} {
			ed := readline.New(line, start)
			want := map[int]int{}
			best := map[string]int{}
			buckets := [][]*readline.Editor{{ed}}
			for c := 0; c < len(buckets); c++ {
				for _, e := range buckets[c] {
					fp := e.Fingerprint(false, true)
					if old, ok := best[fp]; ok && old < c {
						continue
					}
					if p, ok := want[e.Point()]; !ok || c < p {
						want[e.Point()] = c
					}
					for _, k := range dashKeys {
						next := e.Clone()
						Press(next, k)
						nc := c + KeyCost(k)
						nfp := next.Fingerprint(false, true)
						if old, ok := best[nfp]; ok && old <= nc {
							continue
						}
						best[nfp] = nc
						for len(buckets) <= nc {
							buckets = append(buckets, nil)
						}
						buckets[nc] = append(buckets[nc], next)
					}
				}
			}
			got := movePars(ed)
			for pos, w := range want {
				if got[pos] != w {
					t.Errorf("%q from %d: par to %d = %d, editor says %d", line, start, pos, got[pos], w)
				}
			}
		}
	}
}

func TestDashTargetsAreFair(t *testing.T) {
	d := NewDash(rand.New(rand.NewPCG(1, 2)))
	for range 200 {
		if d.Par < 1 || d.Target == d.Ed.Point() {
			t.Fatalf("target %d with par %d from %d", d.Target, d.Par, d.Ed.Point())
		}
		if got := movePars(d.Ed)[d.Target]; got != d.Par {
			t.Fatalf("par %d, recomputed %d", d.Par, got)
		}
		ed := d.Ed.Clone()
		for _, k := range d.Route {
			Press(ed, k)
		}
		if ed.Point() != d.Target || Cost(d.Route) != d.Par {
			t.Fatalf("route %v ends at %d costing %d, want %d costing %d", d.Route, ed.Point(), Cost(d.Route), d.Target, d.Par)
		}
		d.Ed.Reset(d.Ed.Text(), d.Target) // teleport: we only check targets here
		d.Press(readline.Key{Name: "ctrl+f"})
		d.Press(readline.Key{Name: "ctrl+b"})
	}
}

func TestDashScoring(t *testing.T) {
	d := NewDash(rand.New(rand.NewPCG(3, 4)))
	if r := d.Press(readline.Key{Name: "ctrl+w"}); !r.Ignored {
		t.Fatal("editing keys must be ignored")
	}
	if r := d.Press(readline.Key{Name: "x", Text: "x"}); !r.Ignored || d.Keys != 0 {
		t.Fatal("typing must be ignored and not counted")
	}
	text := d.Ed.Text()

	// Reach two targets by the shortest route: combo 1 then 2.
	for i := 1; i <= 2; i++ {
		var res DashPress
		for _, k := range shortestRoute(t, d) {
			res = pressSolverKey(d, k)
		}
		if !res.Hit || !res.Perfect || d.Combo != i || res.Points != 10+5*i {
			t.Fatalf("hit %d: %+v combo %d", i, res, d.Combo)
		}
	}
	if d.Ed.Text() != text {
		t.Fatal("moving must never change the line")
	}

	// A sloppy hit scores the base 10 and resets the combo.
	pressSolverKey(d, "ctrl+f")
	pressSolverKey(d, "ctrl+b")
	var res DashPress
	for _, k := range shortestRoute(t, d) {
		res = pressSolverKey(d, k)
	}
	if !res.Hit || res.Perfect || d.Combo != 0 || res.Points != 10 {
		t.Fatalf("sloppy hit: %+v combo %d", res, d.Combo)
	}
	if d.Score != 15+20+10 || d.BestCombo != 2 || d.Hits != 3 || d.Perfect != 2 {
		t.Fatalf("score %d best combo %d hits %d perfect %d", d.Score, d.BestCombo, d.Hits, d.Perfect)
	}
}

// shortestRoute finds the cheapest movement keys to the current target.
func shortestRoute(t *testing.T, d *Dash) []string {
	t.Helper()
	type node struct {
		ed   *readline.Editor
		keys []string
	}
	queue := []node{{ed: d.Ed.Clone()}}
	seen := map[string]bool{}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if n.ed.Point() == d.Target && Cost(n.keys) == movePars(d.Ed)[d.Target] {
			return n.keys
		}
		for _, k := range dashKeys {
			next := n.ed.Clone()
			Press(next, k)
			fp := next.Fingerprint(false, true) + "|" + string(rune(Cost(n.keys)))
			if !seen[fp] && Cost(n.keys)+KeyCost(k) <= d.Par {
				seen[fp] = true
				queue = append(queue, node{next, append(append([]string{}, n.keys...), k)})
			}
		}
	}
	t.Fatal("no route to target")
	return nil
}

func pressSolverKey(d *Dash, k string) DashPress {
	if k == "ctrl+x ctrl+x" {
		d.Press(readline.Key{Name: "ctrl+x"})
		return d.Press(readline.Key{Name: "ctrl+x"})
	}
	return d.Press(readline.Key{Name: k})
}
