package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iskaa02/bashgolf/internal/readline"
)

// Neon arcade palette.
var (
	pink   = lipgloss.Color("#ff2bd6")
	cyan   = lipgloss.Color("#00e5ff")
	yellow = lipgloss.Color("#ffe600")
	green  = lipgloss.Color("#39ff14")
	red    = lipgloss.Color("#ff3b3b")
	dim    = lipgloss.Color("#6c6c80")
	text   = lipgloss.Color("#e6e6f0")

	titleStyle   = lipgloss.NewStyle().Foreground(pink).Bold(true)
	headingStyle = lipgloss.NewStyle().Foreground(cyan).Bold(true)
	keyStyle     = lipgloss.NewStyle().Foreground(yellow).Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(dim)
	textStyle    = lipgloss.NewStyle().Foreground(text)
	okStyle      = lipgloss.NewStyle().Foreground(green).Bold(true)
	badStyle     = lipgloss.NewStyle().Foreground(red).Bold(true)
	cursorStyle  = lipgloss.NewStyle().Background(pink).Foreground(lipgloss.Color("#000000"))
	panelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(dim).Padding(0, 1)
	helpStyle    = dimStyle.Italic(true)
)

// toKey converts a Bubble Tea keypress into a readline key. Only plain
// typing (no Alt) carries text to insert.
func toKey(msg tea.KeyMsg) readline.Key {
	k := readline.Key{Name: msg.String()}
	if !msg.Alt {
		switch msg.Type {
		case tea.KeyRunes:
			k.Text = string(msg.Runes)
		case tea.KeySpace:
			k.Text = " "
		}
	}
	return k
}

var keySymbols = map[string]string{
	"left": "←", "right": "→", "up": "↑", "down": "↓",
	"backspace": "⌫", "enter": "⏎", "tab": "Tab", "delete": "Del",
	"home": "Home", "end": "End", "esc": "Esc",
}

// prettyKey turns "ctrl+x ctrl+w" into "Ctrl+X Ctrl+W" for display. Plain
// keys like "q" are left alone so they don't read as Shift+Q.
func prettyKey(name string) string {
	parts := strings.Split(name, " ")
	for i, part := range parts {
		if sym, ok := keySymbols[part]; ok {
			parts[i] = sym
			continue
		}
		if !strings.Contains(part, "+") {
			continue
		}
		mods := strings.Split(part, "+")
		for j, m := range mods {
			if m != "" {
				mods[j] = strings.ToUpper(m[:1]) + m[1:]
			}
		}
		parts[i] = strings.Join(mods, "+")
	}
	return strings.Join(parts, " ")
}
