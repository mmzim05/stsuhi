# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

stsuhi ("sti sooee") — a TUI SSH connection manager written in Go, using Bubble Tea (`charmbracelet/bubbletea`). Stores SSH connection profiles (host, user, password/key) encrypted at rest, and launches `ssh` as a subprocess when a connection is selected.

## Commands

```bash
go build -o stsuhi .        # build binary (gitignored; releases are built by .github/workflows/release.yml on v* tags)
go run .                    # run without building
go vet ./...                # static checks
gofmt -l .                  # list unformatted files
```

There are no test files in this repo (`*_test.go` does not exist yet) and no lint/CI config — `go vet` and `gofmt` are the only checks available.

Install/uninstall to `~/.local/bin` + desktop entry: `./install.sh` (builds with Go if present, else downloads the latest release), `./uninstall.sh`. Module path is `github.com/mmzim05/stsuhi`.

## Architecture

**Process flow (`main.go`):** the program loops rather than exiting after one SSH session. Each iteration builds a fresh Bubble Tea `ui.App`, runs it in the alt screen, and inspects the returned model when the TUI exits:
- If the app has a `pendingConnection`, `main` calls `ssh.Connect` (blocking, takes over the terminal), then loops back and re-launches the TUI — passing the previously-entered master password via `NewAppWithPassword` so the user isn't asked to re-authenticate.
- If there's no pending connection, the user quit intentionally and the loop breaks.

This TUI-then-shell-out-then-relaunch-TUI cycle is the central control flow; changes to auth state or connection state must survive across it (hence `App.GetMasterPassword()` / `GetPendingConnection()`).

**`internal/ui`** — single Bubble Tea `App` model (`app.go`) implemented as a state machine over `AppState` (`StateFirstBoot`, `StateLogin`, `StateAutoLoginPrompt`, `StateAutoLoginSetup`, `StateSettings`, `StateList`, `StateAdd`/`StateEdit`, `StateDelete`, `StateConnecting`, `StateQuitting`). `Update`/`View` dispatch on state to per-state `updateX`/`viewX` methods in the same file. Sub-models (`ConnectionList`, `ConnectionForm`, `PasswordInput`, `AutoLoginSetup`, `Settings`) live in their own files under `internal/ui` and follow the same Bubble Tea `Init`/`Update`/`View` pattern, composed into `App`.

Note: `StateAutoLoginPrompt` (the Y/N/D "want auto-login?" screen) is defined and has `update`/`view` methods, but nothing currently transitions `a.state` into it — it's dead code. The only live path into `StateAutoLoginSetup` today is `StateList` → `s` (Settings) → "Enable auto-login" / "Reconfigure auto-login".

`App.Init()` calls `tryCachedMasterPassword`, which does nothing unless `AutoLogin.Enabled`. It first runs `autologin.ShouldClearAutoLogin`; if a rule fired it drops the keyring copy (auto-login stays enabled, so the next manual login re-caches) and returns. Otherwise it reads the password from `EffectiveBackend()` (keyring vs. 1Password), verifies it against `master.key`, and records login metadata. `afterAuthSuccess` writes the master password to the keyring only when auto-login is enabled with the keyring backend; never cache it otherwise. `AutoLoginSetup` (`autologin_setup.go`) has a row-based layout: row 0 is always the backend toggle (space cycles keyring/1Password); row 1 (1Password item name, prefilled `"stsuhi"`) is only navigable when that backend is selected — `navigateUp`/`navigateDown` skip it otherwise. Its `Validate()` does a *live* check for the 1Password path: it actually calls `onepassword.GetPassword` and compares the result against the current session's master password before allowing save, so a wrong item name is caught at setup time instead of failing silently on next launch.

**`internal/models`** — `Connection` struct (server name, user, address, port, password and/or key path, timestamps) with its own `Validate()`. This is the plaintext, in-memory shape; encryption happens at the storage layer, not here.

**`internal/storage`** — `Storage` is an interface (`storage.go`); `FileStorage` (`file_storage.go`) is the only implementation, persisting to `~/.config/stsuhi/`. Each connection is its own `<uuid>.json` under `connections/`; the file name is the ID (`readOnDisk` overrides any `id` inside the file, so an edited file can't redirect writes). Non-secret fields are plaintext; when a connection has a local password/key path/sudo password, those plus server name/user/address/port are sealed together into `encrypted_secret` (`sealSecret`). `load` re-seals a legacy (100k-iteration) secret with the current KDF cost the first time it decrypts; the master password itself is never stored — only a bcrypt hash in `master.key`, used to gate access and (via PBKDF2 from the plaintext password) derive the per-file encryption key.

**`internal/crypto`** — two independent primitives: PBKDF2-derived AES-256-GCM for encrypting connection data (`Encrypt`/`Decrypt`, salt+nonce+ciphertext base64-packed behind a `v2:` prefix meaning `PBKDF2Iter` = 600k; unprefixed values are legacy `LegacyPBKDF2Iter` = 100k and still decrypt, `IsLegacy` reports them), and bcrypt for the master password check (`HashPassword`/`VerifyPassword`). Don't reuse one for the other's purpose.

**`internal/keyring`** — thin wrapper over `zalando/go-keyring` (OS keychain/Secret Service/wincred) storing the plaintext master password under `service=stsuhi, user=master-password`, used only to enable auto-login. Failure to access the keyring is expected/handled, not fatal — `IsAvailable()` gates whether the app even offers auto-login.

**`internal/onepassword`** — alternative auto-login backend alongside the keyring, gated behind `config.AutoLogin.Backend`. Shells out to the `op` CLI rather than a Go SDK (same shell-out pattern as `internal/ssh`): `IsAvailable()` is just `exec.LookPath("op")`; `GetPassword(itemName)` runs `op item get <itemName> --fields password --reveal` to fetch the master password from a 1Password item by name (any vault — no vault/reference string needed), looking for a field labeled `password`. Unlike the keyring, stsuhi never *writes* anything to 1Password — the user creates the item themselves (default expected name: `DefaultItemName = "stsuhi"`). The `op` call is wrapped in a 15s `context.WithTimeout` with `cmd.Stdin = nil`, so a locked/unconfigured `op` (which would otherwise attempt an interactive `op signin`) can't hang the TUI — it just fails and the caller falls through to manual login, same as a keyring miss. Getting `op` itself into a state where this succeeds non-interactively requires the 1Password desktop app running with **Settings → Developer → "Integrate with 1Password CLI"** enabled; `op whoami`/`op item get ...` failing with "account is not signed in" or a dismissed authorization prompt means that integration isn't active yet — that's an environment/`op` problem, not a stsuhi one.

**`internal/autologin`** — policy layer on top of whichever backend is active: decides whether a stored master password should still be trusted, based on `config.SecurityRules` (clear on reboot/lock/idle) compared against boot ID, session ID, and last-login timestamp persisted in `config.json`. Boot/session ID detection is platform-specific (`runtime.GOOS` switch); Linux reads `/proc/sys/kernel/random/boot_id`, macOS/Windows shell out to `sysctl`/`systeminfo`. `ClearAutoLogin` only calls `keyring.DeletePassword()` when the keyring backend was actually in use (1Password never had anything written to delete).

**`internal/config`** — plain JSON config at `~/.config/stsuhi/config.json` (path supplied by `storage.GetConfigPath()`), currently only holding `AutoLoginConfig`. Missing or corrupt config file is non-fatal — `Load` falls back to `GetDefault()`. `AutoLoginConfig.Backend` selects `"keyring"` or `"1password"` (`config.BackendKeyring`/`config.BackendOnePassword`); `AutoLoginConfig.EffectiveBackend()` treats an empty/legacy value as keyring for backward compat with configs written before this field existed — always call this instead of reading `.Backend` directly.

**`internal/ssh`** — `Connect` shells out to the system `ssh` binary rather than using an SSH library. Two paths: no password (key-based auth) runs `ssh` directly attached to the current stdio; with a password, `connectWithPTY` allocates a PTY (`creack/pty`), forwards terminal size (SIGWINCH) and raw-mode stdin, and feeds PTY output into `promptWatcher` (`prompt_watcher.go`), which keeps a virtual screen (`hinshun/vt10x`) and acts only when, after output has been quiet for `promptQuiet`, the text right before the cursor ends in `password:` (SSH login: auto-typed, once) or `[sudo] password for <user>:` (each new prompt: *offered*, not typed). Sudo offers go through `terminalIO` (`sudo_confirm.go`), which owns local stdout writes and stdin forwarding: it shows an inline `[y/N]` hint, sends the sudo password (`SudoPassword`, falling back to `Password`) only on `y`, passes any other key through, and withdraws the offer when the watcher sees the prompt disappear, so a later `y` in a shell can never leak the password. Cursor-position matching, not raw-output matching, is what keeps full-screen remote apps working: a multiplexer like herdr redrawing a tab that still shows an old sudo prompt used to re-send the password on every tab switch. If you touch it, keep the quiet-period check (a mid-redraw cursor can pass over old prompt text) and the per-prompt `answered` dedupe.

## Security-relevant invariants

- All files written under `~/.config/stsuhi/` use `0600` (files) / `0700` (dirs) permissions — preserve this on any new persisted file.
- The master password is never written to disk in plaintext; only its bcrypt hash (`master.key`) and, optionally, an OS-keyring copy (for auto-login) exist outside process memory.
- Connection encryption key is derived per-file from a fresh random salt (PBKDF2), not cached or reused across files.
