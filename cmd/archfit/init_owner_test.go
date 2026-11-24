package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coupling-dev/archfit/internal/config/policyfile"
	"github.com/coupling-dev/archfit/internal/testutil"
	"github.com/coupling-dev/archfit/internal/toolrun"
)

const testOwnerLogin = "alice"
const nextRead = "next: read"

func TestInitOwners(t *testing.T) {
	for _, tc := range []struct{ name, gh, ident, owner, next, notice string }{
		{"authenticated", "alice User", "X <bob@users.noreply.github.com> 1 +0000", testOwnerLogin, nextRead, ""},
		{"org remote", "", "X <bob@users.noreply.github.com> 1 +0000", "bob", nextRead, "not verified"},
		{"noreply id", "", "X <12+bob@users.noreply.github.com> 1 +0000", "bob", nextRead, "not verified"},
		{"no identity", "", "X <x@example.com> 1 +0000", "<your-github-login>", "next: put your GitHub login", "no person"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := initRepo(t, initFixture())
			var out, errout bytes.Buffer
			runner := &toolrun.RunnerMock{DetectFunc: toolrun.New().Detect, RunFunc: func(ctx context.Context, c toolrun.ToolCmd) (toolrun.Output, error) {
				if c.Name == "gh" {
					if c.Timeout != 5*time.Second {
						t.Fatal("unbounded gh")
					}
					return toolrun.Output{Stdout: []byte(tc.gh)}, nil
				}
				if c.Name == "git" && len(c.Args) == 2 && c.Args[0] == "var" && c.Args[1] == "GIT_AUTHOR_IDENT" {
					return toolrun.Output{Stdout: []byte(tc.ident)}, nil
				}
				return toolrun.New().Run(ctx, c)
			}, StreamFunc: toolrun.New().Stream}
			code := runDeps([]string{"init", "--no-hooks", "-C", dir}, &appDeps{Runner: runner, Stdout: &out, Stderr: &errout, now: day(fixtureDate)})
			data, err := os.ReadFile(filepath.Join(dir, defaultConfigPath)) //nolint:gosec // fixture policy
			if err != nil {
				t.Fatal(err)
			}
			policy, decodeErr := policyfile.Decode(data)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if code != 0 || len(policy.Owners) != 1 || policy.Owners[0] != tc.owner || !strings.Contains(out.String(), tc.next) || !strings.Contains(out.String(), tc.notice) {
				t.Fatalf("exit %d policy %s output %s stderr %s", code, data, out.String(), errout.String())
			}
			notes := strings.Count(out.String(), "Owners approve requests: name an owner who does not write most of the changes")
			if wantNotes := map[bool]int{true: 0, false: 1}[tc.owner == "<your-github-login>"]; notes != wantNotes ||
				(notes == 1 && !strings.Contains(out.String(), "owner: "+tc.owner+". Owners approve requests")) {
				t.Errorf("sole owner note %d times, want %d:\n%s", notes, wantNotes, out.String())
			}
			if words := testutil.StoryNotUsedIn(out.String()); len(words) > 0 {
				t.Errorf("init vocabulary %v in:\n%s", words, out.String())
			}
		})
	}
}
