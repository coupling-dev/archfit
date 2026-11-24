package main

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/coupling-dev/archfit/internal/application/changecheck"
	"github.com/coupling-dev/archfit/internal/application/setup"
	historygit "github.com/coupling-dev/archfit/internal/history/git"
	"github.com/coupling-dev/archfit/internal/history/setupgit"
)

// actionRef and actionRefNote pin the Action to an exact commit in a candidate
// build. Both are empty in every released build, so setup pins the release tag.
var (
	actionRef     string
	actionRefNote string
)

func setupDependencies(deps *appDeps, dir string, noHooks bool) (setup.Dependencies, error) {
	ctx := deps.runContext()
	local := *deps
	// Setup commits a real branch. Its Git writes must not go into the
	// command-scoped temporary object store used for read-only checks.
	runner := deps.Runner
	if objects, ok := runner.(*historygit.ObjectStore); ok {
		runner = objects.Runner
	}
	local.Runner = setupgit.NonInteractive{Runner: runner}
	co, err := openCheckout(ctx, &local, dirOr(dir))
	if err != nil {
		return setup.Dependencies{}, err
	}
	files := setupFiles{co: co, workflow: setup.WorkflowPath, body: setup.BodyPath, noHooks: noHooks}
	git := setupgit.Git{
		Root: co.root, Runner: local.Runner, ActionRepository: actionRepo,
		PolicyPath: defaultConfigPath, WorkflowsDir: workflowsDir,
		Policy: files.ValidPolicy, UsesAction: setupUsesAction,
	}
	gh := setupGH{setupgit.GitHub{Root: co.root, Runner: local.Runner, BodyPath: setup.BodyPath}}
	return setup.Dependencies{
		Files: files, Git: git, GitHub: gh, Version: version,
		ActionRef: actionRef, ActionRefNote: actionRefNote,
		Replay: func(ctx context.Context, branch string) (setup.ReplaySummary, error) {
			svc := co.replay(&local)
			svc.History = replayHistory{co: co, ref: setupgit.OriginRef(branch), noLabels: true}
			r, err := svc.Run(ctx, 30)
			summary := setup.ReplaySummary{Read: r.Read, Passed: r.Passed, NeededOwner: r.Asked, ToShowOwner: r.Look}
			for _, l := range r.LookAt {
				summary.ToShow = append(summary.ToShow, l.Name+": "+l.Status)
			}
			return summary, err
		},
		Init: func(ctx context.Context) (string, error) {
			return runInit(ctx, &local, co, noHooks, true)
		},
		Hooks: func(ctx context.Context) string {
			if noHooks {
				return ""
			}
			// Check installed commands rather than Codex trust: reinstalling an
			// untrusted hook cannot grant trust and would rewrite it every run.
			active := true
			for _, harness := range []string{harnessClaude, harnessCodex} {
				positions, err := installedHooks(filepath.Join(co.root, harnessConfigFile(harness)))
				if err != nil {
					active = false
					break
				}
				for _, h := range harnessHooks(harness) {
					if _, ok := positions[h.event+" "+h.run]; !ok {
						active = false
					}
				}
			}
			if active {
				return "hooks: active"
			}
			var b strings.Builder
			installInitHooks(ctx, &local, co.root, &b)
			return b.String()
		},
		Check: func(ctx context.Context, base string) (setup.CheckResult, error) {
			checkCo := co
			checkCo.runner = setupgit.NonInteractive{Runner: deps.Runner}
			out, err := checkCo.changeCheck().Check(ctx, changecheck.Options{Base: base})
			if err != nil {
				return setup.CheckResult{}, err
			}
			r := setup.CheckResult{Status: string(out.Result.Status)}
			if len(out.Result.Unknown) > 0 {
				r.Next = out.Result.Unknown[0].Next.Text
			}
			return r, nil
		},
	}, nil
}

type setupGH struct{ setupgit.GitHub }

func (g setupGH) PullRequest(ctx context.Context, repo, base, branch string) (setup.PullRequest, error) {
	p, err := g.GitHub.PullRequest(ctx, repo, base, branch)
	return setup.PullRequest(p), err
}

func (g setupGH) Create(ctx context.Context, repo, base, branch string) (setup.PullRequest, error) {
	p, err := g.GitHub.Create(ctx, repo, base, branch)
	return setup.PullRequest(p), err
}

func (g setupGH) Auth(ctx context.Context) setup.Login {
	return setup.Login(g.GitHub.Auth(ctx))
}

func (g setupGH) Check(ctx context.Context, repo, head string) (setup.AppCheck, error) {
	c, err := g.GitHub.Check(ctx, repo, head)
	return setup.AppCheck(c), err
}
