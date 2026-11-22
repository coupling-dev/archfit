package setup

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

const (
	workflowAction  = "workflow"
	createAction    = "create"
	switchAction    = "switch"
	pushAction      = "push"
	commitAction    = "commit"
	checkAction     = "check"
	writeBodyAction = "body"
	fixtureBranch   = "main"
	devBuild        = "dev"
	setupPRURL      = "https://github.com/o/r/pull/7"
	runURL          = "https://github.com/o/r/actions/runs/42"
)

func TestSetupResumesAcrossExternalSteps(t *testing.T) {
	for _, tc := range []struct {
		name         string
		ready, scope bool
		createErr    error
		wantState    State
		wantActions  []string
	}{
		{"signed in", true, true, nil, Unpublished, []string{workflowAction, checkAction, switchAction, commitAction, pushAction, writeBodyAction, createAction}},
		{"no gh", false, false, nil, ManualPR, []string{workflowAction, checkAction, switchAction, commitAction, pushAction, writeBodyAction}},
		{"missing scope", true, false, nil, NeedsScope, []string{workflowAction, checkAction, switchAction, commitAction}},
		{"cannot create pull request", true, true, errors.New("no access"), ComparePR, []string{workflowAction, checkAction, switchAction, commitAction, pushAction, writeBodyAction, createAction}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupFixture{branch: fixtureBranch, ready: tc.ready, scope: tc.scope, createErr: tc.createErr}
			deps := f.deps(t)
			init := Run(t.Context(), deps)
			assertOutcome(t, init, NoPolicy, 0, "archfit setup", "for the owner to decide:")
			assertActions(t, f.actions, "init")
			f.actions = nil
			second := Run(t.Context(), deps)
			assertOutcome(t, second, tc.wantState, 0, Next(second.Progress), "kept .archfit.yaml", "check: fail against origin/main")
			assertActions(t, f.actions, tc.wantActions...)
			if tc.wantState == NeedsScope {
				assertOutcome(t, second, NeedsScope, 0, refreshNext)
				f.scope = true
			}
			f.createErr = nil
			f.actions = nil
			third := Run(t.Context(), deps)
			switch tc.wantState {
			case NeedsScope:
				assertOutcome(t, third, Unpublished, 0, Next(third.Progress))
				assertActions(t, f.actions, pushAction, writeBodyAction, createAction)
			case ComparePR:
				assertOutcome(t, third, Unpublished, 0, Next(third.Progress))
				assertActions(t, f.actions, createAction)
			case ManualPR:
				assertOutcome(t, third, ManualPR, 0, Next(third.Progress))
				assertActions(t, f.actions)
				if !strings.Contains(f.body, installURL) || strings.Contains(f.body, "suggested_target_id") {
					t.Errorf("body should contain the plain install link: %s", f.body)
				}
			default:
				assertOutcome(t, third, NoCheck, 0, Next(third.Progress), "App check: none yet")
				assertActions(t, f.actions)
				if !slices.Contains(third.After, "if the check does not start within a minute, re-run it: "+runURL) {
					t.Errorf("no manual re-run link: %v", third.After)
				}
				want := installURL + "/permissions?suggested_target_id=123&repository_ids[]=456"
				if third.Next != "install the App: "+want || !strings.Contains(f.body, want) {
					t.Errorf("missing prefilled install link: %s / %s", third.Next, f.body)
				}
			}
			f.actions = nil
			if f.pr.Number != 0 {
				f.check = AppCheck{Status: "completed", Title: "merge to start", Conclusion: "neutral", HTMLURL: runURL}
				checked := Run(t.Context(), deps)
				assertOutcome(t, checked, CheckPresent, 0, "merge #7: "+setupPRURL, `App check "merge to start" (neutral)`)
				assertActions(t, f.actions)
				checkedAgain := Run(t.Context(), deps)
				assertOutcome(t, checkedAgain, CheckPresent, 0, "merge #7: "+setupPRURL)
				assertActions(t, f.actions)
			}
			f.merged, f.ready = true, false
			merged := Run(t.Context(), deps)
			assertOutcome(t, merged, Merged, 0, "", "advisory:", "setup page:")
			assertActions(t, f.actions)
			mergedAgain := Run(t.Context(), deps)
			assertOutcome(t, mergedAgain, Merged, 0, "", "advisory:", "setup page:")
			assertActions(t, f.actions)
		})
	}
}

func TestSetupStopsWithoutPublishing(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mutate     func(*setupFixture, *Dependencies)
		next, line string
		actions    []string
	}{
		{"other branch", func(f *setupFixture, _ *Dependencies) { f.branch = "feature" }, "git switch main, then archfit setup", "default branch", nil},
		{"unpublished default commits", func(f *setupFixture, _ *Dependencies) { f.ahead = true }, "sync main with origin/main, then archfit setup", "does not match", nil},
		{"non-v1 policy", func(f *setupFixture, _ *Dependencies) { f.policyErr = errors.New("version 2") }, "fix .archfit.yaml, then archfit setup", "version 2", nil},
		{"foreign workflow", func(f *setupFixture, _ *Dependencies) { f.workflowErr = errors.New("foreign workflow") }, "fix or rename " + WorkflowPath + ", then archfit setup", "foreign workflow", nil},
		{devBuild, func(_ *setupFixture, d *Dependencies) { d.Version = devBuild }, "install a released archfit (brew install coupling-dev/tap/archfit), then archfit setup", "development build", nil},
		{"no author", func(f *setupFixture, _ *Dependencies) { f.noAuthor = true }, "set git user.name and user.email, then archfit setup", "git author name or email is missing", []string{workflowAction}},
		{"unknown", func(_ *setupFixture, d *Dependencies) {
			d.Check = func(context.Context, string) (CheckResult, error) {
				return CheckResult{Status: "unknown", Next: "read the missing evidence"}, nil
			}
		}, "read the missing evidence", "check: unknown", []string{workflowAction}},
		{"closed", func(f *setupFixture, _ *Dependencies) {
			f.branch, f.workflow, f.pushed, f.pending, f.ready = Branch, true, true, false, true
			f.pr = PullRequest{Number: 7, State: "CLOSED", URL: setupPRURL}
		}, "reopen #7: " + setupPRURL, "closed without a merge", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupFixture{branch: fixtureBranch, policy: true, pending: true}
			deps := f.deps(t)
			tc.mutate(&f, &deps)
			for _, actions := range [][]string{tc.actions, nil} {
				f.actions = nil
				out := Run(t.Context(), deps)
				assertOutcome(t, out, Blocked, 2, tc.next, tc.line)
				assertActions(t, f.actions, actions...)
			}
		})
	}
}

func assertOutcome(t *testing.T, o Outcome, state State, exit int, next string, lines ...string) {
	t.Helper()
	text := strings.Join(o.Lines, "\n")
	if o.State != state || o.Exit != exit || o.Next != next {
		t.Fatalf("state %s, exit %d, next %q; want %s, %d, %q\n%s", o.State, o.Exit, o.Next, state, exit, next, text)
	}
	for _, line := range lines {
		if !strings.Contains(text, line) {
			t.Errorf("missing %q in:\n%s", line, text)
		}
	}
}

func assertActions(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("actions %v, want %v", got, want)
	}
}

type setupFixture struct {
	branch, body                                                             string
	policy, workflow, pending, merged, ahead, noAuthor, ready, scope, pushed bool
	createErr, policyErr, workflowErr, pushSafetyErr                         error
	pr                                                                       PullRequest
	check                                                                    AppCheck
	ssh                                                                      bool
	actions                                                                  []string
}

func (f *setupFixture) deps(t *testing.T) Dependencies {
	return Dependencies{
		Files: f, Git: fixtureGit{f}, GitHub: fixtureGH{f}, Version: "0.10.0",
		Init: func(context.Context) (string, error) {
			f.actions = append(f.actions, "init")
			f.policy, f.pending = true, true
			return "wrote .archfit.yaml", nil
		},
		Check: func(_ context.Context, base string) (CheckResult, error) {
			f.actions = append(f.actions, checkAction)
			if base != "origin/main" {
				t.Errorf("check base %q", base)
			}
			return CheckResult{Status: "fail"}, nil
		},
		Hooks: func(context.Context) string { return "" },
	}
}

func (f *setupFixture) Policy() (bool, error)   { return f.policy, f.policyErr }
func (f *setupFixture) Workflow() (bool, error) { return f.workflow, f.workflowErr }
func (f *setupFixture) WriteWorkflow(_, _, _ string) error {
	f.actions = append(f.actions, workflowAction)
	f.workflow, f.pending = true, true
	return nil
}
func (f *setupFixture) PinNote() string { return "" }
func (f *setupFixture) Exceptions() ([]string, error) {
	return []string{"exception legacy: expires 2027-02-20"}, nil
}
func (f *setupFixture) WriteBody(body string) error {
	if f.body != body {
		f.actions = append(f.actions, writeBodyAction)
		f.body = body
	}
	return nil
}
func (f *setupFixture) Paths() []string { return []string{".archfit.yaml", WorkflowPath} }

type fixtureGit struct{ *setupFixture }

func (g fixtureGit) Repository(context.Context) (string, error)    { return "o/r", nil }
func (g fixtureGit) Default(context.Context) (string, error)       { return fixtureBranch, nil }
func (g fixtureGit) Branch(context.Context) (string, error)        { return g.branch, nil }
func (g fixtureGit) Fetch(context.Context, string) error           { return nil }
func (g fixtureGit) Aligned(context.Context, string) (bool, error) { return !g.ahead, nil }
func (g fixtureGit) Merged(context.Context, string) (bool, error)  { return g.merged, nil }
func (g fixtureGit) Pending(_ context.Context, paths []string) ([]string, error) {
	if g.pending {
		return paths, nil
	}
	return nil, nil
}
func (g fixtureGit) Pin(context.Context, string) (string, error) {
	return "0123456789012345678901234567890123456789", nil
}
func (g fixtureGit) Identity(context.Context) (string, error) {
	if g.noAuthor {
		return "", errors.New("identity missing")
	}
	return "Setup Author <author@example.invalid>", nil
}
func (g fixtureGit) Switch(_ context.Context, branch, _ string) error {
	g.actions = append(g.actions, switchAction)
	g.branch = branch
	return nil
}
func (g fixtureGit) Commit(context.Context, []string) error {
	g.actions = append(g.actions, commitAction)
	g.pending, g.pushed = false, false
	return nil
}
func (g fixtureGit) Pushed(context.Context, string) (bool, error) { return g.pushed, nil }
func (g fixtureGit) PushSafety(context.Context, string, []string) (bool, error) {
	return !g.ssh, g.pushSafetyErr
}
func (g fixtureGit) Push(context.Context, string) error {
	g.actions = append(g.actions, pushAction)
	g.pushed = true
	return nil
}

type fixtureGH struct{ *setupFixture }

func (h fixtureGH) Auth(context.Context) (bool, bool) { return h.ready, h.scope }
func (h fixtureGH) PullRequest(context.Context, string, string, string) (PullRequest, error) {
	return h.pr, nil
}
func (h fixtureGH) Create(context.Context, string, string, string) (PullRequest, error) {
	h.actions = append(h.actions, createAction)
	if h.createErr != nil {
		return PullRequest{}, h.createErr
	}
	h.pr = PullRequest{Number: 7, State: "OPEN", URL: setupPRURL, HeadRefOid: "fedcba98765"}
	return h.pr, nil
}
func (h fixtureGH) Check(context.Context, string, string) (AppCheck, error) { return h.check, nil }
func (h fixtureGH) RerunLink(context.Context, string, string) string        { return runURL }
func (h fixtureGH) IsFork(context.Context, string) bool                     { return false }
func (h fixtureGH) InstallTarget(context.Context, string) (uint64, uint64)  { return 123, 456 }

func TestPushDestinationErrorsStopBeforePush(t *testing.T) {
	for _, destinationErr := range []error{ErrMultiplePushURLs, ErrPushRepository} {
		t.Run(destinationErr.Error(), func(t *testing.T) {
			f := setupFixture{branch: Branch, policy: true, workflow: true, ready: true, scope: true, pushSafetyErr: destinationErr}
			out := Run(t.Context(), f.deps(t))
			assertOutcome(t, out, Blocked, 2, "fix the origin push URL, then archfit setup", destinationErr.Error())
			assertActions(t, f.actions)
		})
	}
}

func TestWorkflowScopeOnlyForHTTPS(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ssh     bool
		want    State
		actions []string
	}{
		{"ssh skips scope", true, Unpublished, []string{pushAction, writeBodyAction, createAction}},
		{"https needs scope", false, NeedsScope, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupFixture{branch: Branch, policy: true, workflow: true, ready: true, ssh: tc.ssh}
			out := Run(t.Context(), f.deps(t))
			assertOutcome(t, out, tc.want, 0, Next(out.Progress))
			assertActions(t, f.actions, tc.actions...)
		})
	}
}
