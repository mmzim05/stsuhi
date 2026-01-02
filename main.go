package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"stsuhi/internal/storage"
	"stsuhi/internal/ui"
)

func main() {
	// Initialize storage
	store, err := storage.NewFileStorage()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing storage: %v\n", err)
		os.Exit(1)
	}

	// Create storage directory if it doesn't exist
	if err := store.InitStorage(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating storage directory: %v\n", err)
		os.Exit(1)
	}

	// Create and run the Bubble Tea app
	app := ui.NewApp(store)
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		os.Exit(1)
	}
}
