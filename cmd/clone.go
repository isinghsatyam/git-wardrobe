package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/isinghsatyam/git-wardrobe/internal/config"
	"github.com/isinghsatyam/git-wardrobe/internal/gitcfg"
	"github.com/isinghsatyam/git-wardrobe/internal/ui"
)

var cloneAccount string

var cloneCmd = &cobra.Command{
	Use:   "clone <repository-url> [target-dir]",
	Short: "Clone a repository with the right account, into the right place",
	Long: `Accepts any GitHub/GitLab/Bitbucket URL shape (https or ssh), figures
out which account to use (from --account, the current directory, or by
asking), rewrites the URL to that account's ssh alias, clones into the
account's directory, and verifies the resulting identity.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runClone,
}

func init() {
	cloneCmd.Flags().StringVarP(&cloneAccount, "account", "a", "", "account to clone as (default: inferred from cwd, else asked)")
	rootCmd.AddCommand(cloneCmd)
}

// urlRe handles https://host/owner/repo(.git), git@host:owner/repo(.git)
// and ssh://git@host/owner/repo(.git). Repo names may contain dots.
var urlRe = regexp.MustCompile(`^(?:https?://|git@|ssh://git@)([^/:]+)[/:]([^/]+)/(.+?)(?:\.git)?/?$`)

func parseRepoURL(raw string) (host, owner, repo string, err error) {
	m := urlRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return "", "", "", fmt.Errorf("cannot parse repository URL %q", raw)
	}
	return m[1], m[2], m[3], nil
}

func runClone(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Accounts) == 0 {
		return fmt.Errorf("no accounts configured — run `git wardrobe add` first")
	}

	host, owner, repo, err := parseRepoURL(args[0])
	if err != nil {
		return err
	}
	// Azure-style URLs nest project paths in the repo segment; the last
	// element is the repository name for target-directory purposes.
	repo = filepath.Base(repo)

	acct, err := pickAccount(cfg, host)
	if err != nil {
		return err
	}

	// Target: explicit arg > inside account dir if cwd already there > account dir root.
	var target string
	switch {
	case len(args) == 2:
		target = args[1]
	default:
		cwd, _ := os.Getwd()
		if in, ok := cfg.MatchDir(cwd); ok && in.Name == acct.Name {
			target = filepath.Join(cwd, repo)
		} else {
			target = filepath.Join(acct.DirPath(), repo)
		}
	}

	// https/PAT accounts keep the URL untouched (credential helper handles
	// auth, and hosts like Azure DevOps have their own URL shapes).
	cloneURL := strings.TrimSpace(args[0])
	if acct.AuthMode() == "ssh" {
		cloneURL = fmt.Sprintf("git@%s:%s/%s.git", acct.Alias(), owner, repo)
	}
	ui.Infof("account   %s (%s)", ui.Accent.Render(acct.Name), acct.Email)
	ui.Infof("url       %s", cloneURL)
	ui.Infof("target    %s", config.ContractHome(target))

	git := exec.Command("git", "clone", cloneURL, target)
	git.Stdin, git.Stdout, git.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := git.Run(); err != nil {
		return fmt.Errorf("git clone failed — is the key registered on the %s account?", acct.Name)
	}

	// Post-clone identity assertion: catch config drift immediately.
	_, email, _ := gitcfg.EffectiveIdentity(target)
	if email == acct.Email {
		ui.Successf("cloned; commits here will be %s <%s>", acct.GitName, email)
	} else {
		ui.Warnf("cloned, but effective email is %q, expected %q — run `git wardrobe doctor`", email, acct.Email)
	}
	return nil
}

func pickAccount(cfg *config.Config, host string) (*config.Account, error) {
	if cloneAccount != "" {
		a, ok := cfg.Get(cloneAccount)
		if !ok {
			return nil, fmt.Errorf("no account named %q", cloneAccount)
		}
		return a, nil
	}
	cwd, _ := os.Getwd()
	if a, ok := cfg.MatchDir(cwd); ok {
		return a, nil
	}
	// Candidates on the same provider first.
	var sameHost []config.Account
	for _, a := range cfg.Accounts {
		if a.Host == host {
			sameHost = append(sameHost, a)
		}
	}
	if len(sameHost) == 1 {
		a, _ := cfg.Get(sameHost[0].Name)
		return a, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("cannot infer account — pass --account <name>")
	}
	pool := sameHost
	if len(pool) == 0 {
		pool = cfg.Accounts
	}
	var opts []huh.Option[string]
	for _, a := range pool {
		opts = append(opts, huh.NewOption(fmt.Sprintf("%s — %s (%s)", a.Name, a.Email, a.Dir), a.Name))
	}
	var chosen string
	if err := huh.NewSelect[string]().Title("Clone as which account?").Options(opts...).Value(&chosen).Run(); err != nil {
		return nil, err
	}
	a, _ := cfg.Get(chosen)
	return a, nil
}
