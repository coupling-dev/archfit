package requestsend

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

// StoryText reads the latest gate check of the App appSlug for the branch's
// pull request. An empty appSlug trusts no App.
func StoryText(ctx context.Context, dir string, runner toolrun.Runner, appSlug string) (string, string) {
	const notReady = "gh not ready"
	run := func(args ...string) ([]byte, bool) {
		out, err := runner.Run(ctx, toolrun.ToolCmd{Name: "gh", Args: args, WorkDir: dir, Timeout: 10 * time.Second})
		return out.Stdout, err == nil && out.ExitCode == 0
	}
	raw, ok := run("pr", "view", "--json", "number,headRefOid")
	if !ok {
		return "", notReady
	}
	var pr struct {
		Number int    `json:"number"`
		Head   string `json:"headRefOid"`
	}
	if json.Unmarshal(raw, &pr) != nil {
		return "", notReady
	}
	if pr.Number == 0 || pr.Head == "" {
		return "", "no PR"
	}
	raw, ok = run("api", "repos/{owner}/{repo}/commits/"+pr.Head+"/check-runs?check_name=archfit/gate")
	if !ok {
		return "", notReady
	}
	var checks struct {
		Runs []struct {
			StartedAt time.Time `json:"started_at"`
			App       struct {
				Slug string `json:"slug"`
			} `json:"app"`
			Output struct {
				Text string `json:"text"`
			} `json:"output"`
		} `json:"check_runs"`
	}
	if json.Unmarshal(raw, &checks) != nil {
		return "", notReady
	}
	var latest time.Time
	var text string
	for _, check := range checks.Runs {
		if appSlug != "" && check.App.Slug == appSlug && (latest.IsZero() || check.StartedAt.After(latest)) {
			latest = check.StartedAt
			text = check.Output.Text
		}
	}
	return text, ""
}
