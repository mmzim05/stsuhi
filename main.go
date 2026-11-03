package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mmzim05/stsuhi/internal/config"
	"github.com/mmzim05/stsuhi/internal/ssh"
	"github.com/mmzim05/stsuhi/internal/storage"
	"github.com/mmzim05/stsuhi/internal/theme"
	"github.com/mmzim05/stsuhi/internal/ui"
)

func main() {
	debug := flag.Bool("debug", false, "use a temporary storage directory instead of ~/.config/stsuhi, deleted on exit (for testing first-boot flows without touching real data)")
	flag.Parse()

	// Initialize storage
	var store *storage.FileStorage
	var err error
	if *debug {
		tmpDir, mkErr := os.MkdirTemp("", "stsuhi-debug-*")
		if mkErr != nil {
			fmt.Fprintf(os.Stderr, "Error creating debug storage directory: %v\n", mkErr)
			os.Exit(1)
		}
		defer os.RemoveAll(tmpDir)
		fmt.Fprintf(os.Stderr, "debug mode: storage at %s (deleted on exit)\n", tmpDir)
		store = storage.NewFileStorageAt(tmpDir)
	} else {
		store, err = storage.NewFileStorage()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing storage: %v\n", err)
			os.Exit(1)
		}
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
	ui.SetTheme(theme.Resolve(cfg.Theme))

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
					time.Sleep(1 * time.Second)
					continue
				}
				// SSH connection ended normally, add delay before restarting TUI
				time.Sleep(1 * time.Second)
				continue
			}
		}

		// No pending connection - user quit the app
		break
	}
}
