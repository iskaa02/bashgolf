package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termgame/internal/game"
)

const logo = `
 ▀█▀ █▀▀ █▀█ █▀▄▀█ █▀▀ ▄▀█ █▀▄▀█ █▀▀
  █  ██▄ █▀▄ █ ▀ █ █▄█ █▀█ █ ▀ █ ██▄`

type menuItem struct {
	title, desc string
	open        func() Screen
}

var (
	killRingPack = game.MustLoadPack("killring")
	caseFixPack  = game.MustLoadPack("casefix")
)

type menu struct {
	items  []menuItem
	cursor int
	width  int
}

func newMenu(prog *game.Progress) *menu {
	return &menu{items: []menuItem{
		{"Kill Ring Surgeon", "fix broken commands by cutting and pasting", func() Screen { return newLevelSelect(killRingPack, prog, 0) }},
		{"Case Fixer", "Caps Lock accidents and swapped letters", func() Screen { return newLevelSelect(caseFixPack, prog, 0) }},
		{"Cursor Dash", "60 seconds, hit the targets in as few keys as you can", func() Screen { return newDash(prog) }},
		{"Key Check", "see which shortcuts your terminal sends", func() Screen { return newKeyCheck() }},
		{"Sandbox", "a fake shell line to practise every shortcut", func() Screen { return newSandbox() }},
	}}
}

func (m *menu) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "ctrl+p":
			m.cursor = (m.cursor + len(m.items) - 1) % len(m.items)
		case "down", "j", "ctrl+n", "tab":
			m.cursor = (m.cursor + 1) % len(m.items)
		case "enter", " ":
			return m, switchTo(m.items[m.cursor].open())
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *menu) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(logo) + "\n")
	b.WriteString(dimStyle.Render("   learn your shell's keyboard shortcuts") + "\n\n")
	for i, it := range m.items {
		if i == m.cursor {
			b.WriteString(keyStyle.Render(" ▸ "+it.title) + "  " + textStyle.Render(it.desc) + "\n")
		} else {
			b.WriteString(dimStyle.Render("   "+it.title+"  "+it.desc) + "\n")
		}
	}
	b.WriteString("\n" + helpStyle.Render("   ↑/↓ choose · enter start · q quit"))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}
