// Command bashgolf is an arcade game for learning shell keyboard shortcuts.
//
//	bashgolf            open the menu
//	bashgolf keycheck   jump straight to the key check screen
//	bashgolf sandbox    jump straight to the sandbox
//	bashgolf killring   jump straight to Kill Ring Surgeon
//	bashgolf casefix    jump straight to Case Fixer
//	bashgolf history    jump straight to History Detective
//	bashgolf dash       jump straight to Cursor Dash
//	bashgolf rush       jump straight to Line Rush
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/iskaa02/bashgolf/internal/ui"
)

func main() {
	start := ""
	if len(os.Args) > 1 {
		start = os.Args[1]
	}
	p := tea.NewProgram(ui.NewApp(start), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "bashgolf:", err)
		os.Exit(1)
	}
}
