// Package config owns the single source of truth for git-wardrobe:
// ~/.config/git-wardrobe/config.toml. Everything else (ssh config,
// gitconfig fragments) is generated from it.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Account is one git identity: who you are, where it applies, which key.
type Account struct {
	Name       string `toml:"name"`
	GitName    string `toml:"git_name"`
	Email      string `toml:"email"`
	Dir        string `toml:"dir"`         // directory root this identity applies to, e.g. ~/personal
	Host       string `toml:"host"`        // provider host, e.g. github.com
	Auth       string `toml:"auth"`        // "ssh" (default) or "https" (PAT via credential helper)
	Key        string `toml:"key"`         // private key path, e.g. ~/.ssh/id_personal; ssh auth only
	Sign       string `toml:"sign"`        // "ssh", "gpg" or "none"
	SigningKey string `toml:"signing_key"` // GPG key id, only when Sign == "gpg"
	Username   string `toml:"username"`    // provider username, filled after verify
}

// Config is the on-disk TOML document.
type Config struct {
	Version  int       `toml:"version"`
	Accounts []Account `toml:"accounts"`
}

const CurrentVersion = 1

// Dir returns ~/.config/git-wardrobe.
func Dir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "git-wardrobe")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "git-wardrobe")
}

// Path returns the config.toml location.
func Path() string { return filepath.Join(Dir(), "config.toml") }

// Load reads the config, returning an empty one if it does not exist yet.
func Load() (*Config, error) {
	cfg := &Config{Version: CurrentVersion}
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", Path(), err)
	}
	return cfg, nil
}

// Save writes the config atomically (temp file + rename).
func (c *Config) Save() error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	sort.Slice(c.Accounts, func(i, j int) bool { return c.Accounts[i].Name < c.Accounts[j].Name })
	var b strings.Builder
	b.WriteString("# git-wardrobe configuration — edit with `git wardrobe` commands or by hand.\n")
	if err := toml.NewEncoder(&b).Encode(c); err != nil {
		return err
	}
	tmp := Path() + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, Path())
}

// Get returns the account by name.
func (c *Config) Get(name string) (*Account, bool) {
	for i := range c.Accounts {
		if c.Accounts[i].Name == name {
			return &c.Accounts[i], true
		}
	}
	return nil, false
}

// Remove deletes the account by name, reporting whether it existed.
func (c *Config) Remove(name string) bool {
	for i := range c.Accounts {
		if c.Accounts[i].Name == name {
			c.Accounts = append(c.Accounts[:i], c.Accounts[i+1:]...)
			return true
		}
	}
	return false
}

// MatchDir returns the account whose Dir contains path, longest match wins.
func (c *Config) MatchDir(path string) (*Account, bool) {
	var best *Account
	bestLen := -1
	for i := range c.Accounts {
		d := ExpandHome(c.Accounts[i].Dir)
		if d == "" {
			continue
		}
		if path == d || strings.HasPrefix(path, d+string(filepath.Separator)) {
			if len(d) > bestLen {
				best, bestLen = &c.Accounts[i], len(d)
			}
		}
	}
	return best, best != nil
}

// Alias returns the ssh host alias for an account, e.g. wardrobe-personal.
func (a *Account) Alias() string { return "wardrobe-" + a.Name }

// AuthMode returns "ssh" or "https"; empty means ssh for compatibility.
func (a *Account) AuthMode() string {
	if a.Auth == "" {
		return "ssh"
	}
	return a.Auth
}

// KeyPath returns the expanded private key path.
func (a *Account) KeyPath() string { return ExpandHome(a.Key) }

// DirPath returns the expanded directory root.
func (a *Account) DirPath() string { return ExpandHome(a.Dir) }

// Validate rejects account fields that would generate broken configs.
func (a *Account) Validate() error {
	if a.Name == "" || strings.ContainsAny(a.Name, " /\\\t") {
		return fmt.Errorf("account name %q must be non-empty, no spaces or slashes", a.Name)
	}
	if !strings.Contains(a.Email, "@") {
		return fmt.Errorf("email %q does not look like an email", a.Email)
	}
	if a.Dir == "" {
		return fmt.Errorf("directory is required")
	}
	if a.Host == "" {
		return fmt.Errorf("host is required")
	}
	if a.Auth != "" && a.Auth != "ssh" && a.Auth != "https" {
		return fmt.Errorf("auth must be \"ssh\" or \"https\", got %q", a.Auth)
	}
	if a.AuthMode() == "ssh" && a.Key == "" {
		return fmt.Errorf("ssh auth needs a key path")
	}
	if a.Sign == "ssh" && a.AuthMode() == "https" {
		return fmt.Errorf("sign=ssh needs an ssh key — use sign=gpg or sign=none with https auth")
	}
	if a.Sign != "ssh" && a.Sign != "gpg" && a.Sign != "none" {
		return fmt.Errorf("sign must be \"ssh\", \"gpg\" or \"none\", got %q", a.Sign)
	}
	if a.Sign == "gpg" && a.SigningKey == "" {
		return fmt.Errorf("sign=gpg needs a signing_key (GPG key id)")
	}
	return nil
}

// ExpandHome turns ~/x into /home/user/x. Non-~ paths pass through.
func ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
	}
	return p
}

// ContractHome turns /home/user/x back into ~/x for readable generated files.
func ContractHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~/" + filepath.ToSlash(strings.TrimPrefix(p, home+string(filepath.Separator)))
	}
	return p
}
