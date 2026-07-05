package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/isinghsatyam/git-wardrobe/internal/config"
	"github.com/isinghsatyam/git-wardrobe/internal/doctor"
	"github.com/isinghsatyam/git-wardrobe/internal/ui"
)

var doctorNetwork bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Audit the whole multi-account setup",
	Long: `Checks every account (key exists, passphrase set, alias resolves to the
right key, identity applies in its directory) plus the environment around
it: default-host key leaks, global identity bleed, orphan keys.

--network adds live ssh authentication tests against each provider.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		ui.Titlef("── wardrobe doctor ──")
		report := doctor.Run(cfg, doctorNetwork)

		lastArea := ""
		for _, f := range report.Findings {
			if f.Area != lastArea {
				fmt.Println()
				fmt.Println(ui.Header.Render(f.Area))
				lastArea = f.Area
			}
			switch f.Severity {
			case doctor.OK:
				ui.Successf("%s", f.Message)
			case doctor.Info:
				ui.Infof("%s", f.Message)
			case doctor.Warning:
				ui.Warnf("%s", f.Message)
			case doctor.Failure:
				ui.Errorf("%s", f.Message)
			}
			if f.Fix != "" && f.Severity >= doctor.Warning {
				fmt.Println(ui.Dim.Render("    ↳ " + f.Fix))
			}
		}

		ok, info, warn, fail := report.Counts()
		fmt.Println()
		ui.Titlef("%d ok · %d info · %d warnings · %d failures", ok, info, warn, fail)
		if !doctorNetwork {
			ui.Infof("run with --network to also test live ssh authentication")
		}
		if fail > 0 {
			return fmt.Errorf("doctor found %d failure(s)", fail)
		}
		return nil
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorNetwork, "network", false, "run live ssh authentication tests")
	rootCmd.AddCommand(doctorCmd)
}
