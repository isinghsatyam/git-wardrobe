// Package sshcfg generates the managed SSH configuration and handles
// key generation, agent loading and connectivity verification.
//
// Strategy: all wardrobe host aliases live in ~/.ssh/wardrobe.config,
// regenerated wholesale from config.toml on every change. The user's
// own ~/.ssh/config is touched exactly once, to prepend a single
// `Include wardrobe.config` line.
package sshcfg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/isinghsatyam/git-wardrobe/internal/config"
)

const includeMarker = "Include wardrobe.config"

// SSHDir returns ~/.ssh.
func SSHDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh")
}

// ManagedPath returns ~/.ssh/wardrobe.config.
func ManagedPath() string { return filepath.Join(SSHDir(), "wardrobe.config") }

// UserConfigPath returns ~/.ssh/config.
func UserConfigPath() string { return filepath.Join(SSHDir(), "config") }

// Render produces the full managed file contents for the given accounts.
func Render(accounts []config.Account) string {
	var b strings.Builder
	b.WriteString("# Managed by git-wardrobe — do not edit; run `git wardrobe` commands instead.\n")
	b.WriteString("# Regenerated from " + config.ContractHome(config.Path()) + "\n\n")
	for _, a := range accounts {
		fmt.Fprintf(&b, "Host %s\n", a.Alias())
		fmt.Fprintf(&b, "    HostName %s\n", a.Host)
		b.WriteString("    User git\n")
		fmt.Fprintf(&b, "    IdentityFile %s\n", config.ContractHome(a.KeyPath()))
		b.WriteString("    IdentitiesOnly yes\n")
		b.WriteString("    AddKeysToAgent yes\n")
		if runtime.GOOS == "darwin" {
			b.WriteString("    UseKeychain yes\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// WriteManaged regenerates ~/.ssh/wardrobe.config and ensures the user's
// ~/.ssh/config includes it. Creates ~/.ssh and ~/.ssh/config if missing.
func WriteManaged(accounts []config.Account) error {
	if err := os.MkdirAll(SSHDir(), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(ManagedPath(), []byte(Render(accounts)), 0o600); err != nil {
		return err
	}
	return ensureInclude()
}

// ensureInclude prepends the Include line to ~/.ssh/config if absent.
// Prepending matters: OpenSSH applies the first matching value, and an
// Include placed after a `Host *` block would inherit its options.
func ensureInclude() error {
	path := UserConfigPath()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if IncludePresent(string(data)) {
		return nil
	}
	content := "# git-wardrobe managed hosts\n" + includeMarker + "\n\n" + string(data)
	return os.WriteFile(path, []byte(content), 0o600)
}

// IncludePresent reports whether the wardrobe include is already wired in.
func IncludePresent(userConfig string) bool {
	re := regexp.MustCompile(`(?mi)^\s*Include\s+.*wardrobe\.config\s*$`)
	return re.MatchString(userConfig)
}

// GenerateKey creates a new ed25519 key with the given passphrase
// (empty string means no passphrase — callers should discourage that).
func GenerateKey(path, comment, passphrase string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("key %s already exists, refusing to overwrite", path)
	}
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-a", "100", "-C", comment, "-f", path, "-N", passphrase)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh-keygen: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// AddToAgent loads the key into ssh-agent, storing the passphrase in the
// macOS Keychain where supported. When the passphrase is known (we just
// generated the key) it is supplied via a one-shot askpass helper so the
// user is not prompted to retype what they entered seconds ago. The
// passphrase travels through the environment, never argv or the script.
func AddToAgent(keyPath, passphrase string) error {
	args := []string{}
	if runtime.GOOS == "darwin" {
		args = append(args, "--apple-use-keychain")
	}
	args = append(args, keyPath)
	cmd := exec.Command("ssh-add", args...)

	if passphrase == "" {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	dir, err := os.MkdirTemp("", "wardrobe-askpass")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	helper := filepath.Join(dir, "askpass.sh")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf '%s' \"$WARDROBE_ASKPASS\"\n"), 0o700); err != nil {
		return err
	}
	cmd.Env = append(os.Environ(),
		"SSH_ASKPASS="+helper,
		"SSH_ASKPASS_REQUIRE=force",
		"WARDROBE_ASKPASS="+passphrase,
	)
	return cmd.Run()
}

// PublicKey returns the contents of the .pub file for a private key.
func PublicKey(keyPath string) (string, error) {
	data, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// HasPassphrase reports whether the private key is encrypted. It asks
// ssh-keygen to derive the public key with an empty passphrase: success
// means the key is unprotected.
func HasPassphrase(keyPath string) bool {
	cmd := exec.Command("ssh-keygen", "-y", "-P", "", "-f", keyPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() != nil
}

var authOKRe = regexp.MustCompile(`(?i)hi ([\w-]+)!|authenticated via|logged in as ([\w-]+)`)

// Verify runs `ssh -T git@alias` and extracts the provider-side username.
// GitHub intentionally exits 1 on success, so we parse the banner instead
// of trusting the exit code.
func Verify(alias string) (username string, err error) {
	cmd := exec.Command("ssh",
		"-T",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
		"git@"+alias)
	done := make(chan struct{})
	var out []byte
	go func() { out, _ = cmd.CombinedOutput(); close(done) }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("ssh -T git@%s timed out", alias)
	}
	text := string(out)
	if m := authOKRe.FindStringSubmatch(text); m != nil {
		if m[1] != "" {
			return m[1], nil
		}
		return m[2], nil
	}
	return "", fmt.Errorf("authentication failed: %s", strings.TrimSpace(text))
}

// ResolveIdentityFile asks the real OpenSSH resolver (`ssh -G`) which
// key a host alias will use — ground truth, not our own parsing.
func ResolveIdentityFile(host string) (string, error) {
	out, err := exec.Command("ssh", "-G", host).Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "identityfile ") {
			return config.ExpandHome(strings.TrimPrefix(line, "identityfile ")), nil
		}
	}
	return "", fmt.Errorf("no identityfile in ssh -G output for %s", host)
}
