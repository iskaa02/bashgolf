package game

import (
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/iskaa02/termgame/internal/readline"
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
	d := NewDash(rand.New(rand.NewPCG(1, 2)), FreeDash)
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
	d := NewDash(rand.New(rand.NewPCG(3, 4)), FreeDash)
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

func TestDashLevelRamps(t *testing.T) {
	prevLimit, prevPar := time.Hour, 0
	for n := 1; n <= 15; n++ {
		cfg, limit := DashLevel(n)
		if cfg.Targets != 1 || cfg.MinPar > cfg.MaxPar || limit < 3*time.Second {
			t.Errorf("level %d: odd setup %+v, %v", n, cfg, limit)
		}
		if limit > prevLimit || cfg.MinPar < prevPar {
			t.Errorf("level %d (%v, par %d+) is easier than the one before", n, limit, cfg.MinPar)
		}
		prevLimit, prevPar = limit, cfg.MinPar
	}
}

func TestDashLevelRun(t *testing.T) {
	cfg, _ := DashLevel(3)
	d := NewDash(rand.New(rand.NewPCG(5, 6)), cfg)
	line := d.Ed.Text()
	for i := 0; i < cfg.Targets; i++ {
		if d.Par < cfg.MinPar || d.Par > cfg.MaxPar {
			t.Errorf("target %d has par %d, want %d..%d", i, d.Par, cfg.MinPar, cfg.MaxPar)
		}
		if i == 0 {
			// Waste two keys on the first target.
			pressSolverKey(d, "ctrl+f")
			pressSolverKey(d, "ctrl+b")
		}
		for _, k := range shortestRoute(t, d) {
			pressSolverKey(d, k)
		}
	}
	if !d.Done() || d.Target != -1 {
		t.Fatalf("done=%v target=%d after all targets", d.Done(), d.Target)
	}
	if d.Ed.Text() != line {
		t.Error("a level must stay on one line")
	}
	if r := d.Press(readline.Key{Name: "ctrl+a"}); !r.Ignored {
		t.Error("keys after the level is done must be ignored")
	}
	if len(d.Records) != cfg.Targets {
		t.Fatalf("%d records, want %d", len(d.Records), cfg.Targets)
	}
	first := d.Records[0]
	if first.Wasted() != 2 || first.Keys[0] != "ctrl+f" || first.Line != line {
		t.Errorf("first record %+v: want 2 wasted keys starting with ctrl+f", first)
	}
	for _, r := range d.Records[1:] {
		if r.Wasted() != 0 {
			t.Errorf("record %+v: wasted keys on a par route", r)
		}
	}
}

func TestMostMissed(t *testing.T) {
	records := []TargetRecord{
		{Par: 2, Route: []string{"alt+b", "alt+b"}, Keys: []string{"left", "left", "left", "left"}},
		{Par: 2, Route: []string{"ctrl+a", "alt+f"}, Keys: []string{"home", "right", "right", "right"}},
		{Par: 1, Route: []string{"alt+b"}, Keys: []string{"ctrl+b", "ctrl+b"}},
		{Par: 1, Route: []string{"ctrl+e"}, Keys: []string{"ctrl+e"}}, // perfect: ignored
	}
	key, n, wasted := MostMissed(records)
	if key != "alt+b" || n != 2 || wasted != 3 {
		t.Errorf("MostMissed = %q on %d targets wasting %d, want alt+b on 2 wasting 3", key, n, wasted)
	}
	// One very wasteful target outweighs two slightly wasteful ones.
	records = append(records, TargetRecord{Par: 1, Route: []string{"ctrl+a"}, Keys: slices.Repeat([]string{"ctrl+b"}, 30)})
	if key, _, _ := MostMissed(records); key != "ctrl+a" {
		t.Errorf("MostMissed = %q, want ctrl+a (29 keys wasted)", key)
	}
	// Single-character moves are never the lesson.
	charOnly := []TargetRecord{{Par: 3, Route: []string{"alt+b", "alt+b", "ctrl+f"}, Keys: slices.Repeat([]string{"ctrl+b"}, 9)}}
	if key, _, _ := MostMissed(charOnly); key != "alt+b" {
		t.Errorf("MostMissed = %q, want alt+b rather than a single-character move", key)
	}
	if key, _, _ := MostMissed(records[3:4]); key != "" {
		t.Errorf("MostMissed on perfect play = %q, want nothing", key)
	}
}
