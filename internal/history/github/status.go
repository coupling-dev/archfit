package requestsend

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

// StoryText reads the latest gate check for the branch's pull request.
func StoryText(ctx context.Context, dir string, runner toolrun.Runner) (string, string) {
	run := func(args ...string) ([]byte, bool) {
		out, err := runner.Run(ctx, toolrun.ToolCmd{Name: "gh", Args: args, WorkDir: dir, Timeout: 10 * time.Second})
		return out.Stdout, err == nil && out.ExitCode == 0
	}
	raw, ok := run("pr", "view", "--json", "number,headRefOid")
	if !ok {
		return "", "gh not ready"
	}
	var pr struct {
		Number int    `json:"number"`
		Head   string `json:"headRefOid"`
	}
	if json.Unmarshal(raw, &pr) != nil {
		return "", "gh not ready"
	}
	if pr.Number == 0 || pr.Head == "" {
		return "", "no PR"
	}
	raw, ok = run("api", "repos/{owner}/{repo}/commits/"+pr.Head+"/check-runs?check_name=archfit/gate")
	if !ok {
		return "", "gh not ready"
	}
	var checks struct {
		Runs []struct {
			StartedAt time.Time `json:"started_at"`
			Output    struct {
				Text string `json:"text"`
			} `json:"output"`
		} `json:"check_runs"`
	}
	if json.Unmarshal(raw, &checks) != nil {
		return "", "gh not ready"
	}
	var latest time.Time
	var text string
	for _, check := range checks.Runs {
		if latest.IsZero() || check.StartedAt.After(latest) {
			latest = check.StartedAt
			text = check.Output.Text
		}
	}
	return text, ""
}
