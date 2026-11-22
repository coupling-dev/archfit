package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

const (
	lsRemote     = "ls-remote"
	setupFetch   = "fetch"
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
			return toolrun.Output{ExitCode: 1}, nil
		}
		return runner.Run(ctx, cmd)
	}}
	for range 2 {
		var stdout, stderr bytes.Buffer
		code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr})
		if code != exitUnknown || !strings.Contains(stdout.String(), "state: stopped") || !strings.Contains(stdout.String(), "next: git switch main") {
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

func TestSetupDetectsMergedWithoutGitHub(t *testing.T) {
	files := fixtureFiles()
	files[".github/workflows/test.yaml"] = "jobs:\n  archfit:\n    steps:\n      - uses: coupling-dev/archfit-action@v1\n"
	dir := fixtureRepo(t, files, nil)
	gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
	gitFixture(t, dir, "switch", "main")
	runner := toolrun.New()
	mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if cmd.Name == "gh" {
			t.Fatal("merged detection must not require gh")
		}
		if cmd.Name == gitBinary && len(cmd.Args) > 0 && cmd.Args[0] == lsRemote {
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
		if code != 0 || !strings.Contains(stdout.String(), "state: done") || !strings.Contains(stdout.String(), "advisory:") || strings.Contains(stdout.String(), "next:") {
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
			return toolrun.Output{ExitCode: 1}, nil
		}
		if cmd.Name == gitBinary && len(cmd.Args) > 0 && cmd.Args[0] == setupFetch {
			return toolrun.Output{ExitCode: 1}, nil
		}
		return runner.Run(ctx, cmd)
	}}
	var stdout, stderr bytes.Buffer
	code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr, now: day(fixtureDate)})
	if code != 0 || !strings.Contains(stdout.String(), "state: stopped before the push") {
		t.Fatalf("setup: exit %d stdout %s stderr %s", code, stdout.String(), stderr.String())
	}
	gitFixture(t, dir, "cat-file", "-e", "HEAD^{commit}")
}

func TestSetupResumesPublishedBranchWithoutMutations(t *testing.T) { //nolint:gocyclo // process stub covers each external command
	const signedIn = "repo, workflow"
	for _, tc := range []struct {
		name, auth, prs, checks, state, next string
	}{
		{"no gh", "", "", "", "setup pull request not opened; gh is not signed in", "open the pull request:"},
		{"no app check", signedIn, `[{"number":7,"state":"OPEN","url":"https://github.com/o/r/pull/7","headRefOid":"abc"}]`, `{"check_runs":[]}`, "waiting for the App", "install the App:"},
		{"app check", signedIn, `[{"number":7,"state":"OPEN","url":"https://github.com/o/r/pull/7","headRefOid":"abc"}]`, `{"check_runs":[{"app":{"slug":"archfit-review"},"status":"completed","conclusion":"neutral","html_url":"https://github.com/o/r/check","output":{"title":"merge to start"}}]}`, "waiting for the merge", "merge #7:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := fixtureFiles()
			delete(files, ".github/workflows/archfit.yml")
			dir := fixtureRepo(t, files, nil)
			gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
			gitFixture(t, dir, "switch", "-c", "archfit/setup", "main")
			writeFixtureFile(t, dir, ".github/workflows/archfit.yaml", "jobs:\n  archfit:\n    steps:\n      - uses: coupling-dev/archfit-action@v1\n")
			gitFixture(t, dir, "add", ".github/workflows/archfit.yaml")
			gitFixture(t, dir, "commit", "-m", "chore: set up archfit", "--no-gpg-sign")
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
						return toolrun.Output{ExitCode: 1}, nil
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
				wantCode := 0
				if code != wantCode || !strings.Contains(stdout.String(), "state: "+tc.state) || !strings.Contains(stdout.String(), "next: "+tc.next) {
					t.Fatalf("exit %d stdout %s stderr %s", code, stdout.String(), stderr.String())
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
		name     string
		noPolicy bool
	}{
		{"no policy", true},
		{"policy without workflow", false},
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
			runner := toolrun.New()
			var writes []string
			prOpen := false
			pushed := false
			mock := &toolrun.RunnerMock{DetectFunc: runner.Detect, StreamFunc: runner.Stream, RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
				args := strings.Join(cmd.Args, " ")
				switch {
				case cmd.Name == gitBinary && strings.HasPrefix(args, "ls-remote --symref"):
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
				return stdout.String()
			}
			if tc.noPolicy {
				if out := run(); !strings.Contains(out, "state: no workflow yet") {
					t.Fatal(out)
				}
			}
			if out := run(); !strings.Contains(out, "opened setup pull request #7") {
				t.Fatal(out)
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
	for range 2 {
		var stdout, stderr bytes.Buffer
		code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: toolrun.New(), Stdout: &stdout, Stderr: &stderr})
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
			return toolrun.Output{ExitCode: 1}, nil
		}
		return runner.Run(ctx, cmd)
	}}
	var stdout, stderr bytes.Buffer
	code := runDeps([]string{setupCommand, setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr})
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
	gitFixture(t, dir, "commit", "-m", "chore: set up archfit", "--no-gpg-sign")
	runner := toolrun.New()
	mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if cmd.Name == "gh" {
			return toolrun.Output{ExitCode: 1}, nil
		}
		if cmd.Name == gitBinary && len(cmd.Args) > 0 {
			switch cmd.Args[0] {
			case lsRemote:
				if len(cmd.Args) > 1 && cmd.Args[1] == "--heads" {
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
