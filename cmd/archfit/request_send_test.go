package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/testutil"
	"github.com/coupling-dev/archfit/internal/toolrun"
)

const onePullJSON = "[{\"number\":431}]"

func TestRequestSendsFromCLI(t *testing.T) {
	const token = "cli-secret-token"
	const receipt = "https://github.com/acme/shop/issues/431"
	for _, tc := range []struct {
		name          string
		first         []string
		upstream      bool
		pulls         string
		status        int
		noCredentials bool
		code          int
		next          string
		posts         int
		offline       bool
		reply         bool
	}{
		{"pull comment", []string{cmdCheck}, true, onePullJSON, 201, false, 0, "", 1, false, false},
		{"can issue", []string{cmdCan, pkgTC, pkgUtil}, false, "[]", 201, false, 0, "", 1, false, false},
		{"check no upstream", []string{cmdCheck}, false, "[]", 0, false, 2, "this branch has no upstream branch, so archfit cannot find its pull request; nothing sent\nnext: git push -u origin HEAD", 0, false, true},
		{"check no pull", []string{cmdCheck}, true, "[]", 0, false, 2, "open a pull request first", 0, false, true},
		{"ambiguous", []string{cmdCheck}, true, "[{\"number\":431},{\"number\":432}]", 0, false, 2, "#431, #432", 0, false, true},
		{"API failure", []string{cmdCheck}, true, onePullJSON, 403, false, 2, "HTTP 403", 1, false, true},
		{"API failure, outcome unknown", []string{cmdCheck}, true, onePullJSON, 500, false, 2, "outcome unknown", 1, false, false},
		{"no credentials", []string{cmdCheck}, true, "[]", 0, true, 2, "next: gh auth login", 0, false, true},
		{"can, no credentials", []string{cmdCan, pkgTC, pkgUtil}, false, "[]", 0, true, 2, "next: gh auth login", 0, false, false},
		{"GitHub unreachable", []string{cmdCheck}, true, "[]", 0, false, 2, "could not reach GitHub: tls: failed to verify certificate: x509: OSStatus -26276; nothing sent", 0, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GH_TOKEN", token)
			t.Setenv("GITHUB_TOKEN", "")
			if tc.noCredentials {
				t.Setenv("GH_TOKEN", "")
			}
			dir := fixtureRepo(t, fixtureFiles(), map[string]string{tcNetem: tcImportsUtil})
			gitFixture(t, dir, "remote", "add", "origin", "https://github.com/acme/shop.git")
			if tc.upstream {
				gitFixture(t, dir, "branch", "--set-upstream-to=origin/main")
			}
			first := runCLI(t, fixtureDate, append(tc.first, "-C", dir)...)
			before, err := os.ReadFile(filepath.Join(dir, lastFile)) //nolint:gosec // controlled fixture checkout
			if err != nil {
				t.Fatal(err)
			}
			calls, posts := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Bearer "+token {
					t.Error("missing credential")
				}
				if r.Method == http.MethodGet {
					answerLookup(w, r, tc.pulls)
					return
				}
				posts++
				var payload struct{ Body string }
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if !strings.HasPrefix(payload.Body, "/archfit request "+whyGraph+"\n") {
					t.Errorf("body=%q", payload.Body)
				}
				if tc.name == "can issue" && payload.Body != "/archfit request "+whyGraph+"\n\n"+strings.Repeat("`", 3)+"text\n"+first.stdout+strings.Repeat("`", 3)+"\n" {
					t.Errorf("issue does not contain the can record verbatim: %q", payload.Body)
				}
				w.WriteHeader(tc.status)
				if tc.status == http.StatusCreated {
					_ = json.NewEncoder(w).Encode(map[string]any{"number": 431, "id": 7, "html_url": receipt})
				} else {
					_, _ = fmt.Fprint(w, token)
				}
			}))
			defer server.Close()
			target, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			transport := http.DefaultTransport.(*http.Transport).Clone()
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: requestTransport(func(r *http.Request) (*http.Response, error) {
				if tc.offline {
					return nil, errors.New("tls: failed to verify certificate: x509: OSStatus -26276")
				}
				r = r.Clone(r.Context())
				r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
				return transport.RoundTrip(r)
			})}
			processRunner := toolrun.New()
			runner := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
				if cmd.Name == "gh" {
					return toolrun.Output{ExitCode: 1, Stdout: []byte(token), Stderr: []byte(token)}, nil
				}
				return processRunner.Run(ctx, cmd)
			}}
			var stdout, stderr bytes.Buffer
			code := runDeps([]string{cmdRequest, "-C", dir, "-m", whyGraph}, &appDeps{Runner: runner, Stdout: &stdout, Stderr: &stderr, requestClient: client, now: day(fixtureDate)})
			if code != tc.code || posts != tc.posts {
				t.Errorf("code=%d posts=%d, want %d/%d; stderr=%s", code, posts, tc.code, tc.posts, stderr.String())
			}
			if tc.code == 0 {
				if stdout.String() != receipt+"\n" || stderr.Len() != 0 {
					t.Errorf("stdout=%q stderr=%q", stdout.String(), stderr.String())
				}
			} else if stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.next) {
				t.Errorf("stdout=%q stderr=%q; want next %q", stdout.String(), stderr.String(), tc.next)
			}
			reply := "\nor reply on the pull request: /archfit request " + whyGraph + "\n"
			if strings.Contains(stderr.String(), reply) != tc.reply {
				t.Errorf("reply line shown %t, want %t; stderr=%s", !tc.reply, tc.reply, stderr.String())
			}
			if words := testutil.StoryNotUsedIn(stderr.String()); len(words) > 0 {
				t.Errorf("request vocabulary %v in:\n%s", words, stderr.String())
			}
			if strings.Contains(stdout.String()+stderr.String(), token) {
				t.Error("credential in command output")
			}
			after, err := os.ReadFile(filepath.Join(dir, lastFile)) //nolint:gosec // controlled fixture checkout
			if err != nil || !bytes.Equal(before, after) {
				t.Errorf("sender changed the last result: %v", err)
			}
			if tc.noCredentials && calls != 0 {
				t.Errorf("%d API calls without credentials", calls)
			}
		})
	}
}

type requestTransport func(*http.Request) (*http.Response, error)

func (f requestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// answerLookup answers the pull request lookup with pulls. The branch is not
// on GitHub, so a branch lookup is 404.
func answerLookup(w http.ResponseWriter, r *http.Request, pulls string) {
	if strings.Contains(r.URL.Path, "/branches/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	_, _ = fmt.Fprint(w, pulls)
}

// TestRequestFindsThePullRequestWithoutATrackingRef: a single-branch clone
// pushed with -u records the upstream but keeps no remote-tracking ref. The
// request still reaches the branch's pull request when its head is HEAD (N3).
func TestRequestFindsThePullRequestWithoutATrackingRef(t *testing.T) {
	const token = "cli-secret-token"
	const receipt = "https://github.com/acme/shop/pull/431#issuecomment-7"
	t.Setenv("GH_TOKEN", token)
	t.Setenv("GITHUB_TOKEN", "")
	dir := fixtureRepo(t, fixtureFiles(), map[string]string{tcNetem: tcImportsUtil})
	gitFixture(t, dir, "remote", "add", "origin", "https://github.com/acme/shop.git")
	branch := strings.TrimSpace(gitOutput(t, dir, "symbolic-ref", "--short", "HEAD"))
	gitFixture(t, dir, "config", "branch."+branch+".remote", "origin")
	gitFixture(t, dir, "config", "branch."+branch+".merge", "refs/heads/"+branch)
	head := strings.TrimSpace(gitOutput(t, dir, "rev-parse", "HEAD"))
	runCLI(t, fixtureDate, cmdCheck, "-C", dir)
	var posted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if r.URL.Path != "/repos/acme/shop/pulls" || r.URL.Query().Get("head") != "acme:"+branch {
				t.Errorf("lookup %s", r.URL)
			}
			_, _ = fmt.Fprintf(w, `[{"number":431,"head":{"sha":%q}}]`, head)
			return
		}
		posted = append(posted, r.URL.Path)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"number": 431, "id": 7, "html_url": receipt})
	}))
	defer server.Close()
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: requestTransport(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		return transport.RoundTrip(r)
	})}
	processRunner := toolrun.New()
	runner := &toolrun.RunnerMock{RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if cmd.Name == "gh" {
			return toolrun.Output{ExitCode: 1}, nil
		}
		return processRunner.Run(ctx, cmd)
	}}
	var stdout, stderr bytes.Buffer
	code := runDeps([]string{cmdRequest, "-C", dir, "-m", whyGraph}, &appDeps{Runner: runner, Stdout: &stdout, Stderr: &stderr, requestClient: client, now: day(fixtureDate)})
	if code != 0 || stdout.String() != receipt+"\n" || stderr.Len() != 0 || len(posted) != 1 || posted[0] != "/repos/acme/shop/issues/431/comments" {
		t.Fatalf("exit %d, posted %v, stdout %q, stderr %q", code, posted, stdout.String(), stderr.String())
	}
}
