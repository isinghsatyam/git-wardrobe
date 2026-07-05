package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/isinghsatyam/git-wardrobe/internal/config"
	"github.com/isinghsatyam/git-wardrobe/internal/gitcfg"
	"github.com/isinghsatyam/git-wardrobe/internal/sshcfg"
	"github.com/isinghsatyam/git-wardrobe/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show which identity applies in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		ui.Titlef("── wardrobe status: %s ──", config.ContractHome(cwd))

		acct, matched := cfg.MatchDir(cwd)
		if matched {
			fmt.Printf("%s  %s (%s)\n", label("account"), ui.Accent.Render(acct.Name), acct.Email)
		} else {
			ui.Warnf("no wardrobe account covers this directory — commits here use global/default identity")
		}

		name, email, signKey := gitcfg.EffectiveIdentity(cwd)
		fmt.Printf("%s  %s <%s>\n", label("git identity"), orNone(name), orNone(email))
		if signKey != "" {
			fmt.Printf("%s  %s\n", label("signing key"), config.ContractHome(signKey))
		}
		if matched && email != acct.Email {
			ui.Errorf("effective email %q ≠ account email %q — something overrides the wardrobe config (run `git wardrobe doctor`)", email, acct.Email)
		}

		// Inside a repo: resolve the key the origin remote would really use.
		if remote := originURL(cwd); remote != "" {
			fmt.Printf("%s  %s\n", label("origin"), remote)
			if host := sshHostOf(remote); host != "" {
				if keyFile, err := sshcfg.ResolveIdentityFile(host); err == nil {
					fmt.Printf("%s  %s %s\n", label("push key"), config.ContractHome(keyFile), ui.Dim.Render("(via ssh -G "+host+")"))
					if matched && keyFile != acct.KeyPath() {
						ui.Errorf("push would use a key not belonging to account %q", acct.Name)
					}
				}
			}
		}
		return nil
	},
}

func label(s string) string { return ui.Dim.Render(fmt.Sprintf("%14s", s)) }

func orNone(s string) string {
	if s == "" {
		return ui.Bad.Render("(unset)")
	}
	return s
}

func originURL(dir string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// sshHostOf extracts the ssh host/alias from a remote URL, or "" for https.
func sshHostOf(remote string) string {
	if strings.HasPrefix(remote, "git@") {
		if i := strings.Index(remote, ":"); i > 4 {
			return remote[4:i]
		}
	}
	if strings.HasPrefix(remote, "ssh://git@") {
		rest := strings.TrimPrefix(remote, "ssh://git@")
		if i := strings.IndexAny(rest, "/:"); i > 0 {
			return rest[:i]
		}
	}
	return ""
}

func init() { rootCmd.AddCommand(statusCmd) }
