package requestsend

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

const (
	pullsTimeout = 5 * time.Second
	// pullsLimit is how many of the latest merged pull requests are listed.
	pullsLimit = "200"
)

// Pull is a merged pull request.
type Pull struct {
	Number int
	Title  string
}

// MergedPulls returns the latest merged pull requests of the repository in
// dir, by the commit that merged each. The names are only labels: gh missing,
// not signed in, offline or slow all give an empty map, never an error, so a
// caller's result does not depend on them.
// It runs: gh pr list --state merged --limit 200 --json number,title,mergeCommit
func MergedPulls(ctx context.Context, dir string, runner toolrun.Runner) map[string]Pull {
	out, err := runner.Run(ctx, toolrun.ToolCmd{
		Name:    "gh",
		Args:    []string{"pr", "list", "--state", "merged", "--limit", pullsLimit, "--json", "number,title,mergeCommit"},
		WorkDir: dir,
		Timeout: pullsTimeout,
	})
	if err != nil || out.ExitCode != 0 {
		return nil
	}
	var list []struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		MergeCommit *struct {
			Oid string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if json.Unmarshal(out.Stdout, &list) != nil {
		return nil
	}
	pulls := make(map[string]Pull, len(list))
	for _, p := range list {
		if p.MergeCommit != nil && p.MergeCommit.Oid != "" {
			pulls[p.MergeCommit.Oid] = Pull{Number: p.Number, Title: p.Title}
		}
	}
	return pulls
}
