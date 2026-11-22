package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

const setupNoHooks = "--no-hooks"

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
		code := runDeps([]string{"setup", setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr})
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
		code := runDeps([]string{"setup", setupNoHooks, "-C", dir}, &appDeps{Runner: mock, Stdout: &stdout, Stderr: &stderr})
		if code != 0 || !strings.Contains(stdout.String(), "state: done") || !strings.Contains(stdout.String(), "advisory:") || strings.Contains(stdout.String(), "next:") {
			t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
		}
	}
}
