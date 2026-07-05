package main

import (
	"os"

	"github.com/isinghsatyam/git-wardrobe/cmd"
	"github.com/isinghsatyam/git-wardrobe/internal/ui"
)

func main() {
	if err := cmd.Execute(); err != nil {
		ui.Errorf("%v", err)
		os.Exit(1)
	}
}
