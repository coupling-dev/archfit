package setupgit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	urlpkg "net/url"
	"strconv"
	"strings"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

// GitHub runs gh without prompts and leaves the credentials with gh.
type GitHub struct {
	Root     string
	Runner   toolrun.Runner
	BodyPath string
}

// PullRequest describes the setup pull request and its Git head.
type PullRequest struct {
	Number                 int
	State, URL, HeadRefOid string
}

// AppCheck describes one check from the Archfit App.
type AppCheck struct {
	Status, Conclusion, HTMLURL, Title string
}

// Auth reports whether the active login works and whether a known scope permits workflows.
// Missing scope information is not evidence of a missing permission.
func (g GitHub) Auth(ctx context.Context) (bool, bool) {
	out, err := g.command(ctx, "auth", "status", "--hostname", "github.com", "--active")
	if err != nil || out.ExitCode != 0 {
		return false, false
	}
	for _, data := range [][]byte{out.Stdout, out.Stderr} {
		for line := range strings.SplitSeq(string(data), "\n") {
			if scopes, ok := strings.CutPrefix(strings.TrimSpace(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))), "Token scopes:"); ok {
				for scope := range strings.SplitSeq(scopes, ",") {
					if strings.Trim(strings.TrimSpace(scope), "'\"") == "workflow" {
						return true, true
					}
				}
				return true, false
			}
		}
	}
	return true, true
}

// PullRequest finds an open setup pull request from the same repository.
func (g GitHub) PullRequest(ctx context.Context, repo, base, branch string) (PullRequest, error) {
	// gh pr list filters by branch name, not repository. Verify the head
	// repository before using the pull request of a same-named fork.
	var list []struct {
		PullRequest
		IsCrossRepository bool
	}
	err := g.read(ctx, &list, "pr", "list", "--repo", repo, "--head", branch, "--base", base,
		"--state", "all", "--json", "number,state,url,headRefOid,isCrossRepository")
	if err != nil {
		return PullRequest{}, err
	}
	for _, item := range list {
		if !item.IsCrossRepository && item.State == "OPEN" {
			return item.PullRequest, nil
		}
	}
	var head string
	for _, item := range list {
		if item.IsCrossRepository || item.State != "CLOSED" {
			continue
		}
		if head == "" {
			out, err := (NonInteractive{g.Runner}).Run(ctx, toolrun.ToolCmd{
				Name: gitBinary, WorkDir: g.Root, Args: []string{"rev-parse", "--verify", originRef + branch},
			})
			if err != nil {
				return PullRequest{}, err
			}
			if out.ExitCode != 0 {
				return PullRequest{}, gitError("rev-parse", out)
			}
			head = strings.TrimSpace(string(out.Stdout))
		}
		if item.HeadRefOid != "" && item.HeadRefOid == head {
			return item.PullRequest, nil
		}
	}
	return PullRequest{}, nil
}

// Create opens the setup pull request from an already pushed branch.
func (g GitHub) Create(ctx context.Context, repo, base, branch string) (PullRequest, error) {
	url, err := g.run(ctx, "pr", "create", "--repo", repo, "--base", base, "--head", branch,
		"--title", "Set up archfit", "--body-file", g.BodyPath)
	if err != nil {
		return PullRequest{}, err
	}
	prURL := strings.TrimSpace(url)
	parsed, err := urlpkg.Parse(prURL)
	parts := strings.Split(strings.Trim(parsedPath(parsed), "/"), "/")
	repository := strings.Split(repo, "/")
	if err != nil || parsed == nil || parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || len(parts) != 4 || len(repository) != 2 ||
		!strings.EqualFold(parts[0], repository[0]) || !strings.EqualFold(parts[1], repository[1]) || parts[2] != "pull" {
		return PullRequest{}, errors.New("gh did not return the setup pull request URL")
	}
	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return PullRequest{}, errors.New("gh did not return the setup pull request URL")
	}
	return PullRequest{Number: number, State: "OPEN", URL: prURL}, nil
}

// Check finds the named App check without accepting another app as its source.
func (g GitHub) Check(ctx context.Context, repo, head string) (AppCheck, error) {
	var data struct {
		Runs []struct {
			Status, Conclusion string
			HTMLURL            string `json:"html_url"`
			App                struct{ Slug string }
			Output             struct{ Title string }
		} `json:"check_runs"`
	}
	err := g.read(ctx, &data, "api", "--hostname", "github.com", "repos/"+repo+"/commits/"+head+"/check-runs?check_name=archfit%2Fgate")
	if err != nil {
		return AppCheck{}, err
	}
	for _, c := range data.Runs {
		if c.App.Slug == "archfit-review" {
			return AppCheck{Status: c.Status, Conclusion: c.Conclusion, HTMLURL: c.HTMLURL, Title: c.Output.Title}, nil
		}
	}
	return AppCheck{}, nil
}

// RerunLink finds the workflow run for a user to re-run; it never starts a run.
func (g GitHub) RerunLink(ctx context.Context, repo, head string) string {
	var runs []struct{ URL string }
	err := g.read(ctx, &runs, "run", "list", "--repo", repo, "--workflow", "archfit", "--commit", head,
		"--event", "pull_request", "--limit", "1", "--json", "url")
	if err == nil && len(runs) != 0 {
		return runs[0].URL
	}
	return ""
}

// InstallTarget looks up the numeric IDs used by GitHub's install form.
func (g GitHub) InstallTarget(ctx context.Context, repo string) (owner, repository uint64) {
	data, err := g.run(ctx, "api", "--hostname", "github.com", "repos/"+repo, "--jq", ".owner.id, .id")
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(data)
	if len(fields) != 2 {
		return 0, 0
	}
	owner, ownerErr := strconv.ParseUint(fields[0], 10, 64)
	repository, repoErr := strconv.ParseUint(fields[1], 10, 64)
	if ownerErr != nil || repoErr != nil {
		return 0, 0
	}
	return owner, repository
}

// IsFork detects repositories whose pull requests may not receive the expected token.
func (g GitHub) IsFork(ctx context.Context, repo string) bool {
	var data struct{ IsFork bool }
	return g.read(ctx, &data, "repo", "view", repo, "--json", "isFork") == nil && data.IsFork
}

func (g GitHub) read(ctx context.Context, value any, args ...string) error {
	data, err := g.run(ctx, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(data), value); err != nil {
		return fmt.Errorf("gh %s returned invalid JSON: %w", args[0], err)
	}
	return nil
}

func (g GitHub) run(ctx context.Context, args ...string) (string, error) {
	out, err := g.command(ctx, args...)
	if err != nil {
		return "", err
	}
	if out.ExitCode != 0 {
		detail, _, _ := strings.Cut(strings.TrimSpace(string(out.Stderr)), "\n")
		return "", fmt.Errorf("gh %s exited %d: %s", args[0], out.ExitCode, detail)
	}
	return string(out.Stdout), nil
}

func (g GitHub) command(ctx context.Context, args ...string) (toolrun.Output, error) {
	return g.Runner.Run(ctx, toolrun.ToolCmd{
		Name: "gh", WorkDir: g.Root, Args: args, Timeout: gitTimeout,
		Env: []string{"GH_PROMPT_DISABLED=1", "GH_PAGER=cat", "NO_COLOR=1"},
	})
}

func parsedPath(u *urlpkg.URL) string {
	if u == nil {
		return ""
	}
	return u.Path
}
