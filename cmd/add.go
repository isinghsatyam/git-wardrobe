package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/isinghsatyam/git-wardrobe/internal/config"
	"github.com/isinghsatyam/git-wardrobe/internal/gh"
	"github.com/isinghsatyam/git-wardrobe/internal/gitcfg"
	"github.com/isinghsatyam/git-wardrobe/internal/sshcfg"
	"github.com/isinghsatyam/git-wardrobe/internal/ui"
)

var addFlags struct {
	name, gitName, email, dir, host, key, sign string
	generate, noVerify, noUpload, yes          bool
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a git account (interactive wizard, or fully via flags)",
	RunE:  runAdd,
}

func init() {
	f := addCmd.Flags()
	f.StringVar(&addFlags.name, "name", "", "short account name, e.g. personal")
	f.StringVar(&addFlags.gitName, "git-name", "", "commit author name")
	f.StringVar(&addFlags.email, "email", "", "commit author email")
	f.StringVar(&addFlags.dir, "dir", "", "directory root for this identity, e.g. ~/personal")
	f.StringVar(&addFlags.host, "host", "github.com", "provider host")
	f.StringVar(&addFlags.key, "key", "", "existing private key to reuse (default: generate new)")
	f.StringVar(&addFlags.sign, "sign", "ssh", "commit signing: ssh or none")
	f.BoolVar(&addFlags.generate, "generate-key", false, "generate a new ed25519 key without asking")
	f.BoolVar(&addFlags.noVerify, "no-verify", false, "skip the live ssh authentication test")
	f.BoolVar(&addFlags.noUpload, "no-upload", false, "never offer gh key upload")
	f.BoolVar(&addFlags.yes, "yes", false, "non-interactive: accept defaults, require flags for the rest")
	rootCmd.AddCommand(addCmd)
}

func interactive() bool {
	return !addFlags.yes && term.IsTerminal(int(os.Stdin.Fd()))
}

func runAdd(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	a := config.Account{
		Name:    addFlags.name,
		GitName: addFlags.gitName,
		Email:   addFlags.email,
		Dir:     addFlags.dir,
		Host:    addFlags.host,
		Key:     addFlags.key,
		Sign:    addFlags.sign,
	}
	passphrase := ""
	generate := addFlags.generate || a.Key == ""

	if interactive() {
		ui.Titlef("── git wardrobe: new account ──")
		if err := askAccountBasics(cfg, &a); err != nil {
			return err
		}
		if a.Key == "" {
			if err := askKeyChoice(&a, &generate, &passphrase); err != nil {
				return err
			}
		}
	} else if a.Name == "" || a.Email == "" || a.GitName == "" {
		return fmt.Errorf("non-interactive mode needs --name, --git-name and --email")
	}

	// Fill derivable defaults.
	if a.Dir == "" {
		a.Dir = "~/" + a.Name
	}
	if a.Key == "" {
		a.Key = "~/.ssh/wardrobe_" + a.Name
	}
	if _, exists := cfg.Get(a.Name); exists {
		return fmt.Errorf("account %q already exists — `git wardrobe remove %s` first, or pick another name", a.Name, a.Name)
	}
	if err := a.Validate(); err != nil {
		return err
	}

	// 1. Key.
	if generate {
		if err := sshcfg.GenerateKey(a.KeyPath(), a.Email, passphrase); err != nil {
			return err
		}
		ui.Successf("generated ed25519 key %s", config.ContractHome(a.KeyPath()))
		if passphrase == "" {
			ui.Warnf("key has no passphrase — anyone with the file has the account. `ssh-keygen -p -f %s` adds one later.", a.KeyPath())
		} else if err := sshcfg.AddToAgent(a.KeyPath()); err == nil {
			ui.Successf("key loaded into ssh-agent (passphrase remembered by OS keychain)")
		}
	} else if _, err := os.Stat(a.KeyPath()); err != nil {
		return fmt.Errorf("key %s not found", a.KeyPath())
	}

	// 2. Generated configs (ssh + git), atomically from the new account list.
	cfg.Accounts = append(cfg.Accounts, a)
	if err := sshcfg.WriteManaged(cfg.Accounts); err != nil {
		return err
	}
	ui.Successf("ssh alias %s → %s (IdentitiesOnly enforced)", a.Alias(), config.ContractHome(a.KeyPath()))
	if err := gitcfg.WriteAll(cfg.Accounts); err != nil {
		return err
	}
	ui.Successf("git identity for %s/ → %s", a.Dir, a.Email)
	if err := os.MkdirAll(a.DirPath(), 0o755); err != nil {
		return err
	}

	// 3. Public key registration.
	pub, err := sshcfg.PublicKey(a.KeyPath())
	if err == nil {
		offerKeyRegistration(&a, pub)
	}

	// 4. Live verification.
	if !addFlags.noVerify {
		verifyAccount(&a)
	}

	if err := cfg.Save(); err != nil {
		return err
	}
	ui.Successf("account %q saved to %s", a.Name, config.ContractHome(config.Path()))
	fmt.Println()
	ui.Infof("try it:  git wardrobe clone <repo-url>   or clone anything inside %s/", a.Dir)
	return nil
}

func askAccountBasics(cfg *config.Config, a *config.Account) error {
	if a.Host == "" {
		a.Host = "github.com"
	}
	suggestion := func(cur, def string) string {
		if cur != "" {
			return cur
		}
		return def
	}
	a.Dir = suggestion(a.Dir, "")
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Account name").
				Description("Short handle: personal, work, client-acme …").
				Placeholder("personal").
				Value(&a.Name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("required")
					}
					if _, exists := cfg.Get(s); exists {
						return fmt.Errorf("already exists")
					}
					return nil
				}),
			huh.NewInput().Title("Git author name").
				Description("Goes on your commits as user.name").
				Value(&a.GitName).
				Validate(huh.ValidateNotEmpty()),
			huh.NewInput().Title("Email").
				Description("user.email for commits — must match the provider account for verified badges").
				Value(&a.Email).
				Validate(func(s string) error {
					if !strings.Contains(s, "@") {
						return fmt.Errorf("not an email")
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewInput().Title("Directory root").
				DescriptionFunc(func() string {
					return fmt.Sprintf("Repos under this directory use this identity automatically (default: ~/%s)", a.Name)
				}, &a.Name).
				Placeholder("~/personal").
				Value(&a.Dir),
			huh.NewSelect[string]().Title("Provider").
				Options(
					huh.NewOption("GitHub (github.com)", "github.com"),
					huh.NewOption("GitLab (gitlab.com)", "gitlab.com"),
					huh.NewOption("Bitbucket (bitbucket.org)", "bitbucket.org"),
					huh.NewOption("Other / self-hosted", "other"),
				).
				Value(&a.Host),
			huh.NewSelect[string]().Title("Commit signing").
				Description("SSH signing reuses the same key — verified badge, no GPG needed").
				Options(
					huh.NewOption("Sign with SSH key (recommended)", "ssh"),
					huh.NewOption("No signing", "none"),
				).
				Value(&a.Sign),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}
	if a.Host == "other" {
		return huh.NewInput().Title("Host").Placeholder("git.company.com").Value(&a.Host).Run()
	}
	return nil
}

func askKeyChoice(a *config.Account, generate *bool, passphrase *string) error {
	choice := "generate"
	if err := huh.NewSelect[string]().Title("SSH key").
		Options(
			huh.NewOption("Generate a new ed25519 key (recommended)", "generate"),
			huh.NewOption("Reuse an existing key file", "reuse"),
		).Value(&choice).Run(); err != nil {
		return err
	}
	if choice == "reuse" {
		*generate = false
		return huh.NewInput().Title("Private key path").
			Placeholder("~/.ssh/id_ed25519_me").
			Value(&a.Key).
			Validate(func(s string) error {
				if _, err := os.Stat(config.ExpandHome(s)); err != nil {
					return fmt.Errorf("not found")
				}
				return nil
			}).Run()
	}
	*generate = true
	confirm := ""
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Key passphrase").
			Description("Strongly recommended — your OS keychain remembers it, so you type it once. Empty = none.").
			EchoMode(huh.EchoModePassword).
			Value(passphrase),
		huh.NewInput().Title("Confirm passphrase").
			EchoMode(huh.EchoModePassword).
			Value(&confirm),
	))
	if err := form.Run(); err != nil {
		return err
	}
	if *passphrase != confirm {
		return fmt.Errorf("passphrases do not match")
	}
	return nil
}

func offerKeyRegistration(a *config.Account, pub string) {
	title := fmt.Sprintf("git-wardrobe %s@%s", a.Name, hostname())
	if !addFlags.noUpload && a.Host == "github.com" && gh.Available() {
		if login, err := gh.AuthenticatedUser(); err == nil {
			upload := false
			prompt := fmt.Sprintf("gh CLI is logged in as %q — upload the public key to that account?", login)
			if interactive() {
				_ = huh.NewConfirm().Title(prompt).
					Description("Only if this IS the account you are setting up. Otherwise choose No and add the key manually.").
					Value(&upload).Run()
			}
			if upload {
				if err := gh.UploadKey(title, pub); err != nil {
					ui.Errorf("%v", err)
				} else {
					ui.Successf("public key uploaded to GitHub account %s", login)
					return
				}
			}
		}
	}
	fmt.Println()
	ui.Titlef("Register this public key on %s:", a.Host)
	fmt.Println("\n" + pub + "\n")
	if gh.Clipboard(pub) {
		ui.Successf("copied to clipboard")
	}
	ui.Infof("add it here: %s", gh.SettingsURL(a.Host))
	if a.Sign == "ssh" {
		ui.Infof("add it TWICE on GitHub: once as \"Authentication Key\", once as \"Signing Key\" (for verified commits)")
	}
	if interactive() {
		done := true
		_ = huh.NewConfirm().Title("Key registered? (continues to the live connection test)").Value(&done).Run()
	}
}

func verifyAccount(a *config.Account) {
	ui.Infof("testing ssh authentication to %s …", a.Host)
	user, err := sshcfg.Verify(a.Alias())
	if err != nil {
		ui.Warnf("verification failed: %v", err)
		ui.Infof("fix the key registration, then run `git wardrobe doctor --network`")
		return
	}
	a.Username = user
	ui.Successf("authenticated to %s as %s", a.Host, ui.Accent.Render(user))
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "machine"
	}
	return strings.TrimSuffix(h, ".local")
}
