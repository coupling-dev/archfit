package git_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	gitpkg "github.com/coupling-dev/archfit/internal/history/git"
	"github.com/coupling-dev/archfit/internal/toolrun"
)

func TestFirstParentFollowsTheFirstParentOnly(t *testing.T) {
	requireGit(t)
	t.Parallel()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	writeIn(t, dir, "a.txt", "a")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "first")
	gitIn(t, dir, "checkout", "-q", "-b", "feature")
	writeIn(t, dir, "b.txt", "b")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "on the branch")
	gitIn(t, dir, "checkout", "-q", "main")
	gitIn(t, dir, "merge", "-q", "--no-ff", "-m", "merge feature", "feature")

	commits, err := gitpkg.FirstParent(context.Background(), dir, "main", 10, toolrun.New())
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 || commits[0].Subject != "merge feature" || commits[1].Subject != "first" {
		t.Fatalf("got %+v, want the merge and the first commit", commits)
	}
	if len(commits[0].Parents) != 2 || commits[0].Parents[0] != commits[1].SHA {
		t.Errorf("merge parents %v, want two with the first commit first", commits[0].Parents)
	}
	if len(commits[1].Parents) != 0 {
		t.Errorf("first commit parents %v, want none", commits[1].Parents)
	}
	if limited, _ := gitpkg.FirstParent(context.Background(), dir, "main", 1, toolrun.New()); len(limited) != 1 {
		t.Errorf("limit 1 gave %d commits", len(limited))
	}
}

func TestFirstParentRefusesAnOptionAsARef(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	writeIn(t, dir, "a.txt", "a")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "first")
	// Without the guard git reads --all as an option and lists the commit.
	if _, err := gitpkg.FirstParent(context.Background(), dir, "--all", 5, toolrun.New()); err == nil {
		t.Error("a ref that starts with a dash was accepted")
	}
}

func TestShallowClone(t *testing.T) {
	requireGit(t)
	t.Parallel()
	src := t.TempDir()
	gitIn(t, src, "init", "-q")
	for _, name := range []string{"a", "b", "c"} {
		writeIn(t, src, name+".txt", name)
		gitIn(t, src, "add", ".")
		gitIn(t, src, "commit", "-q", "-m", name)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	gitIn(t, src, "clone", "-q", "--depth", "2", "file://"+src, clone)

	ctx, runner := context.Background(), toolrun.New()
	if shallow, err := gitpkg.Shallow(ctx, clone, runner); err != nil || !shallow {
		t.Errorf("clone: Shallow = %v, %v; want true", shallow, err)
	}
	if shallow, err := gitpkg.Shallow(ctx, src, runner); err != nil || shallow {
		t.Errorf("source: Shallow = %v, %v; want false", shallow, err)
	}
	commits, err := gitpkg.FirstParent(ctx, clone, "HEAD", 10, runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 || len(commits[1].Parents) != 0 {
		t.Errorf("got %+v, want two commits, the oldest with no parent", commits)
	}
}

// logRunner answers git log with fixed output, as git does when a signature
// is shown before a commit.
type logRunner struct {
	toolrun.Runner
	args []string
}

func (r *logRunner) Run(_ context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
	r.args = cmd.Args
	out := "gpg: Signature made Sat Nov 22\ngpg: Good signature\n" +
		"aaa\x00bbb\x00signed change\n"
	return toolrun.Output{Stdout: []byte(out)}, nil
}

func TestFirstParentIgnoresSignatureLines(t *testing.T) {
	t.Parallel()
	r := &logRunner{}
	commits, err := gitpkg.FirstParent(context.Background(), t.TempDir(), "origin/main", 5, r)
	if err != nil || len(commits) != 1 || commits[0].SHA != "aaa" || commits[0].Subject != "signed change" {
		t.Errorf("got %+v, %v; want the one commit", commits, err)
	}
	if !slices.Contains(r.args, "--no-show-signature") {
		t.Errorf("git log was run with %v; want --no-show-signature", r.args)
	}
}
