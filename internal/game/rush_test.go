package game

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/iskaa02/termgame/internal/readline"
)

func TestRushTasksAreFixable(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 8))
	slips := map[string]bool{}
	for n := 1; n <= 9; n++ {
		cfg, limit := RushLevel(n)
		if limit <= 0 || cfg.Tasks != 1 {
			t.Fatalf("level %d: %+v %v", n, cfg, limit)
		}
		for range 4 {
			task := GenerateRushTask(rng, cfg)
			slips[task.Slip] = true
			if task.Start == task.Target {
				t.Fatalf("task is already fixed: %+v", task)
			}
			if task.Par < cfg.MinPar || task.Par > cfg.MaxPar || Cost(task.Route) != task.Par {
				t.Errorf("level %d: par %d (route %v) outside %d..%d", n, task.Par, task.Route, cfg.MinPar, cfg.MaxPar)
			}
			ed := readline.New(task.Start, task.StartPoint)
			for _, k := range task.Route {
				Press(ed, k)
			}
			if ed.Text() != task.Target {
				t.Errorf("route %v turns %q into %q, want %q", task.Route, task.Start, ed.Text(), task.Target)
			}
		}
	}
	if len(slips) < 5 {
		t.Errorf("only saw slips %v; generation is too narrow", slips)
	}
}

func TestRushPlay(t *testing.T) {
	task := RushTask{Slip: "test", Start: "ls -la junk", StartPoint: 11, Target: "ls -la", Par: 2, Route: []string{"ctrl+w", "ctrl+h"}}
	r := NewRush(rand.New(rand.NewPCG(1, 1)), RushConfig{Tasks: 1}, &task)
	if r.Task.Start != task.Start {
		t.Fatal("the given first task must be used")
	}
	// Sloppy: three backspaces and a typo fixed with backspace.
	for _, k := range []readline.Key{{Name: "x", Text: "x"}, {Name: "backspace"}, {Name: "ctrl+w"}} {
		if res := r.Press(k); res.Fixed {
			t.Fatal("fixed too early")
		}
	}
	res := r.Press(readline.Key{Name: "backspace"})
	if !res.Fixed || res.Perfect || !r.Done() {
		t.Fatalf("press = %+v, done %v", res, r.Done())
	}
	rec := r.Records[0]
	if !slices.Equal(rec.Keys, []string{"x", "backspace", "ctrl+w", "backspace"}) || rec.Wasted() != 2 {
		t.Errorf("record keys %v wasted %d", rec.Keys, rec.Wasted())
	}
	if key, _, wasted := MostMissedEdit(r.Records); key != "" || wasted != 0 {
		t.Errorf("Ctrl+W was used, so nothing to suggest; got %q", key)
	}
}

func TestRushQueue(t *testing.T) {
	cfg := RushConfig{Slips: []Slip{SlipJunkTail}, MinPar: 1, MaxPar: 3}
	rng := rand.New(rand.NewPCG(2, 2))
	r := NewRush(rng, cfg, nil)
	queued := GenerateRushTask(rng, cfg)
	r.Queue(queued)
	for _, k := range r.Task.Route {
		pressKey(r, k)
	}
	if r.Fixed != 1 || r.Task.Start != queued.Start || r.Task.Target != queued.Target || r.Queued() != 0 {
		t.Fatalf("after a fix the queued task should be next; fixed %d, task %+v", r.Fixed, r.Task)
	}
}

func TestMostMissedEdit(t *testing.T) {
	records := []RushRecord{
		{Task: RushTask{Par: 1, Route: []string{"ctrl+k"}}, Keys: slices.Repeat([]string{"delete"}, 8)},
		{Task: RushTask{Par: 2, Route: []string{"alt+b", "alt+l"}}, Keys: []string{"ctrl+b", "ctrl+b", "ctrl+b", "ctrl+h", "e"}},
	}
	key, n, wasted := MostMissedEdit(records)
	if key != "ctrl+k" || n != 1 || wasted != 7 {
		t.Errorf("MostMissedEdit = %q, %d, %d; want ctrl+k, 1, 7", key, n, wasted)
	}
	// Running out of time with few keys pressed still has a lesson.
	unfinished := []RushRecord{{Task: RushTask{Par: 3, Route: []string{"ctrl+a", "alt+d", "ctrl+d"}}, Keys: []string{"left"}, Unfinished: true}}
	if key, _, _ := MostMissedEdit(unfinished); key != "ctrl+a" {
		t.Errorf("MostMissedEdit on an unfinished fix = %q, want ctrl+a", key)
	}
}

func pressKey(r *Rush, k string) {
	if len([]rune(k)) == 1 {
		r.Press(readline.Key{Name: k, Text: k})
		return
	}
	r.Press(readline.Key{Name: k})
}
