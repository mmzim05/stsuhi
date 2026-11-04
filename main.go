package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mmzim05/stsuhi/internal/config"
	"github.com/mmzim05/stsuhi/internal/keyring"
	"github.com/mmzim05/stsuhi/internal/ssh"
	"github.com/mmzim05/stsuhi/internal/storage"
	"github.com/mmzim05/stsuhi/internal/theme"
	"github.com/mmzim05/stsuhi/internal/ui"
)

// reconnectDelay gives the user a moment to read ssh's last output before
// the TUI takes over the screen again
const reconnectDelay = time.Second

func main() {
	debug := flag.Bool("debug", false,
		"use a temporary storage directory instead of ~/.config/stsuhi, "+
			"deleted on exit (for testing first-boot flows without "+
			"touching real data)")
	flag.Parse()

	if err := run(*debug); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// run holds the program so its deferred cleanup runs before main exits
func run(debug bool) (err error) {
	store, cleanup, err := openStorage(debug)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, cleanup())
	}()

	if err := store.InitStorage(); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}

	cfg, err := config.Load(store.GetConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ui.SetTheme(theme.Resolve(cfg.Theme))

	// The TUI exits to hand the terminal to ssh, then starts again when
	// the session ends, carrying the master password across.
	var masterPassword string
	for {
		var app *ui.App
		if masterPassword != "" {
			app = ui.NewAppWithPassword(store, cfg, masterPassword)
		} else {
			app = ui.NewApp(store, cfg)
		}

		finalModel, err := tea.NewProgram(app, tea.WithAltScreen()).Run()
		if err != nil {
			return fmt.Errorf("run app: %w", err)
		}

		finalApp, ok := finalModel.(*ui.App)
		if !ok {
			return nil
		}
		if password := finalApp.GetMasterPassword(); password != "" {
			masterPassword = password
		}
		conn := finalApp.GetPendingConnection()
		if conn == nil {
			return nil
		}
		// A failed session is shown, then the list comes back.
		if err := ssh.Connect(conn); err != nil {
			fmt.Fprintf(os.Stderr, "SSH connection failed: %v\n", err)
		}
		time.Sleep(reconnectDelay)
	}
}

// openStorage returns the real data store, or a throwaway one in debug
// mode, plus the cleanup to run on exit
func openStorage(debug bool) (*storage.FileStorage, func() error, error) {
	noCleanup := func() error { return nil }
	if !debug {
		store, err := storage.NewFileStorage()
		if err != nil {
			return nil, nil, fmt.Errorf("initialize storage: %w", err)
		}
		return store, noCleanup, nil
	}

	tmpDir, err := os.MkdirTemp("", "stsuhi-debug-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create debug storage directory: %w", err)
	}
	fmt.Fprintf(os.Stderr, "debug mode: storage at %s (deleted on exit)\n",
		tmpDir)
	store := storage.NewFileStorageAt(tmpDir)
	cleanup := func() error {
		var keyErr error
		// The debug key has its own keyring entry; never touch the real one.
		if keyring.IsAvailable() {
			keyErr = keyring.DeletePassword(store.KeyringAccount())
		}
		if err := os.RemoveAll(tmpDir); err != nil {
			return errors.Join(keyErr,
				fmt.Errorf("remove debug storage: %w", err))
		}
		return keyErr
	}
	return store, cleanup, nil
}
