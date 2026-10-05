package ui

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iskaa02/bashgolf/internal/game"
)

type (
	rushTick struct{ run int }
	// rushTaskMsg delivers a task made in the background: for the free
	// run with this run id, or for this level number.
	rushTaskMsg struct {
		run, level int
		task       game.RushTask
	}
)

// rushScreen is Line Rush: fix broken commands against the clock, as a
// free run or level by level with a review after each.
type rushScreen struct {
	prog   *game.Progress
	phase  dashPhase
	mode   dashMode
	choice dashMode

	r     *game.Rush
	run   int
	start time.Time
	limit time.Duration
	left  time.Duration

	level      int
	levelTask  *game.RushTask // the current level's task, for retries
	nextTask   *game.RushTask // made in the background for level+1
	making     bool           // a background task is being made
	cleared    bool
	newBest    bool
	highScore  bool
	trail      []string
	flash      string
	lastReview *game.RushRecord // free run: the last sloppy fix, to learn from
}

func newRush(prog *game.Progress) *rushScreen {
	return &rushScreen{prog: prog, level: 1}
}

func newRNG() *rand.Rand {
	return rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), rand.Uint64()))
}

// makeTask generates a task off the UI goroutine.
func makeTask(cfg game.RushConfig, run, level int) tea.Cmd {
	rng := newRNG()
	return func() tea.Msg {
		return rushTaskMsg{run: run, level: level, task: game.GenerateRushTask(rng, cfg)}
	}
}

func (m *rushScreen) tick() tea.Cmd {
	run := m.run
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return rushTick{run} })
}

func (m *rushScreen) begin(r *game.Rush, limit time.Duration) {
	m.run++
	m.phase = dashRunning
	m.r = r
	m.start = time.Now()
	m.limit, m.left = limit, limit
	m.trail, m.flash, m.lastReview = nil, "", nil
	m.cleared, m.newBest, m.highScore, m.making = false, false, false, false
}

func (m *rushScreen) startFree() tea.Cmd {
	m.mode = dashFree
	m.begin(game.NewRush(newRNG(), game.FreeRush, nil), game.RushDuration)
	return tea.Batch(m.tick(), m.prefetch())
}

// prefetch makes the next free-run task in the background.
func (m *rushScreen) prefetch() tea.Cmd {
	if m.mode != dashFree || m.making || m.r.Queued() > 0 {
		return nil
	}
	m.making = true
	return makeTask(m.r.Config(), m.run, 0)
}

// startLevel plays m.level, reusing its task on a retry.
func (m *rushScreen) startLevel(retry bool) tea.Cmd {
	m.mode = dashLevels
	cfg, limit := game.RushLevel(m.level)
	var first *game.RushTask
	switch {
	case retry && m.levelTask != nil:
		first = m.levelTask
	case m.nextTask != nil:
		first = m.nextTask
	}
	m.nextTask = nil
	m.begin(game.NewRush(newRNG(), cfg, first), limit)
	task := m.r.Task
	m.levelTask = &task
	return m.tick()
}

func (m *rushScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case rushTick:
		if msg.run != m.run || m.phase != dashRunning {
			return m, nil
		}
		m.left = m.limit - time.Since(m.start)
		if m.left <= 0 {
			return m, m.finish()
		}
		return m, m.tick()
	case rushTaskMsg:
		switch {
		case msg.level == 0 && msg.run == m.run && m.phase == dashRunning:
			m.making = false
			m.r.Queue(msg.task)
		case msg.level > 0 && msg.level == m.level+1:
			m.nextTask = &msg.task
		}
		return m, nil
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *rushScreen) key(msg tea.KeyMsg) (Screen, tea.Cmd) {
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
			return m, m.startLevel(false)
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
				return m, m.startLevel(false)
			}
			return m, m.startLevel(true)
		case "r":
			return m, m.startLevel(true)
		case "esc", "q":
			m.phase = dashChoose
		}
	case dashRunning:
		return m, m.press(msg)
	}
	return m, nil
}

func (m *rushScreen) press(msg tea.KeyMsg) tea.Cmd {
	name := msg.String()
	switch {
	case name == "esc" && m.r.Ed.Search().Active:
	case name == "esc":
		m.phase = dashChoose
		m.run++
		return nil
	case name == "ctrl+c":
		m.r.Restart()
		m.flash = dimStyle.Render("line put back (the keys you spent still count)")
		return nil
	case name == "enter" && !m.r.Ed.Search().Active:
		m.flash = dimStyle.Render("no need for Enter: it counts as fixed the moment it matches")
		return nil
	}

	k := toKey(msg)
	res := m.r.Press(k)
	label := prettyKey(name)
	if k.Text != "" {
		label = strings.ReplaceAll(k.Text, " ", "␣")
	}
	m.trail = append(m.trail, label)
	if !res.Fixed {
		m.flash = ""
		return nil
	}
	m.trail = nil
	if m.r.Done() {
		return m.finish()
	}
	last := m.r.Records[len(m.r.Records)-1]
	if res.Perfect {
		m.flash = okStyle.Render(fmt.Sprintf("FIXED, PERFECT +%d", res.Points))
		if m.r.Combo > 1 {
			m.flash += titleStyle.Render(fmt.Sprintf("  combo ×%d", m.r.Combo))
		}
		m.lastReview = nil
	} else {
		m.flash = keyStyle.Render(fmt.Sprintf("fixed +%d", res.Points)) + dimStyle.Render("  combo lost")
		m.lastReview = &last
	}
	return m.prefetch()
}

// finish ends a run: by the clock, or by fixing a level's command.
func (m *rushScreen) finish() tea.Cmd {
	m.run++ // stop the ticker
	if m.mode == dashFree {
		m.phase = dashOver
		m.highScore = m.prog.RecordScore("rush", m.r.Score)
		return nil
	}
	m.phase = dashReview
	m.left = max(0, m.limit-time.Since(m.start))
	m.cleared = m.r.Done()
	if !m.cleared {
		return nil
	}
	m.newBest = m.prog.RecordScore("rush-level", m.level)
	// Make the next level's task while the player reads the review.
	cfg, _ := game.RushLevel(m.level + 1)
	return makeTask(cfg, 0, m.level+1)
}

// --- view ---

func (m *rushScreen) View() string {
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
	sections = append(sections, "", rushLegend())
	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// rushLegend is the always-visible, deliberately quiet key reminder.
func rushLegend() string {
	item := func(keys, what string) string {
		return legendKeyStyle.Render(keys) + dimStyle.Render(" "+what)
	}
	sep := dimStyle.Render("  ·  ")
	return item("Ctrl+A/E", "start/end") + sep + item("Alt+B/F", "word ←/→") + sep + item("Ctrl+W", "cut word ←") + sep + item("Alt+D", "cut word →") + "\n" +
		item("Ctrl+K", "cut to end") + sep + item("Ctrl+U", "cut to start") + sep + item("Ctrl+Y", "paste") + sep + item("Ctrl+_", "undo") + "\n" +
		item("Alt+U/L/C", "UPPER/lower/Capital") + sep + item("Ctrl+T", "swap chars") + sep + item("Alt+T", "swap words") + sep + item("Ctrl+C", "reset")
}

func (m *rushScreen) chooseView() []string {
	option := func(mode dashMode, title, desc, stat string) string {
		pointer, style := "   ", dimStyle
		if m.choice == mode {
			pointer, style = keyStyle.Render(" ▸ "), keyStyle
		}
		return pointer + style.Render(fmt.Sprintf("%-10s", title)) + textStyle.Render(desc) + dimStyle.Render(stat)
	}
	freeStat, levelStat := "", ""
	if hs := m.prog.HighScore("rush"); hs > 0 {
		freeStat = fmt.Sprintf("   high score %d", hs)
	}
	if best := m.prog.HighScore("rush-level"); best > 0 {
		levelStat = fmt.Sprintf("   best level %d", best)
	}
	return []string{
		titleStyle.Render("LINE RUSH"), "",
		textStyle.Render("Each command has a slip in it: a doubled word, Caps Lock, swapped"),
		textStyle.Render("letters... Fix it with as few keys as you can. It counts the moment it matches."),
		"",
		option(dashFree, "Free run", fmt.Sprintf("%d seconds, endless fixes, combo scoring", int(game.RushDuration.Seconds())), freeStat),
		option(dashLevels, "Levels", "one fix per level, then see how you could have done better", levelStat),
		"",
		helpStyle.Render("↑/↓ choose · enter start · esc back"),
	}
}

func (m *rushScreen) runningView() []string {
	r := m.r
	title := titleStyle.Render("LINE RUSH")
	stats := keyStyle.Render(fmt.Sprintf("score %d", r.Score)) + dimStyle.Render(fmt.Sprintf("   fixed %d   combo ×%d", r.Fixed, r.Combo))
	if m.mode == dashLevels {
		title = titleStyle.Render(fmt.Sprintf("LEVEL %d", m.level))
		stats = keyStyle.Render(fmt.Sprintf("fix it in %d %s", r.Task.Par, plural(r.Task.Par, "key", "keys")))
	}
	cur, goal := []rune(r.Ed.Text()), []rune(r.Task.Target)
	clo, chi := diffRange(cur, goal)
	glo, ghi := diffRange(goal, cur)
	prompt := okStyle.Render("$ ")
	if s := r.Ed.Search(); s.Active {
		prompt = keyStyle.Render(fmt.Sprintf("(reverse-i-search)`%s': ", s.Query))
	}
	lines := dimStyle.Render("now   ") + prompt + renderEdit(cur, r.Ed.Point(), clo, chi, wrongStyle) + "\n" +
		dimStyle.Render("goal  ") + okStyle.Render("$ ") + renderEdit(goal, -1, glo, ghi, missingStyle)

	sections := []string{
		title + "  " + timeBar(m.left, m.limit, 30) + textStyle.Render(fmt.Sprintf(" %4.1fs", max(0, m.left.Seconds()))),
		"",
		stats + dimStyle.Render("   slip: ") + textStyle.Render(r.Task.Slip),
		"",
		panelStyle.Render(lines),
		dimStyle.Render(fmt.Sprintf("par %d · keys %d  ", r.Task.Par, r.Keys)) + keyStyle.Render(lastN(m.trail, 14)),
		"",
		m.flash,
	}
	if m.lastReview != nil {
		rec := m.lastReview
		sections = append(sections, dimStyle.Render(fmt.Sprintf("last one: you %d keys, par route ", len(rec.Keys)))+
			okStyle.Render(rushKeyList(rec.Task.Route)))
	}
	return append(sections, "", helpStyle.Render("esc stop"))
}

func (m *rushScreen) overView() []string {
	r := m.r
	result := okStyle.Render(fmt.Sprintf("SCORE %d", r.Score))
	if m.highScore {
		result += titleStyle.Render("  NEW HIGH SCORE!")
	} else {
		result += dimStyle.Render(fmt.Sprintf("  high score %d", m.prog.HighScore("rush")))
	}
	accuracy := 0
	if r.Fixed > 0 {
		accuracy = 100 * r.Perfect / r.Fixed
	}
	sections := []string{
		titleStyle.Render("LINE RUSH") + dimStyle.Render("  time's up"), "",
		panelStyle.BorderForeground(green).Render(result + "\n\n" +
			textStyle.Render(fmt.Sprintf("commands fixed  %d\nperfect         %d  (%d%%)\nbest combo      ×%d", r.Fixed, r.Perfect, accuracy, r.BestCombo))),
	}
	if tip := rushTip(r.Records); tip != nil {
		sections = append(sections, "")
		sections = append(sections, tip...)
	}
	return append(sections, "", helpStyle.Render("enter play again · esc back"))
}

// reviewView shows the level's command, what you pressed against the par
// route, and the shortcut that would have helped most.
func (m *rushScreen) reviewView() []string {
	r := m.r
	rec := r.Current()
	if m.cleared {
		rec = r.Records[0]
	}
	var header string
	if m.cleared {
		header = okStyle.Render(fmt.Sprintf("LEVEL %d FIXED", m.level)) +
			dimStyle.Render(fmt.Sprintf("  in %.1fs of %ds   ", (m.limit-m.left).Seconds(), int(m.limit.Seconds()))) +
			starString(game.Stars(len(rec.Keys), rec.Task.Par)) + dimStyle.Render(fmt.Sprintf("  keys %d · par %d", len(rec.Keys), rec.Task.Par))
		if m.newBest {
			header += titleStyle.Render("  new best level!")
		}
	} else {
		header = badStyle.Render(fmt.Sprintf("LEVEL %d: TIME'S UP", m.level)) + dimStyle.Render(fmt.Sprintf("  (%ds)", int(m.limit.Seconds())))
	}

	start, goal := []rune(rec.Task.Start), []rune(rec.Task.Target)
	slo, shi := diffRange(start, goal)
	glo, ghi := diffRange(goal, start)
	sections := []string{header, "",
		dimStyle.Render("  was   ") + renderEdit(start, -1, slo, shi, wrongStyle) + dimStyle.Render("   ("+rec.Task.Slip+")"),
		dimStyle.Render("  fixed ") + renderEdit(goal, -1, glo, ghi, missingStyle),
		"",
	}
	you := keyStyle.Render(rushKeyList(rec.Keys)) + dimStyle.Render(fmt.Sprintf("  (%d)", len(rec.Keys)))
	if len(rec.Keys) == 0 {
		you = dimStyle.Render("nothing")
	}
	parRoute := okStyle.Render(rushKeyList(rec.Task.Route)) + dimStyle.Render(fmt.Sprintf("  (%d)", rec.Task.Par))
	switch {
	case !m.cleared:
		sections = append(sections, dimStyle.Render("  you   ")+you+badStyle.Render("  not fixed in time"), dimStyle.Render("  par   ")+parRoute)
	case rec.Wasted() == 0:
		sections = append(sections, dimStyle.Render("  you   ")+you+okStyle.Render("  ✓ perfect, as good as par"))
	default:
		sections = append(sections, dimStyle.Render("  you   ")+you+badStyle.Render(fmt.Sprintf("  %d too many", rec.Wasted())), dimStyle.Render("  par   ")+parRoute)
	}

	if tip := rushTip([]game.RushRecord{rec}); tip != nil {
		sections = append(sections, "")
		sections = append(sections, tip...)
	} else if m.cleared {
		sections = append(sections, "", okStyle.Render("Flawless."))
	}

	sections = append(sections, "")
	if m.cleared {
		sections = append(sections, helpStyle.Render(fmt.Sprintf("enter level %d · r retry this level · esc back", m.level+1)))
	} else {
		sections = append(sections, helpStyle.Render("enter retry · esc back"))
	}
	return sections
}

// rushTip names the shortcut that would have saved the most keys.
func rushTip(records []game.RushRecord) []string {
	key, n, wasted := game.MostMissedEdit(records)
	if key == "" {
		return nil
	}
	why := fmt.Sprintf("The par route used it on %d fixes where you didn't, and you lost %d keys there.", n, wasted)
	switch {
	case len(records) == 1 && records[0].Unfinished:
		why = "It's the key to this fix: see the par route above."
	case n == 1:
		why = fmt.Sprintf("The par route used it where you didn't, and you lost %d keys there.", wasted)
	}
	return []string{
		textStyle.Render("Try next time: ") + keyStyle.Render(prettyKey(key)) + textStyle.Render(" — "+game.KeyInfo[key]),
		dimStyle.Render(why),
	}
}

// rushKeyList is keyList that also shows runs of typed characters as text.
func rushKeyList(keys []string) string {
	var parts []string
	var typed strings.Builder
	var run []string
	flushTyped := func() {
		if typed.Len() > 0 {
			parts = append(parts, fmt.Sprintf("type %q", typed.String()))
			typed.Reset()
		}
	}
	flushRun := func() {
		if len(run) > 0 {
			parts = append(parts, keyList(run))
			run = nil
		}
	}
	for _, k := range keys {
		if len([]rune(k)) == 1 {
			flushRun()
			typed.WriteString(k)
			continue
		}
		flushTyped()
		run = append(run, k)
	}
	flushRun()
	flushTyped()
	return strings.Join(parts, " ")
}
