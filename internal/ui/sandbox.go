package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iskaa02/termgame/internal/readline"
)

var sandboxHistory = []string{
	"cd ~/projects/termgame",
	"git status",
	"mkdir -p logs/archive",
	`git commit -m "add kill ring"`,
	"tail -n 20 -f logs/app.log",
}

const sandboxStart = `git commit -m "fix the bug" --amend`

// sandbox is a free-play shell prompt showing the editor's inner state.
type sandbox struct {
	ed       *readline.Editor
	scroll   []string // fake terminal output above the prompt
	log      []string // recent actions, newest last
	flash    string
	width    int
	lastDing bool
}

func newSandbox() *sandbox {
	ed := readline.New(sandboxStart, len(sandboxStart))
	ed.SetHistory(sandboxHistory)
	return &sandbox{ed: ed}
}

func (m *sandbox) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		m.flash = ""
		m.lastDing = false
		name := msg.String()
		if m.ed.Pending() == "" {
			// Keys bash handles outside readline.
			switch name {
			case "esc":
				if m.ed.Search().Active {
					m.ed.Feed(readline.Key{Name: "ctrl+g"})
					return m, nil
				}
				return m, back
			case "ctrl+c":
				m.print("$ " + m.ed.Text() + "^C")
				m.ed.Reset("", 0)
				m.record(name, "interrupt (SIGINT): line abandoned")
				return m, nil
			case "ctrl+l":
				m.scroll = nil
				m.record(name, "clear-screen")
				return m, nil
			case "ctrl+z":
				m.flash = "Ctrl+Z suspends a running program (bring it back with fg). Nothing is running at an empty prompt."
				m.record(name, "suspend (SIGTSTP)")
				return m, nil
			}
		}

		if prefix := m.ed.Pending(); prefix != "" {
			name = prefix + " " + name
		}
		r := m.ed.Feed(toKey(msg))
		switch {
		case r.Pending:
			m.flash = prettyKey(name) + " … waiting for the second key"
		case r.Event == readline.EventAccept:
			line := m.ed.Text()
			ran, err := readline.Expand(line, m.ed.History())
			if err != nil {
				m.ed.Reset("", 0)
				m.print("$ "+line, badStyle.Render("bash: "+err.Error()))
			} else {
				m.ed.Reset(ran, len([]rune(ran)))
				m.ed.Submit()
				m.print("$ " + ran)
				if ran != line {
					m.print(dimStyle.Render("(history expansion: you typed " + line + ")"))
				} else if strings.TrimSpace(ran) != "" {
					m.print(dimStyle.Render("(sandbox: commands don't really run)"))
				}
			}
			m.record(name, "accept-line")
		case r.Event == readline.EventEOF:
			m.flash = "Ctrl+D on an empty line sends EOF: a real shell would exit now!"
			m.record(name, "end-of-file")
		case r.Unbound:
			if !msg.Paste {
				m.record(name, "(not a readline shortcut)")
			}
		default:
			m.lastDing = r.Event == readline.EventDing
			if r.Action != readline.ActSelfInsert {
				m.record(name, r.Action.String())
			}
		}
	}
	return m, nil
}

func (m *sandbox) print(lines ...string) {
	m.scroll = append(m.scroll, lines...)
	if len(m.scroll) > 8 {
		m.scroll = m.scroll[len(m.scroll)-8:]
	}
}

func (m *sandbox) record(key, action string) {
	entry := keyStyle.Render(fmt.Sprintf("%-14s", prettyKey(key))) + textStyle.Render(action)
	if m.lastDing {
		entry += badStyle.Render("  ✗ nothing to do")
	}
	m.log = append(m.log, entry)
	if len(m.log) > 8 {
		m.log = m.log[len(m.log)-8:]
	}
}

// renderLine draws the prompt with a block cursor and a ^ under the mark.
func (m *sandbox) renderLine() string {
	rs := []rune(m.ed.Text())
	p := m.ed.Point()
	at := " "
	if p < len(rs) {
		at = string(rs[p])
	}
	prompt := okStyle.Render("$ ")
	if s := m.ed.Search(); s.Active {
		label := "reverse-i-search"
		if s.Forward {
			label = "i-search"
		}
		if s.Failed {
			label = "failed " + label
		}
		prompt = keyStyle.Render(fmt.Sprintf("(%s)`%s': ", label, s.Query))
	}
	line := prompt + textStyle.Render(string(rs[:p])) + cursorStyle.Render(at)
	if p < len(rs) {
		line += textStyle.Render(string(rs[p+1:]))
	}
	if m.ed.Search().Active {
		return line + "\n" + dimStyle.Render("  type to search · Ctrl+R older · Ctrl+S newer · Ctrl+G cancel")
	}
	mark := min(m.ed.Mark(), len(rs))
	markLine := strings.Repeat(" ", 2+mark) + dimStyle.Render("^ mark (Ctrl+X Ctrl+X jumps here)")
	return line + "\n" + markLine
}

func (m *sandbox) View() string {
	header := titleStyle.Render("SANDBOX") + "  " + dimStyle.Render("try any shortcut · esc: back")

	term := strings.Join(append(append([]string{}, m.scroll...), m.renderLine()), "\n")
	termBox := panelStyle.Width(min(max(40, m.width-6), 100)).Render(term)

	flash := ""
	if m.flash != "" {
		flash = keyStyle.Render("» " + m.flash)
	}

	// Kill ring, newest first, with the entry Ctrl+Y would paste marked.
	entries, idx := m.ed.KillRing().Entries()
	var kr strings.Builder
	kr.WriteString(headingStyle.Render("Kill ring") + dimStyle.Render("  ▸ = next Ctrl+Y") + "\n")
	if len(entries) == 0 {
		kr.WriteString(dimStyle.Render("empty: cut something with Ctrl+W/K/U or Alt+D"))
	}
	for i := len(entries) - 1; i >= 0; i-- {
		mark := "  "
		style := dimStyle
		if i == idx {
			mark, style = "▸ ", keyStyle
		}
		kr.WriteString(style.Render(fmt.Sprintf("%s%q", mark, entries[i])) + "\n")
	}

	var hist strings.Builder
	hist.WriteString(headingStyle.Render("History") + dimStyle.Render("  Ctrl+P/N, Alt+.") + "\n")
	h := m.ed.History()
	for i := max(0, len(h)-6); i < len(h); i++ {
		style := dimStyle
		if i == m.ed.HistoryPos() {
			style = keyStyle
		}
		hist.WriteString(style.Render(fmt.Sprintf("%3d  %s", i+1, h[i])) + "\n")
	}

	var log strings.Builder
	log.WriteString(headingStyle.Render("Last actions") + "\n")
	for _, l := range m.log {
		log.WriteString(l + "\n")
	}

	side := lipgloss.JoinHorizontal(lipgloss.Top,
		panelStyle.Width(38).Render(strings.TrimRight(kr.String(), "\n")),
		panelStyle.Width(38).Render(strings.TrimRight(hist.String(), "\n")))
	if m.width < 82 {
		side = lipgloss.JoinVertical(lipgloss.Left,
			panelStyle.Width(38).Render(strings.TrimRight(kr.String(), "\n")),
			panelStyle.Width(38).Render(strings.TrimRight(hist.String(), "\n")))
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left,
		header, "", termBox, flash, "", side, panelStyle.Width(min(78, max(40, m.width-6))).Render(strings.TrimRight(log.String(), "\n"))))
}
