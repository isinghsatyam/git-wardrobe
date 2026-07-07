package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/isinghsatyam/git-wardrobe/internal/config"
	"github.com/isinghsatyam/git-wardrobe/internal/sshcfg"
	"github.com/isinghsatyam/git-wardrobe/internal/ui"
)

var listCheck bool

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "Show all configured accounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if len(cfg.Accounts) == 0 {
			ui.Infof("no accounts yet — run `git wardrobe add`")
			return nil
		}
		headers := []string{"NAME", "EMAIL", "DIRECTORY", "AUTH", "KEY", "SIGN"}
		if listCheck {
			headers = append(headers, "AUTH")
		}
		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(ui.Dim).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return ui.Header.Underline(false).Padding(0, 1)
				}
				return lipgloss.NewStyle().Padding(0, 1)
			}).
			Headers(headers...)
		for _, a := range cfg.Accounts {
			key := config.ContractHome(a.KeyPath())
			if a.AuthMode() == "https" {
				key = ui.Dim.Render("(PAT)")
			}
			row := []string{
				ui.Accent.Render(a.Name),
				a.Email,
				a.Dir,
				a.AuthMode(),
				key,
				a.Sign,
			}
			if listCheck {
				if user, err := sshcfg.Verify(a.Alias()); err != nil {
					row = append(row, ui.Bad.Render("✗ failed"))
				} else {
					row = append(row, ui.Good.Render("✓ "+user))
				}
			}
			t.Row(row...)
		}
		fmt.Println(t)
		return nil
	},
}

func init() {
	listCmd.Flags().BoolVar(&listCheck, "check", false, "also test ssh authentication for every account (network)")
	rootCmd.AddCommand(listCmd)
}
