package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		args = []string{"."}
	}

	// Resolve all paths to absolute
	absPaths := make([]string, 0, len(args))
	for _, arg := range args {
		abs, err := filepath.Abs(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		absPaths = append(absPaths, abs)
	}

	var m tea.Model
	if len(absPaths) == 1 {
		m = newModel(absPaths[0])
	} else {
		m = newMultiRootModel(absPaths)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
