// Package gitcfg generates git configuration fragments.
//
// Layout:
//
//	~/.config/git-wardrobe/wardrobe.gitconfig   — includeIf routing table
//	~/.config/git-wardrobe/<account>.gitconfig  — one identity each
//
// The user's ~/.gitconfig gains a single `include.path` entry pointing at
// the routing table, added via `git config --global` (never rewritten).
package gitcfg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/isinghsatyam/git-wardrobe/internal/config"
)

// RoutingPath returns the includeIf routing file location.
func RoutingPath() string { return filepath.Join(config.Dir(), "wardrobe.gitconfig") }

// AccountPath returns the identity fragment location for an account name.
func AccountPath(name string) string {
	return filepath.Join(config.Dir(), name+".gitconfig")
}

// RenderRouting produces the includeIf routing table.
func RenderRouting(accounts []config.Account) string {
	var b strings.Builder
	b.WriteString("# Managed by git-wardrobe — do not edit; run `git wardrobe` commands instead.\n\n")
	for _, a := range accounts {
		dir := config.ContractHome(a.DirPath())
		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}
		fmt.Fprintf(&b, "[includeIf \"gitdir:%s\"]\n", dir)
		fmt.Fprintf(&b, "    path = %s\n\n", config.ContractHome(AccountPath(a.Name)))
	}
	return b.String()
}

// RenderAccount produces one identity fragment. The url.insteadOf rewrite
// means a plain `git clone git@github.com:...` inside the account's
// directory transparently uses the account's ssh alias and key.
func RenderAccount(a *config.Account) string {
	var b strings.Builder
	b.WriteString("# Managed by git-wardrobe.\n")
	b.WriteString("[user]\n")
	fmt.Fprintf(&b, "    name = %s\n", a.GitName)
	fmt.Fprintf(&b, "    email = %s\n", a.Email)
	switch a.Sign {
	case "ssh":
		fmt.Fprintf(&b, "    signingkey = %s.pub\n", config.ContractHome(a.KeyPath()))
		b.WriteString("[gpg]\n    format = ssh\n")
		b.WriteString("[commit]\n    gpgsign = true\n")
	case "gpg":
		fmt.Fprintf(&b, "    signingkey = %s\n", a.SigningKey)
		b.WriteString("[gpg]\n    format = openpgp\n")
		b.WriteString("[commit]\n    gpgsign = true\n")
	default:
		b.WriteString("[commit]\n    gpgsign = false\n")
	}
	if a.AuthMode() == "ssh" {
		fmt.Fprintf(&b, "[url \"git@%s:\"]\n", a.Alias())
		fmt.Fprintf(&b, "    insteadOf = git@%s:\n", a.Host)
	}
	return b.String()
}

// WriteAll regenerates routing table plus every account fragment, prunes
// fragments for accounts that no longer exist, and wires the global include.
func WriteAll(accounts []config.Account) error {
	if err := os.MkdirAll(config.Dir(), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(RoutingPath(), []byte(RenderRouting(accounts)), 0o644); err != nil {
		return err
	}
	keep := map[string]bool{filepath.Base(RoutingPath()): true}
	for _, a := range accounts {
		keep[a.Name+".gitconfig"] = true
		if err := os.WriteFile(AccountPath(a.Name), []byte(RenderAccount(&a)), 0o644); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(config.Dir())
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".gitconfig") && !keep[e.Name()] {
			_ = os.Remove(filepath.Join(config.Dir(), e.Name()))
		}
	}
	return ensureGlobalInclude()
}

// ensureGlobalInclude adds include.path to ~/.gitconfig once.
func ensureGlobalInclude() error {
	want := config.ContractHome(RoutingPath())
	out, _ := exec.Command("git", "config", "--global", "--get-all", "include.path").Output()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == want || config.ExpandHome(strings.TrimSpace(line)) == config.ExpandHome(want) {
			return nil
		}
	}
	return exec.Command("git", "config", "--global", "--add", "include.path", want).Run()
}

// GlobalIncludeWired reports whether ~/.gitconfig already includes the
// routing table.
func GlobalIncludeWired() bool {
	out, _ := exec.Command("git", "config", "--global", "--get-all", "include.path").Output()
	want := config.ExpandHome(config.ContractHome(RoutingPath()))
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if config.ExpandHome(strings.TrimSpace(line)) == want {
			return true
		}
	}
	return false
}

// EffectiveIdentity returns user.name, user.email and user.signingkey as
// git resolves them from the given directory.
func EffectiveIdentity(dir string) (name, email, signingKey string) {
	get := func(key string) string {
		cmd := exec.Command("git", "config", "--get", key)
		cmd.Dir = dir
		out, _ := cmd.Output()
		return strings.TrimSpace(string(out))
	}
	return get("user.name"), get("user.email"), get("user.signingkey")
}
