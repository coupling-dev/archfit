package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/application/setup"
	"github.com/coupling-dev/archfit/internal/testutil"
	"github.com/coupling-dev/archfit/internal/toolrun"
)

const (
	lsRemote     = "ls-remote"
	setupFetch   = "fetch"
	setupHeads   = "--heads"
	setupNoHooks = "--no-hooks"
	setupCommand = "setup"
)

func TestSetupWrongBranchStopsWithoutWriting(t *testing.T) {
	dir := fixtureRepo(t, fixtureFiles(), nil)
	gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
	runner := toolrun.New()
	var commands []string
	mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		commands = append(commands, cmd.Name+" "+strings.Join(cmd.Args, " "))
		if cmd.Name == "gh" {
			t.Fatal("wrong branch should not call GitHub")
		}
		if cmd.Name == gitBinary && len(cmd.Args) > 0 && cmd.Args[0] == lsRemote {
			if len(cmd.Args) > 1 && cmd.Args[1] == setupHeads {
				return toolrun.Output{}, nil
			}
			return toolrun.Output{ExitCode: 1}, nil
		}
		return runner.Run(ctx, cmd)
	}}
	for range 2 {
		var stdout, stderr bytes.Buffer
		code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr})
		if words := testutil.StoryNotUsedIn(stdout.String()); len(words) > 0 {
			t.Errorf("setup vocabulary %v in:\n%s", words, stdout.String())
		}
		if code != exitUnknown || !strings.Contains(stdout.String(), "state: stopped") || !strings.Contains(stdout.String(), "next: git switch 'main'") {
			t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
		}
	}
	for _, cmd := range commands {
		for _, forbidden := range []string{"git push", "git commit", "git switch", "gh "} {
			if strings.HasPrefix(cmd, forbidden) {
				t.Errorf("unexpected mutation: %s", cmd)
			}
		}
	}
}

// TestSetupDetectsMergedWithoutGitHub: after the setup pull request is
// merged, setup reports the checks that run, also when the setup branch
// is still there (B3) and when the policy makes the check required.
func TestSetupDetectsMergedWithoutGitHub(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		setupBranch, required bool
		state, next           string
	}{
		{"advisory", false, false, "advisory checks on", "open the setup page: " + setup.AppURL},
		{"setup branch left behind", true, false, "advisory checks on", "open the setup page: " + setup.AppURL},
		{"required", true, true, "required checks on", "check that a branch ruleset requires archfit/gate: https://github.com/o/r/settings/rules"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupMergedRepo(t, tc.setupBranch, tc.required, tc.state, tc.next)
		})
	}
}

func setupMergedRepo(t *testing.T, setupBranch, required bool, state, next string) {
	t.Helper()
	files := fixtureFiles()
	files[".github/workflows/test.yaml"] = "jobs:\n  archfit:\n    steps:\n      - uses: coupling-dev/archfit-action@v1\n"
	if required {
		files[defaultConfigPath] = strings.Replace(fixturePolicy, "version: 1\n", "version: 1\ngate: required\n", 1)
	}
	dir := fixtureRepo(t, files, nil)
	gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
	gitFixture(t, dir, "switch", "main")
	if setupBranch {
		gitFixture(t, dir, "branch", "archfit/setup", "main")
	}
	runner := toolrun.New()
	mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if cmd.Name == "gh" {
			t.Fatal("merged detection must not require gh")
		}
		if cmd.Name == gitBinary && len(cmd.Args) > 0 && cmd.Args[0] == lsRemote {
			if len(cmd.Args) > 1 && cmd.Args[1] == setupHeads {
				return toolrun.Output{}, nil
			}
			return toolrun.Output{ExitCode: 1}, nil
		}
		if cmd.Name == gitBinary && len(cmd.Args) > 0 && cmd.Args[0] == setupFetch {
			return toolrun.Output{ExitCode: 1, Stderr: []byte("offline")}, nil
		}
		return runner.Run(ctx, cmd)
	}}
	for range 2 {
		var stdout, stderr bytes.Buffer
		code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr})
		if words := testutil.StoryNotUsedIn(stdout.String()); len(words) > 0 {
			t.Errorf("setup vocabulary %v in:\n%s", words, stdout.String())
		}
		if code != 0 || !strings.Contains(stdout.String(), "state: "+state+"\n") || !strings.Contains(stdout.String(), "next: "+next+"\n") {
			t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
		}
	}
}

func TestSetupCommitPersistsAfterCommandReturns(t *testing.T) {
	files := fixtureFiles()
	delete(files, ".github/workflows/archfit.yml")
	dir := fixtureRepo(t, files, nil)
	gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
	gitFixture(t, dir, "switch", "main")
	gitFixture(t, dir, "config", "user.name", "Setup Author")
	gitFixture(t, dir, "config", "user.email", "author@example.invalid")
	gitFixture(t, dir, "config", "commit.gpgsign", "false")
	gitFixture(t, dir, "config", "core.hooksPath", ".git/hooks")
	gitFixture(t, dir, "commit", "--allow-empty", "-m", "prior", "--no-gpg-sign")
	gitFixture(t, dir, "update-ref", "refs/remotes/origin/main", headRef)
	writeFixtureFile(t, dir, ".github/workflows/archfit.yaml", "jobs:\n  a:\n    steps:\n      - uses: coupling-dev/archfit-action@v1\n")
	runner := toolrun.New()
	mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if cmd.Name == "gh" {
			return toolrun.Output{Stderr: []byte("- Token scopes: 'repo'\n")}, nil
		}
		if cmd.Name == gitBinary && len(cmd.Args) > 0 && cmd.Args[0] == lsRemote {
			if len(cmd.Args) > 1 && cmd.Args[1] == setupHeads {
				return toolrun.Output{}, nil
			}
			return toolrun.Output{ExitCode: 1}, nil
		}
		if cmd.Name == gitBinary && len(cmd.Args) > 0 && cmd.Args[0] == setupFetch {
			return toolrun.Output{ExitCode: 1}, nil
		}
		return runner.Run(ctx, cmd)
	}}
	var stdout, stderr bytes.Buffer
	code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr, now: day(fixtureDate)})
	if words := testutil.StoryNotUsedIn(stdout.String()); len(words) > 0 {
		t.Errorf("setup vocabulary %v in:\n%s", words, stdout.String())
	}
	if code != 0 || !strings.Contains(stdout.String(), "state: stopped before the push") {
		t.Fatalf("setup: exit %d stdout %s stderr %s", code, stdout.String(), stderr.String())
	}
	gitFixture(t, dir, "cat-file", "-e", "HEAD^{commit}")
}

func TestSetupResumesPublishedBranchWithoutMutations(t *testing.T) { //nolint:gocyclo // process stub covers each external command
	const signedIn = "repo, workflow"
	// What gh 2.105 prints when a TLS-intercepting proxy breaks its call (B6).
	const unreachable = "github.com\n  X Failed to log in to github.com account someone (keyring)\n" +
		"  - The token in keyring is invalid.\n" +
		`  - Error: Get "https://api.github.com/": tls: failed to verify certificate: x509: OSStatus -26276` + "\n"
	for _, tc := range []struct {
		name, auth, authErr, prs, checks, state, next string
	}{
		{"no gh", "", "You are not logged into any GitHub hosts. To log in, run: gh auth login\n", "", "", "setup pull request not opened; gh is not signed in", "open the pull request:"},
		{"gh cannot reach GitHub", "", unreachable, "", "", "setup pull request not opened; could not reach GitHub", "open the pull request:"},
		{"no app check", signedIn, "", `[{"number":7,"state":"OPEN","url":"https://github.com/o/r/pull/7","headRefOid":"abc"}]`, `{"check_runs":[]}`, "waiting for the App", "install the App:"},
		{"app check", signedIn, "", `[{"number":7,"state":"OPEN","url":"https://github.com/o/r/pull/7","headRefOid":"abc"}]`, `{"check_runs":[{"app":{"slug":"archfit-review"},"status":"completed","conclusion":"neutral","html_url":"https://github.com/o/r/check","output":{"title":"merge to start"}}]}`, "waiting for the merge", "merge #7:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := fixtureFiles()
			delete(files, ".github/workflows/archfit.yml")
			dir := fixtureRepo(t, files, nil)
			gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
			gitFixture(t, dir, "switch", "-c", "archfit/setup", "main")
			writeFixtureFile(t, dir, ".github/workflows/archfit.yaml", "jobs:\n  archfit:\n    steps:\n      - uses: coupling-dev/archfit-action@v1\n")
			gitFixture(t, dir, "add", ".github/workflows/archfit.yaml")
			gitRun(t, dir, "commit", "-m", "chore: set up archfit", "--no-gpg-sign")
			process := toolrun.New()
			oid, err := process.Run(t.Context(), toolrun.ToolCmd{Name: gitBinary, WorkDir: dir, Args: []string{"rev-parse", headRef}})
			if err != nil || oid.ExitCode != 0 {
				t.Fatalf("head: %v %s", err, oid.Stderr)
			}
			mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
				args := strings.Join(cmd.Args, " ")
				switch {
				case cmd.Name == gitBinary && strings.HasPrefix(args, "ls-remote --symref"):
					return toolrun.Output{ExitCode: 1}, nil
				case cmd.Name == gitBinary && strings.HasPrefix(args, "fetch ") && strings.Contains(args, "refs/heads/archfit/setup:"):
					return process.Run(ctx, toolrun.ToolCmd{Name: gitBinary, WorkDir: dir, Args: []string{"update-ref", "refs/remotes/origin/archfit/setup", headRef}})
				case cmd.Name == gitBinary && strings.HasPrefix(args, "fetch "):
					return toolrun.Output{ExitCode: 1}, nil
				case cmd.Name == gitBinary && strings.HasPrefix(args, "ls-remote --heads"):
					return toolrun.Output{Stdout: []byte(strings.TrimSpace(string(oid.Stdout)) + "\trefs/heads/archfit/setup\n")}, nil
				case cmd.Name == gitBinary && (strings.HasPrefix(args, "-c push.followTags=false push ") || strings.HasPrefix(args, "commit ")):
					t.Fatalf("unexpected write: %s", args)
				case cmd.Name == "gh" && strings.HasPrefix(args, "auth status"):
					if tc.auth == "" {
						return toolrun.Output{ExitCode: 1, Stderr: []byte(tc.authErr)}, nil
					}
					return toolrun.Output{Stderr: []byte("- Token scopes: 'repo', 'workflow'\n")}, nil
				case cmd.Name == "gh" && strings.HasPrefix(args, "pr list"):
					return toolrun.Output{Stdout: []byte(tc.prs)}, nil
				case cmd.Name == "gh" && strings.Contains(args, "--jq .owner.id"):
					return toolrun.Output{Stdout: []byte("123\n456\n")}, nil
				case cmd.Name == "gh" && strings.Contains(args, "check-runs?"):
					return toolrun.Output{Stdout: []byte(tc.checks)}, nil
				case cmd.Name == "gh" && strings.HasPrefix(args, "run list"):
					return toolrun.Output{Stdout: []byte("[]")}, nil
				case cmd.Name == "gh":
					t.Fatalf("unexpected gh command: %s", args)
				}
				return process.Run(ctx, cmd)
			}}
			for range 2 {
				var stdout, stderr bytes.Buffer
				code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr})
				if words := testutil.StoryNotUsedIn(stdout.String()); len(words) > 0 {
					t.Errorf("setup vocabulary %v in:\n%s", words, stdout.String())
				}
				wantCode := 0
				if code != wantCode || !strings.Contains(stdout.String(), "state: "+tc.state) || !strings.Contains(stdout.String(), "next: "+tc.next) {
					t.Fatalf("exit %d stdout %s stderr %s", code, stdout.String(), stderr.String())
				}
				if tc.authErr == unreachable && (!strings.Contains(stdout.String(), `could not reach GitHub: Get "https://api.github.com/": tls: failed to verify certificate`) || strings.Contains(stdout.String(), "not signed in")) {
					t.Fatalf("unreachable GitHub reported as:\n%s", stdout.String())
				}
			}
		})
	}
}

func TestSetupFirstPublication(t *testing.T) { //nolint:gocyclo // the process boundary enumerates each external response
	old := version
	version = "v0.10.0"
	t.Cleanup(func() { version = old })
	for _, tc := range []struct {
		name         string
		noPolicy     bool
		noOriginHead bool
	}{
		{"no policy", true, false},
		{"policy without workflow", false, false},
		{"missing origin HEAD", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := fixtureFiles()
			delete(files, ".github/workflows/archfit.yml")
			if tc.noPolicy {
				delete(files, defaultConfigPath)
			}
			dir := fixtureRepo(t, files, nil)
			gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
			gitFixture(t, dir, "switch", "main")
			gitFixture(t, dir, "config", "user.name", "Setup Author")
			gitFixture(t, dir, "config", "user.email", "author@example.invalid")
			gitFixture(t, dir, "config", "commit.gpgsign", "false")
			gitFixture(t, dir, "config", "core.hooksPath", ".git/hooks")
			gitFixture(t, dir, "commit", "--allow-empty", "-m", "prior", "--no-gpg-sign")
			gitFixture(t, dir, "update-ref", "refs/remotes/origin/main", headRef)
			if tc.noOriginHead {
				gitFixture(t, dir, "symbolic-ref", "-d", "refs/remotes/origin/HEAD")
			}
			runner := toolrun.New()
			var writes []string
			prOpen := false
			pushed := false
			mock := &toolrun.RunnerMock{DetectFunc: runner.Detect, StreamFunc: runner.Stream, RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
				args := strings.Join(cmd.Args, " ")
				switch {
				case cmd.Name == gitBinary && strings.HasPrefix(args, "ls-remote --symref"):
					if tc.noOriginHead {
						return toolrun.Output{Stdout: []byte("ref: refs/heads/main\tHEAD\n")}, nil
					}
					return toolrun.Output{ExitCode: 1}, nil
				case cmd.Name == gitBinary && strings.HasPrefix(args, "fetch ") && strings.Contains(args, "refs/heads/archfit/setup:"):
					return runner.Run(ctx, toolrun.ToolCmd{Name: gitBinary, WorkDir: dir, Args: []string{"update-ref", "refs/remotes/origin/archfit/setup", headRef}})
				case cmd.Name == gitBinary && strings.HasPrefix(args, "fetch "):
					return toolrun.Output{ExitCode: 1}, nil
				case cmd.Name == gitBinary && strings.HasPrefix(args, "ls-remote https://"):
					return toolrun.Output{Stdout: []byte(strings.Repeat("a", 40) + "\trefs/tags/v0.10.0\n")}, nil
				case cmd.Name == gitBinary && strings.HasPrefix(args, "ls-remote --heads"):
					if pushed {
						head, err := runner.Run(ctx, toolrun.ToolCmd{Name: gitBinary, WorkDir: dir, Args: []string{"rev-parse", headRef}})
						if err != nil {
							return head, err
						}
						return toolrun.Output{Stdout: []byte(strings.TrimSpace(string(head.Stdout)) + "\trefs/heads/archfit/setup\n")}, nil
					}
					return toolrun.Output{}, nil
				case cmd.Name == gitBinary && strings.HasPrefix(args, "-c push.followTags=false push "):
					writes = append(writes, "push")
					pushed = true
					return toolrun.Output{}, nil
				case cmd.Name == gitBinary && strings.HasPrefix(args, "commit "):
					writes = append(writes, "commit")
				case cmd.Name == "gh" && strings.HasPrefix(args, "auth status"):
					return toolrun.Output{Stderr: []byte("- Token scopes: 'repo', 'workflow'\n")}, nil
				case cmd.Name == "gh" && strings.HasPrefix(args, "pr list --state merged"):
					t.Fatal("setup replay must not fetch PR labels")
				case cmd.Name == "gh" && strings.HasPrefix(args, "pr list"):
					if prOpen {
						return toolrun.Output{Stdout: []byte(`[{"number":7,"state":"OPEN","url":"https://github.com/o/r/pull/7","headRefOid":"abc"}]`)}, nil
					}
					return toolrun.Output{Stdout: []byte("[]")}, nil
				case cmd.Name == "gh" && strings.Contains(args, "--jq .owner.id"):
					return toolrun.Output{Stdout: []byte("1\n2\n")}, nil
				case cmd.Name == "gh" && strings.HasPrefix(args, "repo view"):
					return toolrun.Output{Stdout: []byte(`{"isFork":false}`)}, nil
				case cmd.Name == "gh" && strings.HasPrefix(args, "pr create"):
					writes = append(writes, "create")
					prOpen = true
					return toolrun.Output{Stdout: []byte("https://github.com/o/r/pull/7\n")}, nil
				case cmd.Name == "gh" && strings.Contains(args, "check-runs?"):
					return toolrun.Output{Stdout: []byte(`{"check_runs":[]}`)}, nil
				case cmd.Name == "gh" && strings.HasPrefix(args, "run list"):
					return toolrun.Output{Stdout: []byte("[]")}, nil
				case cmd.Name == "gh":
					return toolrun.Output{ExitCode: 1}, nil
				}
				return runner.Run(ctx, cmd)
			}}
			run := func() string {
				t.Helper()
				var stdout, stderr bytes.Buffer
				code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr, now: day(fixtureDate)})
				if code != 0 {
					t.Fatalf("exit %d stdout %s stderr %s", code, stdout.String(), stderr.String())
				}
				if words := testutil.StoryNotUsedIn(stdout.String()); len(words) > 0 {
					t.Errorf("setup vocabulary %v in:\n%s", words, stdout.String())
				}
				return stdout.String()
			}
			if tc.noPolicy {
				if out := run(); !strings.Contains(out, "state: no workflow yet") || !strings.Contains(out, "replay: 2 merged pull requests read:") || strings.Index(out, "replay:") > strings.Index(out, "next:") {
					t.Fatal(out)
				}
			}
			if out := run(); !strings.Contains(out, "opened setup pull request #7") {
				t.Fatal(out)
			}
			body, err := os.ReadFile(filepath.Join(dir, ".archfit-cache/setup-pull-request.md")) //nolint:gosec // dir is a test fixture
			if err != nil || !strings.Contains(string(body), "## Last 30 merged pull requests\n\n1 passed, 0 needed an owner, 1 to show an owner.") {
				t.Fatalf("missing replay in PR body: %s, %v", body, err)
			}
			if words := testutil.StoryNotUsedIn(string(body)); len(words) > 0 {
				t.Errorf("body vocabulary %v in:\n%s", words, body)
			}
			if !slices.Contains(writes, "push") {
				t.Fatalf("push was not called: %v", writes)
			}
			before := len(writes)
			if out := run(); !strings.Contains(out, "state: waiting for the App") {
				t.Fatal(out)
			}
			if len(writes) != before {
				t.Fatalf("second publication repeated writes: %v", writes)
			}
		})
	}
}

func TestSetupRejectsSymlinkedPolicyWithoutReadingIt(t *testing.T) {
	files := fixtureFiles()
	delete(files, defaultConfigPath)
	delete(files, ".github/workflows/archfit.yml")
	dir := fixtureRepo(t, files, nil)
	gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
	gitFixture(t, dir, "switch", "main")
	external := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(external, []byte(fixturePolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(dir, defaultConfigPath)); err != nil {
		t.Fatal(err)
	}
	process := toolrun.New()
	mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if cmd.Name == gitBinary && len(cmd.Args) > 0 && cmd.Args[0] == lsRemote {
			if len(cmd.Args) > 1 && cmd.Args[1] == setupHeads {
				return toolrun.Output{}, nil
			}
			return toolrun.Output{ExitCode: 1}, nil
		}
		return process.Run(ctx, cmd)
	}}
	for range 2 {
		var stdout, stderr bytes.Buffer
		code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr})
		if code != 2 || !strings.Contains(stdout.String(), "next: .archfit.yaml is a symbolic link; replace it with a file, then archfit setup") {
			t.Fatalf("exit %d stdout %s stderr %s", code, stdout.String(), stderr.String())
		}
	}
}

func TestSetupRefusesExistingIgnoredHarnessSettings(t *testing.T) {
	files := fixtureFiles()
	delete(files, ".github/workflows/archfit.yml")
	dir := fixtureRepo(t, files, nil)
	gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
	gitFixture(t, dir, "switch", "main")
	writeFixtureFile(t, dir, ".gitignore", ".claude/settings.json\n")
	writeFixtureFile(t, dir, ".claude/settings.json", `{"env":{"PRIVATE":"local"}}`)
	runner := toolrun.New()
	mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if cmd.Name == gitBinary && len(cmd.Args) > 0 && cmd.Args[0] == lsRemote {
			if len(cmd.Args) > 1 && cmd.Args[1] == setupHeads {
				return toolrun.Output{}, nil
			}
			return toolrun.Output{ExitCode: 1}, nil
		}
		return runner.Run(ctx, cmd)
	}}
	var stdout, stderr bytes.Buffer
	code := runDeps([]string{setupCommand, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr})
	if code != 2 || !strings.Contains(stdout.String(), "next: .claude/settings.json is ignored by git; setup will not publish it.") {
		t.Fatalf("exit %d stdout %s stderr %s", code, stdout.String(), stderr.String())
	}
}

func TestSetupRejectsMultiplePushDestinations(t *testing.T) {
	files := fixtureFiles()
	delete(files, ".github/workflows/archfit.yml")
	dir := fixtureRepo(t, files, nil)
	gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
	gitFixture(t, dir, "remote", "set-url", "--add", "--push", "origin", "https://github.com/o/r.git")
	gitFixture(t, dir, "remote", "set-url", "--add", "--push", "origin", "https://github.com/other/r.git")
	gitFixture(t, dir, "switch", "-c", "archfit/setup", "main")
	writeFixtureFile(t, dir, ".github/workflows/archfit.yaml", "jobs:\n  archfit:\n    steps:\n      - uses: coupling-dev/archfit-action@v1\n")
	gitFixture(t, dir, "add", ".github/workflows/archfit.yaml")
	gitRun(t, dir, "commit", "-m", "chore: set up archfit", "--no-gpg-sign")
	runner := toolrun.New()
	mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if cmd.Name == "gh" {
			return toolrun.Output{ExitCode: 1}, nil
		}
		if cmd.Name == gitBinary && len(cmd.Args) > 0 {
			if slices.Contains(cmd.Args, "push") {
				t.Fatal("unexpected push")
			}
			switch cmd.Args[0] {
			case lsRemote:
				if len(cmd.Args) > 1 && cmd.Args[1] == setupHeads {
					return toolrun.Output{}, nil
				}
				return toolrun.Output{ExitCode: 1}, nil
			case setupFetch:
				return toolrun.Output{ExitCode: 1}, nil
			}
		}
		return runner.Run(ctx, cmd)
	}}
	var stdout, stderr bytes.Buffer
	code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr})
	if code != 2 || !strings.Contains(stdout.String(), "origin has more than one push URL") || !strings.Contains(stdout.String(), "next: fix the origin push URL, then archfit setup") {
		t.Fatalf("exit %d stdout %s stderr %s", code, stdout.String(), stderr.String())
	}
}

func TestSetupNoHooksIgnoresPrivateHarnessAfterMerge(t *testing.T) {
	dir := fixtureRepo(t, fixtureFiles(), nil)
	gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
	gitFixture(t, dir, "switch", "main")
	writeFixtureFile(t, dir, ".gitignore", ".claude/settings.json\n")
	writeFixtureFile(t, dir, ".claude/settings.json", `{"env":{"PRIVATE":"local"}}`)
	runner := toolrun.New()
	mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if cmd.Name == gitBinary && len(cmd.Args) > 0 && (cmd.Args[0] == lsRemote || cmd.Args[0] == setupFetch) {
			if cmd.Args[0] == lsRemote && len(cmd.Args) > 1 && cmd.Args[1] == setupHeads {
				return toolrun.Output{}, nil
			}
			return toolrun.Output{ExitCode: 1}, nil
		}
		return runner.Run(ctx, cmd)
	}}
	var stdout, stderr bytes.Buffer
	code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr})
	if code != 0 || !strings.Contains(stdout.String(), "state: advisory checks on") {
		t.Fatalf("exit %d stdout %s stderr %s", code, stdout.String(), stderr.String())
	}
}

func TestSetupContinuationPreservesFlags(t *testing.T) {
	for _, tc := range []struct {
		cmd        SetupCmd
		next, want string
	}{
		{SetupCmd{NoHooks: true}, "archfit setup", "archfit setup --no-hooks"},
		{SetupCmd{NoHooks: true}, "then archfit setup --no-hooks", "then archfit setup --no-hooks"},
		{SetupCmd{NoHooks: true, Dir: "/tmp/a b"}, "then archfit setup", "then archfit setup --no-hooks -C '/tmp/a b'"},
		{SetupCmd{Dir: "repo's path"}, "archfit setup", "archfit setup -C 'repo'\\''s path'"},
	} {
		if got := tc.cmd.continuation(tc.next); got != tc.want {
			t.Errorf("got %q want %q", got, tc.want)
		}
	}
}

func TestSetupOnMainStopsForExistingBranchBeforeWrites(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(fmt.Sprintf("remote=%t", remote), func(t *testing.T) {
			files := fixtureFiles()
			delete(files, defaultConfigPath)
			dir := fixtureRepo(t, files, nil)
			gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
			gitFixture(t, dir, "switch", "main")
			if !remote {
				gitFixture(t, dir, "branch", "archfit/setup")
			}
			process := toolrun.New()
			mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
				if cmd.Name == gitBinary && len(cmd.Args) > 1 && cmd.Args[0] == lsRemote {
					if cmd.Args[1] == setupHeads && remote {
						return toolrun.Output{Stdout: []byte(strings.Repeat("a", 40) + "\trefs/heads/archfit/setup\n")}, nil
					}
					return toolrun.Output{ExitCode: 1}, nil
				}
				return process.Run(ctx, cmd)
			}}
			var stdout, stderr bytes.Buffer
			code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr})
			if code != 2 || !strings.Contains(stdout.String(), "next: git switch archfit/setup, then archfit setup --no-hooks") {
				t.Fatalf("exit %d stdout %s stderr %s", code, stdout.String(), stderr.String())
			}
			if _, err := os.Stat(filepath.Join(dir, defaultConfigPath)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("policy was written: %v", err)
			}
		})
	}
}
