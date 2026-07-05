# git-wardrobe 🚪👔

**One wardrobe, many git identities.** Manage multiple git accounts — work, personal, clients — on one machine without ever committing as the wrong person again.

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

```sh
go install github.com/isinghsatyam/git-wardrobe@latest
```

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

## Security posture

- **ed25519 only** for generated keys, 100 KDF rounds
- **`IdentitiesOnly yes` on every alias** — ssh-agent can never offer the wrong key
- Passphrases prompted for by default, remembered by the OS keychain — secure *and* frictionless
- SSH commit signing out of the box (same key, verified badge on GitHub) — no GPG keyring to babysit
- The tool never stores passphrases or tokens; the config file contains no secrets
- Managed files are regenerated wholesale — no sed-into-your-configs surgery
- `doctor` treats your *whole* environment as in scope, not just what wardrobe created

## Comparison

| | git-wardrobe | [git-ego](https://github.com/bgreenwell/git-ego) | [git-switcher](https://github.com/TheYkk/git-switcher) |
|---|---|---|---|
| Per-directory auto identity (`includeIf`) | ✅ | ✅ | ❌ (manual switch) |
| Generates ssh config + enforces `IdentitiesOnly` | ✅ | ❌ | ❌ |
| SSH key generation in setup | ✅ | ❌ | ❌ |
| Key upload / registration flow | ✅ | ❌ | ❌ |
| Setup audit (`doctor`) | ✅ | ❌ | ❌ |
| Identity-verified clone helper | ✅ | ❌ | ❌ |
| SSH commit signing setup | ✅ | ❌ | ❌ |
| Live auth verification | ✅ | ❌ | ❌ |

## FAQ

**Does it touch my existing setup?**
It adds one `Include` line to `~/.ssh/config` and one `include.path` to your global git config. Everything else lives in its own files. Deleting those two lines fully disables it.

**I already have keys/aliases set up by hand.**
Point `add` at your existing key (`--key ~/.ssh/id_work` or the "reuse" wizard option) and wardrobe adopts it. Run `doctor` afterwards — it audits pre-existing config too and will tell you what your hand-rolled setup got wrong.

**What about HTTPS remotes?**
Wardrobe's identity routing (`includeIf`) applies regardless of transport, so commit author/signing is always right. Key routing is SSH; for HTTPS credential separation, `gh auth switch` or per-account credential helpers are the right tool.

**Non-GitHub providers?**
GitLab, Bitbucket and self-hosted hosts work for keys, aliases and identities. Key *upload* automation is GitHub-only (via `gh`); elsewhere you get the key on your clipboard and the right settings URL.

## Roadmap

- `import` — adopt an existing hand-rolled multi-account setup in one command
- `doctor --fix` — apply suggested remedies automatically
- Pre-commit guard hook (defense in depth against identity drift)
- Homebrew tap and prebuilt binaries

## License

MIT
