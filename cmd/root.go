// Package cmd wires the git-wardrobe CLI.
package cmd

import (
	"github.com/spf13/cobra"
)

// Version is stamped by goreleaser / -ldflags at build time.
var Version = "0.2.0"

var rootCmd = &cobra.Command{
	Use:     "git-wardrobe",
	Version: Version,
	Short:   "One wardrobe, many git identities",
	Long: `git-wardrobe manages multiple git accounts (work, personal, clients)
on one machine: SSH keys, ssh config, and per-directory git identities,
all generated from a single config file.

Install it on PATH and git picks it up as a subcommand:

    git wardrobe add       set up a new account (interactive)
    git wardrobe list      show all accounts
    git wardrobe status    which identity applies right here?
    git wardrobe doctor    audit the whole setup
    git wardrobe clone     clone with the right identity, always`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI.
func Execute() error { return rootCmd.Execute() }
