package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termgame/internal/game"
	"termgame/internal/readline"
)

type puzzlePhase int

const (
	playing puzzlePhase = iota
	solved
)

const ghostDelay = 550 * time.Millisecond

type ghostTick struct{ run int }

// puzzle is the play screen for one level: edit the line until it matches
// the target, in as few keys as possible.
type puzzle struct {
	pack  *game.Pack
	prog  *game.Progress
	idx   int
	level game.Level

	ed    *readline.Editor
	keys  int
	trail []string // keys pressed, pretty and styled
	flash string
	phase puzzlePhase

	stars    int
	improved bool
	width    int

	// Hint ghost: replays the par solution one key per tick.
	ghostRun  int // bumps on every (re)start so stale ticks are ignored
	ghostEd   *readline.Editor
	ghostStep int
}

func newPuzzle(pack *game.Pack, prog *game.Progress, idx int) *puzzle {
	m := &puzzle{pack: pack, prog: prog, idx: idx, level: pack.Levels[idx]}
	m.restart()
	return m
}

func (m *puzzle) restart() {
	m.ed = m.level.Editor()
	m.keys = 0
	m.trail = nil
	m.flash = ""
	m.phase = playing
	m.ghostEd = nil
	m.ghostRun++
}

func (m *puzzle) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case ghostTick:
		return m, m.ghostAdvance(msg)
	case tea.KeyMsg:
		if m.phase == solved {
			return m.updateSolved(msg)
		}
		return m.updatePlaying(msg)
	}
	return m, nil
}

func (m *puzzle) updatePlaying(msg tea.KeyMsg) (Screen, tea.Cmd) {
	name := msg.String()
	if m.ed.Pending() == "" {
		switch name {
		case "esc":
			return m, switchTo(newLevelSelect(m.pack, m.prog, m.idx))
		case "ctrl+c":
			m.restart()
			m.flash = "Restarted."
			return m, nil
		}
	}

	k := toKey(msg)
	if k.Text != "" && m.level.NoTyping {
		m.flash = "No typing on this level: shortcuts only!"
		return m, nil
	}
	m.flash = ""
	if msg.Paste {
		m.keys += len(msg.Runes)
	} else {
		m.keys++
	}

	r := m.ed.Feed(k)
	label := prettyKey(name)
	if k.Text == " " {
		label = "␣"
	}
	switch {
	case r.Unbound:
		m.trail = append(m.trail, dimStyle.Render(label))
	case r.Event == readline.EventDing:
		m.trail = append(m.trail, badStyle.Render(label))
	default:
		m.trail = append(m.trail, keyStyle.Render(label))
	}
	switch {
	case r.Event == readline.EventEOF:
		m.flash = "Ctrl+D on an empty line would close a real shell! Ctrl+C restarts the level."
	case r.Event == readline.EventAccept:
		m.flash = "Not yet: the line has to match the goal exactly."
	}

	if m.ed.Text() == m.level.Target {
		m.phase = solved
		m.stars = game.Stars(m.keys, m.level.Par)
		m.improved = m.prog.Record(m.pack, m.idx, m.keys)
	}
	return m, nil
}

func (m *puzzle) updateSolved(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "enter", "n":
		if m.idx+1 < len(m.pack.Levels) {
			return m, switchTo(newPuzzle(m.pack, m.prog, m.idx+1))
		}
		return m, switchTo(newLevelSelect(m.pack, m.prog, m.idx))
	case "r", "ctrl+c":
		m.restart()
	case "g":
		return m, m.ghostStart()
	case "esc", "q":
		return m, switchTo(newLevelSelect(m.pack, m.prog, m.idx))
	}
	return m, nil
}

func (m *puzzle) ghostStart() tea.Cmd {
	m.ghostRun++
	m.ghostEd = m.level.Editor()
	m.ghostStep = 0
	run := m.ghostRun
	return tea.Tick(ghostDelay, func(time.Time) tea.Msg { return ghostTick{run} })
}

func (m *puzzle) ghostAdvance(t ghostTick) tea.Cmd {
	if t.run != m.ghostRun || m.ghostEd == nil || m.ghostStep >= len(m.level.Solution) {
		return nil
	}
	game.Press(m.ghostEd, m.level.Solution[m.ghostStep])
	m.ghostStep++
	if m.ghostStep >= len(m.level.Solution) {
		return nil
	}
	return tea.Tick(ghostDelay, func(time.Time) tea.Msg { return ghostTick{t.run} })
}

// --- view ---

// diffRange returns the part of a that differs from b, after trimming the
// common prefix and suffix.
func diffRange(a, b []rune) (lo, hi int) {
	for lo < len(a) && lo < len(b) && a[lo] == b[lo] {
		lo++
	}
	hi = len(a)
	for j := len(b); hi > lo && j > lo && a[hi-1] == b[j-1]; hi, j = hi-1, j-1 {
	}
	return lo, hi
}

// renderEdit draws text with the cursor at point (-1 for none) and the
// [lo, hi) range highlighted. A space at either edge of that range is drawn
// as a dot, since a stray or missing space is otherwise invisible.
func renderEdit(text []rune, point, lo, hi int, diff lipgloss.Style) string {
	var b strings.Builder
	for i, r := range text {
		s := string(r)
		style := textStyle
		if i >= lo && i < hi {
			style = diff
			if r == ' ' && (i == lo || i == hi-1) {
				s = "·"
			}
		}
		if i == point {
			style = cursorStyle
		}
		b.WriteString(style.Render(s))
	}
	if point == len(text) {
		b.WriteString(cursorStyle.Render(" "))
	}
	return b.String()
}

var (
	wrongStyle   = lipgloss.NewStyle().Foreground(red).Underline(true)
	missingStyle = lipgloss.NewStyle().Foreground(yellow).Underline(true)
)

func (m *puzzle) View() string {
	l := m.level
	var sections []string

	header := titleStyle.Render(strings.ToUpper(m.pack.Title)) +
		dimStyle.Render(fmt.Sprintf("  level %d/%d  ", m.idx+1, len(m.pack.Levels))) +
		headingStyle.Render(l.Title)
	sections = append(sections, header, "")

	for _, k := range l.New {
		sections = append(sections, okStyle.Render("NEW ")+keyStyle.Render(prettyKey(k))+"  "+textStyle.Render(game.KeyInfo[k]))
	}
	if l.Tip != "" {
		wrap := 80
		if m.width > 0 {
			wrap = min(wrap, m.width-6)
		}
		sections = append(sections, helpStyle.Width(wrap).Render(l.Tip))
	}
	sections = append(sections, "")

	cur, goal := []rune(m.ed.Text()), []rune(l.Target)
	clo, chi := diffRange(cur, goal)
	glo, ghi := diffRange(goal, cur)
	point := m.ed.Point()
	if m.phase == solved {
		point = -1
	}
	lines := dimStyle.Render("now   ") + okStyle.Render("$ ") + renderEdit(cur, point, clo, chi, wrongStyle) + "\n" +
		dimStyle.Render("goal  ") + okStyle.Render("$ ") + renderEdit(goal, -1, glo, ghi, missingStyle)
	sections = append(sections, panelStyle.Render(lines))

	score := fmt.Sprintf("keys %d", m.keys)
	switch {
	case m.keys > l.Par:
		score = badStyle.Render(score)
	default:
		score = keyStyle.Render(score)
	}
	status := score + dimStyle.Render(fmt.Sprintf("  ·  par %d", l.Par))
	if m.ed.Pending() != "" {
		status += keyStyle.Render("   " + prettyKey(m.ed.Pending()) + " …")
	}
	sections = append(sections, status, lastN(m.trail, 16))

	if entries, idx := m.ed.KillRing().Entries(); len(entries) > 0 {
		var ring []string
		for i := len(entries) - 1; i >= 0 && len(entries)-i <= 4; i-- {
			e := fmt.Sprintf("%q", entries[i])
			if i == idx {
				ring = append(ring, keyStyle.Render("▸"+e))
			} else {
				ring = append(ring, dimStyle.Render(" "+e))
			}
		}
		sections = append(sections, dimStyle.Render("kill ring ")+strings.Join(ring, " "))
	}

	if m.flash != "" {
		sections = append(sections, "", keyStyle.Render("» "+m.flash))
	}

	if m.phase == solved {
		sections = append(sections, "", m.solvedView())
	} else {
		sections = append(sections, "", helpStyle.Render("ctrl+c restart · esc levels"))
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

func (m *puzzle) solvedView() string {
	l := m.level
	var b strings.Builder
	b.WriteString(starString(m.stars) + "  " + okStyle.Render(fmt.Sprintf("SOLVED in %d %s", m.keys, plural(m.keys, "key", "keys"))) +
		dimStyle.Render(fmt.Sprintf(" (par %d)", l.Par)))
	switch {
	case m.keys < l.Par:
		b.WriteString(titleStyle.Render("  UNDER PAR! You beat the solver."))
	case m.improved:
		b.WriteString(keyStyle.Render("  new best"))
	}
	b.WriteString("\n")

	route := make([]string, len(l.Solution))
	for i, k := range l.Solution {
		s := prettyKey(k)
		if k == " " {
			s = "␣"
		}
		switch {
		case m.ghostEd != nil && i == m.ghostStep-1:
			route[i] = cursorStyle.Render(s)
		case m.ghostEd != nil && i < m.ghostStep:
			route[i] = keyStyle.Render(s)
		default:
			route[i] = dimStyle.Render(s)
		}
	}
	b.WriteString(dimStyle.Render("par route  ") + strings.Join(route, " ") + "\n")
	if m.ghostEd != nil {
		g := []rune(m.ghostEd.Text())
		b.WriteString(dimStyle.Render("ghost      ") + okStyle.Render("$ ") + renderEdit(g, m.ghostEd.Point(), 0, 0, textStyle) + "\n")
	}

	next := "enter next level"
	if m.idx+1 >= len(m.pack.Levels) {
		next = "enter back to levels (pack complete!)"
	}
	b.WriteString("\n" + helpStyle.Render(next+" · g watch the par route · r retry · esc levels"))
	return panelStyle.BorderForeground(green).Render(b.String())
}

// lastN joins the last n trail entries, with an ellipsis if some were cut.
func lastN(trail []string, n int) string {
	if len(trail) <= n {
		return strings.Join(trail, " ")
	}
	return dimStyle.Render("… ") + strings.Join(trail[len(trail)-n:], " ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
