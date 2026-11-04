# stsuhi

*pronounced "sti sooee"*

A terminal UI for managing and launching SSH connections, written in Go with
[Bubble Tea](https://github.com/charmbracelet/bubbletea).

Save your servers once, pick one from a list, and stsuhi opens a normal
`ssh` session. When the session ends, you are back in the list.

## Features

- **One list for all your servers.** Add, edit, and delete connections from
  the TUI.
- **Any auth method, per connection:** password, private key, SSH agent
  (including a custom agent socket such as 1Password's), or a 1Password
  item.
- **Password auto-fill.** stsuhi types a stored password at the SSH login
  prompt. At a `[sudo] password for <user>:` prompt it asks first, and
  inserts the same password only when you press `y`.
- **Encrypted at rest.** Saved passwords, key paths, and the host details
  that go with them are encrypted with AES-256-GCM.
- **1Password integration.** Pull username, password, and host from a
  1Password item, so nothing secret is stored on disk.
- **No master password to remember** on systems with a keyring: the
  encryption key is created for you and kept in the OS keyring.
- **Themes**, including automatic matching of your
  [Omarchy](https://omarchy.org) theme.
- Uses your system `ssh`, so `~/.ssh/config`, `known_hosts`, and agent
  forwarding all work as usual.

## Requirements

- Linux (amd64 or arm64)
- OpenSSH client (`ssh`)
- Optional: [1Password CLI](https://developer.1password.com/docs/cli/get-started/)
  (`op`) for the 1Password features

## Install

### Prebuilt binary

```bash
mkdir -p ~/.local/bin
curl -fsSL -o ~/.local/bin/stsuhi \
  https://github.com/mmzim05/stsuhi/releases/latest/download/stsuhi-linux-amd64
chmod +x ~/.local/bin/stsuhi
```

On ARM, use `stsuhi-linux-arm64`. Make sure `~/.local/bin` is in your
`PATH`.

### With Go

```bash
go install github.com/mmzim05/stsuhi@latest
```

### From source

```bash
git clone https://github.com/mmzim05/stsuhi.git
cd stsuhi
./install.sh
```

`install.sh` builds with Go if it is installed and otherwise downloads the
latest release. It installs to `~/.local/bin/stsuhi` and, on desktop
systems, adds an app launcher entry. Run `./uninstall.sh` to remove both.

## Usage

```bash
stsuhi
```

| Key           | Action                        |
| ------------- | ----------------------------- |
| `↑` `↓`       | Move through the list         |
| `enter`       | Connect                       |
| `a`           | Add a connection              |
| `e`           | Edit the selected connection  |
| `d`           | Delete the selected connection|
| `s`           | Settings                      |
| `q`           | Quit                          |

With a system keyring (GNOME Keyring, KWallet, macOS Keychain), stsuhi
never asks for a master password. Without one, it asks after you save a
server with a password, and only then. Connections that use only an SSH
agent or 1Password never need one. On such a system, **Settings → Unlock
with 1Password** can read the master password from a 1Password item.

Lost the master password, or wiped the keyring? Press `ctrl+r` at the
password prompt to reset. This deletes the connections that had a saved
password or key and keeps the rest.

Run `stsuhi -debug` to try it with a throwaway data directory that is
deleted on exit.

## 1Password

1. Install the 1Password app and the `op` CLI.
2. In the 1Password app, open **Settings → Developer** and enable
   **Integrate with 1Password CLI**.
3. Check that `op vault list` works.
4. In stsuhi, mark a connection as 1Password-backed and give the item name
   (and, optionally, the vault).

stsuhi reads the item's `username` and `password` fields. If the item has a
`url`, `endpoint`, or `website` field in the form `host:port`, that host
and port are used too. **Settings → 1Password setup instructions** in the
app has the full guide.

## Security

- Data lives in `~/.config/stsuhi/` with `0700` directories and `0600`
  files.
- With a keyring, the encryption key is a random value stored in the OS
  keyring. Anyone who can read your keyring can read your saved
  passwords, so keep the keyring locked with your login password.
- Without a keyring, the master password is never stored. Only a bcrypt
  hash is kept, to check it.
- Each saved secret is encrypted with AES-256-GCM, using a key derived from
  the master key with PBKDF2-SHA256 and a fresh random salt.
- Connections that use only an SSH agent or 1Password store no secrets.

stsuhi types the login password when the screen shows a password prompt,
and offers it at a sudo prompt only after you confirm. Only save passwords
for servers you trust.

Found a security problem? Please report it privately through
[GitHub security advisories](https://github.com/mmzim05/stsuhi/security/advisories/new)
instead of a public issue.

## License

[GPL-3.0](LICENSE). You may use, change, and share stsuhi. Any modified
version you distribute must also be released under GPL-3.0, with source.
