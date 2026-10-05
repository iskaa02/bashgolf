package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iskaa02/bashgolf/internal/game"
)

type levelSelect struct {
	pack   *game.Pack
	prog   *game.Progress
	cursor int
}

func newLevelSelect(pack *game.Pack, prog *game.Progress, cursor int) *levelSelect {
	return &levelSelect{pack: pack, prog: prog, cursor: cursor}
}

func (m *levelSelect) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		n := len(m.pack.Levels)
		switch msg.String() {
		case "up", "k", "ctrl+p":
			m.cursor = (m.cursor + n - 1) % n
		case "down", "j", "ctrl+n", "tab":
			m.cursor = (m.cursor + 1) % n
		case "enter", " ":
			if m.prog.Unlocked(m.pack, m.cursor) {
				return m, switchTo(newPuzzle(m.pack, m.prog, m.cursor))
			}
		case "esc", "q":
			return m, back
		}
	}
	return m, nil
}

func starString(n int) string {
	return okStyle.Render(strings.Repeat("★", n)) + dimStyle.Render(strings.Repeat("☆", 3-n))
}

func (m *levelSelect) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(strings.ToUpper(m.pack.Title)) + "\n")
	b.WriteString(dimStyle.Render(m.pack.Desc) + "\n\n")
	for i, l := range m.pack.Levels {
		pointer := "   "
		if i == m.cursor {
			pointer = keyStyle.Render(" ▸ ")
		}
		num := fmt.Sprintf("%2d  ", i+1)
		switch best, done := m.prog.Best(m.pack, i); {
		case done:
			b.WriteString(pointer + textStyle.Render(num+fmt.Sprintf("%-20s", l.Title)) + starString(game.Stars(best, l.Par)) +
				dimStyle.Render(fmt.Sprintf("  best %d · par %d", best, l.Par)) + "\n")
		case m.prog.Unlocked(m.pack, i):
			b.WriteString(pointer + textStyle.Render(num+fmt.Sprintf("%-20s", l.Title)) + starString(0) +
				dimStyle.Render(fmt.Sprintf("  par %d", l.Par)) + "\n")
		default:
			b.WriteString(pointer + dimStyle.Render(num+"locked") + "\n")
		}
	}
	b.WriteString("\n" + helpStyle.Render("↑/↓ choose · enter play · esc back"))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}
