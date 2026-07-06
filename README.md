<div align="center">

![git-wardrobe](https://capsule-render.vercel.app/api?type=waving&color=0%3Af12711%2C100%3Af5af19&height=220&section=header&text=git-wardrobe&fontSize=64&fontColor=ffffff&animation=fadeIn&fontAlignY=35&desc=One%20wardrobe%2C%20many%20git%20identities&descSize=22&descAlignY=58)

### 😮‍💨 Tired of switching git profiles manually?

**Work, personal, client accounts on one machine — and never a commit as the wrong person again.**

[![CI](https://github.com/isinghsatyam/git-wardrobe/actions/workflows/ci.yml/badge.svg)](https://github.com/isinghsatyam/git-wardrobe/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/isinghsatyam/git-wardrobe)](https://github.com/isinghsatyam/git-wardrobe/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](#platforms--permissions)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#)

`add` an account once →  every repo in that account's folder **automatically** gets the right email, the right SSH key, the right signing key. Clone, commit, push — zero switching, ever. And `doctor` audits the whole setup so it *stays* right.

![git-wardrobe demo](docs/demo.gif)

</div>

---

`git-wardrobe` sets up and maintains everything the multi-account dance needs: SSH keys, ssh config host aliases, and per-directory git identities — all generated from a single config file, all auditable with one command.

```
$ git wardrobe add          # interactive wizard: key → ssh → git → upload → verify
$ git wardrobe doctor       # audit the whole setup for leaks and misconfigurations
$ git wardrobe clone URL    # clone with the right identity, into the right place
```

## Why

Running work and personal GitHub accounts on one laptop usually ends in one of these:

- 😱 A personal side-project commit authored as `you@dayjob.com`
- 🔑 ssh-agent silently authenticating with the **wrong key**, because `IdentitiesOnly` wasn't set
- 🧩 Hand-maintained config spread across `~/.ssh/config`, `~/.gitconfig`, and N `~/.gitconfig-<client>` files that drift apart
- 🗝️ A pile of passphrase-less keys in `~/.ssh` nobody remembers creating

The building blocks to do this *right* have existed in git and OpenSSH for years — `includeIf`, host aliases, `IdentitiesOnly`, `url.insteadOf` — but wiring them together by hand is fiddly and easy to get subtly wrong. `git-wardrobe` wires them for you, and `doctor` proves they stay wired.

## How it works

One config file is the single source of truth:

```toml
# ~/.config/git-wardrobe/config.toml
[[accounts]]
name  = "personal"
git_name = "Satyam Kumar"
email = "me@example.com"
dir   = "~/personal"          # repos under here use this identity
host  = "github.com"
key   = "~/.ssh/wardrobe_personal"
sign  = "ssh"                 # commit signing with the same SSH key
```

From it, `git-wardrobe` generates:

```
~/.ssh/wardrobe.config              ssh host aliases (IdentitiesOnly enforced)
~/.config/git-wardrobe/
  wardrobe.gitconfig                includeIf routing: directory → identity
  personal.gitconfig                one identity fragment per account
```

Your own `~/.ssh/config` and `~/.gitconfig` are each touched **exactly once**, to add a single `Include` line. Nothing of yours is rewritten, ever. Remove the include lines and you're back to where you started.

The result, at git level:

```
~/personal/anything/        →  commits as me@example.com,  key wardrobe_personal
~/work/anything/            →  commits as me@dayjob.com,   key wardrobe_work
anywhere else               →  your global config (doctor suggests fail-closed)
```

Because each identity fragment carries `url."git@wardrobe-<name>:".insteadOf = git@github.com:`, a plain `git clone git@github.com:you/repo.git` **inside an account directory automatically uses that account's key**. No URL surgery, no remembering aliases.

## Install

### Option 1 — prebuilt binary (no Go needed)

Grab the archive for your OS from the [latest release](https://github.com/isinghsatyam/git-wardrobe/releases/latest), verify against `checksums.txt`, extract, and put `git-wardrobe` anywhere on your PATH. Done — git now knows `git wardrobe`.

### Option 2 — with Go

**Prerequisite: Go 1.22+.** No Go yet? One line:

```sh
# macOS
brew install go
# Debian/Ubuntu
sudo apt install golang-go
# Fedora
sudo dnf install golang
# Windows
winget install GoLang.Go
```

(or grab the official installer from [go.dev/dl](https://go.dev/dl/)). Then:

```sh
go install github.com/isinghsatyam/git-wardrobe@latest
```

`go install` drops the binary in `~/go/bin` — make sure that's on your PATH (`export PATH="$HOME/go/bin:$PATH"` in your shell profile).

or build from source:

```sh
git clone https://github.com/isinghsatyam/git-wardrobe && cd git-wardrobe
go build -o git-wardrobe . && mv git-wardrobe ~/bin/   # anywhere on PATH
```

Any binary named `git-wardrobe` on PATH is automatically a git subcommand: `git wardrobe <cmd>`.

## Commands

### `git wardrobe add`

Interactive wizard that takes an account from nothing to verified:

1. **Who** — account name, author name, email
2. **Where** — directory root (`~/personal` style) and provider (GitHub / GitLab / Bitbucket / self-hosted)
3. **Key** — generates a fresh ed25519 key (passphrase strongly encouraged; macOS Keychain remembers it so you type it once) or reuses an existing one
4. **Register** — uploads the public key via `gh` (only after confirming the gh-authenticated user matches), or copies it to your clipboard and points you at the right settings page
5. **Verify** — live `ssh -T` test; records which provider account actually answered

Fully scriptable too:

```sh
git wardrobe add --yes --name work --git-name "Satyam Kumar" \
  --email me@dayjob.com --dir ~/work --generate-key --sign ssh
```

### `git wardrobe doctor`

The command the other tools don't have. Audits everything:

| Check | Catches |
|---|---|
| key exists, `0600`, passphrase set | stolen-laptop = stolen-account setups |
| alias resolves to the right key (`ssh -G`, ground truth) | shadowed Host blocks, agent-order surprises |
| identity resolves correctly inside each account dir | drifted or overridden `includeIf` chains |
| bare `git@github.com` key leak | pushing to personal repos as your employer |
| global `user.email` bleed | wrong-author commits outside managed dirs |
| orphan private keys in `~/.ssh` | forgotten keys still registered somewhere |
| `--network`: live auth test per account | key registered on the *wrong* provider account |

Exit code is non-zero on failures, so it slots into dotfile CI.

### `git wardrobe clone <url> [dir]`

Accepts any URL shape (`https://`, `git@`, `ssh://`), picks the account (from `--account`, the current directory, or an interactive picker), rewrites to the account's alias, clones into the account's directory, and **asserts the resulting identity** — a wrong email is caught before your first commit, not after.

### `git wardrobe status`

"Which hat am I wearing right here?" Shows the matching account, the identity git will actually use (name/email/signing key), and — inside a repo — which SSH key a push to `origin` would really use, resolved by OpenSSH itself.

### `git wardrobe list` / `remove`

`list` renders the account table (`--check` adds live auth status). `remove <name>` deletes an account and regenerates all managed files; the key stays unless you pass `--delete-key`.

## Platforms & permissions

| Platform | Status | Notes |
|---|---|---|
| 🍎 macOS | ✅ Full | Key passphrases stored in Keychain (`UseKeychain`), clipboard via `pbcopy` |
| 🐧 Linux | ✅ Full | Needs a running `ssh-agent` (default on desktop distros); clipboard via `wl-copy`/`xclip` when present |
| 🪟 Windows 10/11 | ⚠️ Experimental | Works with the built-in OpenSSH client + Git for Windows. Enable the agent once (admin PowerShell): `Set-Service ssh-agent -StartupType Automatic; Start-Service ssh-agent` |

**No `sudo`. Ever.** Everything git-wardrobe touches lives in *your* home directory — `~/.ssh`, `~/.config/git-wardrobe`, your global git config — and it creates those files with the right permissions itself (`700` for `~/.ssh`, `600` for keys and ssh configs). If a permission error ever appears, it means those files were already root-owned by some earlier accident; fix ownership once with `sudo chown -R $USER ~/.ssh` and never think about it again.

The only step that may involve elevated rights is *installing the binary* if you choose a system location (`sudo mv git-wardrobe /usr/local/bin/`) — dropping it in `~/bin` or using `go install` (installs to `~/go/bin`) avoids even that.

## Security posture

- **ed25519 only** for generated keys, 100 KDF rounds
- **`IdentitiesOnly yes` on every alias** — ssh-agent can never offer the wrong key
- Passphrases prompted for by default, remembered by the OS keychain — secure *and* frictionless
- SSH commit signing out of the box (same key, verified badge on GitHub) — no GPG keyring to babysit
- The tool never stores passphrases or tokens; the config file contains no secrets
- Managed files are regenerated wholesale — no sed-into-your-configs surgery
- `doctor` treats your *whole* environment as in scope, not just what wardrobe created

## How it compares

| | git-wardrobe | typical profile switchers | hand-rolled setup |
|---|---|---|---|
| Per-directory auto identity (`includeIf`) | ✅ | sometimes — often manual switching | ✅ if you wire it yourself |
| Generates ssh config + enforces `IdentitiesOnly` | ✅ | ❌ ssh side left to you | easy to forget — the classic wrong-key bug |
| SSH key generation in setup | ✅ | ❌ | `ssh-keygen` by hand |
| Key upload / registration flow | ✅ | ❌ | copy-paste into settings |
| Setup audit (`doctor`) | ✅ | ❌ | ❌ |
| Identity-verified clone helper | ✅ | ❌ | ❌ |
| SSH commit signing setup | ✅ | ❌ | GPG wrestling |
| Live auth verification | ✅ | ❌ | `ssh -T`, if you remember |

## FAQ

**Does it touch my existing setup?**
It adds one `Include` line to `~/.ssh/config` and one `include.path` to your global git config. Everything else lives in its own files. Deleting those two lines fully disables it.

**I already have keys/aliases set up by hand.**
Point `add` at your existing key (`--key ~/.ssh/id_work` or the "reuse" wizard option) and wardrobe adopts it. Run `doctor` afterwards — it audits pre-existing config too and will tell you what your hand-rolled setup got wrong.

**What about HTTPS remotes?**
Wardrobe's identity routing (`includeIf`) applies regardless of transport, so commit author/signing is always right. Key routing is SSH; for HTTPS credential separation, `gh auth switch` or per-account credential helpers are the right tool.

**Does the `gh` CLI need switching per project?**
Rarely. Plain git (clone/commit/push) never touches `gh` — wardrobe's SSH routing covers it. Only `gh`-specific commands (creating repos, PRs, issues, releases) use gh's own login, which is global, not per-directory. gh handles multiple accounts natively:

```sh
gh auth login                  # run once per account — all get stored
gh auth switch -u <username>   # switch the active one when needed
GH_TOKEN=<token> gh pr list    # or override for a single command
```

Practical setup: keep gh logged into your main account and switch only when a project actually needs gh commands under another identity. Auto-warning on gh/directory mismatch is on the roadmap.

**Non-GitHub providers?**
GitLab, Bitbucket and self-hosted hosts work for keys, aliases and identities. Key *upload* automation is GitHub-only (via `gh`); elsewhere you get the key on your clipboard and the right settings URL.

## Roadmap

- `import` — adopt an existing hand-rolled multi-account setup in one command
- `doctor --fix` — apply suggested remedies automatically
- Pre-commit guard hook (defense in depth against identity drift)
- Warn when the active `gh` CLI account doesn't match the directory's account
- Homebrew tap

## Support

If git-wardrobe saved you from a `you@dayjob.com` commit on your side project, consider fueling development:

<a href="https://paypal.me/isinghsatyam"><img src="https://img.shields.io/badge/☕_Buy_me_a_coffee-PayPal-0070BA?logo=paypal&logoColor=white" alt="Buy me a coffee" height="32"></a>

## License

MIT

<div align="center">

![footer](https://capsule-render.vercel.app/api?type=waving&color=0%3Af12711%2C100%3Af5af19&height=110&section=footer)

</div>
