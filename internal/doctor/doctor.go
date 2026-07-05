// Package doctor audits the whole multi-account setup — wardrobe-managed
// pieces and the surrounding ssh/git environment — and reports concrete,
// fixable findings.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/isinghsatyam/git-wardrobe/internal/config"
	"github.com/isinghsatyam/git-wardrobe/internal/gitcfg"
	"github.com/isinghsatyam/git-wardrobe/internal/sshcfg"
)

type Severity int

const (
	OK Severity = iota
	Info
	Warning
	Failure
)

type Finding struct {
	Severity Severity
	Area     string // e.g. "account:personal", "ssh", "git"
	Message  string
	Fix      string // one-line suggested remedy, empty if none
}

type Report struct{ Findings []Finding }

func (r *Report) add(sev Severity, area, msg, fix string) {
	r.Findings = append(r.Findings, Finding{sev, area, msg, fix})
}

func (r *Report) Counts() (ok, info, warn, fail int) {
	for _, f := range r.Findings {
		switch f.Severity {
		case OK:
			ok++
		case Info:
			info++
		case Warning:
			warn++
		case Failure:
			fail++
		}
	}
	return
}

// Run executes every check. network=true adds live ssh auth tests.
func Run(cfg *config.Config, network bool) *Report {
	r := &Report{}
	r.checkWiring(cfg)
	for i := range cfg.Accounts {
		r.checkAccount(&cfg.Accounts[i], network)
	}
	r.checkDefaultHostLeak(cfg)
	r.checkGlobalIdentity(cfg)
	r.checkOrphanKeys(cfg)
	return r
}

func (r *Report) checkWiring(cfg *config.Config) {
	if len(cfg.Accounts) == 0 {
		r.add(Info, "setup", "no accounts configured yet", "run `git wardrobe add`")
		return
	}
	if data, err := os.ReadFile(sshcfg.UserConfigPath()); err != nil || !sshcfg.IncludePresent(string(data)) {
		r.add(Failure, "ssh", "~/.ssh/config does not include wardrobe.config", "run any `git wardrobe add/remove` to rewire, or add `Include wardrobe.config` at the top")
	} else {
		r.add(OK, "ssh", "wardrobe.config included from ~/.ssh/config", "")
	}
	if _, err := os.Stat(sshcfg.ManagedPath()); err != nil {
		r.add(Failure, "ssh", "managed file ~/.ssh/wardrobe.config missing", "run any `git wardrobe add/remove` to regenerate")
	}
	if gitcfg.GlobalIncludeWired() {
		r.add(OK, "git", "wardrobe.gitconfig included from global git config", "")
	} else {
		r.add(Failure, "git", "global git config does not include wardrobe routing table", "run any `git wardrobe add/remove` to rewire")
	}
}

func (r *Report) checkAccount(a *config.Account, network bool) {
	area := "account:" + a.Name
	key := a.KeyPath()

	if fi, err := os.Stat(key); err != nil {
		r.add(Failure, area, fmt.Sprintf("private key %s missing", config.ContractHome(key)), "generate or restore the key, then re-run `git wardrobe add`")
		return
	} else if fi.Mode().Perm()&0o077 != 0 {
		r.add(Warning, area, fmt.Sprintf("key %s is group/world readable (%o)", config.ContractHome(key), fi.Mode().Perm()), fmt.Sprintf("chmod 600 %s", key))
	}

	if sshcfg.HasPassphrase(key) {
		r.add(OK, area, "key is passphrase-protected", "")
	} else {
		r.add(Warning, area, fmt.Sprintf("key %s has NO passphrase — file theft equals account access", config.ContractHome(key)), fmt.Sprintf("ssh-keygen -p -f %s", key))
	}

	if resolved, err := sshcfg.ResolveIdentityFile(a.Alias()); err != nil {
		r.add(Failure, area, fmt.Sprintf("ssh cannot resolve alias %s: %v", a.Alias(), err), "")
	} else if resolved != key {
		r.add(Failure, area, fmt.Sprintf("alias %s resolves to %s, expected %s — another ssh config rule is shadowing it", a.Alias(), config.ContractHome(resolved), config.ContractHome(key)), "check Host blocks above the wardrobe Include in ~/.ssh/config")
	} else {
		r.add(OK, area, fmt.Sprintf("alias %s → %s", a.Alias(), config.ContractHome(key)), "")
	}

	if _, err := os.Stat(a.DirPath()); err != nil {
		r.add(Warning, area, fmt.Sprintf("directory %s does not exist yet", a.Dir), fmt.Sprintf("mkdir -p %s", a.DirPath()))
	} else if repo := findRepoUnder(a.DirPath()); repo != "" {
		_, email, _ := gitcfg.EffectiveIdentity(repo)
		if email == a.Email {
			r.add(OK, area, fmt.Sprintf("identity check in %s: %s", config.ContractHome(repo), email), "")
		} else {
			r.add(Failure, area, fmt.Sprintf("repo %s resolves email %q, expected %q", config.ContractHome(repo), email, a.Email), "a local repo config or another include is overriding the wardrobe identity")
		}
	}

	if network {
		if user, err := sshcfg.Verify(a.Alias()); err != nil {
			r.add(Failure, area, fmt.Sprintf("ssh auth to %s failed: %v", a.Host, err), "is the public key registered on the account? `git wardrobe add` can re-upload it")
		} else {
			msg := fmt.Sprintf("authenticated to %s as %s", a.Host, user)
			if a.Username != "" && !strings.EqualFold(a.Username, user) {
				r.add(Failure, area, fmt.Sprintf("authenticated as %q but account is registered as %q — key registered on the wrong account", user, a.Username), "")
			} else {
				r.add(OK, area, msg, "")
			}
		}
	}
}

// checkDefaultHostLeak warns when a bare `git@github.com` (outside any
// account directory) silently authenticates with one account's key.
func (r *Report) checkDefaultHostLeak(cfg *config.Config) {
	hosts := map[string]bool{}
	for _, a := range cfg.Accounts {
		hosts[a.Host] = true
	}
	for host := range hosts {
		resolved, err := sshcfg.ResolveIdentityFile(host)
		if err != nil {
			continue
		}
		if _, statErr := os.Stat(resolved); statErr != nil {
			continue // default id_rsa etc. that doesn't exist: nothing leaks
		}
		owner := "an unmanaged key"
		for _, a := range cfg.Accounts {
			if a.KeyPath() == resolved {
				owner = fmt.Sprintf("account %q", a.Name)
			}
		}
		r.add(Warning, "ssh",
			fmt.Sprintf("bare `git@%s` (outside account directories) authenticates with %s (%s)", host, owner, config.ContractHome(resolved)),
			"intentional? if not, remove the default IdentityFile for that host from ~/.ssh/config")
	}
}

// checkGlobalIdentity flags a global user.email, which silently signs
// commits in unmanaged directories with that identity.
func (r *Report) checkGlobalIdentity(cfg *config.Config) {
	out, _ := exec.Command("git", "config", "--global", "--get", "user.email").Output()
	email := strings.TrimSpace(string(out))
	if email == "" {
		r.add(OK, "git", "no global user.email — identities only apply inside account directories", "")
		return
	}
	ucOut, _ := exec.Command("git", "config", "--global", "--get", "user.useConfigOnly").Output()
	if strings.TrimSpace(string(ucOut)) == "true" {
		r.add(OK, "git", "user.useConfigOnly=true guards against wrong-identity commits", "")
		return
	}
	r.add(Warning, "git",
		fmt.Sprintf("global user.email=%s applies to every repo outside account directories", email),
		"consider `git config --global user.useConfigOnly true` and removing the global email so git fails closed")
}

var privKeyHeader = regexp.MustCompile(`-----BEGIN (OPENSSH|RSA|EC|DSA) PRIVATE KEY-----`)

// checkOrphanKeys lists private keys in ~/.ssh referenced by neither
// wardrobe nor the user's own ssh config.
func (r *Report) checkOrphanKeys(cfg *config.Config) {
	referenced := map[string]bool{}
	for _, a := range cfg.Accounts {
		referenced[a.KeyPath()] = true
	}
	for _, p := range []string{sshcfg.UserConfigPath(), sshcfg.ManagedPath()} {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			f := strings.Fields(strings.TrimSpace(line))
			if len(f) == 2 && strings.EqualFold(f[0], "IdentityFile") {
				referenced[config.ExpandHome(f[1])] = true
			}
		}
	}
	entries, err := os.ReadDir(sshcfg.SSHDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".pub") {
			continue
		}
		path := filepath.Join(sshcfg.SSHDir(), e.Name())
		head := make([]byte, 64)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		n, _ := f.Read(head)
		f.Close()
		if !privKeyHeader.Match(head[:n]) {
			continue
		}
		if !referenced[path] {
			r.add(Info, "ssh",
				fmt.Sprintf("private key %s is referenced by no ssh config entry", config.ContractHome(path)),
				"if unused, remove it from the provider account and delete the file")
		}
	}
}

// findRepoUnder locates one git repo beneath dir (depth ≤ 3) to test
// identity resolution against. Empty string when none found.
func findRepoUnder(dir string) string {
	var found string
	maxDepth := strings.Count(dir, string(filepath.Separator)) + 3
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return filepath.SkipAll
		}
		if !d.IsDir() {
			return nil
		}
		if strings.Count(path, string(filepath.Separator)) > maxDepth {
			return filepath.SkipDir
		}
		if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
