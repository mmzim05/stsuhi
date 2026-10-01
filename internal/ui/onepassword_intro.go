package ui

import "strings"

// onePasswordSetupInstructions is the static walkthrough shown to a user who
// enables 1Password mode at first boot. It covers what stsuhi itself needs
// (the `op` CLI plus the desktop app's CLI-integration setting) and the
// parts that are the user's/`op`'s responsibility, not stsuhi's, so the two
// don't get conflated when something doesn't work.
const onePasswordSetupInstructions = `1) Install the 1Password desktop app and CLI ("op").
   Arch/Omarchy: both ship in the AUR/official packages, e.g.
     yay -S 1password 1password-cli

2) Add yourself to the "onepassword" group so the CLI/SSH-agent socket
   under ~/.1password/ is readable by your user, then re-login (or reboot)
   so the new group membership takes effect:
     sudo usermod -aG onepassword $USER

3) In the 1Password desktop app: Settings -> Developer, enable
   "Integrate with 1Password CLI".

4) Verify the CLI can reach your vault without a signin prompt:
     op vault list
   If that fails with "account is not signed in" or hangs, re-check step 3 -
   this is an op/1Password state issue, not something stsuhi controls.

5) For each server, create a Login item with "username" and "password"
   fields - stsuhi will ask for that item's name (and vault, if you use
   more than one) when you add the connection, and looks the credentials
   up at connect time. Nothing is ever written back to 1Password.`

func viewOnePasswordPromptBody() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Enable 1Password Mode?"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("1Password mode skips the master password entirely: connections"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("are stored in plaintext (no password to derive an encryption key"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("from), and each one only stores a 1Password item reference -"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("no local username, password, or SSH key fields. Credentials are"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("fetched from 1Password via the op CLI each time you connect."))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("[Y]"))
	b.WriteString(" Yes, enable 1Password mode (see setup steps)\n")
	b.WriteString(labelStyle.Render("[N]"))
	b.WriteString(" No, use a master password with encrypted storage\n\n")
	b.WriteString(helpStyle.Render("y: yes • n: no • esc: quit"))
	return b.String()
}

func viewOnePasswordInfoBody() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("1Password Mode Setup"))
	b.WriteString("\n\n")
	b.WriteString(mutedTextStyle.Render(onePasswordSetupInstructions))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("enter: enable 1Password mode & continue • esc: back"))
	return b.String()
}
