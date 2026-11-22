package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

const (
	cmdReplay     = "replay"
	tcMayUseUtil  = "  pkg_tc:\n    paths: [pkg/tc/**]\n    volatility: high\n    depends_on: [pkg_util]\n"
	tcNoDeps      = "  pkg_tc:\n    paths: [pkg/tc/**]\n    volatility: high\n"
	quietEdit     = "package runtime\n\n// Do runs, and says so.\nfunc Do() {}\n"
	replayThreeOf = "replay: 3 merged pull requests read: 2 passed, 1 would have needed an owner, 0 to look at first\n"
)

// replayRepo commits the base on main, then three changes on main: a quiet
// edit, a new dependency of pkg/tc on pkg/util, and another quiet edit. The
// policy allows no dependency of tc on util. origin/HEAD names main.
func replayRepo(t *testing.T) string {
	t.Helper()
	requireGitBinary(t)
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "main")
	writeFiles(t, dir, fixtureFiles())
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-qm", "base", "--no-gpg-sign")
	commitChange(t, dir, "quiet edit", map[string]string{runtimeGo: quietEdit})
	commitChange(t, dir, "tc uses util", map[string]string{tcNetem: tcImportsUtil})
	commitChange(t, dir, "another quiet edit", map[string]string{runtimeGo: quietEdit + "\n// more\n"})
	pointOriginAt(t, dir, "main")
	return dir
}

func commitChange(t *testing.T, dir, message string, files map[string]string) {
	t.Helper()
	writeFiles(t, dir, files)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-qm", message, "--no-gpg-sign")
}

func pointOriginAt(t *testing.T, dir, branch string) {
	t.Helper()
	gitRun(t, dir, "update-ref", "refs/remotes/origin/"+branch, branch)
	gitRun(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/"+branch)
}

func gitShow(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // the test's own git commands
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// repoState is what a replay must leave as it was.
type repoState struct{ worktrees, status, last string }

func stateOf(t *testing.T, dir string) repoState {
	t.Helper()
	last, _ := os.ReadFile(filepath.Join(dir, lastFile)) //nolint:gosec // the test's own file
	return repoState{
		worktrees: gitShow(t, dir, "worktree", "list", "--porcelain"),
		status:    sourceStatus(t, dir),
		last:      string(last),
	}
}

// sourceStatus is git status without archfit's own cache directory, which
// the other commands that check out a commit create as well.
func sourceStatus(t *testing.T, dir string) string {
	t.Helper()
	var keep []string
	for _, line := range strings.Split(gitShow(t, dir, "status", "--porcelain"), "\n") {
		if !strings.Contains(line, cacheDirName) {
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}

func TestReplayCountsEveryChangeAndLeavesTheRepositoryAsItWas(t *testing.T) {
	dir := replayRepo(t)
	before := stateOf(t, dir)
	got := runCLI(t, fixtureDate, cmdReplay, "--last", "3", "-C", dir)
	if got.code != 0 {
		t.Fatalf("exit %d\n%s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.HasPrefix(got.stdout, replayThreeOf) {
		t.Errorf("stdout:\n%s\nwant it to start with %q", got.stdout, replayThreeOf)
	}
	for _, want := range []string{
		"source: first-parent history of origin/main",
		"a pull request merged by rebase shows as several changes",
		"would have needed an owner, for example:",
		"  " + shortOf(t, dir, "HEAD~1") + " tc uses util:",
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, got.stdout)
		}
	}
	if after := stateOf(t, dir); after != before {
		t.Errorf("the repository changed:\nbefore %+v\nafter  %+v", before, after)
	}
}

func shortOf(t *testing.T, dir, rev string) string {
	t.Helper()
	return strings.TrimSpace(gitShow(t, dir, "rev-parse", "--short=7", rev))
}

func TestReplayReadsOnlyTheLastN(t *testing.T) {
	dir := replayRepo(t)
	got := runCLI(t, fixtureDate, cmdReplay, "--last", "1", "-C", dir)
	if want := "replay: 1 merged pull requests read: 1 passed, 0 would have needed an owner, 0 to look at first\n"; !strings.HasPrefix(got.stdout, want) {
		t.Errorf("stdout:\n%s\nwant it to start with %q", got.stdout, want)
	}
}

func TestReplayJudgesUnderTheCurrentPolicyNotTheOneTheChangeHad(t *testing.T) {
	dir := replayRepo(t)
	// An edit of the working tree that is not committed: tc may use util.
	policy := strings.Replace(fixturePolicy, tcNoDeps, tcMayUseUtil, 1)
	writeFiles(t, dir, map[string]string{defaultConfigPath: policy})
	before := stateOf(t, dir)
	got := runCLI(t, fixtureDate, cmdReplay, "--last", "3", "-C", dir)
	if want := "replay: 3 merged pull requests read: 3 passed, 0 would have needed an owner, 0 to look at first\n"; !strings.HasPrefix(got.stdout, want) {
		t.Errorf("stdout:\n%s%s\nwant it to start with %q", got.stdout, got.stderr, want)
	}
	if after := stateOf(t, dir); after != before {
		t.Errorf("a dirty working tree changed:\nbefore %+v\nafter  %+v", before, after)
	}
}

func TestReplayOfAMergeCommitIsOneChangeAndHasNoRebaseNote(t *testing.T) {
	requireGitBinary(t)
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "main")
	writeFiles(t, dir, fixtureFiles())
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-qm", "base", "--no-gpg-sign")
	commitChange(t, dir, "before the branch", map[string]string{runtimeGo: quietEdit})
	gitRun(t, dir, "checkout", "-q", "-b", "feature")
	commitChange(t, dir, "step one", map[string]string{runtimeGo: quietEdit + "\n// step\n"})
	commitChange(t, dir, "step two", map[string]string{tcNetem: tcImportsUtil})
	gitRun(t, dir, "checkout", "-q", "main")
	gitRun(t, dir, "merge", "-q", "--no-ff", "-m", "Merge feature", "feature", "--no-gpg-sign")
	pointOriginAt(t, dir, "main")

	got := runCLI(t, fixtureDate, cmdReplay, "--last", "2", "-C", dir)
	want := "replay: 2 merged pull requests read: 1 passed, 1 would have needed an owner, 0 to look at first\n"
	if !strings.HasPrefix(got.stdout, want) {
		t.Errorf("stdout:\n%s%s\nwant it to start with %q", got.stdout, got.stderr, want)
	}
	if strings.Contains(got.stdout, "merged by rebase") {
		t.Errorf("a history with a merge commit has a rebase note:\n%s", got.stdout)
	}
}

func TestReplayOfAShallowCloneSaysTheHistoryIsIncomplete(t *testing.T) {
	src := replayRepo(t)
	clone := filepath.Join(t.TempDir(), "clone")
	gitRun(t, src, "clone", "-q", "--depth", "2", "file://"+src, clone)
	got := runCLI(t, fixtureDate, cmdReplay, "--last", "5", "-C", clone)
	if got.code != 0 {
		t.Fatalf("exit %d\n%s%s", got.code, got.stdout, got.stderr)
	}
	for _, want := range []string{
		// The newest change passes; the oldest one has no parent here.
		"replay: 2 merged pull requests read: 1 passed, 0 would have needed an owner, 1 to look at first\n",
		"incomplete history: this clone is shallow",
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, got.stdout)
		}
	}
}

// ghStub answers `gh pr list` with a list and runs everything else for real.
type ghStub struct {
	toolrun.Runner
	list string
	// onWorktreeAdd runs before each `git worktree add`, with its number.
	onWorktreeAdd func(n int)
	mu            sync.Mutex
	adds          int
}

func (r *ghStub) Run(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
	switch {
	case cmd.Name == "gh":
		if r.list == "" {
			return toolrun.Output{ExitCode: 4}, nil
		}
		return toolrun.Output{Stdout: []byte(r.list)}, nil
	case cmd.Name == harnessGit && len(cmd.Args) > 1 && cmd.Args[0] == "worktree" && cmd.Args[1] == "add":
		r.mu.Lock()
		r.adds++
		n := r.adds
		r.mu.Unlock()
		if r.onWorktreeAdd != nil {
			r.onWorktreeAdd(n)
		}
	}
	return r.Runner.Run(ctx, cmd)
}

func replayWith(t *testing.T, runner toolrun.Runner, args ...string) cliRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runDeps(args, &appDeps{Runner: runner, Stdout: &stdout, Stderr: &stderr, now: day(fixtureDate)})
	return cliRun{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestReplayLabelsChangesWithGhAndCountsTheSameWithoutIt(t *testing.T) {
	dir := replayRepo(t)
	sha := strings.TrimSpace(gitShow(t, dir, "rev-parse", "HEAD~1"))
	list := `[{"number":12,"title":"Add the tc shaper","mergeCommit":{"oid":"` + sha + `"}}]`

	with := replayWith(t, &ghStub{Runner: toolrun.New(), list: list}, cmdReplay, "--last", "3", "-C", dir)
	without := replayWith(t, &ghStub{Runner: toolrun.New()}, cmdReplay, "--last", "3", "-C", dir)

	if !strings.Contains(with.stdout, "  #12 Add the tc shaper: ") {
		t.Errorf("with gh, the example has no pull request label:\n%s", with.stdout)
	}
	if !strings.HasPrefix(with.stdout, replayThreeOf) || !strings.HasPrefix(without.stdout, replayThreeOf) {
		t.Errorf("the counts differ:\nwith gh:\n%s\nwithout:\n%s", with.stdout, without.stdout)
	}
}

// stopAtSecondCheckout is a runner that ends cancel as the second change is
// checked out.
func stopAtSecondCheckout(cancel context.CancelFunc) *ghStub {
	return &ghStub{Runner: toolrun.New(), onWorktreeAdd: func(n int) {
		if n == 2 {
			cancel()
		}
	}}
}

func assertNothingLeft(t *testing.T, dir string, before repoState) {
	t.Helper()
	if after := stateOf(t, dir); after != before {
		t.Errorf("a stopped replay left the repository changed:\nbefore %+v\nafter  %+v", before, after)
	}
	// The lock files of internal/history/git stay, as after every --base run.
	entries, _ := os.ReadDir(filepath.Join(dir, cacheDirName, "worktrees"))
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("worktree directory left behind: %s", e.Name())
		}
	}
}

// The deadline of `archfit setup` (plan C5): the service stops and returns
// what it has read.
func TestReplayServiceStopsAtTheDeadlineWithAPartialCount(t *testing.T) {
	dir := replayRepo(t)
	before := stateOf(t, dir)
	ctx, deadline := context.WithCancel(context.Background())
	defer deadline()
	deps := &appDeps{Runner: stopAtSecondCheckout(deadline), Stdout: io.Discard, Stderr: io.Discard, now: day(fixtureDate)}
	co, err := openCheckout(ctx, deps, dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := co.replay(deps).Run(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Stopped || res.Read != 1 || res.Total != 3 || res.Passed != 1 {
		t.Errorf("got %+v, want stopped with 1 of 3 read and passed", res)
	}
	assertNothingLeft(t, dir, before)
}

func TestReplayInterruptedExitsWithNoVerdictAndLeavesNothingBehind(t *testing.T) {
	dir := replayRepo(t)
	before := stateOf(t, dir)
	ctx, interrupt := context.WithCancel(context.Background())
	defer interrupt()
	var stdout, stderr bytes.Buffer
	code := runDeps([]string{cmdReplay, "--last", "3", "-C", dir},
		&appDeps{ctx: ctx, Runner: stopAtSecondCheckout(interrupt), Stdout: &stdout, Stderr: &stderr, now: day(fixtureDate)})
	if code != 2 || !strings.HasPrefix(stdout.String(), "replay: stopped, 1 of 3 read\n") {
		t.Errorf("exit %d, stdout:\n%s%s", code, stdout.String(), stderr.String())
	}
	assertNothingLeft(t, dir, before)
}

func TestReplayErrors(t *testing.T) {
	t.Run("last below 1", func(t *testing.T) {
		got := runCLI(t, fixtureDate, cmdReplay, "--last", "0", "-C", replayRepo(t))
		if got.code != 2 || !strings.Contains(got.stderr, "--last must be at least 1") {
			t.Errorf("exit %d, stderr %q", got.code, got.stderr)
		}
	})
	t.Run("no policy", func(t *testing.T) {
		dir := replayRepo(t)
		if err := os.Remove(filepath.Join(dir, defaultConfigPath)); err != nil {
			t.Fatal(err)
		}
		got := runCLI(t, fixtureDate, cmdReplay, "-C", dir)
		if got.code != 2 || !strings.Contains(got.stderr, "run archfit init") {
			t.Errorf("exit %d, stderr %q", got.code, got.stderr)
		}
	})
	t.Run("no default branch", func(t *testing.T) {
		dir := replayRepo(t)
		gitRun(t, dir, "symbolic-ref", "--delete", "refs/remotes/origin/HEAD")
		got := runCLI(t, fixtureDate, cmdReplay, "-C", dir)
		if got.code != 2 || !strings.Contains(got.stderr, "origin/HEAD is not set") {
			t.Errorf("exit %d, stderr %q", got.code, got.stderr)
		}
	})
	t.Run("not a repository", func(t *testing.T) {
		got := runCLI(t, fixtureDate, cmdReplay, "-C", t.TempDir())
		if got.code != 2 || !strings.Contains(got.stderr, notARepo) {
			t.Errorf("exit %d, stderr %q", got.code, got.stderr)
		}
	})
}
