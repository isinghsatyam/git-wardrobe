package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/isinghsatyam/git-wardrobe/internal/config"
	"github.com/isinghsatyam/git-wardrobe/internal/gitcfg"
	"github.com/isinghsatyam/git-wardrobe/internal/sshcfg"
	"github.com/isinghsatyam/git-wardrobe/internal/ui"
)

var syncCmd = &cobra.Command{
	Use:     "sync",
	Aliases: []string{"apply"},
	Short:   "Regenerate all managed files from config.toml",
	Long: `The config file is the source of truth — edit it freely (change an
account's key, email, directory, signing mode…), then run sync to
regenerate the managed ssh and git config files to match.

Validates every account first; an invalid edit changes nothing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		for i := range cfg.Accounts {
			if err := cfg.Accounts[i].Validate(); err != nil {
				return fmt.Errorf("account %q: %w — fix %s and re-run", cfg.Accounts[i].Name, err, config.ContractHome(config.Path()))
			}
		}
		if err := sshcfg.WriteManaged(cfg.Accounts); err != nil {
			return err
		}
		if err := gitcfg.WriteAll(cfg.Accounts); err != nil {
			return err
		}
		ui.Successf("regenerated managed files for %d account(s)", len(cfg.Accounts))
		ui.Infof("run `git wardrobe doctor` to verify the result")
		return nil
	},
}

func init() { rootCmd.AddCommand(syncCmd) }
