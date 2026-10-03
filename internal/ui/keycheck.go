package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// keyCheckItem is one shortcut to detect. seq is one key name, or two for a
// chord like Ctrl+X Ctrl+X.
type keyCheckItem struct {
	label string
	seq   []string
	desc  string
	hint  string // shown when the key is not detected yet
}

type keyCheckSection struct {
	title string
	items []keyCheckItem
}

func k(label, desc string, seq ...string) keyCheckItem {
	return keyCheckItem{label: label, seq: seq, desc: desc}
}

func (i keyCheckItem) withHint(h string) keyCheckItem { i.hint = h; return i }

var keyCheckSections = []keyCheckSection{
	{"Navigation", []keyCheckItem{
		k("Ctrl+A", "start of line", "ctrl+a"),
		k("Ctrl+E", "end of line", "ctrl+e"),
		k("Ctrl+F", "forward char", "ctrl+f"),
		k("Ctrl+B", "back char", "ctrl+b"),
		k("Alt+F", "forward word", "alt+f"),
		k("Alt+B", "back word", "alt+b"),
		k("Ctrl+X Ctrl+X", "toggle start/cursor", "ctrl+x", "ctrl+x"),
	}},
	{"Cut & Paste", []keyCheckItem{
		k("Ctrl+U", "cut to start", "ctrl+u"),
		k("Ctrl+K", "cut to end", "ctrl+k"),
		k("Ctrl+W", "cut word before", "ctrl+w"),
		k("Alt+D", "cut word after", "alt+d"),
		k("Ctrl+D", "delete char", "ctrl+d"),
		k("Ctrl+H", "backspace", "ctrl+h").withHint("press Ctrl+H itself; Backspace sends a different code"),
		k("Ctrl+Y", "paste", "ctrl+y"),
		k("Alt+Y", "rotate paste", "alt+y"),
		k("Ctrl+_", "undo", "ctrl+_").withHint("try Ctrl+Shift+- or Ctrl+/"),
		k("Alt+R", "revert line", "alt+r"),
	}},
	{"Case & Swap", []keyCheckItem{
		k("Alt+U", "UPPERCASE word", "alt+u"),
		k("Alt+L", "lowercase word", "alt+l"),
		k("Alt+C", "Capitalize word", "alt+c"),
		k("Ctrl+T", "swap chars", "ctrl+t"),
		k("Alt+T", "swap words", "alt+t"),
	}},
	{"History", []keyCheckItem{
		k("Ctrl+R", "search back", "ctrl+r"),
		k("Ctrl+S", "search forward", "ctrl+s"),
		k("Ctrl+G", "cancel search", "ctrl+g"),
		k("Ctrl+P", "previous command", "ctrl+p"),
		k("Ctrl+N", "next command", "ctrl+n"),
		k("Alt+.", "last argument", "alt+."),
		k("Alt+_", "last argument", "alt+_").withHint("Alt+Shift+-"),
	}},
	{"Process & Screen", []keyCheckItem{
		k("Ctrl+L", "clear screen", "ctrl+l"),
		k("Ctrl+C", "interrupt", "ctrl+c"),
		k("Ctrl+Z", "suspend", "ctrl+z"),
		k("Ctrl+Q", "resume output", "ctrl+q"),
		k("Ctrl+X Ctrl+E", "edit in $EDITOR", "ctrl+x", "ctrl+e"),
	}},
}

type keyCheck struct {
	seen    map[string]bool // by label
	prev    string
	last    string
	lastRaw string
	matched string
	width   int
}

func newKeyCheck() *keyCheck {
	return &keyCheck{seen: map[string]bool{}}
}

func (m *keyCheck) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		name := msg.String()
		if name == "esc" {
			return m, back
		}
		m.last = name
		m.lastRaw = fmt.Sprintf("type=%d runes=%q alt=%v", msg.Type, msg.Runes, msg.Alt)
		m.matched = ""
		for _, sec := range keyCheckSections {
			for _, it := range sec.items {
				hit := len(it.seq) == 1 && it.seq[0] == name ||
					len(it.seq) == 2 && it.seq[0] == m.prev && it.seq[1] == name
				if hit {
					m.seen[it.label] = true
					m.matched = it.label
				}
			}
		}
		// A completed chord shouldn't also start the next one.
		if m.matched != "" && strings.Contains(m.matched, " ") {
			m.prev = ""
		} else {
			m.prev = name
		}
	}
	return m, nil
}

func (m *keyCheck) View() string {
	total := 0
	for _, sec := range keyCheckSections {
		total += len(sec.items)
	}

	var panels []string
	for _, sec := range keyCheckSections {
		var b strings.Builder
		b.WriteString(headingStyle.Render(sec.title) + "\n")
		for _, it := range sec.items {
			label := fmt.Sprintf("%-14s", it.label)
			if m.seen[it.label] {
				b.WriteString(okStyle.Render("✓ "+label) + textStyle.Render(it.desc) + "\n")
			} else {
				line := dimStyle.Render("· " + label + it.desc)
				if it.hint != "" {
					line += "\n" + helpStyle.Render("    "+it.hint)
				}
				b.WriteString(line + "\n")
			}
		}
		panels = append(panels, panelStyle.Width(44).Render(strings.TrimRight(b.String(), "\n")))
	}

	cols := max(1, min(3, m.width/48))
	var rows []string
	for i := 0; i < len(panels); i += cols {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, panels[i:min(i+cols, len(panels))]...))
	}

	last := dimStyle.Render("press any shortcut…")
	if m.last != "" {
		last = "last key: " + keyStyle.Render(prettyKey(m.last))
		if m.matched != "" {
			last += okStyle.Render("  ✓ " + m.matched)
		} else if m.prev == "ctrl+x" {
			last += dimStyle.Render("  (waiting for second key)")
		}
		last += "\n" + dimStyle.Render(m.lastRaw)
	}

	header := titleStyle.Render("KEY CHECK") + "  " +
		textStyle.Render(fmt.Sprintf("%d/%d detected", len(m.seen), total))
	help := helpStyle.Render("Press each shortcut. Anything still grey isn't reaching the game: check your terminal's keybinding settings. esc: back")

	return lipgloss.NewStyle().Padding(1, 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, "", last, "", strings.Join(rows, "\n"), "", help))
}
