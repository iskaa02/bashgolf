package ui

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termgame/internal/game"
)

type dashPhase int

const (
	dashReady dashPhase = iota
	dashRunning
	dashOver
)

type dashTick struct{ run int }

var targetStyle = lipgloss.NewStyle().Background(yellow).Foreground(lipgloss.Color("#000000")).Bold(true)

// dashScreen is the Cursor Dash arcade mode.
type dashScreen struct {
	prog  *game.Progress
	phase dashPhase
	d     *game.Dash
	run   int // bumps per run so stale ticks are dropped
	end   time.Time
	left  time.Duration

	trail     []string // keys pressed for the current target
	flash     string
	lastRoute string // shown after a sloppy hit
	highScore bool
}

func newDash(prog *game.Progress) *dashScreen {
	return &dashScreen{prog: prog}
}

func (m *dashScreen) tick() tea.Cmd {
	run := m.run
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return dashTick{run} })
}

func (m *dashScreen) start() tea.Cmd {
	m.run++
	m.phase = dashRunning
	m.d = game.NewDash(rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0)))
	m.end = time.Now().Add(game.DashDuration)
	m.left = game.DashDuration
	m.trail, m.flash, m.lastRoute, m.highScore = nil, "", "", false
	return m.tick()
}

func (m *dashScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case dashTick:
		if msg.run != m.run || m.phase != dashRunning {
			return m, nil
		}
		m.left = time.Until(m.end)
		if m.left <= 0 {
			m.phase = dashOver
			m.highScore = m.prog.RecordScore("dash", m.d.Score)
			return m, nil
		}
		return m, m.tick()

	case tea.KeyMsg:
		name := msg.String()
		switch m.phase {
		case dashReady, dashOver:
			switch name {
			case "enter", " ":
				return m, m.start()
			case "esc", "q":
				return m, back
			}
		case dashRunning:
			if name == "esc" {
				m.phase = dashReady
				m.run++
				return m, nil
			}
			m.press(msg)
		}
	}
	return m, nil
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
		m.flash = okStyle.Render(fmt.Sprintf("PERFECT +%d", res.Points))
		if m.d.Combo > 1 {
			m.flash += titleStyle.Render(fmt.Sprintf("  combo ×%d", m.d.Combo))
		}
		m.lastRoute = ""
	} else {
		m.flash = keyStyle.Render(fmt.Sprintf("hit +%d", res.Points)) + dimStyle.Render("  combo lost")
		route := make([]string, len(res.Route))
		for i, k := range res.Route {
			route[i] = prettyKey(k)
		}
		m.lastRoute = strings.Join(route, " ")
	}
}

func (m *dashScreen) View() string {
	var sections []string
	header := titleStyle.Render("CURSOR DASH")
	switch m.phase {
	case dashReady:
		sections = append(sections, header, "",
			textStyle.Render("Targets light up on a long command. Move the cursor onto each one."),
			textStyle.Render(fmt.Sprintf("You have %d seconds. The line is read-only: only movement keys work.", int(game.DashDuration.Seconds()))),
			"",
			keyStyle.Render("Ctrl+A Ctrl+E")+dimStyle.Render("  line start / end")+"   "+keyStyle.Render("Alt+F Alt+B")+dimStyle.Render("  word hops"),
			keyStyle.Render("Ctrl+F Ctrl+B")+dimStyle.Render("  one char")+"          "+keyStyle.Render("Ctrl+X Ctrl+X")+dimStyle.Render("  jump to mark"),
			"",
			textStyle.Render("Hit a target in par keys for a ")+okStyle.Render("PERFECT")+textStyle.Render(": each one in a row grows your combo."),
		)
		if hs := m.prog.HighScore("dash"); hs > 0 {
			sections = append(sections, "", dimStyle.Render(fmt.Sprintf("high score %d", hs)))
		}
		sections = append(sections, "", helpStyle.Render("enter start · esc back"))

	case dashRunning:
		d := m.d
		secs := max(0, m.left.Seconds())
		bar := timeBar(m.left, game.DashDuration, 30)
		sections = append(sections,
			header+"  "+bar+textStyle.Render(fmt.Sprintf(" %4.1fs", secs)),
			"",
			keyStyle.Render(fmt.Sprintf("score %d", d.Score))+dimStyle.Render(fmt.Sprintf("   hits %d   combo ×%d", d.Hits, d.Combo)),
			"",
			panelStyle.Render(okStyle.Render("$ ")+renderDashLine([]rune(d.Ed.Text()), d.Ed.Point(), d.Target)),
			dimStyle.Render(fmt.Sprintf("par %d · keys %d  ", d.Par, d.Keys))+keyStyle.Render(strings.Join(m.trail, " ")),
			"",
			m.flash,
		)
		if m.lastRoute != "" {
			sections = append(sections, dimStyle.Render("par route was: ")+keyStyle.Render(m.lastRoute))
		}
		sections = append(sections, "", helpStyle.Render("esc stop"))

	case dashOver:
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
		sections = append(sections, header+dimStyle.Render("  time's up"), "",
			panelStyle.BorderForeground(green).Render(result+"\n\n"+
				textStyle.Render(fmt.Sprintf("targets hit   %d\nperfect       %d  (%d%%)\nbest combo    ×%d", d.Hits, d.Perfect, accuracy, d.BestCombo))),
			"", helpStyle.Render("enter play again · esc back"))
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// renderDashLine draws the command with the cursor block and the target.
func renderDashLine(text []rune, point, target int) string {
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
		case i < len(text):
			b.WriteString(textStyle.Render(s))
		}
	}
	return b.String()
}

// timeBar draws the remaining time, turning red near the end.
func timeBar(left, total time.Duration, width int) string {
	filled := int(float64(width) * left.Seconds() / total.Seconds())
	filled = max(0, min(width, filled))
	style := okStyle
	switch {
	case left < 10*time.Second:
		style = badStyle
	case left < 25*time.Second:
		style = keyStyle
	}
	return style.Render(strings.Repeat("█", filled)) + dimStyle.Render(strings.Repeat("░", width-filled))
}
