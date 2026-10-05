// Command termgame is an arcade game for learning shell keyboard shortcuts.
//
//	termgame            open the menu
//	termgame keycheck   jump straight to the key check screen
//	termgame sandbox    jump straight to the sandbox
//	termgame killring   jump straight to Kill Ring Surgeon
//	termgame casefix    jump straight to Case Fixer
//	termgame history    jump straight to History Detective
//	termgame dash       jump straight to Cursor Dash
//	termgame rush       jump straight to Line Rush
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/iskaa02/termgame/internal/ui"
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
