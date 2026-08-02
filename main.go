package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	topN := flag.Int("n", 1000, "max items in LARGEST tab")
	readOnly := flag.Bool("read-only", false, "read-only mode: disable deletion")
	skipConfirm := flag.Bool("y", false, "skip delete confirmation")
	noMouse := flag.Bool("no-mouse", false, "disable mouse support")
	flag.Parse()
	args := flag.Args()

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
		m = newModel(absPaths[0], *topN, *readOnly, *skipConfirm)
	} else {
		m = newMultiRootModel(absPaths, *topN, *readOnly, *skipConfirm)
	}

	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if !*noMouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(m, opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
