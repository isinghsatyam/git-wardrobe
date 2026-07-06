package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/isinghsatyam/git-wardrobe/internal/config"
	"github.com/isinghsatyam/git-wardrobe/internal/gitcfg"
	"github.com/isinghsatyam/git-wardrobe/internal/sshcfg"
	"github.com/isinghsatyam/git-wardrobe/internal/ui"
)

var removeDeleteKey bool

var removeCmd = &cobra.Command{
	Use:     "remove [name]",
	Aliases: []string{"rm"},
	Short:   "Remove an account and regenerate the managed configs",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		var name string
		switch {
		case len(args) == 1:
			name = args[0]
		case len(cfg.Accounts) == 0:
			return fmt.Errorf("no accounts configured")
		case term.IsTerminal(int(os.Stdin.Fd())):
			var opts []huh.Option[string]
			for _, acc := range cfg.Accounts {
				opts = append(opts, huh.NewOption(fmt.Sprintf("%s — %s (%s)", acc.Name, acc.Email, acc.Dir), acc.Name))
			}
			if err := huh.NewSelect[string]().Title("Remove which account?").Options(opts...).Value(&name).Run(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("pass the account name: git wardrobe remove <name>")
		}
		a, ok := cfg.Get(name)
		if !ok {
			return fmt.Errorf("no account named %q", name)
		}
		keyPath := a.KeyPath()

		if term.IsTerminal(int(os.Stdin.Fd())) {
			confirmed := false
			if err := huh.NewConfirm().
				Title(fmt.Sprintf("Remove account %q?", a.Name)).
				Description("Repositories and the key file stay; only wardrobe config entries go.").
				Value(&confirmed).Run(); err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
		}

		cfg.Remove(name)
		if err := sshcfg.WriteManaged(cfg.Accounts); err != nil {
			return err
		}
		if err := gitcfg.WriteAll(cfg.Accounts); err != nil {
			return err
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		ui.Successf("account %q removed; managed configs regenerated", name)

		if removeDeleteKey {
			if err := os.Remove(keyPath); err == nil {
				_ = os.Remove(keyPath + ".pub")
				ui.Successf("deleted key %s", config.ContractHome(keyPath))
				ui.Warnf("also remove the public key from the provider account settings")
			}
		} else {
			ui.Infof("key %s kept — delete with --delete-key next time, or by hand", config.ContractHome(keyPath))
		}
		return nil
	},
}

func init() {
	removeCmd.Flags().BoolVar(&removeDeleteKey, "delete-key", false, "also delete the ssh key pair from disk")
	rootCmd.AddCommand(removeCmd)
}
