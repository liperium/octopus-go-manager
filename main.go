package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
)

func main() {
	m := initialModel()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--preview":
			fmt.Println(m.View())
			return
		case "--help", "-h":
			fmt.Println("Octopus · fleet TUI prototype\nRun without arguments for the TUI; --preview prints a screen.\nAll actions use mock state and timers. No commands or network calls.")
			return
		default:
			fmt.Fprintln(os.Stderr, "Unknown option. Use --help.")
			os.Exit(2)
		}
	}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
