// Package setupgit adapts noninteractive Git and GitHub operations for setup.
package setupgit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/coupling-dev/archfit/internal/history/git"
	requestsend "github.com/coupling-dev/archfit/internal/history/github"
	"github.com/coupling-dev/archfit/internal/toolrun"
)

const (
	gitBinary  = "git"
	gitHead    = "HEAD"
	gitCommit  = "commit"
	gitTimeout = 30 * time.Second
	originHead = "refs/remotes/origin/HEAD"
	originRef  = "refs/remotes/origin/"
	headsRef   = "refs/heads/"
)

// NonInteractive prevents Git from prompting during setup, including in the
// existing init and check use cases that setup calls.
type NonInteractive struct{ toolrun.Runner }

// Run executes the command with terminal Git prompts disabled.
func (r NonInteractive) Run(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
	return r.Runner.Run(ctx, nonInteractive(cmd))
}

// Stream executes the streamed command with terminal Git prompts disabled.
func (r NonInteractive) Stream(ctx context.Context, cmd toolrun.ToolCmd, consume func(io.Reader) error) (toolrun.Output, error) {
	return r.Runner.Stream(ctx, nonInteractive(cmd), consume)
}

func nonInteractive(cmd toolrun.ToolCmd) toolrun.ToolCmd {
	if cmd.Name == gitBinary {
		cmd.Env = append(slices.Clone(cmd.Env), "GIT_TERMINAL_PROMPT=0")
	}
	return cmd
}

// Git supplies Git operations scoped to the checkout.
type Git struct {
	Root                 string
	Runner               toolrun.Runner
	ActionRepository     string
	PolicyPath           string
	WorkflowsDir         string
	Version1, UsesAction func([]byte) bool
}

// Repository verifies that the origin fetch and push URLs identify one repository.
func (g Git) Repository(ctx context.Context) (string, error) {
	remote, ok, err := git.OriginURL(ctx, g.Root, NonInteractive{g.Runner})
	if err != nil || !ok {
		return "", errors.New("no GitHub origin")
	}
	repo := requestsend.Repository(remote)
	// A separate push URL pointing at another repository would make the
	// compare link and GitHub lookups refer to the wrong branch.
	if repo == "" {
		return "", errors.New("origin is not a github.com repository")
	}
	return repo, nil
}

// Default finds the default branch even when origin/HEAD is not set locally.
func (g Git) Default(ctx context.Context) (string, string, error) {
	localRef, _ := g.run(ctx, "symbolic-ref", "--quiet", originHead)
	local := strings.TrimPrefix(localRef, originRef)
	out, err := (NonInteractive{g.Runner}).Run(ctx, toolrun.ToolCmd{
		Name: gitBinary, WorkDir: g.Root, Timeout: 15 * time.Second,
		Args: []string{"ls-remote", "--symref", "origin", gitHead},
	})
	if err == nil && out.ExitCode == 0 {
		for line := range strings.SplitSeq(string(out.Stdout), "\n") {
			ref, name, _ := strings.Cut(line, "\t")
			if branch, ok := strings.CutPrefix(ref, "ref: "+headsRef); name == gitHead && ok && branch != "" {
				valid, validErr := g.run(ctx, "check-ref-format", "--branch", branch)
				if validErr != nil {
					return "", "", validErr
				}
				if local != "" && local != valid {
					return valid, fmt.Sprintf("origin default branch is %s; local origin/HEAD points to %s", valid, local), nil
				}
				return valid, "", nil
			}
		}
	}
	if local != "" {
		return local, "could not read the remote default branch; using origin/HEAD", nil
	}
	return "", "", errors.New("cannot find the default branch of origin")
}

// Branch resolves HEAD to the current branch, rejecting a detached checkout.
func (g Git) Branch(ctx context.Context) (string, error) {
	return g.run(ctx, "symbolic-ref", "--quiet", "--short", gitHead)
}

// Fetch updates only the specified remote-tracking branch, without tags.
func (g Git) Fetch(ctx context.Context, branch string) error {
	_, err := g.run(ctx, "fetch", "--no-tags", "origin", headsRef+branch+":"+originRef+branch)
	return err
}

// Aligned reports whether the local default branch has no unpublished commits.
// A local branch behind origin after a squash merge is still set up.
func (g Git) Aligned(ctx context.Context, branch string) (bool, error) {
	o, err := (NonInteractive{g.Runner}).Run(ctx, toolrun.ToolCmd{
		Name: gitBinary, WorkDir: g.Root, Timeout: gitTimeout,
		Args: []string{"merge-base", "--is-ancestor", headsRef + branch, originRef + branch},
	})
	switch {
	case err != nil:
		return false, err
	case o.ExitCode == 0:
		return true, nil
	case o.ExitCode == 1:
		return false, nil
	default:
		return false, gitError("merge-base", o)
	}
}

// Merged detects an onboarded default branch without relying on pull request history.
func (g Git) Merged(ctx context.Context, branch string) (bool, error) {
	ref := originRef + branch
	names, err := g.run(ctx, "ls-tree", "-z", "-r", "--name-only", ref, "--", g.PolicyPath, g.WorkflowsDir)
	if err != nil {
		return false, err
	}
	files := strings.Split(names, "\x00")
	if !slices.Contains(files, g.PolicyPath) {
		return false, nil
	}
	policy, err := g.run(ctx, "show", ref+":"+g.PolicyPath)
	if err != nil || !g.Version1([]byte(policy)) {
		return false, err
	}
	for _, file := range files {
		if ext := path.Ext(file); path.Dir(file) != g.WorkflowsDir || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		data, err := g.run(ctx, "show", ref+":"+file)
		if err != nil {
			return false, err
		}
		if g.UsesAction([]byte(data)) {
			return true, nil
		}
	}
	return false, nil
}

// Pending lists modified or untracked setup paths, preserving the input order.
func (g Git) Pending(ctx context.Context, paths []string) ([]string, error) {
	var pending []string
	for _, file := range paths {
		tracked, err := g.run(ctx, "diff", "--name-only", gitHead, "--", file)
		if err != nil {
			return nil, err
		}
		untracked, err := g.run(ctx, "ls-files", "--others", "--exclude-standard", "--", file)
		if err != nil {
			return nil, err
		}
		ignored, err := g.run(ctx, "ls-files", "--others", "--ignored", "--exclude-standard", "--", file)
		if err != nil {
			return nil, err
		}
		if tracked != "" || untracked != "" || ignored != "" {
			pending = append(pending, file)
		}
	}
	return pending, nil
}

var sha1 = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Pin resolves the exact tag, preferring the commit behind an annotated tag.
func (g Git) Pin(ctx context.Context, tag string) (string, error) {
	const pinTimeout = 15 * time.Second
	ref := "refs/tags/" + tag
	peeled := ref + "^{}"
	o, err := (NonInteractive{g.Runner}).Run(ctx, toolrun.ToolCmd{
		Name: gitBinary, WorkDir: g.Root, Timeout: pinTimeout,
		Args: []string{"ls-remote", "https://github.com/" + g.ActionRepository, ref, peeled},
	})
	if err != nil {
		return "", err
	}
	if o.ExitCode != 0 {
		return "", gitError("ls-remote", o)
	}
	pins := make(map[string]string, 2)
	for line := range strings.SplitSeq(string(o.Stdout), "\n") {
		sha, name, _ := strings.Cut(line, "\t")
		if name == ref || name == peeled {
			pins[name] = sha
		}
	}
	sha, ok := pins[peeled]
	if !ok {
		sha, ok = pins[ref]
	}
	if !ok {
		return "", fmt.Errorf("no Action tag %s", tag)
	}
	if !sha1.MatchString(sha) {
		return "", fmt.Errorf("no valid commit SHA for Action tag %s", tag)
	}
	return sha, nil
}

// Identity returns the author Git would use for the setup commit.
func (g Git) Identity(ctx context.Context) (string, error) {
	ident, err := g.run(ctx, "var", "GIT_AUTHOR_IDENT")
	if err != nil {
		return "", err
	}
	if end := strings.LastIndex(ident, ">"); end >= 0 {
		return ident[:end+1], nil
	}
	return "", errors.New("git returned an invalid author identity")
}

// Switch creates the setup branch only from the remote default branch tip.
func (g Git) Switch(ctx context.Context, branch, base string) error {
	local, err := g.run(ctx, "rev-parse", gitHead)
	if err != nil {
		return err
	}
	remote, err := g.run(ctx, "rev-parse", originRef+base)
	if err != nil {
		return err
	}
	if local != remote {
		return fmt.Errorf("sync the local default branch with origin/%s before creating %s", base, branch)
	}
	_, err = g.run(ctx, "switch", "-c", branch)
	return err
}

// Commit includes only setup paths, even when other changes are staged.
func (g Git) Commit(ctx context.Context, paths []string) error {
	if _, err := g.run(ctx, append([]string{"add", "--force", "--"}, paths...)...); err != nil {
		return err
	}
	_, err := g.run(ctx, append([]string{gitCommit, "-m", "chore: set up archfit", "--"}, paths...)...)
	return err
}

// Pushed checks the live remote, not a possibly stale tracking ref.
func (g Git) Pushed(ctx context.Context, branch string) (bool, error) {
	local, err := g.run(ctx, "rev-parse", "--verify", headsRef+branch)
	if err != nil {
		return false, err
	}
	remote, err := g.run(ctx, "ls-remote", "--heads", "origin", headsRef+branch)
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(remote, "\n") {
		sha, ref, _ := strings.Cut(line, "\t")
		if ref == headsRef+branch {
			return sha == local, nil
		}
	}
	return false, nil
}

// PushSafety rejects multiple destinations and unrelated commits.
func (g Git) PushSafety(ctx context.Context, base string, paths []string) (bool, error) {
	fetch, err := g.run(ctx, "remote", "get-url", "origin")
	if err != nil {
		return false, err
	}
	push, err := g.run(ctx, "remote", "get-url", "--push", "--all", "origin")
	if err != nil {
		return false, err
	}
	urls := strings.Split(push, "\n")
	if len(urls) != 1 {
		return false, destinationError("origin has more than one push URL")
	}
	if repo := requestsend.Repository(fetch); repo == "" || !strings.EqualFold(repo, requestsend.Repository(push)) {
		return false, destinationError("origin pushes to another repository")
	}
	if err := g.validateRange(ctx, base, gitHead, paths); err != nil {
		return false, err
	}
	return strings.HasPrefix(push, "https://"), nil
}

// ValidateRemote fetches and validates the actual branch used for the pull request.
func (g Git) ValidateRemote(ctx context.Context, branch, base string, paths []string) error {
	ref := originRef + branch
	if _, err := g.run(ctx, "fetch", "--no-tags", "origin", headsRef+branch+":"+ref); err != nil {
		return err
	}
	remote, err := g.run(ctx, "rev-parse", ref)
	if err != nil {
		return err
	}
	local, err := g.run(ctx, "rev-parse", gitHead)
	if err != nil {
		return err
	}
	if remote != local {
		return branchError("remote setup branch does not match the local branch")
	}
	return g.validateRange(ctx, base, ref, paths)
}

func (g Git) validateRange(ctx context.Context, base, head string, paths []string) error {
	commits, err := g.run(ctx, "rev-list", "--reverse", originRef+base+".."+head)
	if err != nil {
		return err
	}
	for commit := range strings.SplitSeq(commits, "\n") {
		if commit == "" {
			continue
		}
		parents, err := g.run(ctx, "rev-list", "--parents", "-n", "1", commit)
		if err != nil {
			return err
		}
		if len(strings.Fields(parents)) != 2 {
			return branchError("setup branch contains a merge commit; commits beyond the default branch:\n" + commits)
		}
		subject, err := g.run(ctx, "show", "-s", "--format=%s", commit)
		if err != nil {
			return err
		}
		if subject != "chore: set up archfit" {
			return branchError("setup branch contains a commit not made by setup; commits beyond the default branch:\n" + commits)
		}
		files, err := g.run(ctx, "diff-tree", "--root", "--no-commit-id", "--name-only", "-z", "-r", commit)
		if err != nil {
			return err
		}
		for file := range strings.SplitSeq(files, "\x00") {
			if file != "" && !slices.Contains(paths, file) {
				if file == ".claude/settings.json" || file == ".codex/hooks.json" {
					return hookHistoryError("setup branch contains hook settings; commits beyond the default branch:\n" + commits)
				}
				return branchError("setup branch contains a commit changing " + file + "; commits beyond the default branch:\n" + commits)
			}
		}
	}
	return nil
}

// Push publishes the setup branch without rewriting remote history.
func (g Git) Push(ctx context.Context, branch string) error {
	ref := headsRef + branch
	out, err := (NonInteractive{g.Runner}).Run(ctx, toolrun.ToolCmd{
		Name: gitBinary, WorkDir: g.Root, Timeout: gitTimeout,
		Args: []string{"-c", "push.followTags=false", "push", "--no-follow-tags", "origin", ref + ":" + ref},
	})
	if err != nil {
		return err
	}
	if out.ExitCode != 0 {
		return gitError("push", out)
	}
	return nil
}

func (g Git) run(ctx context.Context, args ...string) (string, error) {
	// Commits can run the user's hooks and signing agent.
	timeout := gitTimeout
	if args[0] == gitCommit {
		timeout = 0
	}
	o, err := (NonInteractive{g.Runner}).Run(ctx, toolrun.ToolCmd{
		Name: gitBinary, Args: args, WorkDir: g.Root, Timeout: timeout,
	})
	if err != nil {
		return "", err
	}
	if o.ExitCode != 0 {
		return "", gitError(args[0], o)
	}
	// Keep NUL-separated paths and leading YAML whitespace intact.
	return strings.TrimRight(string(o.Stdout), "\r\n"), nil
}

func gitError(command string, o toolrun.Output) error {
	detail := strings.TrimSpace(string(o.Stderr))
	switch command {
	case gitCommit:
		lines := strings.Split(detail, "\n")
		detail = strings.Join(lines[max(0, len(lines)-4):], "\n")
	case "push":
		// The workflow permission rejection commonly follows the "To" line.
		for line := range strings.SplitSeq(detail, "\n") {
			if strings.Contains(strings.ToLower(line), "workflow") {
				detail = line
				break
			}
		}
		fallthrough
	default:
		detail, _, _ = strings.Cut(detail, "\n")
	}
	return fmt.Errorf("git %s exited %d: %s", command, o.ExitCode, detail)
}

type destinationError string

func (e destinationError) Error() string    { return string(e) }
func (destinationError) UnsafeDestination() {}

type branchError string

func (e branchError) Error() string { return string(e) }
func (branchError) UnsafeBranch()   {}

type hookHistoryError string

func (e hookHistoryError) Error() string { return string(e) }
func (hookHistoryError) HookHistory()    {}
