package ui

import "strings"

// onePasswordSetupInstructions is the static walkthrough shown from
// Settings -> "1Password setup instructions". It covers what stsuhi itself
// needs (the `op` CLI plus the desktop app's CLI-integration setting) and
// the parts that are the user's/`op`'s responsibility, not stsuhi's, so the
// two don't get conflated when something doesn't work.
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
   more than one) when you toggle "Fetch username/password from
   1Password" on a connection, and looks the credentials up at connect
   time. Nothing is ever written back to 1Password.

6) Optional: add a custom text field labeled "url", formatted as
   "address:port" (port optional, e.g. "10.0.0.5:2222" or just
   "10.0.0.5"). If present, stsuhi uses it instead of whatever's typed
   locally - leave the connection's Address field blank and nothing about
   that server (host, port, or credentials) is ever stored on this
   machine, only the item/vault reference needed to look it up.`

func viewOnePasswordInfoBody() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("1Password Setup Instructions"))
	b.WriteString("\n\n")
	b.WriteString(mutedTextStyle.Render(onePasswordSetupInstructions))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("enter/esc: back to settings"))
	return b.String()
}
