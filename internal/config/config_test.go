package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchDirLongestWins(t *testing.T) {
	home, _ := os.UserHomeDir()
	cfg := &Config{Accounts: []Account{
		{Name: "broad", Dir: "~/work"},
		{Name: "narrow", Dir: "~/work/client"},
	}}
	a, ok := cfg.MatchDir(filepath.Join(home, "work", "client", "repo"))
	if !ok || a.Name != "narrow" {
		t.Fatalf("expected narrow, got %+v ok=%v", a, ok)
	}
	a, ok = cfg.MatchDir(filepath.Join(home, "work", "other"))
	if !ok || a.Name != "broad" {
		t.Fatalf("expected broad, got %+v ok=%v", a, ok)
	}
	if _, ok := cfg.MatchDir("/somewhere/else"); ok {
		t.Fatal("expected no match outside account dirs")
	}
	// Prefix that is not a path boundary must not match.
	if _, ok := cfg.MatchDir(filepath.Join(home, "workshop")); ok {
		t.Fatal("~/workshop must not match ~/work")
	}
}

func TestValidate(t *testing.T) {
	for _, good := range []Account{
		{Name: "p", Email: "a@b.c", Dir: "~/p", Host: "github.com", Key: "~/.ssh/k", Sign: "ssh"},
		{Name: "pat", Email: "a@b.c", Dir: "~/p2", Host: "dev.azure.com", Auth: "https", Sign: "none"},
		{Name: "patgpg", Email: "a@b.c", Dir: "~/p3", Host: "dev.azure.com", Auth: "https", Sign: "gpg", SigningKey: "ABC123"},
	} {
		if err := good.Validate(); err != nil {
			t.Fatalf("valid account %q rejected: %v", good.Name, err)
		}
	}
	for _, bad := range []Account{
		{Name: "has space", Email: "a@b.c", Dir: "~/x", Host: "h", Key: "~/.ssh/k", Sign: "ssh"},
		{Name: "x", Email: "nomail", Dir: "~/x", Host: "h", Key: "~/.ssh/k", Sign: "ssh"},
		{Name: "x", Email: "a@b.c", Dir: "", Host: "h", Key: "~/.ssh/k", Sign: "ssh"},
		{Name: "x", Email: "a@b.c", Dir: "~/x", Host: "h", Key: "~/.ssh/k", Sign: "gpg2"},
		{Name: "x", Email: "a@b.c", Dir: "~/x", Host: "h", Key: "~/.ssh/k", Sign: "gpg"}, // gpg without key id
		{Name: "x", Email: "a@b.c", Dir: "~/x", Host: "h", Sign: "ssh"},                  // ssh auth without key
		{Name: "x", Email: "a@b.c", Dir: "~/x", Host: "h", Auth: "https", Sign: "ssh"},   // ssh signing without ssh key
		{Name: "x", Email: "a@b.c", Dir: "~/x", Host: "h", Auth: "ftp", Sign: "none"},    // bogus auth
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("expected rejection: %+v", bad)
		}
	}
}

func TestExpandContractRoundtrip(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := ExpandHome("~/x/y"); got != filepath.Join(home, "x", "y") {
		t.Errorf("ExpandHome: %q", got)
	}
	if got := ContractHome(filepath.Join(home, "x")); got != "~/x" {
		t.Errorf("ContractHome: %q", got)
	}
	if got := ExpandHome("/abs/path"); got != "/abs/path" {
		t.Errorf("absolute path must pass through, got %q", got)
	}
}
