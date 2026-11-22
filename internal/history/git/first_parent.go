package git

import (
	"context"
	"fmt"
	"strings"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

// Commit is one commit of a first-parent history.
type Commit struct {
	SHA string
	// Parents are the parents this clone holds: empty for the first commit of
	// a repository and for the oldest commit of a shallow clone.
	Parents []string
	Subject string
}

// FirstParent returns up to n commits of the first-parent history of ref,
// newest first. It reads this clone only and does not fetch.
// It runs: git log --first-parent -n <n> --format=%H%x00%P%x00%s <ref> --
func FirstParent(ctx context.Context, dir, ref string, n int, runner toolrun.Runner) ([]Commit, error) {
	if err := checkRef("ref", ref); err != nil {
		return nil, err
	}
	out, err := run(ctx, dir, runner, "log", "--first-parent", fmt.Sprintf("-n%d", n), "--format=%H%x00%P%x00%s", ref, "--")
	if err != nil {
		return nil, err
	}
	if out.ExitCode != 0 {
		return nil, fmt.Errorf("git log %s exited %d: %s", ref, out.ExitCode, strings.TrimSpace(string(out.Stderr)))
	}
	var commits []Commit
	for line := range strings.SplitSeq(strings.TrimRight(string(out.Stdout), "\n"), "\n") {
		if line == "" {
			continue
		}
		sha, rest, _ := strings.Cut(line, "\x00")
		parents, subject, _ := strings.Cut(rest, "\x00")
		commits = append(commits, Commit{SHA: sha, Parents: strings.Fields(parents), Subject: subject})
	}
	return commits, nil
}

// Shallow reports whether dir is a shallow clone: its history ends before the
// first commit.
// It runs: git rev-parse --is-shallow-repository
func Shallow(ctx context.Context, dir string, runner toolrun.Runner) (bool, error) {
	out, err := runGitRevParse(ctx, dir, runner, []string{"--is-shallow-repository"}, "rev-parse --is-shallow-repository")
	return out == "true", err
}
