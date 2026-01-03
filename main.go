package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"stsuhi/internal/config"
	"stsuhi/internal/ssh"
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

	// Load configuration
	cfg, err := config.Load(store.GetConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Main loop - restart TUI after SSH connection ends
	var masterPassword string
	for {
		// Create and run the Bubble Tea app
		var app *ui.App
		if masterPassword != "" {
			// Already authenticated - skip login
			app = ui.NewAppWithPassword(store, cfg, masterPassword)
		} else {
			// First run - need to authenticate
			app = ui.NewApp(store, cfg)
		}

		p := tea.NewProgram(app, tea.WithAltScreen())

		finalModel, err := p.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
			os.Exit(1)
		}

		// Check if there's a pending SSH connection to establish
		if finalApp, ok := finalModel.(*ui.App); ok {
			// Preserve master password for next iteration
			if finalApp.GetMasterPassword() != "" {
				masterPassword = finalApp.GetMasterPassword()
			}

			if conn := finalApp.GetPendingConnection(); conn != nil {
				// Execute the SSH connection
				if err := ssh.Connect(conn); err != nil {
					fmt.Fprintf(os.Stderr, "SSH connection failed: %v\n", err)
					// Don't exit, just loop back to TUI to show error
					continue
				}
				// SSH connection ended normally, loop back to TUI
				continue
			}
		}

		// No pending connection - user quit the app
		break
	}
}
