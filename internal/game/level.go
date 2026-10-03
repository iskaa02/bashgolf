// Package game holds level data, scoring and the par solver. It knows
// nothing about the UI.
package game

import (
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"termgame/internal/readline"
)

// CursorMark marks the starting cursor position in level text.
const CursorMark = "‸"

//go:embed packs/*.json
var packFiles embed.FS

// Level is one puzzle: turn Start into Target.
type Level struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Start    string   `json:"start"` // contains CursorMark
	Target   string   `json:"target"`
	New      []string `json:"new"` // shortcuts this level introduces
	Tip      string   `json:"tip"`
	NoTyping bool     `json:"noTyping"` // forbid typing: shortcuts only
	Kills    []string `json:"kills"`    // kill ring at the start, oldest first

	// Par and Solution are the target score and the route the hint ghost
	// replays. Tests check the solution works and try to beat the par.
	Par      int      `json:"par"`
	Solution []string `json:"solution"`
}

// Pack is an ordered set of levels for one game mode.
type Pack struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Desc   string   `json:"desc"`
	Known  []string `json:"known"` // shortcuts taught by earlier packs
	Levels []Level  `json:"levels"`
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
	for _, l := range p.Levels {
		if strings.Count(l.Start, CursorMark) != 1 {
			return nil, fmt.Errorf("pack %s level %s: start needs exactly one %s", id, l.ID, CursorMark)
		}
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
	return ed
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
