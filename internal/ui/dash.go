package ui

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iskaa02/termgame/internal/game"
)

type dashPhase int

const (
	dashChoose dashPhase = iota
	dashRunning
	dashOver   // free run finished
	dashReview // level finished or failed
)

type dashMode int

const (
	dashFree dashMode = iota
	dashLevels
)

type dashTick struct{ run int }

var (
	targetStyle    = lipgloss.NewStyle().Background(yellow).Foreground(lipgloss.Color("#000000")).Bold(true)
	startStyle     = lipgloss.NewStyle().Background(lipgloss.Color("#4a4a5a")).Foreground(text)
	legendKeyStyle = dimStyle.Bold(true)
)

// dashScreen is Cursor Dash: a free 60-second run, or levels with a review
// after each one.
type dashScreen struct {
	prog   *game.Progress
	phase  dashPhase
	mode   dashMode
	choice dashMode // highlighted on the choose screen

	d     *game.Dash
	run   int // bumps per run so stale ticks are dropped
	start time.Time
	limit time.Duration
	left  time.Duration

	level     int
	levelSeed uint64 // retrying a level replays the same line and first target
	cleared   bool
	newBest   bool

	trail     []string // keys pressed for the current target
	flash     string
	lastRoute string // free mode: shown after a sloppy hit
	highScore bool
}

func newDash(prog *game.Progress) *dashScreen {
	return &dashScreen{prog: prog, level: 1}
}

func (m *dashScreen) tick() tea.Cmd {
	run := m.run
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return dashTick{run} })
}

func (m *dashScreen) begin(cfg game.DashConfig, limit time.Duration, seed uint64) tea.Cmd {
	m.run++
	m.phase = dashRunning
	m.d = game.NewDash(rand.New(rand.NewPCG(seed, 0)), cfg)
	m.start = time.Now()
	m.limit, m.left = limit, limit
	m.trail, m.flash, m.lastRoute = nil, "", ""
	m.highScore, m.cleared, m.newBest = false, false, false
	return m.tick()
}

func (m *dashScreen) startFree() tea.Cmd {
	m.mode = dashFree
	return m.begin(game.FreeDash, game.DashDuration, uint64(time.Now().UnixNano()))
}

// startLevel plays m.level; a fresh seed gives a new random level.
func (m *dashScreen) startLevel(fresh bool) tea.Cmd {
	m.mode = dashLevels
	if fresh || m.levelSeed == 0 {
		m.levelSeed = uint64(time.Now().UnixNano())
	}
	cfg, limit := game.DashLevel(m.level)
	return m.begin(cfg, limit, m.levelSeed)
}

func (m *dashScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case dashTick:
		if msg.run != m.run || m.phase != dashRunning {
			return m, nil
		}
		m.left = m.limit - time.Since(m.start)
		if m.left <= 0 {
			m.finish()
			return m, nil
		}
		return m, m.tick()

	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *dashScreen) key(msg tea.KeyMsg) (Screen, tea.Cmd) {
	name := msg.String()
	switch m.phase {
	case dashChoose:
		switch name {
		case "up", "down", "k", "j", "tab", "ctrl+p", "ctrl+n":
			m.choice = 1 - m.choice
		case "enter", " ":
			if m.choice == dashFree {
				return m, m.startFree()
			}
			return m, m.startLevel(true)
		case "esc", "q":
			return m, back
		}
	case dashOver:
		switch name {
		case "enter", " ":
			return m, m.startFree()
		case "esc", "q":
			m.phase = dashChoose
		}
	case dashReview:
		switch name {
		case "enter", " ", "n":
			if m.cleared {
				m.level++
				return m, m.startLevel(true)
			}
			return m, m.startLevel(false)
		case "r":
			return m, m.startLevel(false)
		case "esc", "q":
			m.phase = dashChoose
		}
	case dashRunning:
		if name == "esc" {
			m.phase = dashChoose
			m.run++
			return m, nil
		}
		m.press(msg)
		if m.d.Done() {
			m.finish()
		}
	}
	return m, nil
}

// finish ends a run: by the clock, or by hitting every target of a level.
func (m *dashScreen) finish() {
	m.run++ // stop the ticker
	switch m.mode {
	case dashFree:
		m.phase = dashOver
		m.highScore = m.prog.RecordScore("dash", m.d.Score)
	case dashLevels:
		m.phase = dashReview
		m.left = max(0, m.limit-time.Since(m.start))
		m.cleared = m.d.Done()
		if m.cleared {
			m.newBest = m.prog.RecordScore("dash-level", m.level)
		}
	}
}

func (m *dashScreen) press(msg tea.KeyMsg) {
	res := m.d.Press(toKey(msg))
	if res.Ignored {
		m.flash = badStyle.Render("movement keys only: the line is read-only here")
		return
	}
	m.trail = append(m.trail, prettyKey(msg.String()))
	if !res.Hit {
		m.flash = ""
		return
	}
	m.trail = nil
	if res.Perfect {
		m.flash = okStyle.Render("PERFECT")
		if m.mode == dashFree {
			m.flash += okStyle.Render(fmt.Sprintf(" +%d", res.Points))
			if m.d.Combo > 1 {
				m.flash += titleStyle.Render(fmt.Sprintf("  combo ×%d", m.d.Combo))
			}
		}
		m.lastRoute = ""
		return
	}
	m.flash = keyStyle.Render("hit")
	if m.mode == dashFree {
		m.flash += keyStyle.Render(fmt.Sprintf(" +%d", res.Points)) + dimStyle.Render("  combo lost")
		m.lastRoute = keyList(res.Route)
	}
}

// --- view ---

func (m *dashScreen) View() string {
	var sections []string
	switch m.phase {
	case dashChoose:
		sections = m.chooseView()
	case dashRunning:
		sections = m.runningView()
	case dashOver:
		sections = m.overView()
	case dashReview:
		sections = m.reviewView()
	}
	sections = append(sections, "", dashLegend())
	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// dashLegend is the always-visible, deliberately quiet key reminder.
func dashLegend() string {
	item := func(keys, what string) string {
		return legendKeyStyle.Render(keys) + dimStyle.Render(" "+what)
	}
	sep := dimStyle.Render("  ·  ")
	return item("Ctrl+A", "line start") + sep + item("Ctrl+E", "line end") + sep + item("Alt+B/Alt+F", "word ←/→") + "\n" +
		item("Ctrl+B/Ctrl+F", "char ←/→") + sep + item("Ctrl+X Ctrl+X", "jump to mark (line start, then back)")
}

func (m *dashScreen) chooseView() []string {
	option := func(mode dashMode, title, desc, stat string) string {
		pointer, style := "   ", dimStyle
		if m.choice == mode {
			pointer, style = keyStyle.Render(" ▸ "), keyStyle
		}
		return pointer + style.Render(fmt.Sprintf("%-10s", title)) + textStyle.Render(desc) + dimStyle.Render(stat)
	}
	freeStat, levelStat := "", ""
	if hs := m.prog.HighScore("dash"); hs > 0 {
		freeStat = fmt.Sprintf("   high score %d", hs)
	}
	if best := m.prog.HighScore("dash-level"); best > 0 {
		levelStat = fmt.Sprintf("   best level %d", best)
	}
	return []string{
		titleStyle.Render("CURSOR DASH"), "",
		textStyle.Render("Targets light up on a long command. Move the cursor onto each one"),
		textStyle.Render("in as few keys as you can. The line is read-only: only movement works."),
		"",
		option(dashFree, "Free run", fmt.Sprintf("%d seconds, endless targets, combo scoring", int(game.DashDuration.Seconds())), freeStat),
		option(dashLevels, "Levels", "beat the clock, then see how you could have done better", levelStat),
		"",
		helpStyle.Render("↑/↓ choose · enter start · esc back"),
	}
}

func (m *dashScreen) runningView() []string {
	d := m.d
	title := titleStyle.Render("CURSOR DASH")
	stats := keyStyle.Render(fmt.Sprintf("score %d", d.Score)) + dimStyle.Render(fmt.Sprintf("   hits %d   combo ×%d", d.Hits, d.Combo))
	if m.mode == dashLevels {
		title = titleStyle.Render(fmt.Sprintf("LEVEL %d", m.level))
		stats = keyStyle.Render(fmt.Sprintf("reach the target in %d %s", d.Par, plural(d.Par, "key", "keys")))
	}
	sections := []string{
		title + "  " + timeBar(m.left, m.limit, 30) + textStyle.Render(fmt.Sprintf(" %4.1fs", max(0, m.left.Seconds()))),
		"",
		stats,
		"",
		panelStyle.Render(okStyle.Render("$ ") + renderDashLine([]rune(d.Ed.Text()), d.Ed.Point(), d.Target, -1)),
		dimStyle.Render(fmt.Sprintf("par %d · keys %d  ", d.Par, d.Keys)) + keyStyle.Render(strings.Join(m.trail, " ")),
		"",
		m.flash,
	}
	if m.lastRoute != "" {
		sections = append(sections, dimStyle.Render("par route was: ")+keyStyle.Render(m.lastRoute))
	}
	return append(sections, "", helpStyle.Render("esc stop"))
}

func (m *dashScreen) overView() []string {
	d := m.d
	result := okStyle.Render(fmt.Sprintf("SCORE %d", d.Score))
	if m.highScore {
		result += titleStyle.Render("  NEW HIGH SCORE!")
	} else {
		result += dimStyle.Render(fmt.Sprintf("  high score %d", m.prog.HighScore("dash")))
	}
	accuracy := 0
	if d.Hits > 0 {
		accuracy = 100 * d.Perfect / d.Hits
	}
	return []string{
		titleStyle.Render("CURSOR DASH") + dimStyle.Render("  time's up"), "",
		panelStyle.BorderForeground(green).Render(result + "\n\n" +
			textStyle.Render(fmt.Sprintf("targets hit   %d\nperfect       %d  (%d%%)\nbest combo    ×%d", d.Hits, d.Perfect, accuracy, d.BestCombo))),
		"", helpStyle.Render("enter play again · esc back"),
	}
}

// reviewView shows each target of the level: where it was, what you
// pressed and a par route, then the one shortcut that would help most.
func (m *dashScreen) reviewView() []string {
	d := m.d
	records := d.Records
	unfinished := !m.cleared
	if unfinished {
		records = append(records, d.Current())
	}

	keys, par := 0, 0
	for _, r := range d.Records {
		keys += len(r.Keys)
		par += r.Par
	}
	var header string
	if m.cleared {
		header = okStyle.Render(fmt.Sprintf("LEVEL %d CLEARED", m.level)) +
			dimStyle.Render(fmt.Sprintf("  in %.1fs of %ds   ", (m.limit-m.left).Seconds(), int(m.limit.Seconds()))) +
			starString(game.Stars(keys, par)) + dimStyle.Render(fmt.Sprintf("  keys %d · par %d", keys, par))
		if m.newBest {
			header += titleStyle.Render("  new best level!")
		}
	} else {
		header = badStyle.Render(fmt.Sprintf("LEVEL %d: TIME'S UP", m.level)) +
			dimStyle.Render(fmt.Sprintf("  (%ds)", int(m.limit.Seconds())))
	}

	sections := []string{header, ""}
	for i, r := range records {
		last := unfinished && i == len(records)-1
		indent := "  "
		if len(records) > 1 {
			indent = fmt.Sprintf("%2d ", i+1)
		}
		pad := strings.Repeat(" ", len(indent))
		sections = append(sections, dimStyle.Render(indent)+renderDashLine([]rune(r.Line), -1, r.Target, r.From))

		you := keyStyle.Render(keyList(r.Keys)) + dimStyle.Render(fmt.Sprintf("  (%d)", len(r.Keys)))
		if len(r.Keys) == 0 {
			you = dimStyle.Render("nothing")
		}
		parRoute := okStyle.Render(keyList(r.Route)) + dimStyle.Render(fmt.Sprintf("  (%d)", r.Par))
		switch {
		case last:
			sections = append(sections,
				dimStyle.Render(pad+"you  ")+you+badStyle.Render("  didn't reach it"),
				dimStyle.Render(pad+"par  ")+parRoute)
		case r.Wasted() == 0:
			sections = append(sections,
				dimStyle.Render(pad+"you  ")+you+okStyle.Render("  ✓ perfect, as good as par"))
		default:
			sections = append(sections,
				dimStyle.Render(pad+"you  ")+you+badStyle.Render(fmt.Sprintf("  %d too many", r.Wasted())),
				dimStyle.Render(pad+"par  ")+parRoute)
		}
	}

	sections = append(sections, "")
	if key, n, wasted := game.MostMissed(records); key != "" {
		targets := "1 target"
		if n > 1 {
			targets = fmt.Sprintf("%d targets", n)
		}
		sections = append(sections,
			textStyle.Render("Try next time: ")+keyStyle.Render(prettyKey(key))+textStyle.Render(" — "+game.KeyInfo[key]),
			dimStyle.Render(fmt.Sprintf("The par route used it on %s where you didn't, and you lost %d keys there.", targets, wasted)))
	} else if m.cleared {
		sections = append(sections, okStyle.Render("Flawless."))
	}
	sections = append(sections, dimStyle.Render("grey block: where the cursor started · yellow: the target"), "")

	if m.cleared {
		sections = append(sections, helpStyle.Render(fmt.Sprintf("enter level %d · r retry this level · esc back", m.level+1)))
	} else {
		sections = append(sections, helpStyle.Render("enter retry · esc back"))
	}
	return sections
}

// keyList shows keys compactly: "Alt+B ×3 Ctrl+F".
func keyList(keys []string) string {
	var parts []string
	for i := 0; i < len(keys); {
		j := i
		for j < len(keys) && keys[j] == keys[i] {
			j++
		}
		k := prettyKey(keys[i])
		if keys[i] == "ctrl+x" { // chord halves read better unmerged
			for range j - i {
				parts = append(parts, k)
			}
		} else if j-i > 1 {
			parts = append(parts, fmt.Sprintf("%s ×%d", k, j-i))
		} else {
			parts = append(parts, k)
		}
		i = j
	}
	return strings.Join(parts, " ")
}

// renderDashLine draws the command with the cursor block (point), the
// target and, for reviews, where the cursor started (-1 to leave out).
func renderDashLine(text []rune, point, target, from int) string {
	var b strings.Builder
	for i := 0; i <= len(text); i++ {
		s := " "
		if i < len(text) {
			s = string(text[i])
		}
		switch {
		case i == point:
			b.WriteString(cursorStyle.Render(s))
		case i == target:
			b.WriteString(targetStyle.Render(s))
		case i == from:
			b.WriteString(startStyle.Render(s))
		case i < len(text):
			b.WriteString(textStyle.Render(s))
		}
	}
	return b.String()
}

// timeBar draws the remaining time, turning yellow then red as it runs out.
func timeBar(left, total time.Duration, width int) string {
	filled := int(float64(width) * left.Seconds() / total.Seconds())
	filled = max(0, min(width, filled))
	style := okStyle
	switch frac := left.Seconds() / total.Seconds(); {
	case frac < 0.2:
		style = badStyle
	case frac < 0.45:
		style = keyStyle
	}
	return style.Render(strings.Repeat("█", filled)) + dimStyle.Render(strings.Repeat("░", width-filled))
}
