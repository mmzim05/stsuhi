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

There are no test files in this repo (`*_test.go` does not exist yet) and no lint config. Code follows gofmt/Effective Go with no discarded errors; it is kept clean under `errcheck -blank ./...`, `staticcheck ./...` and `revive ./...` (not vendored — `go install` them to check).

Install/uninstall to `~/.local/bin` + desktop entry: `./install.sh` (builds with Go if present, else downloads the latest release), `./uninstall.sh`. Module path is `github.com/mmzim05/stsuhi`.

## Architecture

**Process flow (`main.go`):** `main` only parses flags and calls `run`, so deferred cleanup (the `-debug` temp dir) runs on every exit path. `run` loops rather than exiting after one SSH session. Each iteration builds a fresh Bubble Tea `ui.App`, runs it in the alt screen, and inspects the returned model when the TUI exits:
- If the app has a `pendingConnection`, `main` calls `ssh.Connect` (blocking, takes over the terminal), then loops back and re-launches the TUI — passing the previously-entered master password via `NewAppWithPassword` so the user isn't asked to re-authenticate.
- If there's no pending connection, the user quit intentionally and the loop breaks.

This TUI-then-shell-out-then-relaunch-TUI cycle is the central control flow; changes to auth state or connection state must survive across it (hence `App.GetMasterPassword()` / `GetPendingConnection()`).

**`internal/ui`** — single Bubble Tea `App` model (`app.go`) implemented as a state machine over `AppState` (`StateFirstBoot`, `StateLogin`, `StateResetConfirm`, `StateOnePasswordUnlock`, `StateSettings`, `StateList`, `StateAdd`/`StateEdit`, `StateDelete`, `StateConnecting`, `StateQuitting`). `Update`/`View` dispatch on state to per-state `updateX`/`viewX` methods in the same file. Sub-models (`ConnectionList`, `ConnectionForm`, `PasswordInput`, `OnePasswordUnlock`, `Settings`) live in their own files under `internal/ui` and follow the same Bubble Tea `Init`/`Update`/`View` pattern, composed into `App`.

`App.View` renders the state's screen via `screen()`, pads it to one width, and centers it with `lipgloss.Place` in the real terminal size (`WindowSizeMsg` always resizes the list, capped at `maxListWidth`). Form inputs need a nonzero `Width`: bubbles' textinput shows only the placeholder's first rune at width 0.

**Master key (`unlock` in `app.go`).** Called from `Init` and before anything needs a secret. On a machine with a working keyring (`keyring.IsAvailable()`), a data dir with no `master.key` gets a random key from `crypto.NewRandomKey` (`createKeyringKey`: keyring first, then `master.key`), so the user never sees a password prompt. Otherwise it tries the keyring entry, then a 1Password item if `config.Unlock.UsesOnePassword()`. A password typed at the prompt (no keyring, or a pre-keyring data dir) or read from 1Password is saved to the keyring by `rememberInKeyring`, so it's asked for at most once per machine. The prompt itself only appears via `beginMasterPasswordPrompt`, after the form is submitted. `ctrl+r` at the login prompt goes to `StateResetConfirm`, which calls `storage.ResetMasterPassword` (deletes sealed connections and `master.key`) — the way out when the keyring lost a generated key nobody can type. `OnePasswordUnlock` (`onepassword_unlock.go`) is reachable from Settings only when there's no keyring; its `Validate()` does a *live* `onepassword.GetPassword` and compares against the session's master password before saving.

**`internal/models`** — `Connection` struct (server name, user, address, port, password and/or key path, timestamps) with its own `Validate()`. This is the plaintext, in-memory shape; encryption happens at the storage layer, not here.

**`internal/storage`** — `Storage` is an interface (`storage.go`); `FileStorage` (`file_storage.go`) is the only implementation, persisting to `~/.config/stsuhi/`. Each connection is its own `<uuid>.json` under `connections/`; the file name is the ID (`readOnDisk` overrides any `id` inside the file, so an edited file can't redirect writes). Non-secret fields are plaintext; when a connection has a local password/key path, those plus server name/user/address/port are sealed together into `encrypted_secret` (`sealSecret`). `load` re-seals a legacy (100k-iteration) secret with the current KDF cost the first time it decrypts; the master password itself is never stored — only a bcrypt hash in `master.key`, used to gate access and (via PBKDF2 from the plaintext password) derive the per-file encryption key.

**`internal/crypto`** — two independent primitives: PBKDF2-derived AES-256-GCM for encrypting connection data (`Encrypt`/`Decrypt`, salt+nonce+ciphertext base64-packed behind a `v2:` prefix meaning `PBKDF2Iter` = 600k; unprefixed values are legacy `LegacyPBKDF2Iter` = 100k and still decrypt, `IsLegacy` reports them), and bcrypt for the master password check (`HashPassword`/`VerifyPassword`). Don't reuse one for the other's purpose.

**`internal/keyring`** — thin wrapper over `zalando/go-keyring` (OS keychain/Secret Service/wincred) holding the master key under `service=stsuhi`, account `storage.KeyringAccount()`: `master-password` for `~/.config/stsuhi` (the name older auto-login versions used, so a cached password carries over) and `master-password:<dir>` for any other data dir, so `-debug` never touches the real key (`main` deletes the debug entry on exit). A locked or missing keyring makes `IsAvailable()` false and the app falls back to the master password prompt.

**`internal/onepassword`** — per-connection credentials (`GetCredential`) and, on machines without a keyring, the master password source (`GetPassword`, gated by `config.Unlock.UsesOnePassword()`). Shells out to the `op` CLI rather than a Go SDK (same shell-out pattern as `internal/ssh`): `IsAvailable()` is just `exec.LookPath("op")`; `GetPassword(itemName)` runs `op item get <itemName> --fields password --reveal` to fetch the master password from a 1Password item by name (any vault — no vault/reference string needed), looking for a field labeled `password`. Unlike the keyring, stsuhi never *writes* anything to 1Password — the user creates the item themselves (default expected name: `DefaultItemName = "stsuhi"`). The `op` call is wrapped in a 15s `context.WithTimeout` with `cmd.Stdin = nil`, so a locked/unconfigured `op` (which would otherwise attempt an interactive `op signin`) can't hang the TUI — it just fails and the caller falls through to manual login, same as a keyring miss. Getting `op` itself into a state where this succeeds non-interactively requires the 1Password desktop app running with **Settings → Developer → "Integrate with 1Password CLI"** enabled; `op whoami`/`op item get ...` failing with "account is not signed in" or a dismissed authorization prompt means that integration isn't active yet — that's an environment/`op` problem, not a stsuhi one.

**`internal/config`** — plain JSON config at `~/.config/stsuhi/config.json` (path supplied by `storage.GetConfigPath()`), holding the theme and `UnlockConfig` (JSON key `auto_login`, kept from older versions so an existing 1Password setup survives; old rule/boot-ID fields are ignored and dropped on the next save). Missing or corrupt config file is non-fatal — `Load` falls back to `GetDefault()`. Use `UnlockConfig.UsesOnePassword()` rather than reading `Enabled`/`Backend` directly.

**`internal/ssh`** — `Connect` shells out to the system `ssh` binary rather than using an SSH library. Two paths: no password (key-based auth) runs `ssh` directly attached to the current stdio; with a password, `connectWithPTY` allocates a PTY (`creack/pty`), forwards terminal size (SIGWINCH) and raw-mode stdin, and feeds PTY output into `promptWatcher` (`prompt_watcher.go`), which keeps a virtual screen (`hinshun/vt10x`) and acts only when, after output has been quiet for `promptQuiet`, the text right before the cursor ends in `password:` (SSH login: auto-typed, once) or `[sudo] password for <user>:` (each new prompt: *offered*, not typed). Sudo offers go through `terminalIO` (`sudo_confirm.go`), which owns local stdout writes and stdin forwarding: it shows an inline `[y/N]` hint, sends the connection's one `Password` only on `y`, passes any other key through, and withdraws the offer when the watcher sees the prompt disappear, so a later `y` in a shell can never leak the password. Cursor-position matching, not raw-output matching, is what keeps full-screen remote apps working: a multiplexer like herdr redrawing a tab that still shows an old sudo prompt used to re-send the password on every tab switch. If you touch it, keep the quiet-period check (a mid-redraw cursor can pass over old prompt text) and the per-prompt `answered` dedupe.

## Security-relevant invariants

- All files written under `~/.config/stsuhi/` use `0600` (files) / `0700` (dirs) permissions — preserve this on any new persisted file.
- The master password is never written to disk in plaintext; only its bcrypt hash (`master.key`) and the OS-keyring copy exist outside process memory. With a keyring the master key is random and generated; anyone who can read the keyring can decrypt the connections.
- Connection encryption key is derived per-file from a fresh random salt (PBKDF2), not cached or reused across files.
