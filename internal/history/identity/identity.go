// Package identity resolves GitHub identities for repository setup.
package identity

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

// PlaceholderOwner must be replaced with a person's GitHub login.
const PlaceholderOwner = "<your-github-login>"

var loginPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9_-]{0,37}[A-Za-z0-9])?$`)

func validLogin(login string) bool {
	return loginPattern.MatchString(login) && !strings.Contains(login, "--") && !strings.EqualFold(login, PlaceholderOwner)
}

// OwnerIdentity selects the authenticated person or an unverified noreply login.
func OwnerIdentity(ctx context.Context, root string, runner toolrun.Runner) (string, bool) {
	out, err := githubAPI(ctx, root, runner, "user", `.login + " " + .type`)
	if err == nil && out.ExitCode == 0 {
		fields := strings.Fields(string(out.Stdout))
		if len(fields) == 2 && fields[1] == "User" && validLogin(fields[0]) {
			return fields[0], true
		}
	}
	out, err = runner.Run(ctx, toolrun.ToolCmd{Name: "git", Args: []string{"var", "GIT_AUTHOR_IDENT"}, Timeout: 5 * time.Second, WorkDir: root})
	if err == nil && out.ExitCode == 0 {
		_, rest, ok := strings.Cut(string(out.Stdout), " <")
		email, _, end := strings.Cut(rest, ">")
		local, domain, at := strings.Cut(email, "@")
		if ok && end && at && strings.EqualFold(domain, "users.noreply.github.com") {
			if id, login, plus := strings.Cut(local, "+"); plus {
				if id == "" || strings.Trim(id, "0123456789") != "" {
					return PlaceholderOwner, false
				}
				local = login
			}
			if validLogin(local) {
				return local, false
			}
		}
	}
	return PlaceholderOwner, false
}

// PersonOwner reports whether any listed owner is a GitHub user and whether
// an otherwise eligible login could not be verified.
func PersonOwner(ctx context.Context, root string, runner toolrun.Runner, owners []string) (person, unverified bool) {
	for _, owner := range owners {
		if !validLogin(owner) {
			continue
		}
		out, err := githubAPI(ctx, root, runner, "users/"+owner, ".type")
		if err != nil || out.ExitCode != 0 {
			unverified = true
			continue
		}
		switch strings.TrimSpace(string(out.Stdout)) {
		case "User":
			return true, false
		case "Organization", "Bot":
		default:
			unverified = true
		}
	}
	return false, unverified
}

func githubAPI(ctx context.Context, dir string, runner toolrun.Runner, endpoint, query string) (toolrun.Output, error) {
	return runner.Run(ctx, toolrun.ToolCmd{Name: "gh", Args: []string{"api", "--hostname", "github.com", endpoint, "--jq", query}, Timeout: 5 * time.Second, WorkDir: dir})
}
