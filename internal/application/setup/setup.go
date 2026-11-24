// Package setup advances a repository through advisory onboarding.
package setup

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Paths and the branch name shared by the setup workflow and its adapters.
const (
	Command      = "archfit setup"
	Branch       = "archfit/setup"
	WorkflowPath = ".github/workflows/archfit.yaml"
	BodyPath     = ".archfit-cache/setup-pull-request.md"
	refreshNext  = "gh auth refresh -s workflow"
)

// AppURL is the hosted App that a setup workflow uploads to. InstallURL is the
// App installation page. A candidate build overrides both with -X; the defaults
// are the production values, so an unmodified build behaves exactly as before.
var (
	AppURL     = "https://app.coupling.dev"
	InstallURL = "https://github.com/apps/archfit-review/installations/new"
)

// Files reads and writes onboarding files in the current checkout.
type Files interface {
	SafePaths() (string, error)
	Policy() (exists bool, err error)
	Workflow() (exists bool, err error)
	WriteWorkflow(branch, tag, sha string) error
	PinNote() string
	Exceptions() ([]string, error)
	WriteBody(body string) error
	Paths() []string
	ValidationPaths() []string
	// HookPaths are the hook settings files among Paths; none with --no-hooks.
	HookPaths() []string
}

type unsafeDestination interface{ UnsafeDestination() }
type unsafeBranch interface{ UnsafeBranch() }
type hookHistory interface{ HookHistory() }

// Git reads the checkout and publishes only the setup paths.
type Git interface {
	Repository(context.Context) (repo string, err error)
	Default(context.Context) (string, string, error)
	Branch(context.Context) (string, error)
	SetupBranchExists(context.Context, string) (bool, error)
	Fetch(context.Context, string) error
	Aligned(context.Context, string) (bool, error)
	Merged(context.Context, string) (merged, required bool, err error)
	Pending(context.Context, []string) ([]string, error)
	Pin(context.Context, string) (string, error)
	Identity(context.Context) (string, error)
	Switch(context.Context, string, string) error
	Commit(context.Context, []string) error
	Pushed(context.Context, string) (bool, error)
	PushSafety(context.Context, string, []string) (https bool, err error)
	ValidateRemote(context.Context, string, string, []string) error
	Push(context.Context, string) error
}

// PullRequest describes the head and state of a setup pull request.
type PullRequest struct {
	Number                 int
	State, URL, HeadRefOid string
}

// AppCheck is the check published by the Archfit App on that head.
type AppCheck struct {
	Status, Conclusion, HTMLURL, Title string
}

// Login is what gh reports about its GitHub login. Unreachable holds gh's
// error when it could not ask GitHub; Ready is then false, but the login may
// be fine, so setup does not say that gh is not signed in.
type Login struct {
	Ready, Workflow bool
	Unreachable     string
}

// GitHub reads and opens pull requests and inspects the App check.
type GitHub interface {
	Auth(context.Context) Login
	PullRequest(context.Context, string, string, string) (PullRequest, error)
	Create(context.Context, string, string, string) (PullRequest, error)
	Check(context.Context, string, string) (AppCheck, error)
	RerunLink(context.Context, string, string) string
	IsFork(context.Context, string) bool
	InstallTarget(context.Context, string) (uint64, uint64)
}

// Dependencies contains the ports that carry out setup.
type Dependencies struct {
	Files     Files
	Git       Git
	GitHub    GitHub
	Init      func(context.Context) (string, error)
	Check     func(context.Context, string) (CheckResult, error)
	Hooks     func(context.Context) string
	Replay    func(context.Context, string) (ReplaySummary, error)
	Decisions func() ([]string, error)
	Version   string
	// ActionRef and ActionRefNote pin the Action to an exact commit instead of
	// a released tag. Only a candidate build sets both; the defaults are empty,
	// and setup refuses a partial override instead of guessing.
	ActionRef, ActionRefNote string
}

// ReplaySummary counts where the current policy would have asked for help.
type ReplaySummary struct {
	Read, Passed, NeededOwner, ToShowOwner int
	// ToShow names up to three of the changes to show an owner, newest
	// first, each with what the owner would see: "#12: fail".
	ToShow []string
}

func (r ReplaySummary) counts() string {
	counts := fmt.Sprintf("%d passed, %d needed an owner, %d to show an owner", r.Passed, r.NeededOwner, r.ToShowOwner)
	if len(r.ToShow) == 0 {
		return counts
	}
	names := slices.Clone(r.ToShow)
	if more := r.ToShowOwner - len(names); more > 0 {
		names = append(names, fmt.Sprintf("%d more", more))
	}
	return counts + " (" + strings.Join(names, ", ") + ")"
}

// CheckResult carries the local verdict and its continuation, if any.
type CheckResult struct {
	Status, Next string
}

// State is the onboarding state printed to the user.
type State string

// States reported by setup, including states that require a person.
const (
	Blocked      State = "stopped"
	Advisory     State = "advisory checks on"
	Required     State = "required checks on"
	NoPolicy     State = "no workflow yet"
	NoWorkflow   State = "policy, no workflow"
	Unpublished  State = "setup pull request open; the App is not installed"
	ComparePR    State = "setup pull request not opened"
	ManualPR     State = "setup pull request not opened; gh is not signed in"
	OfflinePR    State = "setup pull request not opened; could not reach GitHub"
	NeedsScope   State = "stopped before the push"
	NoCheck      State = "waiting for the App"
	CheckPresent State = "waiting for the merge"
	Closed       State = "closed"
)

// Progress supplies the facts needed for the next action.
type Progress struct {
	State                     State
	Repo, Default, InstallURL string
	PR                        PullRequest
}

// Outcome is the text and exit code for one invocation of setup.
type Outcome struct {
	Progress
	Lines []string
	Next  string
	Exit  int
	After []string
}

// Next describes the single action to take after a step.
func Next(p Progress) string {
	link := p.InstallURL
	if link == "" {
		link = InstallURL
	}
	switch p.State {
	case NoPolicy:
		return Command
	case NeedsScope:
		return refreshNext
	case NoWorkflow, Unpublished, NoCheck:
		return "install the App: " + link
	case ManualPR, OfflinePR, ComparePR:
		return fmt.Sprintf("open the pull request: https://github.com/%s/compare/%s...%s?expand=1",
			p.Repo, url.PathEscape(p.Default), Branch)
	case CheckPresent:
		return fmt.Sprintf("merge #%d: %s", p.PR.Number, p.PR.URL)
	case Closed:
		return fmt.Sprintf("pull request #%d was closed; reopen it, or delete the branch archfit/setup, then archfit setup", p.PR.Number)
	case Advisory:
		return "open the setup page: " + AppURL
	case Required:
		return fmt.Sprintf("check that a branch ruleset requires archfit/gate: https://github.com/%s/settings/rules", p.Repo)
	default:
		return ""
	}
}

var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$`)

// Tag accepts only engine versions that have a corresponding Action tag.
func Tag(version string) (string, error) {
	tag := "v" + strings.TrimPrefix(version, "v")
	if !releaseTag.MatchString(tag) {
		return "", fmt.Errorf("this archfit is a development build (%s): it has no Action release to pin", version)
	}
	return tag, nil
}

// actionRefLength is the length of a full git commit id.
const actionRefLength = 40

// fullCommit accepts only a full lower-case commit id. It decodes the value
// instead of matching an expression, so the pattern is not a second copy of
// the one setupgit uses for a resolved tag.
func fullCommit(ref string) bool {
	if len(ref) != actionRefLength || ref != strings.ToLower(ref) {
		return false
	}
	_, err := hex.DecodeString(ref)
	return err == nil
}

// validActionOverride accepts only a complete pair: a full lower-case commit
// and the note that names it in the workflow comment. A partial or malformed
// override refuses; it never falls back to a release tag.
func validActionOverride(note, ref string) error {
	if note == "" || ref == "" {
		return errors.New("the Action override sets one of the ref and its note: set both, or neither")
	}
	if !fullCommit(ref) {
		return errors.New("the Action override is not a full lower-case commit SHA")
	}
	return nil
}

func installLink(owner, repo uint64) string {
	if owner == 0 || repo == 0 {
		return InstallURL
	}
	return fmt.Sprintf("%s/permissions?suggested_target_id=%d&repository_ids[]=%d", InstallURL, owner, repo)
}

// Body constructs the setup pull request text without local checkout paths.
// pinNote is the workflow comment of an Action pinned by commit in a candidate
// build; it is empty for a release.
func Body(version, link, pinNote string, replay *ReplaySummary, decisions []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "This pull request sets up archfit %s: the policy, the workflow and the instructions for agents.\nChecks are advisory: nothing blocks a merge until an owner makes the check required.\n", strings.TrimPrefix(version, "v"))
	if pinNote != "" {
		fmt.Fprintf(&b, "The workflow pins a candidate build of the Action: a commit made for testing before a release, marked `# %s` instead of a release tag.\n", pinNote)
	}
	if replay != nil {
		fmt.Fprintf(&b, "\n## Last 30 merged pull requests\n\n%s. Read them with `archfit replay --last 30`.\n", replay.counts())
	}
	if len(decisions) > 0 {
		b.WriteString("\n## For the owner to decide\n\n")
		for _, line := range decisions {
			fmt.Fprintf(&b, "- %s\n", line)
		}
	}
	fmt.Fprintf(&b, "\n## Next\n\n1. Install the Archfit App: %s\n2. Merge this pull request.\n", link)
	return b.String()
}

// Detection is the state and branch metadata read before setup advances.
type Detection struct {
	Progress
	branch string
	pushed bool
	login  Login
}

// Detect reads the checkout and the remote default branch before any setup writes.
func Detect(ctx context.Context, d Dependencies, out *Outcome) (Detection, bool) {
	var s Detection
	var err error
	s.Repo, err = d.Git.Repository(ctx)
	if err != nil {
		out.stop(err, "set origin to a github.com repository, then archfit setup")
		return s, false
	}
	var defaultNote string
	s.Default, defaultNote, err = d.Git.Default(ctx)
	out.append(defaultNote)
	if err != nil {
		out.stop(err, "git remote set-head origin --auto, then archfit setup")
		return s, false
	}
	s.branch, err = d.Git.Branch(ctx)
	if err != nil || (s.branch != s.Default && s.branch != Branch) {
		out.stop(errors.New("start from the default branch or archfit/setup"), "git switch "+ShellQuote(s.Default)+", then archfit setup")
		return s, false
	}
	if next, err := d.Files.SafePaths(); err != nil {
		out.stop(err, next)
		return s, false
	}
	policy, err := d.Files.Policy()
	if err != nil {
		out.stop(err, "fix .archfit.yaml, then archfit setup")
		return s, false
	}
	workflow, err := d.Files.Workflow()
	if err != nil {
		out.stop(err, "fix or rename "+WorkflowPath+", then archfit setup")
		return s, false
	}
	if err := d.Git.Fetch(ctx, s.Default); err != nil {
		out.add("could not fetch origin/%s; using the existing remote-tracking ref", s.Default)
	}
	aligned, err := d.Git.Aligned(ctx, s.Default)
	if err != nil || !aligned {
		out.stop(errors.New("local "+s.Default+" does not match origin/"+s.Default), "sync "+ShellQuote(s.Default)+" with "+ShellQuote("origin/"+s.Default)+", then archfit setup")
		return s, false
	}
	// A merged setup leaves its branch behind, so the merge is read first.
	merged, required, err := d.Git.Merged(ctx, s.Default)
	if err != nil {
		out.stop(err, "check "+ShellQuote("origin/"+s.Default)+", then archfit setup")
		return s, false
	}
	if merged {
		s.State = Advisory
		if required {
			s.State = Required
		}
		return s, true
	}
	if s.branch == s.Default {
		exists, err := d.Git.SetupBranchExists(ctx, Branch)
		if err != nil {
			out.stop(err, "check the setup branch on origin, then archfit setup")
			return s, false
		}
		if exists {
			out.stop(errors.New("the setup branch already exists"), "git switch archfit/setup, then archfit setup")
			return s, false
		}
	}
	switch {
	case !policy:
		s.State = NoPolicy
	case !workflow:
		s.State = NoWorkflow
	default:
		s.State = Unpublished
		if s.branch == Branch {
			s.pushed, err = d.Git.Pushed(ctx, Branch)
			if err != nil {
				out.stop(err, "check the remote setup branch, then archfit setup")
				return s, false
			}
		}
	}
	return s, true
}

// Run detects the current state and advances to the next manual step.
func Run(ctx context.Context, d Dependencies) Outcome {
	var out Outcome
	s, ok := Detect(ctx, d, &out)
	if !ok {
		return out
	}
	out.Progress = s.Progress
	switch s.State {
	case Advisory:
		out.add("advisory: checks report on each pull request and do not block a merge")
		out.After = append(out.After, "an owner approves the first policy there, then makes the check required")
	case Required:
		out.add("required: a fail blocks a merge when a branch ruleset requires archfit/gate")
		out.add("setup page: %s", AppURL)
	case NoPolicy:
		lines, err := d.Init(ctx)
		out.append(lines)
		if err != nil {
			out.stop(err, "fix the init error, then archfit setup")
			return out
		}
		if summary := out.replay(ctx, s.Default, d.Replay); summary != nil {
			out.add("replay: %d merged pull requests read: %s", summary.Read, summary.counts())
		}
		decisions, err := decisionLines(d)
		if err != nil {
			out.stop(err, "fix .archfit.yaml")
			return out
		}
		if len(decisions) != 0 {
			out.add("for the owner to decide:")
			for _, line := range decisions {
				out.add("  %s", line)
			}
		}
		// The run that commits prints its lines only at its end, so this is
		// the last point where --no-hooks keeps the hooks out of the branch.
		if hooks := d.Files.HookPaths(); len(hooks) > 0 {
			out.add("the next %s commits the setup files, with %s, on %s and pushes the branch; add --no-hooks to that run to leave out the hook files",
				Command, strings.Join(hooks, " and "), Branch)
		}
	default:
		publish(ctx, d, s, &out)
	}
	if out.Next == "" {
		out.Next = Next(out.Progress)
	}
	return out
}

func publish(ctx context.Context, d Dependencies, s Detection, out *Outcome) {
	// A draft is checked and committed only when there are setup paths to publish.
	out.add("kept .archfit.yaml")
	if s.State == NoWorkflow {
		tag, sha := d.ActionRefNote, d.ActionRef
		overridden := tag != "" || sha != ""
		var err error
		if !overridden {
			tag, err = Tag(d.Version)
			if err != nil {
				out.stop(err, "install a released archfit (brew install coupling-dev/tap/archfit), then archfit setup")
				return
			}
			sha, err = d.Git.Pin(ctx, tag)
		} else {
			err = validActionOverride(tag, sha)
		}
		if err == nil {
			err = d.Files.WriteWorkflow(s.Default, tag, sha)
		}
		if err != nil {
			out.stop(err, "fix the Action pin or workflow error, then archfit setup")
			return
		}
		out.add("wrote %s: coupling-dev/archfit-action@%.12s… # %s", WorkflowPath, sha, tag)
	} else {
		out.append(d.Files.PinNote())
	}
	out.append(d.Hooks(ctx))
	if next, err := d.Files.SafePaths(); err != nil {
		out.stop(err, next)
		return
	}
	paths, err := d.Git.Pending(ctx, d.Files.Paths())
	if err != nil {
		out.stop(err, "fix the git error, then archfit setup")
		return
	}
	if len(paths) > 0 && !commit(ctx, d, &s, out, paths) {
		return
	}
	if s.branch != Branch {
		out.stop(errors.New("no setup files are available to commit"), "review the setup files, then archfit setup")
		return
	}
	s.login = d.GitHub.Auth(ctx)
	if !s.pushed {
		https, err := d.Git.PushSafety(ctx, s.Default, d.Files.Paths())
		if err != nil {
			next := "check the git error, then archfit setup"
			var branch unsafeBranch
			if errors.As(err, &branch) {
				next = "delete the branch archfit/setup only if you do not need its commits, then archfit setup"
			}
			var hooks hookHistory
			if errors.As(err, &hooks) {
				next = "delete the branch archfit/setup only if you do not need its commits (it contains hook settings), then archfit setup --no-hooks"
			}
			var destination unsafeDestination
			if errors.As(err, &destination) {
				next = "fix the origin push URL, then archfit setup"
			}
			out.stop(err, next)
			return
		}
		if s.login.Ready && https && !s.login.Workflow {
			out.State = NeedsScope
			out.add("the gh login has no workflow scope, so GitHub would refuse the workflow file")
			return
		}
		if err := d.Git.Push(ctx, Branch); err != nil {
			next := "git push -u origin " + Branch + ", then archfit setup"
			if strings.Contains(strings.ToLower(err.Error()), "workflow") {
				next = "give your token permission to change workflows (gh auth refresh -s workflow for gh), then archfit setup"
			}
			out.stop(err, next)
			return
		}
		out.add("pushed %s", Branch)
	}
	if err := d.Git.ValidateRemote(ctx, Branch, s.Default, d.Files.Paths()); err != nil {
		next := "could not reach origin; check the connection, then archfit setup"
		var branch unsafeBranch
		if errors.As(err, &branch) {
			next = "delete the branch archfit/setup only if you do not need its commits, then archfit setup"
		}
		var hooks hookHistory
		if errors.As(err, &hooks) {
			next = "delete the branch archfit/setup only if you do not need its commits (it contains hook settings), then archfit setup --no-hooks"
		}
		out.stop(err, next)
		return
	}
	out.State = Unpublished
	publishPR(ctx, d, s, out)
}

func commit(ctx context.Context, d Dependencies, s *Detection, out *Outcome, paths []string) bool {
	identity, err := d.Git.Identity(ctx)
	if err != nil {
		out.stop(errors.New("git author name or email is missing"), "set git user.name and user.email, then archfit setup")
		return false
	}
	base := "origin/" + s.Default
	check, err := d.Check(ctx, base)
	if err != nil {
		out.stop(err, "fix the check error, then archfit setup")
		return false
	}
	out.add("check: %s against %s", check.Status, base)
	if check.Status != "pass" && check.Status != "fail" {
		out.stop(nil, check.Next)
		if out.Next == "" {
			out.Next = "resolve the unknown check, then archfit setup"
		}
		return false
	}
	out.add("setup commits %s on %s and pushes the branch", strings.Join(paths, ", "), Branch)
	if s.branch != Branch {
		if err := d.Git.Switch(ctx, Branch, s.Default); err != nil {
			out.stop(err, "sync the default branch or switch to "+Branch+", then archfit setup")
			return false
		}
		s.branch = Branch
		out.add("created branch %s", Branch)
	}
	if err := d.Git.Commit(ctx, paths); err != nil {
		out.stop(err, "fix the commit error, then archfit setup")
		return false
	}
	out.add("committed as %s", identity)
	s.pushed = false
	return true
}

func decisionLines(d Dependencies) ([]string, error) {
	if d.Decisions != nil {
		return d.Decisions()
	}
	return d.Files.Exceptions()
}

func publishPR(ctx context.Context, d Dependencies, s Detection, out *Outcome) {
	out.InstallURL = InstallURL
	switch {
	case s.login.Ready:
		pr, err := d.GitHub.PullRequest(ctx, s.Repo, s.Default, Branch)
		if err != nil {
			out.stop(err, "fix the pull request lookup, then archfit setup")
			return
		}
		if pr.Number != 0 {
			if pr.State == "OPEN" {
				out.InstallURL = installLink(d.GitHub.InstallTarget(ctx, s.Repo))
			}
			showPR(ctx, d.GitHub, pr, s.Repo, out)
			return
		}
		out.InstallURL = installLink(d.GitHub.InstallTarget(ctx, s.Repo))
		if d.GitHub.IsFork(ctx, s.Repo) {
			out.add("warning: fork pull requests do not receive an id-token")
		}
	case s.login.Unreachable != "":
		// Not knowing is not the same as no pull request: say both.
		out.add("could not reach GitHub: %s", s.login.Unreachable)
		out.add("pull request: open it, or check on GitHub; gh cannot show it")
	default:
		out.add("pull request: open it, or check on GitHub; gh cannot show it")
	}
	decisions, err := decisionLines(d)
	if err != nil {
		out.stop(err, "fix .archfit.yaml")
		return
	}
	var pinNote string
	if d.ActionRef != "" {
		pinNote = d.ActionRefNote
	}
	if err := d.Files.WriteBody(Body(d.Version, out.InstallURL, pinNote, out.replay(ctx, s.Default, d.Replay), decisions)); err != nil {
		out.stop(err, "fix the pull request body file")
		return
	}
	switch {
	case s.login.Ready:
		pr, err := d.GitHub.Create(ctx, s.Repo, s.Default, Branch)
		if err == nil {
			out.add("opened setup pull request #%d: %s", pr.Number, pr.URL)
			return
		}
		out.add("could not open the pull request: %v", err)
		out.State = ComparePR
	case s.login.Unreachable != "":
		out.State = OfflinePR
	default:
		out.State = ManualPR
	}
	out.add("pull request body: %s", BodyPath)
	out.After = append(out.After, "the App, once the pull request is open: "+out.InstallURL)
}

func showPR(ctx context.Context, gh GitHub, pr PullRequest, repo string, out *Outcome) {
	out.PR = pr
	if pr.State != "OPEN" {
		out.stop(fmt.Errorf("setup pull request #%d was closed without a merge: %s", pr.Number, pr.URL), Next(Progress{State: Closed, PR: pr}))
		return
	}
	check, err := gh.Check(ctx, repo, pr.HeadRefOid)
	if err != nil {
		out.stop(err, "fix the App check lookup, then archfit setup")
		return
	}
	out.add("setup pull request #%d is open: %s", pr.Number, pr.URL)
	if check.Status == "" {
		out.State = NoCheck
		if link := gh.RerunLink(ctx, repo, pr.HeadRefOid); link != "" {
			out.After = append(out.After, "if the check does not start within a minute, re-run it: "+link)
		}
		out.add("App check: none yet on %.7s", pr.HeadRefOid)
		out.add("setup page: %s", AppURL)
		return
	}
	out.State = CheckPresent
	label := check.Status
	if check.Conclusion != "" {
		label += ", " + check.Conclusion
	}
	if check.Title != "" && check.Conclusion != "" {
		out.add("setup pull request #%d: App check %q (%s) %s", pr.Number, check.Title, check.Conclusion, check.HTMLURL)
	} else {
		out.add("setup pull request #%d: App check %s %s", pr.Number, label, check.HTMLURL)
	}
}

func (o *Outcome) replay(ctx context.Context, ref string, replay func(context.Context, string) (ReplaySummary, error)) *ReplaySummary {
	if replay == nil {
		return nil
	}
	bounded, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	summary, err := replay(bounded, ref)
	switch {
	case errors.Is(bounded.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
		o.add("replay: stopped after 60 s, %d of 30 read; run archfit replay later", summary.Read)
	case err != nil:
		o.add("replay: %v; run archfit replay later", err)
	default:
		return &summary
	}
	return nil
}

func (o *Outcome) stop(err error, next string) {
	o.State, o.Exit = Blocked, 2
	o.Next = strings.TrimPrefix(next, "next: ")
	if err != nil {
		o.add("%s", err)
	}
}

func (o *Outcome) add(format string, args ...any) {
	o.Lines = append(o.Lines, fmt.Sprintf(format, args...))
}

func (o *Outcome) append(lines string) {
	if lines = strings.TrimSpace(lines); lines != "" {
		o.Lines = slices.Concat(o.Lines, strings.Split(lines, "\n"))
	}
}

// ShellQuote produces one shell argument without allowing command substitution.
func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
