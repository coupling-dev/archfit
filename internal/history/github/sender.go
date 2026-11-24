package requestsend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

const timeout = 30 * time.Second

// Sender posts a request to GitHub. Client may be supplied for a custom HTTP
// transport. Every call has a timeout and refuses redirects; credentials must
// never be forwarded to another host.
type Sender struct {
	dir    string
	runner toolrun.Runner
	Client *http.Client
}

// New wires the GitHub adapter. A non-GitHub checkout returns an empty delivery
// without looking up credentials, preserving the printed offline request.
func New(dir string, runner toolrun.Runner) *Sender {
	return &Sender{dir: dir, runner: runner}
}

// Send finds exactly one open pull request and posts one comment. Without a
// pull request, only a can answer may open an issue. POST is never retried.
func (s *Sender) Send(ctx context.Context, r Input) (Outcome, error) {
	at, err := s.repository(ctx)
	if err != nil {
		return Outcome{}, err
	}
	if at.repo == "" {
		return Outcome{}, nil
	}
	head := at.head
	if head == "" && r.Can == nil {
		return Outcome{}, at.noHead
	}
	token, err := s.credentials(ctx)
	if err != nil {
		return Outcome{}, err
	}
	path := "/repos/" + at.repo
	var pulls []pull
	if head != "" {
		q := url.Values{"state": {"open"}, "head": {head}, "per_page": {"100"}}
		more, err := s.call(ctx, token, http.MethodGet, path+"/pulls?"+q.Encode(), nil, &pulls)
		if err != nil {
			return Outcome{}, err
		}
		if pulls == nil {
			return Outcome{}, notSent("GitHub lookup returned no list; nothing sent")
		}
		if len(pulls) > 1 || more {
			numbers := make([]string, 0, len(pulls))
			for _, p := range pulls {
				numbers = append(numbers, "#"+strconv.Itoa(p.Number))
			}
			return Outcome{}, notSent(fmt.Sprintf("several open pull requests match: %s; nothing sent", strings.Join(numbers, ", ")))
		}
		if at.untracked != nil {
			if err := s.untracked(ctx, token, *at.untracked, pulls, r.Can != nil); err != nil {
				return Outcome{}, err
			}
		}
	}
	var payload any
	if len(pulls) == 1 {
		if pulls[0].Number < 1 {
			return Outcome{}, notSent("GitHub returned an invalid pull request; nothing sent")
		}
		path += "/issues/" + strconv.Itoa(pulls[0].Number) + "/comments"
		payload = struct {
			Body string `json:"body"`
		}{Line(r)}
	} else {
		if r.Can == nil {
			return Outcome{}, notSent("no open pull request for " + head + "; nothing sent\nnext: open a pull request first")
		}
		path += "/issues"
		payload = struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		}{"archfit request: " + r.Can.From + " -> " + r.Can.To, issueBody(r)}
	}
	var sent struct {
		Number int    `json:"number"`
		ID     int    `json:"id"`
		URL    string `json:"html_url"`
	}
	if _, err := s.call(ctx, token, http.MethodPost, path, payload, &sent); err != nil {
		return Outcome{}, err
	}
	// A comment has an id, not an issue number. Preserve the containing PR number.
	number := sent.Number
	if len(pulls) == 1 {
		number = pulls[0].Number
	}
	u, err := url.Parse(sent.URL)
	repoPath := "/" + at.repo + "/"
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" ||
		len(u.Path) < len(repoPath) || !strings.EqualFold(u.Path[:len(repoPath)], repoPath) || strings.Contains(sent.URL, token) ||
		strings.Contains(u.Path+"#"+u.Fragment, token) || number < 1 {
		return Outcome{}, errors.New("GitHub returned an invalid receipt; outcome unknown: check GitHub before trying again")
	}
	return Outcome{Number: number, URL: sent.URL}, nil
}

// pull is an open pull request from the lookup.
type pull struct {
	Number int `json:"number"`
	Head   struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

// guess is a pull request head read from the branch name, because Git keeps
// no remote-tracking ref for the branch: a single-branch clone pushed with -u.
type guess struct {
	pushRepo, branch string
	// notPushed is the answer when the branch is not on the push remote.
	notPushed error
}

// untracked guards a guessed head. It posts only on a pull request whose head
// is this checkout's HEAD, so a same-named branch of another pull request never
// gets the comment. Without a pull request it says there is no upstream when
// the branch is not on GitHub; a can answer still opens an issue.
func (s *Sender) untracked(ctx context.Context, token string, g guess, pulls []pull, can bool) error {
	if len(pulls) == 1 {
		local, code, err := s.git(ctx, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		if code != 0 || local == "" {
			return notSent("cannot read HEAD; nothing sent")
		}
		if pulls[0].Head.SHA != local {
			return notSent(fmt.Sprintf("pull request #%d is at %.7s, this checkout is at %.7s; nothing sent\nnext: git push",
				pulls[0].Number, pulls[0].Head.SHA, local))
		}
		return nil
	}
	if can {
		return nil
	}
	segments := strings.Split(g.branch, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	var branch struct {
		Name string `json:"name"`
	}
	if _, err := s.call(ctx, token, http.MethodGet, "/repos/"+g.pushRepo+"/branches/"+strings.Join(segments, "/"), nil, &branch); err != nil {
		if errors.Is(err, errNotFound) {
			return g.notPushed
		}
		return err
	}
	return nil
}

// NothingSent marks an error after which nothing reached GitHub, so the
// request may be posted another way without a duplicate.
type nothingSent string

func (e nothingSent) Error() string { return string(e) }

// NothingSent marks the error.
func (nothingSent) NothingSent() {}

func notSent(message string) error { return nothingSent(message) }

// Why the current checkout names no pull request head.
var (
	errNotFound = notSent("GitHub returned HTTP 404; nothing sent")
	errDetached = notSent("HEAD is not on a branch, so archfit cannot find its pull request; nothing sent\nnext: switch to the branch of the pull request")
	remoteName  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// noUpstream names the push remote in the command only when the name is safe
// to paste into a shell.
func noUpstream(remote string) error {
	next := "push the branch with -u to its remote"
	if remoteName.MatchString(remote) {
		next = "git push -u " + remote + " HEAD"
	}
	return notSent("this branch has no upstream branch, so archfit cannot find its pull request; nothing sent\nnext: " + next)
}

// transportCause is the first line of the network error under err, never the
// URL or a response, and nothing that contains the token.
func transportCause(err error, token string) string {
	var u *url.Error
	if !errors.As(err, &u) {
		return ""
	}
	cause, _, _ := strings.Cut(u.Err.Error(), "\n")
	if cause == "" || strings.Contains(cause, token) {
		return ""
	}
	return ": " + cause
}

// credentials never returns the output or error of gh: either can contain a
// credential. Tokens are passed only in the HTTP Authorization header.
func (s *Sender) credentials(ctx context.Context) (string, error) {
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if token := strings.TrimSpace(os.Getenv(name)); token != "" {
			return token, nil
		}
	}
	out, err := s.runner.Run(ctx, toolrun.ToolCmd{Name: "gh", Args: []string{"auth", "token"}, WorkDir: s.dir, Timeout: timeout})
	if err == nil && out.ExitCode == 0 {
		if token := strings.TrimSpace(string(out.Stdout)); token != "" {
			return token, nil
		}
	}
	return "", notSent("no GitHub credentials; nothing sent\nnext: gh auth login")
}

// call deliberately omits response bodies and URLs from diagnostics: an
// upstream or proxy error may echo the Authorization header. A transport
// error shows only its cause, and none that contains the token.
func (s *Sender) call(ctx context.Context, token, method, path string, payload, result any) (bool, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return false, notSent("cannot encode the request; nothing sent")
		}
		// A reader rather than a bytes.Reader leaves GetBody nil: no automatic
		// replay of a POST by net/http.
		body = io.LimitReader(bytes.NewReader(data), int64(len(data)))
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.github.com"+path, body)
	if err != nil {
		return false, notSent("cannot build GitHub request; nothing sent")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := http.Client{}
	if s.Client != nil {
		client = *s.Client
	}
	if client.Timeout <= 0 || client.Timeout > timeout {
		client.Timeout = timeout
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		if method == http.MethodPost {
			return false, errors.New("GitHub send failed; outcome unknown: check GitHub before trying again (not retried)")
		}
		return false, notSent("could not reach GitHub" + transportCause(err, token) + "; nothing sent")
	}
	defer func() { _ = response.Body.Close() }()
	want := http.StatusOK
	if method == http.MethodPost {
		want = http.StatusCreated
	}
	if response.StatusCode != want {
		if method == http.MethodPost && (response.StatusCode < 400 || response.StatusCode >= 500) {
			return false, fmt.Errorf("GitHub returned HTTP %d; outcome unknown: check GitHub before trying again (not retried)", response.StatusCode)
		}
		if response.StatusCode == http.StatusNotFound {
			return false, errNotFound
		}
		return false, notSent(fmt.Sprintf("GitHub returned HTTP %d; nothing sent", response.StatusCode))
	}
	dec := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	err = dec.Decode(result)
	if err == nil {
		var extra any
		if dec.Decode(&extra) != io.EOF {
			err = errors.New("trailing JSON")
		}
	}
	if err != nil {
		if method == http.MethodPost {
			return false, errors.New("GitHub receipt unreadable; outcome unknown: check GitHub before trying again")
		}
		return false, notSent("GitHub lookup unreadable; nothing sent")
	}
	return strings.Contains(response.Header.Get("Link"), `rel="next"`), nil
}

// git runs only read-only commands and never surfaces remote URLs or stderr.
func (s *Sender) git(ctx context.Context, args ...string) (string, int, error) {
	out, err := s.runner.Run(ctx, toolrun.ToolCmd{Name: "git", Args: args, WorkDir: s.dir, Timeout: timeout})
	if err != nil {
		return "", 0, notSent("cannot read GitHub repository from git; nothing sent")
	}
	return strings.TrimSpace(string(out.Stdout)), out.ExitCode, nil
}

// target is where Send looks for the pull request.
type target struct {
	// repo is the repository of the pull request; empty for a checkout
	// without a GitHub origin.
	repo string
	// head is owner:branch. Empty when noHead says why; a can answer still
	// opens an issue then.
	head   string
	noHead error
	// untracked is set when head is guessed from the branch name.
	untracked *guess
}

// repository returns the target repository and the pull request head.
func (s *Sender) repository(ctx context.Context) (target, error) {
	origin, code, err := s.git(ctx, "config", "--get", "remote.origin.url")
	if err != nil {
		return target{}, err
	}
	if code != 0 && code != 1 {
		return target{}, notSent("cannot read origin; nothing sent")
	}
	repo := githubRepository(origin)
	if repo == "" {
		return target{}, nil
	}
	upstreamURL, _, err := s.git(ctx, "config", "--get", "remote.upstream.url")
	if err != nil {
		return target{}, err
	}
	if upstream := githubRepository(upstreamURL); upstream != "" {
		repo = upstream
	}
	branch, code, err := s.git(ctx, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return target{}, err
	}
	if code == 1 {
		return target{repo: repo, noHead: errDetached}, nil
	}
	if code != 0 {
		return target{}, notSent("cannot read current branch; nothing sent")
	}
	upstream, code, err := s.git(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		return target{}, err
	}
	hasUpstream := code == 0
	pushRemote, _, err := s.git(ctx, "for-each-ref", "--format=%(push:remotename)", "refs/heads/"+branch)
	if err != nil {
		return target{}, err
	}
	if pushRemote == "" {
		pushRemote = "origin"
	}
	remote, code, err := s.git(ctx, "remote", "get-url", "--push", pushRemote)
	if err != nil {
		return target{}, err
	}
	pushRepo := githubRepository(remote)
	if !hasUpstream {
		if code != 0 || pushRepo == "" {
			return target{repo: repo, noHead: noUpstream(pushRemote)}, nil
		}
		name, err := s.pushedName(ctx, branch, pushRemote)
		if err != nil {
			return target{}, err
		}
		owner, _, _ := strings.Cut(pushRepo, "/")
		return target{repo: repo, head: owner + ":" + name,
			untracked: &guess{pushRepo: pushRepo, branch: name, notPushed: noUpstream(pushRemote)}}, nil
	}
	if code != 0 || pushRepo == "" {
		return target{}, notSent("cannot identify the GitHub push destination; nothing sent")
	}
	_, remoteBranch, ok := strings.Cut(upstream, "/")
	if !ok || remoteBranch == "" {
		return target{}, notSent("cannot identify the upstream branch; nothing sent")
	}
	owner, _, _ := strings.Cut(pushRepo, "/")
	return target{repo: repo, head: owner + ":" + remoteBranch}, nil
}

// pushedName is the branch's name on the push remote: the merge ref that
// `git push -u` records, when it records it for that remote, else the local
// name. A merge ref of another remote is never paired with the push owner.
func (s *Sender) pushedName(ctx context.Context, branch, pushRemote string) (string, error) {
	remote, _, err := s.git(ctx, "config", "--get", "branch."+branch+".remote")
	if err != nil || remote != pushRemote {
		return branch, err
	}
	merge, _, err := s.git(ctx, "config", "--get", "branch."+branch+".merge")
	if name, ok := strings.CutPrefix(merge, "refs/heads/"); err == nil && ok && name != "" {
		return name, nil
	}
	return branch, err
}

// githubRepository accepts GitHub HTTPS, SSH and scp-style clone URLs. Reject
// local paths, arbitrary hosts and malformed names before any credential lookup.
func githubRepository(remote string) string {
	if strings.HasPrefix(remote, "git@github.com:") {
		remote = "ssh://git@github.com/" + strings.TrimPrefix(remote, "git@github.com:")
	}
	u, err := url.Parse(remote)
	if err != nil || (u.Scheme != "https" && u.Scheme != "ssh") || u.Host != "github.com" || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	if u.User != nil && (u.Scheme != "ssh" || u.User.String() != "git") {
		return ""
	}
	path := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return ""
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return ""
		}
		for _, c := range part {
			switch {
			case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
			default:
				return ""
			}
		}
	}
	return path
}
