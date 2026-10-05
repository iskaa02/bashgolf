package game

import (
	"math/rand/v2"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/iskaa02/bashgolf/internal/readline"
)

// RushDuration is how long a free Line Rush run lasts.
const RushDuration = 90 * time.Second

// Line Rush pars are computed over the shortcuts that can help with each
// kind of slip. Offering every shortcut for every slip made the search ten
// times slower without finding better fixes (pasting or retyping never
// shortens a pure deletion, for example).
var (
	rushMoves   = []string{"ctrl+a", "ctrl+e", "ctrl+f", "ctrl+b", "alt+f", "alt+b"}
	rushDeletes = []string{"ctrl+k", "ctrl+u", "ctrl+w", "alt+d", "ctrl+d", "ctrl+h"}
	deleteKeys  = slices.Concat(rushMoves, rushDeletes)
	caseKeys    = slices.Concat(rushMoves, []string{"alt+u", "alt+l", "alt+c"})
	typoKeys    = slices.Concat(rushMoves, []string{"ctrl+t", "ctrl+d", "ctrl+h"})
	swapKeys    = slices.Concat(rushMoves, rushDeletes, []string{"ctrl+y", "alt+t", " "})
)

// rushKeys is every shortcut Line Rush pars can use, in the order used to
// break ties when picking a lesson.
var rushKeys = slices.Concat(rushMoves, rushDeletes, []string{"ctrl+y", "alt+u", "alt+l", "alt+c", "ctrl+t", "alt+t"})

// rushSolveStates bounds the par search. Tasks that need more are dropped
// and another is generated, so the game never stalls.
const rushSolveStates = 60_000

// Slip is a kind of mistake Line Rush puts into a command.
type Slip struct {
	Name  string // shown to the player as a hint
	apply func(words []string, rng *rand.Rand) ([]string, bool)
	keys  []string // shortcuts the par search may use to fix it
}

var junkTails = []string{"| less", "--verbose", "&& exit", "2>/dev/null", "-x"}
var junkHeads = []string{"time", "sudo", "nohup", "echo"}
var junkWords = []string{"-x", "--force", "foo", "-v", "-q"}

// Slips, roughly from easiest to fix to hardest.
var (
	SlipJunkTail = Slip{"junk at the end", func(w []string, rng *rand.Rand) ([]string, bool) {
		return append(w, junkTails[rng.IntN(len(junkTails))]), true
	}, deleteKeys}
	SlipCaps = Slip{"Caps Lock word", func(w []string, rng *rand.Rand) ([]string, bool) {
		i := rng.IntN(len(w))
		up := strings.ToUpper(w[i])
		if up == w[i] || !hasLetters(w[i]) {
			return nil, false
		}
		w[i] = up
		return w, true
	}, caseKeys}
	SlipDouble = Slip{"word typed twice", func(w []string, rng *rand.Rand) ([]string, bool) {
		i := rng.IntN(len(w))
		return slices.Insert(w, i, w[i]), true
	}, deleteKeys}
	SlipJunkHead = Slip{"junk at the start", func(w []string, rng *rand.Rand) ([]string, bool) {
		return slices.Insert(w, 0, junkHeads[rng.IntN(len(junkHeads))]), true
	}, deleteKeys}
	SlipTypo = Slip{"letters swapped", func(w []string, rng *rand.Rand) ([]string, bool) {
		i := rng.IntN(len(w))
		r := []rune(w[i])
		var spots []int
		for j := 0; j+1 < len(r); j++ {
			if unicode.IsLetter(r[j]) && unicode.IsLetter(r[j+1]) && r[j] != r[j+1] {
				spots = append(spots, j)
			}
		}
		if len(spots) == 0 {
			return nil, false
		}
		j := spots[rng.IntN(len(spots))]
		r[j], r[j+1] = r[j+1], r[j]
		w[i] = string(r)
		return w, true
	}, typoKeys}
	SlipSwap = Slip{"words swapped", func(w []string, rng *rand.Rand) ([]string, bool) {
		if len(w) < 2 {
			return nil, false
		}
		i := rng.IntN(len(w) - 1)
		if w[i] == w[i+1] {
			return nil, false
		}
		w[i], w[i+1] = w[i+1], w[i]
		return w, true
	}, swapKeys}
	SlipJunkWord = Slip{"stray word", func(w []string, rng *rand.Rand) ([]string, bool) {
		i := 1 + rng.IntN(len(w)-1)
		return slices.Insert(w, i, junkWords[rng.IntN(len(junkWords))]), true
	}, deleteKeys}
)

func hasLetters(s string) bool {
	return strings.IndexFunc(s, unicode.IsLetter) >= 0
}

// RushConfig shapes a run.
type RushConfig struct {
	Tasks          int    // fixes before the run is done; 0 = endless
	Slips          []Slip // kinds of mistake to choose from
	MinPar, MaxPar int
}

// FreeRush is the endless mode: any slip, any size.
var FreeRush = RushConfig{
	Slips:  []Slip{SlipJunkTail, SlipCaps, SlipDouble, SlipJunkHead, SlipTypo, SlipSwap, SlipJunkWord},
	MinPar: 2, MaxPar: 7,
}

// RushLevel returns the setup and time limit for level n (from 1): one
// fix per level, with harder kinds of slip and less time as levels go up.
func RushLevel(n int) (RushConfig, time.Duration) {
	slips := []Slip{SlipJunkTail, SlipCaps, SlipDouble}
	switch {
	case n >= 7:
		slips = FreeRush.Slips
	case n >= 4:
		slips = append(slips, SlipJunkHead, SlipTypo, SlipSwap)
	}
	minPar := min(2+(n-1)/3, 5)
	cfg := RushConfig{Tasks: 1, Slips: slips, MinPar: minPar, MaxPar: minPar + 2}
	secs := max(5, 14-float64(n-1))
	return cfg, time.Duration(secs * float64(time.Second))
}

// RushTask is one broken command to fix.
type RushTask struct {
	Slip       string
	Start      string // the broken command
	StartPoint int
	Target     string // the command as it should be
	Par        int
	Route      []string
}

// RushRecord is what happened on one task, for the review screen.
type RushRecord struct {
	Task       RushTask
	Keys       []string // key names; typed text as the characters themselves
	Unfinished bool     // time ran out before the fix
}

// Wasted is how many keys more than par were used.
func (r RushRecord) Wasted() int { return max(0, len(r.Keys)-r.Task.Par) }

// cost is how much a record says about missing shortcuts: the keys wasted,
// or for an unfinished fix, the whole par still needed.
func (r RushRecord) cost() int {
	if r.Unfinished {
		return max(r.Task.Par, r.Wasted())
	}
	return r.Wasted()
}

// RushPress describes one keypress.
type RushPress struct {
	Fixed   bool
	Perfect bool
	Points  int
}

// Rush is one Line Rush run: fix each broken command in as few keys as
// possible. Fixes at par grow the combo, as in Cursor Dash.
type Rush struct {
	Ed   *readline.Editor
	Task RushTask
	Keys int

	Score, Fixed, Perfect, Combo, BestCombo int
	Records                                 []RushRecord

	cfg     RushConfig
	rng     *rand.Rand
	current RushRecord
	queued  []RushTask // made ahead of time, see Queue
}

// NewRush starts a run with the given first task, or a fresh one if nil.
func NewRush(rng *rand.Rand, cfg RushConfig, first *RushTask) *Rush {
	r := &Rush{cfg: cfg, rng: rng}
	if first != nil {
		r.queued = []RushTask{*first}
	}
	r.next()
	return r
}

// Queue adds a task to use next. Generating one can take a few hundred
// milliseconds, so the UI makes the next one in the background.
func (r *Rush) Queue(t RushTask) { r.queued = append(r.queued, t) }

// Queued is how many tasks are waiting.
func (r *Rush) Queued() int { return len(r.queued) }

// Config is the run's setup.
func (r *Rush) Config() RushConfig { return r.cfg }

// Done reports whether every task of a run with a fixed count was fixed.
func (r *Rush) Done() bool { return r.cfg.Tasks > 0 && r.Fixed >= r.cfg.Tasks }

// Current is the record for the task in progress, marked unfinished.
func (r *Rush) Current() RushRecord {
	rec := r.current
	rec.Unfinished = true
	return rec
}

func (r *Rush) next() {
	if len(r.queued) > 0 {
		r.Task, r.queued = r.queued[0], r.queued[1:]
	} else {
		r.Task = GenerateRushTask(r.rng, r.cfg)
	}
	r.Ed = readline.New(r.Task.Start, r.Task.StartPoint)
	r.Keys = 0
	r.current = RushRecord{Task: r.Task}
}

// GenerateRushTask makes a broken command whose par fits the config.
func GenerateRushTask(rng *rand.Rand, cfg RushConfig) RushTask {
	for {
		line := dashLines[rng.IntN(len(dashLines))]
		slip := cfg.Slips[rng.IntN(len(cfg.Slips))]
		words, ok := slip.apply(strings.Fields(line), rng)
		if !ok {
			continue
		}
		start := strings.Join(words, " ")
		if start == line || len([]rune(start)) > dashMaxLine+8 {
			continue
		}
		// The broken command was just recalled from history, so the cursor
		// is at the end, as after pressing Up.
		task := RushTask{Slip: slip.Name, Start: start, StartPoint: len([]rune(start)), Target: line}
		ed := readline.New(task.Start, task.StartPoint)
		sol, err := Solve(ed, line, slip.keys, SolveOptions{MaxStates: rushSolveStates, MaxCost: cfg.MaxPar})
		if err != nil || sol.Cost < cfg.MinPar {
			continue
		}
		task.Par, task.Route = sol.Cost, sol.Keys
		return task
	}
}

// Press handles one key. Every key counts; typed text counts per character.
func (r *Rush) Press(k readline.Key) RushPress {
	if r.Done() {
		return RushPress{}
	}
	if k.Text != "" {
		for _, c := range k.Text {
			r.current.Keys = append(r.current.Keys, string(c))
			r.Keys++
		}
	} else {
		r.current.Keys = append(r.current.Keys, k.Name)
		r.Keys++
	}
	r.Ed.Feed(k)
	if r.Ed.Text() != r.Task.Target || r.Ed.Search().Active {
		return RushPress{}
	}

	res := RushPress{Fixed: true, Points: 10}
	r.Fixed++
	r.Records = append(r.Records, r.current)
	if r.Keys <= r.Task.Par {
		r.Combo++
		r.BestCombo = max(r.BestCombo, r.Combo)
		r.Perfect++
		res.Perfect = true
		res.Points += 5 * r.Combo
	} else {
		r.Combo = 0
	}
	r.Score += res.Points
	if !r.Done() {
		r.next()
	}
	return res
}

// Restart puts the broken command back, keeping the keys already spent.
func (r *Rush) Restart() {
	r.Ed = readline.New(r.Task.Start, r.Task.StartPoint)
}

// MostMissedEdit is MostMissed for Line Rush: the shortcut the par routes
// used that you didn't, weighted by keys wasted (or, for an unfinished
// fix, by its par). Single-character moves, deletes and typing are never
// the lesson.
func MostMissedEdit(records []RushRecord) (key string, tasks, wasted int) {
	counts := map[string]int{}
	waste := map[string]int{}
	for _, r := range records {
		if r.cost() == 0 {
			continue
		}
		used := map[string]bool{}
		for _, k := range r.Keys {
			if n, ok := normalKey[k]; ok {
				k = n
			}
			used[k] = true
		}
		seen := map[string]bool{}
		for _, k := range r.Task.Route {
			if !used[k] && !seen[k] {
				seen[k] = true
				counts[k]++
				waste[k] += r.cost()
			}
		}
	}
	for _, k := range rushKeys {
		switch k {
		case "ctrl+f", "ctrl+b", "ctrl+d", "ctrl+h":
			continue
		}
		if waste[k] > wasted {
			key, tasks, wasted = k, counts[k], waste[k]
		}
	}
	return key, tasks, wasted
}
