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
  inserts the saved sudo password only when you press `y`.
- **Encrypted at rest.** Saved passwords, key paths, and the host details
  that go with them are encrypted with AES-256-GCM under a master password.
- **1Password integration.** Pull username, password, and host from a
  1Password item, so nothing secret is stored on disk.
- **Optional auto-login** through the OS keyring or 1Password, with rules to
  forget the master password on reboot, lock, or idle.
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

The master password is only asked for when a connection needs it.
Connections that use only an SSH agent or 1Password never need one.

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
- The master password is never stored. Only a bcrypt hash is kept, to check
  it.
- Each saved secret is encrypted with AES-256-GCM, using a key derived from
  the master password with PBKDF2-SHA256 and a fresh random salt.
- Connections that use only an SSH agent or 1Password store no secrets.
- Auto-login is off by default. If you turn it on, the master password is
  kept in your OS keyring (or read from 1Password) and forgotten again
  under the rules you pick: on reboot, on logout/lock, or after a set
  idle time.

stsuhi types the login password when the screen shows a password prompt,
and offers the sudo password only after you confirm. Only save passwords
for servers you trust.

Found a security problem? Please report it privately through
[GitHub security advisories](https://github.com/mmzim05/stsuhi/security/advisories/new)
instead of a public issue.

## License

[GPL-3.0](LICENSE). You may use, change, and share stsuhi. Any modified
version you distribute must also be released under GPL-3.0, with source.
