package cmd

import "testing"

func TestParseRepoURL(t *testing.T) {
	cases := []struct {
		in                string
		host, owner, repo string
		wantErr           bool
	}{
		{"https://github.com/vercel/next.js", "github.com", "vercel", "next.js", false},
		{"https://github.com/vercel/next.js.git", "github.com", "vercel", "next.js", false},
		{"git@github.com:torvalds/linux.git", "github.com", "torvalds", "linux", false},
		{"git@github.com:torvalds/linux", "github.com", "torvalds", "linux", false},
		{"ssh://git@gitlab.com/group/project.git", "gitlab.com", "group", "project", false},
		{"https://bitbucket.org/team/repo/", "bitbucket.org", "team", "repo", false},
		{"git@wardrobe-personal:me/dotfiles.git", "wardrobe-personal", "me", "dotfiles", false},
		{"not a url", "", "", "", true},
		{"https://github.com/onlyowner", "", "", "", true},
	}
	for _, c := range cases {
		host, owner, repo, err := parseRepoURL(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("%q: expected error, got %s/%s/%s", c.in, host, owner, repo)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error %v", c.in, err)
			continue
		}
		if host != c.host || owner != c.owner || repo != c.repo {
			t.Errorf("%q: got %s/%s/%s, want %s/%s/%s", c.in, host, owner, repo, c.host, c.owner, c.repo)
		}
	}
}

func TestSSHHostOf(t *testing.T) {
	cases := map[string]string{
		"git@wardrobe-personal:me/repo.git": "wardrobe-personal",
		"ssh://git@github.com/me/repo.git":  "github.com",
		"https://github.com/me/repo.git":    "",
		"git@github.com:me/repo.git":        "github.com",
	}
	for in, want := range cases {
		if got := sshHostOf(in); got != want {
			t.Errorf("sshHostOf(%q) = %q, want %q", in, got, want)
		}
	}
}
