package game

import (
	"math/rand/v2"
	"slices"
	"time"

	"github.com/iskaa02/bashgolf/internal/readline"
)

// DashDuration is how long a free Cursor Dash run lasts.
const DashDuration = 60 * time.Second

// dashLines are long, realistic commands to race around. They stay within
// dashMaxLine runes so the screen fits an 80-column terminal.
const dashMaxLine = 66

var dashLines = []string{
	`rsync -avz --progress ~/site/ deploy@prod:/var/www/html/`,
	`find /var/log -type f -name "*.log" -mtime -7 -exec gzip {} \;`,
	`docker run -it --rm -v $(pwd):/app -w /app node:20 npm test`,
	`git log --oneline --graph --decorate --all -n 20`,
	`tar -czvf backup.tar.gz ~/documents ~/projects --exclude=.git`,
	`grep -rn "TODO" src/ --include=*.go | sort | uniq -c | sort -rn`,
	`ssh -p 2222 -i ~/.ssh/id_ed25519 admin@192.168.1.42`,
	`curl -sSL https://example.com/install.sh | bash -s -- --yes`,
	`journalctl -u nginx --since yesterday -f | grep -i error`,
	`awk -F: '{print $1, $7}' /etc/passwd | column -t`,
	`chown -R www-data:www-data /var/www && chmod -R 755 /var/www`,
	`kill -9 $(lsof -t -i :8080) && npm run dev -- --port 8080`,
}

// dashKeys are the moves the par is computed from. Arrow keys, Home and End
// do the same as some of these, so they are allowed but never needed.
var dashKeys = []string{"ctrl+a", "ctrl+e", "ctrl+f", "ctrl+b", "alt+f", "alt+b", "ctrl+x ctrl+x"}

// DashConfig shapes a run.
type DashConfig struct {
	Targets        int // targets to hit before the run is done; 0 = endless
	MinPar, MaxPar int // how many keys each target should take
	TargetsPerLine int // targets before switching line; 0 = keep one line
}

// FreeDash is the endless 60-second mode.
var FreeDash = DashConfig{MinPar: 2, MaxPar: 6, TargetsPerLine: 4}

// DashLevel returns the setup and time limit for level n (from 1). A level
// is a single target. Later levels put it farther away (more keys at par)
// and give less time.
func DashLevel(n int) (DashConfig, time.Duration) {
	minPar := min(2+(n-1)/3, 5) // 2,2,2,3,3,3,4,4,4,5...
	cfg := DashConfig{Targets: 1, MinPar: minPar, MaxPar: minPar + 1}
	secs := max(3, 8-0.5*float64(n-1))
	return cfg, time.Duration(secs * float64(time.Second))
}

// TargetRecord is what happened on one target, for the review screen.
type TargetRecord struct {
	Line   string
	From   int      // where the cursor was when the target appeared
	Target int      // where the target was
	Par    int      // fewest keys from there
	Route  []string // one way to do it in Par keys
	Keys   []string // what was actually pressed
}

// Wasted is how many keys more than par were used.
func (r TargetRecord) Wasted() int { return max(0, len(r.Keys)-r.Par) }

// DashPress describes one keypress in a dash.
type DashPress struct {
	Ignored bool     // not a movement key: the line is read-only here
	Hit     bool     // the cursor reached the target
	Perfect bool     // ...in par keys
	Points  int      // scored by this hit
	Route   []string // on a hit: the par route to the target just hit
}

// Dash is one Cursor Dash run: move the cursor onto each target with as few
// keys as possible. Hitting a target in par keys grows the combo, which
// multiplies the bonus for the next perfect hit.
type Dash struct {
	Ed     *readline.Editor
	Target int      // index in the line the cursor must reach; -1 when done
	Par    int      // fewest keys from where the target appeared
	Route  []string // a cheapest way there
	Keys   int      // keys pressed for this target so far

	Score, Hits, Perfect, Combo, BestCombo int
	Records                                []TargetRecord // one per target hit

	cfg       DashConfig
	rng       *rand.Rand
	lineOrder []int
	lineIdx   int
	onLine    int // targets hit on the current line
	current   TargetRecord
}

// NewDash starts a run. Pass a seeded rng for repeatable runs in tests.
func NewDash(rng *rand.Rand, cfg DashConfig) *Dash {
	d := &Dash{cfg: cfg, rng: rng, lineOrder: rng.Perm(len(dashLines))}
	d.loadLine()
	return d
}

// Done reports whether every target of a run with a fixed count was hit.
func (d *Dash) Done() bool { return d.cfg.Targets > 0 && d.Hits >= d.cfg.Targets }

// TargetCount is the number of targets in the run, or 0 if endless.
func (d *Dash) TargetCount() int { return d.cfg.Targets }

// Current is the record for the target in progress.
func (d *Dash) Current() TargetRecord { return d.current }

func (d *Dash) loadLine() {
	text := dashLines[d.lineOrder[d.lineIdx%len(d.lineOrder)]]
	d.lineIdx++
	d.onLine = 0
	// Like recalling a command from history: cursor at the end, mark at 0.
	d.Ed = readline.New(text, len(text))
	d.newTarget()
}

// newTarget picks a target that takes MinPar to MaxPar keys to reach. Most
// targets sit on word edges, where Alt+F and Alt+B shine; some are anywhere.
func (d *Dash) newTarget() {
	text := []rune(d.Ed.Text())
	search := searchMoves(d.Ed)
	pars := search.pars
	pick := func(lo, hi int) []int {
		var edges, any []int
		for pos := 0; pos <= len(text); pos++ {
			par, ok := pars[pos]
			if !ok || par < lo || par > hi {
				continue
			}
			any = append(any, pos)
			if isWordEdge(text, pos) {
				edges = append(edges, pos)
			}
		}
		if len(edges) > 0 && d.rng.IntN(10) < 7 {
			return edges
		}
		return any
	}
	pool := pick(d.cfg.MinPar, d.cfg.MaxPar)
	if len(pool) == 0 { // short line or odd cursor spot: take anything
		pool = pick(1, len(text)+2)
	}
	d.Target = pool[d.rng.IntN(len(pool))]
	d.Par = pars[d.Target]
	d.Route = search.route(d.Target)
	d.Keys = 0
	d.current = TargetRecord{Line: d.Ed.Text(), From: d.Ed.Point(), Target: d.Target, Par: d.Par, Route: d.Route}
}

func isWordEdge(text []rune, pos int) bool {
	before := pos > 0 && readline.IsWordRune(text[pos-1])
	after := pos < len(text) && readline.IsWordRune(text[pos])
	return before != after
}

// Press handles one key. Only cursor movement is allowed.
func (d *Dash) Press(k readline.Key) DashPress {
	if d.Done() || d.Ed.Pending() == "" && k.Name != "ctrl+x" && !readline.Keymap[k.Name].IsMovement() {
		return DashPress{Ignored: true}
	}
	d.Keys++
	d.current.Keys = append(d.current.Keys, k.Name)
	d.Ed.Feed(k)
	if d.Ed.Point() != d.Target || d.Ed.Pending() != "" {
		return DashPress{}
	}

	res := DashPress{Hit: true, Points: 10, Route: d.Route}
	d.Hits++
	d.Records = append(d.Records, d.current)
	if d.Keys <= d.Par {
		d.Combo++
		d.BestCombo = max(d.BestCombo, d.Combo)
		d.Perfect++
		res.Perfect = true
		res.Points += 5 * d.Combo
	} else {
		d.Combo = 0
	}
	d.Score += res.Points

	d.onLine++
	switch {
	case d.Done():
		d.Target = -1
	case d.cfg.TargetsPerLine > 0 && d.onLine >= d.cfg.TargetsPerLine:
		d.loadLine()
	default:
		d.newTarget()
	}
	return res
}

// normalKey maps keys with the same effect onto the names par routes use.
var normalKey = map[string]string{"left": "ctrl+b", "right": "ctrl+f", "home": "ctrl+a", "end": "ctrl+e", "backspace": "ctrl+h", "delete": "ctrl+d"}

// MostMissed looks at the targets that took more than par and returns the
// shortcut their par routes used that you didn't, weighted by how many keys
// those targets wasted. It also returns how many targets that was and the
// keys they wasted. key is "" when there is nothing to learn. Ctrl+F and
// Ctrl+B are never the answer: they're just the arrow keys, which everyone
// already knows; the lesson is always a bigger jump.
func MostMissed(records []TargetRecord) (key string, targets, wasted int) {
	counts := map[string]int{}
	waste := map[string]int{}
	for _, r := range records {
		if r.Wasted() == 0 {
			continue
		}
		used := map[string]bool{}
		for i, k := range r.Keys {
			if n, ok := normalKey[k]; ok {
				k = n
			}
			used[k] = true
			if k == "ctrl+x" && i > 0 && r.Keys[i-1] == "ctrl+x" {
				used["ctrl+x ctrl+x"] = true
			}
		}
		seen := map[string]bool{}
		for _, k := range r.Route {
			if !used[k] && !seen[k] {
				seen[k] = true
				counts[k]++
				waste[k] += r.Wasted()
			}
		}
	}
	for _, k := range dashKeys { // fixed order breaks ties the same way every time
		if k == "ctrl+f" || k == "ctrl+b" {
			continue
		}
		if waste[k] > wasted {
			key, targets, wasted = k, counts[k], waste[k]
		}
	}
	return key, targets, wasted
}

// moveState is everything movement depends on: the cursor and the mark.
type moveState struct{ point, mark int }

type moveStep struct {
	from moveState
	key  string
}

// moveSearch holds the cheapest way to every cursor position.
type moveSearch struct {
	pars   map[int]int       // position -> fewest keys
	reach  map[int]moveState // position -> state where that's achieved
	parent map[moveState]moveStep
	start  moveState
}

// route returns the keys of a cheapest path to pos.
func (m *moveSearch) route(pos int) []string {
	var keys []string
	for s := m.reach[pos]; s != m.start; s = m.parent[s].from {
		keys = append(keys, m.parent[s].key)
	}
	slices.Reverse(keys)
	return keys
}

// movePars returns, for every cursor position, the fewest movement keys to
// reach it from the editor's current state (a chord counts as two).
func movePars(start *readline.Editor) map[int]int { return searchMoves(start).pars }

// searchMoves is a uniform-cost search over cursor moves. Moving never
// changes the text, so the state is just the cursor and the mark.
func searchMoves(ed *readline.Editor) *moveSearch {
	text := []rune(ed.Text())
	n := len(text)
	moves := func(s moveState) []moveStep {
		steps := []moveStep{
			{moveState{0, s.mark}, "ctrl+a"},
			{moveState{n, s.mark}, "ctrl+e"},
			{moveState{min(s.point+1, n), s.mark}, "ctrl+f"},
			{moveState{max(s.point-1, 0), s.mark}, "ctrl+b"},
			{moveState{readline.ForwardWord(text, s.point), s.mark}, "alt+f"},
			{moveState{readline.BackwardWord(text, s.point), s.mark}, "alt+b"},
		}
		if s.mark <= n {
			steps = append(steps, moveStep{moveState{s.mark, s.point}, "ctrl+x ctrl+x"})
		}
		return steps // each step's "from" field holds the destination here
	}

	start := moveState{ed.Point(), ed.Mark()}
	m := &moveSearch{pars: map[int]int{}, reach: map[int]moveState{}, parent: map[moveState]moveStep{}, start: start}
	best := map[moveState]int{start: 0}
	buckets := [][]moveState{{start}}
	for c := 0; c < len(buckets); c++ {
		for _, s := range buckets[c] {
			if best[s] < c {
				continue
			}
			if p, ok := m.pars[s.point]; !ok || c < p {
				m.pars[s.point] = c
				m.reach[s.point] = s
			}
			for _, step := range moves(s) {
				next, nc := step.from, c+KeyCost(step.key)
				if old, ok := best[next]; ok && old <= nc {
					continue
				}
				best[next] = nc
				m.parent[next] = moveStep{from: s, key: step.key}
				for len(buckets) <= nc {
					buckets = append(buckets, nil)
				}
				buckets[nc] = append(buckets[nc], next)
			}
		}
	}
	return m
}
