// Package game holds level data, scoring and the par solver. It knows
// nothing about the UI.
package game

import (
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/iskaa02/termgame/internal/readline"
)

// CursorMark marks the starting cursor position in level text.
const CursorMark = "‸"

//go:embed packs/*.json
var packFiles embed.FS

// Level modes.
const (
	ModeEdit = ""    // make the line match Target
	ModeRun  = "run" // run (press Enter on) a command that expands to Target
)

// Level is one puzzle: turn Start into Target, or in run mode, run Target.
type Level struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Start    string   `json:"start"` // contains CursorMark
	Target   string   `json:"target"`
	New      []string `json:"new"` // shortcuts this level introduces
	Tip      string   `json:"tip"`
	NoTyping bool     `json:"noTyping"` // forbid typing: shortcuts only
	Kills    []string `json:"kills"`    // kill ring at the start, oldest first
	Mode     string   `json:"mode"`
	Brief    string   `json:"brief"`   // run mode: the mission, in words
	History  []string `json:"history"` // added after the pack's history

	// Par and Solution are the target score and the route the hint ghost
	// replays. Tests check the solution works and, for edit levels, try to
	// beat the par. A 'quoted' solution entry means typing that text.
	Par      int      `json:"par"`
	Solution []string `json:"solution"`
}

// Pack is an ordered set of levels for one game mode.
type Pack struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Desc    string   `json:"desc"`
	Known   []string `json:"known"`   // shortcuts taught by earlier packs
	History []string `json:"history"` // shell history every level starts with
	Levels  []Level  `json:"levels"`
}

// LoadPack reads an embedded pack by id, e.g. "killring".
func LoadPack(id string) (*Pack, error) {
	data, err := packFiles.ReadFile("packs/" + id + ".json")
	if err != nil {
		return nil, err
	}
	var p Pack
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("pack %s: %w", id, err)
	}
	for i, l := range p.Levels {
		if strings.Count(l.Start, CursorMark) != 1 {
			return nil, fmt.Errorf("pack %s level %s: start needs exactly one %s", id, l.ID, CursorMark)
		}
		p.Levels[i].History = append(slices.Clone(p.History), l.History...)
	}
	return &p, nil
}

// MustLoadPack is LoadPack for packs that are embedded and tested.
func MustLoadPack(id string) *Pack {
	p, err := LoadPack(id)
	if err != nil {
		panic(err)
	}
	return p
}

// Editor returns a fresh editor at the level's starting line.
func (l Level) Editor() *readline.Editor {
	i := strings.Index(l.Start, CursorMark)
	text := strings.Replace(l.Start, CursorMark, "", 1)
	ed := readline.New(text, len([]rune(l.Start[:i])))
	ed.SetKillRing(l.Kills)
	ed.SetHistory(l.History)
	return ed
}

// Run is what pressing Enter does in run mode: expand the line against the
// history like bash, check it against the target, and record it in the
// history (as bash does, whether or not it was the right command).
func (l Level) Run(ed *readline.Editor) (ran string, solved bool, err error) {
	ran, err = readline.Expand(ed.Text(), ed.History())
	if err != nil {
		ed.Reset("", 0)
		return "", false, err
	}
	ed.Reset(ran, len([]rune(ran)))
	ed.Submit()
	return ran, ran == l.Target, nil
}

// SolverKeys is what the par solver may press on level i: the base keys,
// the pack's known keys, every shortcut taught so far in the pack, and, unless typing is
// forbidden, a space plus any characters the target needs that neither the
// start nor the kill ring can supply.
func (p *Pack) SolverKeys(i int) []string {
	keys := slices.Clone(baseKeys)
	for _, k := range p.Known {
		if !slices.Contains(keys, k) {
			keys = append(keys, k)
		}
	}
	for _, l := range p.Levels[:i+1] {
		for _, k := range l.New {
			if !slices.Contains(keys, k) {
				keys = append(keys, k)
			}
		}
	}
	l := p.Levels[i]
	if !l.NoTyping {
		keys = append(keys, " ")
		have := map[rune]int{}
		for _, r := range l.Start + strings.Join(l.Kills, "") {
			have[r]++
		}
		for _, r := range l.Target {
			have[r]--
			if have[r] < 0 && !slices.Contains(keys, string(r)) {
				keys = append(keys, string(r))
			}
		}
	}
	return keys
}

// Stars rates a finished level: 3 at or under par, 2 within a few extra
// keys, 1 for any finish.
func Stars(keys, par int) int {
	switch {
	case keys <= par:
		return 3
	case keys <= par+3:
		return 2
	default:
		return 1
	}
}
