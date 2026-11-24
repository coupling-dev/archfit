package requestsend

import (
	"context"
	"errors"
	"testing"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

func TestRoutingWithRealGit(t *testing.T) {
	dir := t.TempDir()
	runner := toolrun.New()
	git := func(args ...string) {
		t.Helper()
		out, err := runner.Run(context.Background(), toolrun.ToolCmd{Name: "git", Args: args, WorkDir: dir})
		if err != nil || out.ExitCode != 0 {
			t.Fatalf("git %v: exit %d, %v, %s", args, out.ExitCode, err, out.Stderr)
		}
	}
	git("init", "-q", "-b", "feature")
	git("config", "user.email", "test@example.invalid")
	git("config", "user.name", "test")
	git("commit", "--allow-empty", "-qm", "base", "--no-gpg-sign")
	git("remote", "add", "origin", "https://github.com/acme/shop.git")
	git("remote", "add", "fork", "git@github.com:worker/shop.git")
	git("update-ref", "refs/remotes/origin/feature", "HEAD")
	git("branch", "--set-upstream-to=origin/feature")
	git("config", "branch.feature.pushRemote", "fork")
	git("config", "push.default", "current")
	s := New(dir, runner)
	at, err := s.repository(t.Context())
	if err != nil || at.repo != testRepo || at.head != forkHead || at.noHead != nil || at.untracked != nil {
		t.Fatalf("target=%+v err=%v", at, err)
	}
	// A single-branch clone pushed with -u: Git records the upstream but keeps
	// no remote-tracking ref, so @{upstream} does not resolve (N3).
	git("checkout", "-q", "-b", "solo")
	git("config", "branch.solo.remote", "origin")
	git("config", "branch.solo.merge", "refs/heads/solo-remote")
	at, err = s.repository(t.Context())
	if err != nil || at.repo != testRepo || at.head != "acme:solo-remote" || at.untracked == nil ||
		at.untracked.pushRepo != testRepo || at.untracked.branch != "solo-remote" {
		t.Fatalf("untracked: target=%+v err=%v", at, err)
	}
	git("checkout", "--detach", "-q")
	at, err = s.repository(t.Context())
	if err != nil || at.repo != testRepo || at.head != "" || !errors.Is(at.noHead, errDetached) {
		t.Fatalf("detached: target=%+v err=%v", at, err)
	}
}
