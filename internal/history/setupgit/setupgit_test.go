package setupgit

import (
	"context"
	"errors"

	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

const (
	tag       = "v0.10.0"
	ref       = "refs/tags/" + tag
	peeled    = ref + "^{}"
	tagSHA    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	commitSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	repo      = "o/r"
)

func TestPinUsesOnlyExactRefs(t *testing.T) {
	for _, tc := range []struct {
		name, stdout, want, errorText string
		exitCode                      int
		runError                      error
	}{
		{name: "annotated", stdout: commitSHA + "\t" + peeled, want: commitSHA},
		{name: "lightweight", stdout: tagSHA + "\t" + ref, want: tagSHA},
		{name: "peeled wins", stdout: commitSHA + "\t" + peeled + "\n" + tagSHA + "\t" + ref, want: commitSHA},
		{name: "suffix ignored", stdout: tagSHA + "\trefs/tags/x/" + tag, errorText: "no Action tag"},
		{name: "missing", errorText: "no Action tag"},
		{name: "malformed peeled", stdout: tagSHA + "\t" + ref + "\n123\t" + peeled, errorText: "no valid commit SHA"},
		{name: "git error", exitCode: 128, errorText: "git ls-remote exited 128: network down"},
		{name: "timeout", runError: context.DeadlineExceeded, errorText: "context deadline exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := Git{ActionRepository: "coupling-dev/archfit-action", Runner: &toolrun.RunnerMock{
				RunFunc: func(_ context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
					expected := []string{"ls-remote", "https://github.com/coupling-dev/archfit-action", ref, peeled}
					if cmd.Name != "git" || cmd.Timeout != 15*time.Second || !slices.Contains(cmd.Env, "GIT_TERMINAL_PROMPT=0") || !slices.Equal(cmd.Args, expected) {
						t.Errorf("pin command: %+v", cmd)
					}
					return toolrun.Output{ExitCode: tc.exitCode, Stdout: []byte(tc.stdout), Stderr: []byte("network down\nextra detail\n")}, tc.runError
				},
			}}
			pin, err := g.Pin(t.Context(), tag)
			if pin != tc.want || (tc.errorText == "" && err != nil) || (tc.errorText != "" && (err == nil || !strings.Contains(err.Error(), tc.errorText))) {
				t.Errorf("pin %q, error %v; want %q, error %q", pin, err, tc.want, tc.errorText)
			}
		})
	}
}

func TestAuthChecksWorkflowScope(t *testing.T) {
	for _, tc := range []struct {
		name, output    string
		exit            int
		ready, workflow bool
	}{
		{"valid", "  - Token scopes: 'repo', 'workflow'\n", 0, true, true},
		{"unknown scopes", "  - Token: fine-grained\n", 0, true, true},
		{"missing", "  - Token scopes: 'repo', 'not-workflow'\n", 0, true, false},
		{"account not signed in", "  - Token scopes: 'workflow'\n", 1, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gh := mockGH(t, "auth status --hostname github.com --active", "", tc.output, tc.exit)
			ready, workflow := gh.Auth(t.Context())
			if ready != tc.ready || workflow != tc.workflow {
				t.Errorf("got %t, %t; want %t, %t", ready, workflow, tc.ready, tc.workflow)
			}
		})
	}
}

func TestPullRequestChoosesOpenSameRepository(t *testing.T) {
	gh := mockGH(t, "pr list --repo o/r --head archfit/setup --base main --state all --json number,state,url,headRefOid,isCrossRepository", `[
		{"number":11,"state":"OPEN","isCrossRepository":true},
		{"number":4,"state":"CLOSED","isCrossRepository":false},
		{"number":7,"state":"OPEN","isCrossRepository":false,"url":"https://github.com/o/r/pull/7","headRefOid":"abc"}
	]`, "", 0)
	got, err := gh.PullRequest(t.Context(), repo, "main", "archfit/setup")
	want := PullRequest{Number: 7, State: "OPEN", URL: "https://github.com/o/r/pull/7", HeadRefOid: "abc"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("pull request is %+v, %v; want %+v", got, err, want)
	}
}

func TestAppCheckIgnoresOtherApps(t *testing.T) {
	for _, tc := range []struct {
		name, second string
		want         AppCheck
	}{
		{"none", "", AppCheck{}},
		{"matched", `, {"app":{"slug":"archfit-review"},"status":"completed","conclusion":"neutral","html_url":"https://github.com/o/r/runs/7","output":{"title":"merge to start"}}`,
			AppCheck{Status: "completed", Conclusion: "neutral", HTMLURL: "https://github.com/o/r/runs/7", Title: "merge to start"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gh := mockGH(t, "api --hostname github.com repos/o/r/commits/abc/check-runs?check_name=archfit%2Fgate", `{
				"check_runs":[{"app":{"slug":"someone-else"},"status":"completed","conclusion":"success"}`+tc.second+`]
			}`, "", 0)
			got, err := gh.Check(t.Context(), repo, "abc")
			if err != nil || got != tc.want {
				t.Errorf("App check %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

func mockGH(t *testing.T, args, stdout, stderr string, exit int) GitHub {
	t.Helper()
	return GitHub{Runner: &toolrun.RunnerMock{RunFunc: func(_ context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if cmd.Name != "gh" || strings.Join(cmd.Args, " ") != args || !slices.Contains(cmd.Env, "GH_PROMPT_DISABLED=1") {
			t.Errorf("gh command: %+v", cmd)
		}
		return toolrun.Output{ExitCode: exit, Stdout: []byte(stdout), Stderr: []byte(stderr)}, nil
	}}}
}

func TestInstallTarget(t *testing.T) {
	for _, tc := range []struct {
		name, stdout string
		status       int
		owner, repo  uint64
	}{
		{"prefill", "123\n456\n", 0, 123, 456},
		{"not signed in", "", 1, 0, 0},
		{"malformed", "123\ntext\n", 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gh := mockGH(t, "api --hostname github.com repos/o/r --jq .owner.id, .id", tc.stdout, "", tc.status)
			owner, repository := gh.InstallTarget(t.Context(), repo)
			if owner != tc.owner || repository != tc.repo {
				t.Errorf("got %d/%d, want %d/%d", owner, repository, tc.owner, tc.repo)
			}
		})
	}
}

func TestCommitKeepsUnrelatedChangesStaged(t *testing.T) {
	const policy = ".archfit.yaml"
	root := t.TempDir()
	runner := toolrun.New()
	g := Git{Root: root, Runner: runner}
	run := func(args ...string) string {
		t.Helper()
		out, err := g.run(t.Context(), args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	write := func(path, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "--initial-branch=main")
	run("config", "user.name", "Setup Author")
	run("config", "user.email", "author@example.invalid")
	run("config", "commit.gpgsign", "false")
	run("config", "core.hooksPath", ".git/hooks")
	run("commit", "--allow-empty", "-m", "base")
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	write(policy, "version: 1\n")
	write("not-setup", "leave staged\n")
	run("add", "not-setup")
	paths, err := g.Pending(t.Context(), []string{policy, "AGENTS.md"})
	if err != nil || !slices.Equal(paths, []string{policy}) {
		t.Fatalf("pending %q, %v", paths, err)
	}
	if err := g.Switch(t.Context(), "archfit/setup", "main"); err != nil {
		t.Fatal(err)
	}
	if err := g.Commit(t.Context(), paths); err != nil {
		t.Fatal(err)
	}
	if got := run("diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD"); got != policy {
		t.Errorf("commit contains %q", got)
	}
	if got := run("diff", "--cached", "--name-only"); got != "not-setup" {
		t.Errorf("staged paths: %q", got)
	}
}

func TestDefaultBranchFromRemoteSymref(t *testing.T) {
	const defaultBranch = "main"
	for _, tc := range []struct {
		name, symref, remote, want string
		symrefExit                 int
		wantError                  bool
	}{
		{"local", "refs/remotes/origin/trunk", "", "trunk", 0, false},
		{"remote wins", "refs/remotes/origin/old", "ref: refs/heads/main\tHEAD", defaultBranch, 0, false},
		{"network fallback", "", "ref: refs/heads/main\tHEAD\n" + commitSHA + "\tHEAD", defaultBranch, 1, false},
		{"no remote symref", "", commitSHA + "\tHEAD", "", 1, true},
		{"invalid", "", "ref: refs/heads/-invalid\tHEAD", "", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := Git{Runner: &toolrun.RunnerMock{RunFunc: func(_ context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
				if !slices.Contains(cmd.Env, "GIT_TERMINAL_PROMPT=0") {
					t.Error("git prompt not disabled")
				}
				switch cmd.Args[0] {
				case "symbolic-ref":
					return toolrun.Output{Stdout: []byte(tc.symref), ExitCode: tc.symrefExit}, nil
				case "ls-remote":
					return toolrun.Output{Stdout: []byte(tc.remote)}, nil
				case "check-ref-format":
					if strings.HasPrefix(cmd.Args[2], "-") {
						return toolrun.Output{ExitCode: 1}, nil
					}
					return toolrun.Output{Stdout: []byte(cmd.Args[2])}, nil
				default:
					t.Fatalf("unexpected command %v", cmd.Args)
					return toolrun.Output{}, nil
				}
			}}}
			got, _, err := g.Default(t.Context())
			if got != tc.want || (err != nil) != tc.wantError {
				t.Errorf("got %q, %v; want %q, error %v", got, err, tc.want, tc.wantError)
			}
		})
	}
}

func TestSwitchRefusesStaleDefaultBranch(t *testing.T) {
	root := t.TempDir()
	g := Git{Root: root, Runner: toolrun.New()}
	run := func(args ...string) {
		t.Helper()
		if _, err := g.run(t.Context(), args...); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "--initial-branch=main")
	run("config", "user.name", "Setup Author")
	run("config", "user.email", "author@example.invalid")
	run("config", "commit.gpgsign", "false")
	run("commit", "--allow-empty", "-m", "one")
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	run("commit", "--allow-empty", "-m", "two")
	if err := g.Switch(t.Context(), "archfit/setup", "main"); err == nil {
		t.Fatal("created setup branch from unpublished default commit")
	}
	branch, err := g.Branch(t.Context())
	if err != nil || branch != "main" {
		t.Fatalf("branch %q, %v", branch, err)
	}
}

func TestMergedReadsDefaultBranchContent(t *testing.T) {
	root := t.TempDir()
	g := Git{
		Root: root, Runner: toolrun.New(), PolicyPath: ".archfit.yaml",
		WorkflowsDir: ".github/workflows",
		Version1:     func(data []byte) bool { return strings.TrimSpace(string(data)) == "version: 1" },
		UsesAction:   func(data []byte) bool { return strings.Contains(string(data), "coupling-dev/archfit-action@") },
	}
	run := func(args ...string) {
		t.Helper()
		if _, err := g.run(t.Context(), args...); err != nil {
			t.Fatal(err)
		}
	}
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "--initial-branch=main")
	run("config", "user.name", "Setup Author")
	run("config", "user.email", "author@example.invalid")
	run("config", "commit.gpgsign", "false")
	run("commit", "--allow-empty", "-m", "initial")
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	assertMerged := func(want bool) {
		t.Helper()
		got, err := g.Merged(t.Context(), "main")
		if err != nil || got != want {
			t.Fatalf("merged %t, error %v; want %t", got, err, want)
		}
	}
	assertMerged(false)
	write(".archfit.yaml", "version: 1\n")
	write(".github/workflows/other.yaml", "- uses: coupling-dev/archfit-action@abcd\n")
	run("add", "-A")
	run("commit", "-m", "onboard")
	assertMerged(false)
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	assertMerged(true)
	write(".archfit.yaml", "version: 2\n")
	run("add", "-A")
	run("commit", "-m", "invalid policy")
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	assertMerged(false)
}

func TestPendingDetectsNetChangesAndIgnoresOtherFiles(t *testing.T) {
	const tracked = "tracked"
	const deleted = "deleted"
	const ignored = "ignored"
	const newFile = "new"
	root := t.TempDir()
	g := Git{Root: root, Runner: toolrun.New()}
	run := func(args ...string) {
		t.Helper()
		if _, err := g.run(t.Context(), args...); err != nil {
			t.Fatal(err)
		}
	}
	write := func(name, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "--initial-branch=main")
	run("config", "user.name", "Setup Author")
	run("config", "user.email", "author@example.invalid")
	run("config", "commit.gpgsign", "false")
	write(tracked, "before")
	write(deleted, "before")
	write(".gitignore", "ignored\n")
	run("add", "-A")
	run("commit", "-m", "base")
	write(tracked, "staged")
	run("add", tracked)
	write(tracked, "unstaged")
	if err := os.Remove(filepath.Join(root, deleted)); err != nil {
		t.Fatal(err)
	}
	write(newFile, "untracked")
	write(ignored, ignored)
	write("unrelated", "not a setup path")
	got, err := g.Pending(t.Context(), []string{tracked, deleted, newFile, ignored, "absent"})
	want := []string{tracked, deleted, newFile, ignored}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("got %v, %v; want %v", got, err, want)
	}
	write(tracked, "before")
	got, err = g.Pending(t.Context(), []string{tracked})
	if err != nil || len(got) != 0 {
		t.Fatalf("net-unchanged path %v, %v", got, err)
	}
}

func TestPushSafetyRejectsMergeAndHandlesUnusualPaths(t *testing.T) {
	for _, tc := range []struct {
		name         string
		merge, allow bool
	}{
		{"allowed unicode path", false, true},
		{"unrelated unicode path", false, false},
		{"merge commit", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			g := Git{Root: root, Runner: toolrun.New()}
			run := func(args ...string) {
				t.Helper()
				if _, err := g.run(t.Context(), args...); err != nil {
					t.Fatal(err)
				}
			}
			run("init", "--initial-branch=main")
			run("config", "user.name", "Setup Author")
			run("config", "user.email", "author@example.invalid")
			run("config", "commit.gpgsign", "false")
			run("remote", "add", "origin", "https://github.com/o/r.git")
			run("commit", "--allow-empty", "-m", "base")
			run("update-ref", "refs/remotes/origin/main", "HEAD")
			run("switch", "-c", "archfit/setup")
			const unusual = "setup café.yaml"
			if err := os.WriteFile(filepath.Join(root, unusual), []byte("data"), 0o600); err != nil {
				t.Fatal(err)
			}
			run("add", unusual)
			run("commit", "-m", "chore: set up archfit")
			if tc.merge {
				run("switch", "-c", "other", "main")
				run("commit", "--allow-empty", "-m", "other")
				run("switch", "archfit/setup")
				run("merge", "--no-ff", "-m", "chore: set up archfit", "other")
			}
			paths := []string{".archfit.yaml"}
			if tc.allow {
				paths = append(paths, unusual)
			}
			https, err := g.PushSafety(t.Context(), "main", paths)
			if (err == nil) != (tc.allow && !tc.merge) || (err == nil && !https) {
				t.Fatalf("https=%t error=%v", https, err)
			}
		})
	}
}

func TestPushSafetyChecksAllPushURLs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		urls  []string
		want  error
		https bool
	}{
		{"one https", []string{"https://github.com/o/r.git"}, nil, true},
		{"one ssh", []string{"git@github.com:o/r.git"}, nil, false},
		{"two", []string{"https://github.com/o/r.git", "https://github.com/other/r.git"}, destinationError("origin has more than one push URL"), false},
		{"another repository", []string{"https://github.com/other/r.git"}, destinationError("origin pushes to another repository"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			g := Git{Root: root, Runner: toolrun.New()}
			run := func(args ...string) {
				t.Helper()
				if _, err := g.run(t.Context(), args...); err != nil {
					t.Fatal(err)
				}
			}
			run("init", "--initial-branch=main")
			run("config", "user.name", "Setup Author")
			run("config", "user.email", "author@example.invalid")
			run("config", "commit.gpgsign", "false")
			run("remote", "add", "origin", "https://github.com/o/r.git")
			run("commit", "--allow-empty", "-m", "base")
			run("update-ref", "refs/remotes/origin/main", "HEAD")
			for _, u := range tc.urls {
				run("remote", "set-url", "--add", "--push", "origin", u)
			}
			https, err := g.PushSafety(t.Context(), "main", nil)
			if !errors.Is(err, tc.want) || https != tc.https {
				t.Fatalf("https %t error %v, want %t %v", https, err, tc.https, tc.want)
			}
		})
	}
}

func TestPushPinsDestinationAndDisablesTags(t *testing.T) {
	var called bool
	g := Git{Runner: &toolrun.RunnerMock{RunFunc: func(_ context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		called = true
		want := []string{"-c", "push.followTags=false", "push", "--no-follow-tags", "origin", "refs/heads/archfit/setup:refs/heads/archfit/setup"}
		if !slices.Equal(cmd.Args, want) || !slices.Contains(cmd.Env, "GIT_TERMINAL_PROMPT=0") {
			t.Errorf("command: %+v", cmd)
		}
		return toolrun.Output{}, nil
	}}}
	if err := g.Push(t.Context(), "archfit/setup"); err != nil || !called {
		t.Fatalf("called %v, error %v", called, err)
	}
}

func TestValidateRemoteRejectsForeignCommits(t *testing.T) {
	root := t.TempDir()
	process := toolrun.New()
	g := Git{Root: root, Runner: process}
	run := func(args ...string) string {
		t.Helper()
		out, err := g.run(t.Context(), args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	run("init", "--initial-branch=main")
	run("config", "user.name", "Setup Author")
	run("config", "user.email", "author@example.invalid")
	run("config", "commit.gpgsign", "false")
	run("commit", "--allow-empty", "-m", "base")
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	run("switch", "-c", "archfit/setup")
	run("commit", "--allow-empty", "-m", "private work")
	mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if len(cmd.Args) > 0 && cmd.Args[0] == "fetch" {
			return process.Run(ctx, toolrun.ToolCmd{Name: gitBinary, WorkDir: root, Args: []string{"update-ref", "refs/remotes/origin/archfit/setup", "HEAD"}})
		}
		return process.Run(ctx, cmd)
	}}
	g.Runner = mock
	if err := g.ValidateRemote(t.Context(), "archfit/setup", "main", nil); err == nil || !strings.Contains(err.Error(), "not made by setup") {
		t.Fatalf("foreign commit accepted: %v", err)
	}
}

func TestCommitAddsIgnoredSetupPathOnly(t *testing.T) {
	const setupPolicy = ".archfit.yaml"
	root := t.TempDir()
	g := Git{Root: root, Runner: toolrun.New()}
	run := func(args ...string) string {
		t.Helper()
		out, err := g.run(t.Context(), args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "--initial-branch=main")
	run("config", "user.name", "Setup Author")
	run("config", "user.email", "author@example.invalid")
	run("config", "commit.gpgsign", "false")
	write(".gitignore", ".archfit.yaml\nunrelated\n")
	run("add", ".gitignore")
	run("commit", "-m", "base")
	write(setupPolicy, "version: 1\n")
	write("unrelated", "private\n")
	paths, err := g.Pending(t.Context(), []string{setupPolicy})
	if err != nil || !slices.Equal(paths, []string{setupPolicy}) {
		t.Fatalf("pending: %v %v", paths, err)
	}
	if err := g.Commit(t.Context(), paths); err != nil {
		t.Fatal(err)
	}
	if got := run("diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD"); got != setupPolicy {
		t.Fatalf("committed %s", got)
	}
}

func TestPullRequestIgnoresHistoricalMerged(t *testing.T) {
	gh := mockGH(t, "pr list --repo o/r --head archfit/setup --base main --state all --json number,state,url,headRefOid,isCrossRepository",
		`[{"number":4,"state":"MERGED","url":"https://github.com/o/r/pull/4"}]`, "", 0)
	got, err := gh.PullRequest(t.Context(), repo, "main", "archfit/setup")
	if err != nil || got.Number != 0 {
		t.Fatalf("historical pull request: %+v, %v", got, err)
	}
}

func TestPullRequestReturnsClosedOnlyForCurrentHead(t *testing.T) {
	for _, tc := range []struct {
		name, head string
		want       int
	}{
		{"same head", "abc", 7},
		{"recreated branch", "new", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gh := GitHub{Runner: &toolrun.RunnerMock{RunFunc: func(_ context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
				if cmd.Name == gitBinary {
					return toolrun.Output{Stdout: []byte(tc.head + "\n")}, nil
				}
				return toolrun.Output{Stdout: []byte(`[{"number":4,"state":"MERGED"},{"number":7,"state":"CLOSED","headRefOid":"abc","url":"https://github.com/o/r/pull/7"}]`)}, nil
			}}}
			got, err := gh.PullRequest(t.Context(), repo, "main", "archfit/setup")
			if err != nil || got.Number != tc.want {
				t.Fatalf("pull request %+v, error %v", got, err)
			}
		})
	}
}
func TestPushSafetyRejectsHookHistoryWhenDisabled(t *testing.T) {
	root := t.TempDir()
	g := Git{Root: root, Runner: toolrun.New()}
	run := func(args ...string) {
		t.Helper()
		if _, err := g.run(t.Context(), args...); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "--initial-branch=main")
	run("config", "user.name", "Setup Author")
	run("config", "user.email", "author@example.invalid")
	run("config", "commit.gpgsign", "false")
	run("remote", "add", "origin", "https://github.com/o/r.git")
	run("commit", "--allow-empty", "-m", "base")
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	run("switch", "-c", "archfit/setup")
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude/settings.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".claude/settings.json")
	run("commit", "-m", "chore: set up archfit")
	_, err := g.PushSafety(t.Context(), "main", nil)
	var hooks hookHistoryError
	if !errors.As(err, &hooks) {
		t.Fatalf("expected hook history rejection: %v", err)
	}
}

func TestValidateRangePropagatesGitErrors(t *testing.T) {
	for _, fail := range []string{"rev-list --parents", "show -s"} {
		t.Run(fail, func(t *testing.T) {
			g := Git{Runner: &toolrun.RunnerMock{RunFunc: func(_ context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
				args := strings.Join(cmd.Args, " ")
				if strings.HasPrefix(args, fail) {
					return toolrun.Output{ExitCode: 128, Stderr: []byte("repository unavailable")}, nil
				}
				if strings.HasPrefix(args, "rev-list --reverse") {
					return toolrun.Output{Stdout: []byte(commitSHA + "\n")}, nil
				}
				return toolrun.Output{Stdout: []byte(commitSHA + " " + tagSHA + "\n")}, nil
			}}}
			err := g.validateRange(t.Context(), "main", "HEAD", nil)
			var branch branchError
			if err == nil || errors.As(err, &branch) || !strings.Contains(err.Error(), "repository unavailable") {
				t.Fatalf("error %v", err)
			}
		})
	}
}

func TestFetchUpdatesRewrittenRemoteBranch(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(t.TempDir(), "origin.git")
	process := toolrun.New()
	run := func(dir string, args ...string) string {
		t.Helper()
		out, err := process.Run(t.Context(), toolrun.ToolCmd{Name: gitBinary, WorkDir: dir, Args: args})
		if err != nil || out.ExitCode != 0 {
			t.Fatalf("git %v: %v %s", args, err, out.Stderr)
		}
		return strings.TrimSpace(string(out.Stdout))
	}
	run(root, "init", "--initial-branch=main")
	run(root, "config", "user.name", "Setup Author")
	run(root, "config", "user.email", "author@example.invalid")
	run(root, "config", "commit.gpgsign", "false")
	run(root, "commit", "--allow-empty", "-m", "base")
	run(root, "init", "--bare", remote)
	run(root, "remote", "add", "origin", remote)
	run(root, "push", "origin", "HEAD:refs/heads/main")
	run(root, "update-ref", "refs/remotes/origin/main", "HEAD")
	run(root, "switch", "-c", "archfit/setup")
	run(root, "commit", "--allow-empty", "-m", "chore: set up archfit")
	old := run(root, "rev-parse", "HEAD")
	run(root, "push", "origin", "HEAD:refs/heads/archfit/setup")
	run(root, "update-ref", "refs/remotes/origin/archfit/setup", old)
	run(root, "reset", "--hard", "main")
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("new setup"), 0o600); err != nil {
		t.Fatal(err)
	}
	run(root, "add", "AGENTS.md")
	run(root, "commit", "-m", "chore: set up archfit")
	run(root, "push", "--force", "origin", "HEAD:refs/heads/archfit/setup")
	g := Git{Root: root, Runner: process}
	if err := g.ValidateRemote(t.Context(), "archfit/setup", "main", []string{"AGENTS.md"}); err != nil {
		t.Fatal(err)
	}
	if got := run(root, "rev-parse", "refs/remotes/origin/archfit/setup"); got == old {
		t.Fatal("tracking ref remained stale")
	}
}

func TestCreateAcceptsCaseInsensitiveRepositoryURL(t *testing.T) {
	args := "pr create --repo o/r --base main --head archfit/setup --title Set up archfit --body-file body"
	for _, tc := range []struct {
		url string
		ok  bool
	}{
		{"https://github.com/O/R/pull/7", true},
		{"https://github.com/other/r/pull/7", false},
		{"https://github.com.evil.invalid/o/r/pull/7", false},
	} {
		t.Run(tc.url, func(t *testing.T) {
			gh := mockGH(t, args, tc.url, "", 0)
			gh.BodyPath = "body"
			got, err := gh.Create(t.Context(), repo, "main", "archfit/setup")
			if (err == nil) != tc.ok || (tc.ok && got.Number != 7) {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}
