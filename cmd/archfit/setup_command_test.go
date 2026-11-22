package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

const (
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
		if cmd.Name == gitBinary && len(cmd.Args) > 0 && cmd.Args[0] == "fetch" {
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
	gitFixture(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	writeFixtureFile(t, dir, ".github/workflows/archfit.yaml", "jobs:\n  a:\n    steps:\n      - uses: coupling-dev/archfit-action@v1\n")
	runner := toolrun.New()
	mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if cmd.Name == "gh" {
			return toolrun.Output{Stderr: []byte("- Token scopes: 'repo'\n")}, nil
		}
		if cmd.Name == gitBinary && len(cmd.Args) > 0 && cmd.Args[0] == "fetch" {
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

func TestSetupResumesPublishedBranchWithoutMutations(t *testing.T) {
	const signedIn = "repo, workflow"
	for _, tc := range []struct {
		name, auth, prs, checks, state, next string
	}{
		{"no gh", "", "", "", "setup pull request not opened; gh is not signed in", "open the pull request:"},
		{"closed", signedIn, `[{"number":7,"state":"CLOSED","url":"https://github.com/o/r/pull/7"}]`, "", "stopped", "reopen #7:"},
		{"no app check", signedIn, `[{"number":7,"state":"OPEN","url":"https://github.com/o/r/pull/7","headRefOid":"abc"}]`, `{"check_runs":[]}`, "waiting for the App", "install the App:"},
		{"app check", signedIn, `[{"number":7,"state":"OPEN","url":"https://github.com/o/r/pull/7","headRefOid":"abc"}]`, `{"check_runs":[{"app":{"slug":"archfit-review"},"status":"completed","conclusion":"neutral","html_url":"https://github.com/o/r/check","output":{"title":"merge to start"}}]}`, "waiting for the merge", "merge #7:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := fixtureFiles()
			delete(files, ".github/workflows/archfit.yml")
			dir := fixtureRepo(t, files, nil)
			gitFixture(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
			gitFixture(t, dir, "switch", "-c", "archfit/setup", "main")
			writeFixtureFile(t, dir, ".github/workflows/archfit.yaml", "- uses: coupling-dev/archfit-action@v1\n")
			gitFixture(t, dir, "add", ".github/workflows/archfit.yaml")
			gitFixture(t, dir, "commit", "-m", "chore: set up archfit", "--no-gpg-sign")
			process := toolrun.New()
			oid, err := process.Run(t.Context(), toolrun.ToolCmd{Name: gitBinary, WorkDir: dir, Args: []string{"rev-parse", "HEAD"}})
			if err != nil || oid.ExitCode != 0 {
				t.Fatalf("head: %v %s", err, oid.Stderr)
			}
			mock := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
				args := strings.Join(cmd.Args, " ")
				switch {
				case cmd.Name == gitBinary && strings.HasPrefix(args, "fetch "):
					return toolrun.Output{ExitCode: 1}, nil
				case cmd.Name == gitBinary && strings.HasPrefix(args, "ls-remote --heads"):
					return toolrun.Output{Stdout: []byte(strings.TrimSpace(string(oid.Stdout)) + "\trefs/heads/archfit/setup\n")}, nil
				case cmd.Name == gitBinary && (strings.HasPrefix(args, "push ") || strings.HasPrefix(args, "commit ")):
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
				if tc.name == "closed" {
					wantCode = 2
				}
				if code != wantCode || !strings.Contains(stdout.String(), "state: "+tc.state) || !strings.Contains(stdout.String(), "next: "+tc.next) {
					t.Fatalf("exit %d stdout %s stderr %s", code, stdout.String(), stderr.String())
				}
			}
		})
	}
}
