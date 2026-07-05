// Package gh integrates with the GitHub CLI for optional SSH key upload.
//
// Caveat handled here: `gh` is authenticated as ONE account, but wardrobe
// manages many. Upload is only offered when the gh-authenticated user is
// plausibly the account being set up; otherwise we fall back to clipboard
// plus the settings URL.
package gh

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Available reports whether the gh CLI is installed and authenticated.
func Available() bool {
	if _, err := exec.LookPath("gh"); err != nil {
		return false
	}
	return exec.Command("gh", "auth", "status").Run() == nil
}

// AuthenticatedUser returns the login gh is currently authenticated as.
func AuthenticatedUser() (string, error) {
	out, err := exec.Command("gh", "api", "user", "-q", ".login").Output()
	if err != nil {
		return "", fmt.Errorf("gh api user: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// UploadKey registers a public key with the gh-authenticated account.
func UploadKey(title, publicKey string) error {
	cmd := exec.Command("gh", "api", "--method", "POST", "user/keys",
		"-f", "title="+title,
		"-f", "key="+publicKey)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh key upload failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// Clipboard copies text to the system clipboard, best effort.
func Clipboard(text string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			return false
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return false
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run() == nil
}

// SettingsURL returns the "add SSH key" page for a provider host.
func SettingsURL(host string) string {
	switch host {
	case "github.com":
		return "https://github.com/settings/ssh/new"
	case "gitlab.com":
		return "https://gitlab.com/-/user_settings/ssh_keys"
	case "bitbucket.org":
		return "https://bitbucket.org/account/settings/ssh-keys/"
	default:
		return "https://" + host
	}
}
