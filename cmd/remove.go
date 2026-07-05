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
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove an account and regenerate the managed configs",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		a, ok := cfg.Get(args[0])
		if !ok {
			return fmt.Errorf("no account named %q", args[0])
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

		cfg.Remove(args[0])
		if err := sshcfg.WriteManaged(cfg.Accounts); err != nil {
			return err
		}
		if err := gitcfg.WriteAll(cfg.Accounts); err != nil {
			return err
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		ui.Successf("account %q removed; managed configs regenerated", args[0])

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
