package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/coupling-dev/archfit/internal/application/changecheck"
	"github.com/coupling-dev/archfit/internal/application/replay"
	gitpkg "github.com/coupling-dev/archfit/internal/history/git"
	requestsend "github.com/coupling-dev/archfit/internal/history/github"
	"github.com/coupling-dev/archfit/internal/model/result"
)

// This file is the composition root of `replay`: it gives the ports of
// internal/application/replay their adapters over git, gh and the check.

func (co checkout) replay(deps *appDeps) replay.Service {
	return replay.Service{
		History: replayHistory{co: co},
		Checker: replayChecker{co: co, deps: deps, policy: co.policyPath("")},
	}
}

// replayHistory reads first-parent history from this clone. Standalone replay
// defaults to origin/HEAD and labels changes with merged pull requests.
type replayHistory struct {
	co       checkout
	ref      string
	noLabels bool
}

func (h replayHistory) Changes(ctx context.Context, n int) (replay.Source, error) {
	ref := h.ref
	ok := ref != ""
	if !ok {
		ref, ok = gitpkg.OriginHead(ctx, h.co.root, h.co.runner)
	}
	if !ok {
		if err := ctx.Err(); err != nil {
			return replay.Source{}, err
		}
		return replay.Source{}, errors.New("no default branch: origin/HEAD is not set; run git remote set-head origin --auto")
	}
	commits, err := gitpkg.FirstParent(ctx, h.co.root, ref, n, h.co.runner)
	if err != nil {
		return replay.Source{}, err
	}
	src := replay.Source{Ref: ref, Changes: make([]replay.Change, 0, len(commits))}
	// The oldest change has no parent in a shallow clone, and in a clone that
	// holds the whole history only when it is the first commit.
	if len(commits) > 0 && len(commits[len(commits)-1].Parents) == 0 {
		if src.Short, err = gitpkg.Shallow(ctx, h.co.root, h.co.runner); err != nil {
			return replay.Source{}, err
		}
	}
	var pulls map[string]requestsend.Pull
	if !h.noLabels {
		pulls = requestsend.MergedPulls(ctx, h.co.root, h.co.runner)
	}
	for _, c := range commits {
		ch := replay.Change{Commit: c.SHA, Merge: len(c.Parents) > 1, Subject: c.Subject}
		if len(c.Parents) > 0 {
			ch.Parent = c.Parents[0]
		}
		if p, ok := pulls[c.SHA]; ok {
			ch.PR, ch.Title = p.Number, p.Title
		}
		src.Changes = append(src.Changes, ch)
	}
	return src, nil
}

// replayChecker is replay.Checker: archfit check of a change in a temporary
// worktree of its commit, against its first parent, with the policy file of
// the working tree. The result is not stored, so the last result that
// archfit request reads stays the one of the working tree.
type replayChecker struct {
	co     checkout
	deps   *appDeps
	policy string
}

func (c replayChecker) Check(ctx context.Context, ch replay.Change) (result.Result, error) {
	root, cleanup, err := gitpkg.Worktree{Runner: c.co.runner}.Checkout(ctx, ch.Commit, c.co.root, c.co.root)
	defer cleanup()
	if err != nil {
		return result.Result{}, fmt.Errorf("check out %s: %w", ch.Commit, err)
	}
	co, err := openCheckout(ctx, c.deps, root)
	if err != nil {
		return result.Result{}, err
	}
	svc := co.changeCheck()
	svc.Store = noStore{}
	out, err := svc.Check(ctx, changecheck.Options{Base: ch.Parent, Policy: c.policy})
	if err != nil {
		return result.Result{}, err
	}
	return out.Result, nil
}
