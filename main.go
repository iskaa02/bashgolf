// Command termgame is an arcade game for learning shell keyboard shortcuts.
//
//	termgame            open the menu
//	termgame keycheck   jump straight to the key check screen
//	termgame sandbox    jump straight to the sandbox
//	termgame killring   jump straight to Kill Ring Surgeon
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"termgame/internal/ui"
)

func main() {
	start := ""
	if len(os.Args) > 1 {
		start = os.Args[1]
	}
	p := tea.NewProgram(ui.NewApp(start), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "termgame:", err)
		os.Exit(1)
	}
}
