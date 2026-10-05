// Package ui holds the Bubble Tea screens.
package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/iskaa02/bashgolf/internal/game"
)

// Screen is one full-screen mode. Screens get every key, including Ctrl+C,
// so each one must offer its own way back (usually Esc).
type Screen interface {
	Update(tea.Msg) (Screen, tea.Cmd)
	View() string
}

type (
	switchMsg struct{ to Screen }
	backMsg   struct{}
)

func switchTo(s Screen) tea.Cmd { return func() tea.Msg { return switchMsg{s} } }
func back() tea.Msg             { return backMsg{} }

// App routes messages to the current screen.
type App struct {
	screen        Screen
	prog          *game.Progress
	width, height int
}

// NewApp starts on the named screen ("keycheck", "sandbox", "killring",
// "casefix", "history", "dash", "rush") or the menu.
func NewApp(start string) App {
	a := App{prog: game.NewProgress()}
	switch start {
	case "keycheck":
		a.screen = newKeyCheck()
	case "sandbox":
		a.screen = newSandbox()
	case "killring":
		a.screen = newLevelSelect(killRingPack, a.prog, 0)
	case "casefix":
		a.screen = newLevelSelect(caseFixPack, a.prog, 0)
	case "history":
		a.screen = newLevelSelect(historyPack, a.prog, 0)
	case "dash":
		a.screen = newDash(a.prog)
	case "rush":
		a.screen = newRush(a.prog)
	default:
		a.screen = newMenu(a.prog)
	}
	return a
}

func (a App) Init() tea.Cmd { return nil }

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
	case switchMsg:
		a.screen = msg.to
		return a, func() tea.Msg { return tea.WindowSizeMsg{Width: a.width, Height: a.height} }
	case backMsg:
		a.screen = newMenu(a.prog)
		return a, nil
	}
	var cmd tea.Cmd
	a.screen, cmd = a.screen.Update(msg)
	return a, cmd
}

func (a App) View() string { return a.screen.View() }
