package requestsend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/coupling-dev/archfit/internal/testutil"
	"github.com/coupling-dev/archfit/internal/toolrun"
)

const (
	testReason        = "why"
	testRepo          = "acme/shop"
	unknownOutcome    = "outcome unknown"
	secondCredential  = "second"
	thirdCredential   = "third"
	forkHead          = "worker:feature"
	originCommand     = "git config --get remote.origin.url"
	pushURLCommand    = "git remote get-url --push origin"
	upstreamCommand   = "git rev-parse --abbrev-ref --symbolic-full-name @{upstream}"
	branchCommand     = "git symbolic-ref --quiet --short HEAD"
	pushRemoteCommand = "git for-each-ref --format=%(push:remotename) refs/heads/feature"
	branchRemote      = "git config --get branch.feature.remote"
	branchMerge       = "git config --get branch.feature.merge"
	headCommand       = "git rev-parse HEAD"
	localSHA          = "2222222222222222222222222222222222222222"
	forkPushURL       = "git remote get-url --push fork"
	originHead        = "acme:feature"
	commentsPath      = "/repos/acme/shop/issues/431/comments"
	issuesPath        = "/repos/acme/shop/issues"
)

const testToken = "secret-test-token"
const issueURL = "https://github.com/acme/shop/issues/431"
const commentURL = "https://github.com/acme/shop/pull/431#issuecomment-9"
const onePull = "[{\"number\":431}]"

func TestSendPaths(t *testing.T) {
	for _, tc := range []struct {
		name              string
		can               bool
		pulls             string
		wantPath, wantURL string
		wantError         string
	}{
		{"check comment", false, onePull, commentsPath, commentURL, ""},
		{"can comment", true, onePull, commentsPath, commentURL, ""},
		{"can issue", true, "[]", issuesPath, issueURL, ""},
		{"check without pull", false, "[]", "", "", "no open pull request for acme:feature; nothing sent\nnext: open a pull request first"},
		{"ambiguous", false, "[{\"number\":431},{\"number\":432}]", "", "", "#431, #432"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GH_TOKEN", testToken)
			var posts []string
			r := Input{Reason: testReason}
			if tc.can {
				r.Can = canAnswer()
			}
			s := fakeSender(t, nil, func(w http.ResponseWriter, req *http.Request) {
				if req.Header.Get("Authorization") != "Bearer "+testToken {
					t.Error("missing credential")
				}
				if req.Method == http.MethodGet {
					if req.URL.Path != "/repos/acme/shop/pulls" || req.URL.Query().Get("state") != "open" || req.URL.Query().Get("head") != originHead {
						t.Errorf("lookup: %s", req.URL)
					}
					_, _ = fmt.Fprint(w, tc.pulls)
					return
				}
				posts = append(posts, req.URL.Path)
				if req.URL.Path != tc.wantPath {
					t.Errorf("POST path = %s", req.URL.Path)
				}
				var payload struct{ Title, Body string }
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if tc.wantPath == issuesPath {
					if payload.Title != "archfit request: a -> b" || payload.Body != issueBody(r) {
						t.Errorf("issue: %+v", payload)
					}
				} else if payload.Body != Line(r) || payload.Title != "" {
					t.Errorf("comment: %+v", payload)
				}
				created(w, tc.wantURL)
			})
			got, err := s.Send(t.Context(), r)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) || got != (Outcome{}) || len(posts) != 0 {
					t.Fatalf("delivery=%+v err=%v posts=%v", got, err, posts)
				}
			} else if err != nil || got.URL != tc.wantURL || got.Number != 431 || len(posts) != 1 {
				t.Fatalf("delivery=%+v err=%v posts=%v", got, err, posts)
			}
		})
	}
}

func TestAPIFailuresNeverRetryOrLeak(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		lookupStatus, sendStatus int
		response                 string
		want                     string
		posts                    int
	}{
		{"lookup unauthorized", 401, 0, testToken, "HTTP 401", 0},
		{"lookup rate limit", 429, 0, testToken, "HTTP 429", 0},
		{"lookup invalid", 200, 0, testToken, "lookup unreadable", 0},
		{"send forbidden", 200, 403, testToken, "HTTP 403", 1},
		{"send validation", 200, 422, testToken, "HTTP 422", 1},
		{"send server", 200, 500, testToken, unknownOutcome, 1},
		{"send invalid receipt", 200, 201, testToken, unknownOutcome, 1},
		{"send wrong url", 200, 201, "{\"number\":1,\"html_url\":\"https://evil.invalid/secret-test-token\"}", unknownOutcome, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GH_TOKEN", testToken)
			posts := 0
			s := fakeSender(t, nil, func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodGet {
					w.WriteHeader(tc.lookupStatus)
					if tc.lookupStatus == 200 && tc.name != "lookup invalid" {
						_, _ = fmt.Fprint(w, onePull)
					} else {
						_, _ = fmt.Fprint(w, tc.response)
					}
					return
				}
				posts++
				w.WriteHeader(tc.sendStatus)
				_, _ = fmt.Fprint(w, tc.response)
			})
			got, err := s.Send(t.Context(), Input{Reason: testReason})
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), testToken) || got != (Outcome{}) || posts != tc.posts {
				t.Fatalf("delivery=%+v err=%v posts=%d", got, err, posts)
			}
			// An unknown outcome may have reached GitHub: it must not invite
			// the same request a second way.
			if marked := nothingSentMarked(err); marked != (tc.want != unknownOutcome) {
				t.Errorf("nothing-sent mark %t on %q", marked, err)
			}
		})
	}
}

func TestSendOutcomeUnknownAfterConnectionLost(t *testing.T) {
	t.Setenv("GH_TOKEN", testToken)
	var posts atomic.Int32
	s := fakeSender(t, nil, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, onePull)
			return
		}
		posts.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	})
	got, err := s.Send(t.Context(), Input{Reason: testReason})
	if err == nil || !strings.Contains(err.Error(), unknownOutcome) || nothingSentMarked(err) || got != (Outcome{}) || posts.Load() != 1 {
		t.Fatalf("delivery=%+v err=%v posts=%d", got, err, posts.Load())
	}
}

func nothingSentMarked(err error) bool {
	var marked interface{ NothingSent() }
	return errors.As(err, &marked)
}

func TestNoCredentialsSendsNothing(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	calls := 0
	s := fakeSender(t, map[string]toolrun.Output{"gh auth token": {ExitCode: 1, Stdout: []byte(testToken), Stderr: []byte(testToken)}}, func(http.ResponseWriter, *http.Request) { calls++ })
	got, err := s.Send(t.Context(), Input{Reason: testReason})
	if err == nil || !strings.Contains(err.Error(), "next: gh auth login") || strings.Contains(err.Error(), testToken) || calls != 0 || got != (Outcome{}) {
		t.Fatalf("delivery=%+v err=%v calls=%d", got, err, calls)
	}
}

func TestCredentialsPrecedence(t *testing.T) {
	for _, tc := range []struct {
		gh, github, cli, want string
		ghCalls               int
	}{
		{"first", secondCredential, thirdCredential, "first", 0},
		{"", secondCredential, thirdCredential, secondCredential, 0},
		{"", "", " third\n", thirdCredential, 1},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Setenv("GH_TOKEN", tc.gh)
			t.Setenv("GITHUB_TOKEN", tc.github)
			calls := 0
			s := New(".", &toolrun.RunnerMock{RunFunc: func(_ context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
				calls++
				if cmd.Name != "gh" || !reflect.DeepEqual(cmd.Args, []string{"auth", "token"}) {
					t.Errorf("command: %+v", cmd)
				}
				return toolrun.Output{Stdout: []byte(tc.cli)}, nil
			}})
			got, err := s.credentials(t.Context())
			if err != nil || got != tc.want || calls != tc.ghCalls {
				t.Fatalf("credential mismatch, err=%v, calls=%d", err, calls)
			}
		})
	}
}

func TestRepositoryRouting(t *testing.T) {
	for _, tc := range []struct {
		name       string
		override   map[string]toolrun.Output
		repo, head string
	}{
		{"origin", nil, testRepo, originHead},
		{"fork push", map[string]toolrun.Output{
			pushRemoteCommand: {Stdout: []byte("fork\n")},
			forkPushURL:       {Stdout: []byte("git@github.com:worker/shop.git\n")},
		}, testRepo, forkHead},
		{"origin fork upstream", map[string]toolrun.Output{
			originCommand:                          {Stdout: []byte("https://github.com/worker/shop.git")},
			"git config --get remote.upstream.url": {Stdout: []byte("https://github.com/acme/shop.git")},
		}, testRepo, forkHead},
		{"no upstream: the local name", map[string]toolrun.Output{upstreamCommand: {ExitCode: 128}}, testRepo, originHead},
		{"no tracking ref: the merge ref of the push remote", map[string]toolrun.Output{
			upstreamCommand: {ExitCode: 128},
			branchRemote:    {Stdout: []byte("origin\n")},
			branchMerge:     {Stdout: []byte("refs/heads/work/part\n")},
		}, testRepo, "acme:work/part"},
		{"no tracking ref: a merge ref of another remote is not paired with the push owner", map[string]toolrun.Output{
			upstreamCommand:   {ExitCode: 128},
			pushRemoteCommand: {Stdout: []byte("fork\n")},
			forkPushURL:       {Stdout: []byte("git@github.com:worker/shop.git\n")},
			branchRemote:      {Stdout: []byte("origin\n")},
		}, testRepo, forkHead},
		{"no upstream, push remote not on GitHub", map[string]toolrun.Output{
			upstreamCommand: {ExitCode: 128},
			pushURLCommand:  {Stdout: []byte("git@gitlab.com:acme/shop.git\n")},
		}, testRepo, ""},
		{"detached", map[string]toolrun.Output{branchCommand: {ExitCode: 1}}, testRepo, ""},
		{"non GitHub", map[string]toolrun.Output{originCommand: {Stdout: []byte("git@gitlab.com:acme/shop.git")}}, "", ""},
		{"no origin", map[string]toolrun.Output{originCommand: {ExitCode: 1}}, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(".", gitRunner(t, tc.override))
			at, err := s.repository(t.Context())
			guessed := tc.override[upstreamCommand].ExitCode != 0 && tc.head != ""
			if err != nil || at.repo != tc.repo || at.head != tc.head || (at.untracked != nil) != guessed {
				t.Fatalf("target=%+v err=%v", at, err)
			}
		})
	}
}

// TestNoHeadSaysWhy: a branch pushed without -u has a pull request that
// archfit cannot find; it must not claim there is none (B6).
func TestNoHeadSaysWhy(t *testing.T) {
	for _, tc := range []struct {
		name     string
		override map[string]toolrun.Output
		want     string
	}{
		{"no upstream, push remote not on GitHub", map[string]toolrun.Output{upstreamCommand: {ExitCode: 128}, pushURLCommand: {Stdout: []byte("/srv/shop.git\n")}},
			"this branch has no upstream branch, so archfit cannot find its pull request; nothing sent\nnext: git push -u origin HEAD"},
		{"no upstream, unsafe remote name", map[string]toolrun.Output{upstreamCommand: {ExitCode: 128}, pushRemoteCommand: {Stdout: []byte("$(touch x)\n")},
			"git remote get-url --push $(touch x)": {ExitCode: 2}},
			"this branch has no upstream branch, so archfit cannot find its pull request; nothing sent\nnext: push the branch with -u to its remote"},
		{"detached", map[string]toolrun.Output{branchCommand: {ExitCode: 1}},
			"HEAD is not on a branch, so archfit cannot find its pull request; nothing sent\nnext: switch to the branch of the pull request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(".", gitRunner(t, tc.override))
			got, err := s.Send(t.Context(), Input{Reason: testReason})
			var marked interface{ NothingSent() }
			if err == nil || err.Error() != tc.want || !errors.As(err, &marked) || got != (Outcome{}) {
				t.Fatalf("delivery=%+v err=%v", got, err)
			}
		})
	}
}

// TestUntrackedBranchFindsItsPullRequest: a single-branch clone pushed with
// -u keeps no remote-tracking ref. The pull request is found by the branch
// name and gets the comment only when its head is HEAD; the no-upstream
// answer is kept for a branch that is not on GitHub (N3).
func TestUntrackedBranchFindsItsPullRequest(t *testing.T) {
	const (
		noUpstream    = "this branch has no upstream branch, so archfit cannot find its pull request; nothing sent\nnext: git push -u "
		otherCommit   = `[{"number":431,"head":{"sha":"1111111111111111111111111111111111111111"}}]`
		atOtherCommit = "pull request #431 is at 1111111, this checkout is at 2222222; nothing sent\nnext: "
	)
	untracked := map[string]toolrun.Output{upstreamCommand: {ExitCode: 128}}
	recorded := map[string]toolrun.Output{
		upstreamCommand: {ExitCode: 128},
		branchRemote:    {Stdout: []byte("origin\n")},
		branchMerge:     {Stdout: []byte("refs/heads/feature\n")},
	}
	forkPush := map[string]toolrun.Output{
		upstreamCommand:   {ExitCode: 128},
		pushRemoteCommand: {Stdout: []byte("fork\n")},
		forkPushURL:       {Stdout: []byte("git@github.com:worker/shop.git\n")},
	}
	for _, tc := range []struct {
		name         string
		override     map[string]toolrun.Output
		can          bool
		head, pulls  string
		branchStatus int
		branchPath   string
		wantPost     string
		wantError    string
	}{
		{"its pull request at HEAD gets the comment", untracked, false, originHead,
			`[{"number":431,"head":{"sha":"` + localSHA + `"}}]`, 0, "", commentsPath, ""},
		{"a pull request at another commit gets nothing; push sets the upstream", untracked, false, originHead,
			otherCommit, 0, "", "", atOtherCommit + "git push -u origin HEAD"},
		{"a recorded upstream only needs a push", recorded, false, originHead,
			otherCommit, 0, "", "", atOtherCommit + "git push"},
		{"a can answer also needs HEAD", untracked, true, originHead,
			otherCommit, 0, "", "", atOtherCommit + "git push -u origin HEAD"},
		{"not on GitHub: no upstream", untracked, false, originHead, "[]",
			http.StatusNotFound, "/repos/acme/shop/branches/feature", "", noUpstream + "origin HEAD"},
		{"not on the fork: no upstream", forkPush, false, forkHead, "[]",
			http.StatusNotFound, "/repos/worker/shop/branches/feature", "", noUpstream + "fork HEAD"},
		{"on GitHub without a pull request", map[string]toolrun.Output{
			upstreamCommand: {ExitCode: 128},
			branchRemote:    {Stdout: []byte("origin\n")},
			branchMerge:     {Stdout: []byte("refs/heads/feature/part\n")},
		}, false, "acme:feature/part", "[]", http.StatusOK, "/repos/acme/shop/branches/feature/part", "",
			"no open pull request for acme:feature/part; nothing sent\nnext: open a pull request first"},
		{"a failed branch lookup says so", untracked, false, originHead, "[]",
			http.StatusBadGateway, "/repos/acme/shop/branches/feature", "", "GitHub returned HTTP 502; nothing sent"},
		{"a can answer without a pull request opens an issue", untracked, true, originHead, "[]", 0, "", issuesPath, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GH_TOKEN", testToken)
			r := Input{Reason: testReason}
			if tc.can {
				r.Can = canAnswer()
			}
			var posts, branchLookups []string
			s := fakeSender(t, tc.override, func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/repos/acme/shop/pulls":
					if got := req.URL.Query().Get("head"); got != tc.head {
						t.Errorf("lookup head %q, want %q", got, tc.head)
					}
					_, _ = fmt.Fprint(w, tc.pulls)
				case req.Method == http.MethodGet:
					branchLookups = append(branchLookups, req.URL.Path)
					w.WriteHeader(tc.branchStatus)
					_, _ = fmt.Fprint(w, `{"name":"feature"}`)
				default:
					posts = append(posts, req.URL.Path)
					created(w, commentURL)
				}
			})
			_, err := s.Send(t.Context(), r)
			wantLookups := []string(nil)
			if tc.branchPath != "" {
				wantLookups = []string{tc.branchPath}
			}
			if !reflect.DeepEqual(branchLookups, wantLookups) {
				t.Errorf("branch lookups %v, want %v", branchLookups, wantLookups)
			}
			if tc.wantError != "" {
				var marked interface{ NothingSent() }
				if err == nil || err.Error() != tc.wantError || !errors.As(err, &marked) || len(posts) != 0 {
					t.Fatalf("err=%v posts=%v, want %q", err, posts, tc.wantError)
				}
				if words := testutil.StoryNotUsedIn(err.Error()); len(words) > 0 {
					t.Errorf("%q uses %v", err, words)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(posts, []string{tc.wantPost}) {
				t.Fatalf("err=%v posts=%v, want %s", err, posts, tc.wantPost)
			}
		})
	}
}

// TestUnreachableGitHubNamesTheCause: a lookup that never reaches GitHub
// says so with the network error, never the URL or the token (B6).
func TestUnreachableGitHubNamesTheCause(t *testing.T) {
	for _, tc := range []struct{ name, cause, want string }{
		{"tls", "tls: failed to verify certificate: x509: OSStatus -26276",
			"could not reach GitHub: tls: failed to verify certificate: x509: OSStatus -26276; nothing sent"},
		{"token in the cause", "proxy said Bearer " + testToken, "could not reach GitHub; nothing sent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GH_TOKEN", testToken)
			s := New(".", gitRunner(t, nil))
			s.Client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New(tc.cause)
			})}
			got, err := s.Send(t.Context(), Input{Reason: testReason})
			var marked interface{ NothingSent() }
			if err == nil || err.Error() != tc.want || !errors.As(err, &marked) || got != (Outcome{}) {
				t.Fatalf("delivery=%+v err=%v", got, err)
			}
		})
	}
}

func TestOfflineDoesNotReadCredentials(t *testing.T) {
	s := New(".", gitRunner(t, map[string]toolrun.Output{originCommand: {ExitCode: 1}}))
	got, err := s.Send(t.Context(), Input{Reason: testReason})
	if err != nil || got != (Outcome{}) {
		t.Fatalf("delivery=%+v err=%v", got, err)
	}
}

func TestGitHubRemote(t *testing.T) {
	for _, tc := range []struct{ remote, want string }{
		{"https://github.com/acme/shop.git", testRepo},
		{"ssh://git@github.com/acme/shop.git", testRepo},
		{"git@github.com:acme/shop.git", testRepo},
		{"https://github.com.evil.invalid/acme/shop", ""},
		{"https://github.com/acme/../shop", ""},
		{"file:///acme/shop", ""},
		{"https://github.com/acme/shop?token=secret", ""},
		{"https://user@github.com/acme/shop.git", ""},
		{"https://user:password@github.com/acme/shop.git", ""},
		{"ssh://git:password@github.com/acme/shop.git", ""},
	} {
		if got := githubRepository(tc.remote); got != tc.want {
			t.Errorf("repository(%q) = %q, want %q", tc.remote, got, tc.want)
		}
	}
}

func TestTransportFailureAndTimeoutDoNotLeak(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		s := New(".", gitRunner(t, nil))
		s.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if _, ok := r.Context().Deadline(); !ok {
				t.Error("no HTTP timeout")
			}
			if method == http.MethodPost && r.GetBody != nil {
				t.Error("POST can be replayed")
			}
			return nil, errors.New(testToken)
		})}
		var out any
		_, err := s.call(t.Context(), testToken, method, "/test", map[string]string{"body": "request"}, &out)
		if err == nil || strings.Contains(err.Error(), testToken) {
			t.Fatalf("error=%v", err)
		}
		if method == http.MethodPost && !strings.Contains(err.Error(), unknownOutcome) {
			t.Fatalf("error=%v", err)
		}
	}
}

func TestRedirectNeverForwardsCredentials(t *testing.T) {
	t.Setenv("GH_TOKEN", testToken)
	calls := 0
	s := fakeSender(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Location", "https://evil.invalid/"+testToken)
		w.WriteHeader(http.StatusTemporaryRedirect)
	})
	_, err := s.Send(t.Context(), Input{Reason: testReason})
	if err == nil || strings.Contains(err.Error(), testToken) || calls != 1 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func canAnswer() *CanRecord {
	return &CanRecord{From: "a", To: "b", Record: "a -> b: graph: not an allowed dependency\n"}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// created answers a POST with the receipt of comment 9 on pull request 431.
func created(w http.ResponseWriter, receipt string) {
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"number": 431, "id": 9, "html_url": receipt})
}

func fakeSender(t *testing.T, override map[string]toolrun.Output, handler http.HandlerFunc) *Sender {
	t.Helper()
	var logs bytes.Buffer
	server := httptest.NewUnstartedServer(handler)
	server.Config.ErrorLog = log.New(&logs, "", 0)
	server.Start()
	t.Cleanup(func() {
		server.Close()
		if strings.Contains(logs.String(), testToken) {
			t.Error("token leaked to server logs")
		}
	})
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	t.Cleanup(transport.CloseIdleConnections)
	s := New(".", gitRunner(t, override))
	s.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		return transport.RoundTrip(r)
	})}
	return s
}

func gitRunner(t *testing.T, override map[string]toolrun.Output) *toolrun.RunnerMock {
	t.Helper()
	commands := map[string]toolrun.Output{
		originCommand:                          {Stdout: []byte("https://github.com/acme/shop.git\n")},
		"git config --get remote.upstream.url": {ExitCode: 1},
		branchCommand:                          {Stdout: []byte("feature\n")},
		upstreamCommand:                        {Stdout: []byte("origin/feature\n")},
		pushRemoteCommand:                      {},
		branchRemote:                           {ExitCode: 1},
		branchMerge:                            {ExitCode: 1},
		headCommand:                            {Stdout: []byte(localSHA + "\n")},
	}
	for key, value := range override {
		commands[key] = value
	}
	if _, ok := override[pushURLCommand]; !ok {
		commands[pushURLCommand] = commands[originCommand]
	}
	return &toolrun.RunnerMock{RunFunc: func(_ context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		key := cmd.Name + " " + strings.Join(cmd.Args, " ")
		if got, ok := commands[key]; ok {
			return got, nil
		}
		t.Errorf("unexpected command %s", key)
		return toolrun.Output{}, errors.New("unexpected command")
	}}
}
