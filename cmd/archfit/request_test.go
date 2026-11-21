package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coupling-dev/archfit/internal/application/request"
	"github.com/coupling-dev/archfit/internal/model/result"
)

// requestGoldens holds the text of `archfit request` without the App (D5).
const requestGoldens = "testdata/request"

const (
	cmdRequest      = "request"
	whyGraph        = "netem needs the shared retry helper"
	ruleGraph       = "graph"
	ruleDuplication = "duplication"
	flagExpires     = "--expires"
	checkFirst      = "run archfit check first"
	moduleTC        = "pkg_tc"
)

// TestRequestFromTheCLI: check or can, then request, on the examples of
// design 8.1. Without the App the request is printed and the exit is 2.
func TestRequestFromTheCLI(t *testing.T) {
	t.Parallel()
	base := fixtureFiles()
	unprotected := fixtureFiles()
	delete(unprotected, ".github/workflows/archfit.yml")
	for _, tc := range []struct {
		name       string
		base, head map[string]string
		first      []string
		request    []string
	}{
		{name: ruleGraph, base: base, head: map[string]string{tcNetem: tcImportsUtil},
			first: []string{cmdCheck}, request: []string{"-m", whyGraph}},
		{name: ruleDuplication, base: base, head: map[string]string{tcInterface: tcCopiesLiteral},
			first:   []string{cmdCheck},
			request: []string{"-m", "tc parses the name before pkg_util loads", flagExpires, "2026-12-10"}},
		{name: "can", base: base, first: []string{cmdCan, pkgTC, pkgUtil}, request: []string{"-m", whyGraph}},
		{name: caseNotProtected, base: unprotected, head: map[string]string{tcNetem: tcImportsUtil},
			first: []string{cmdCheck}, request: []string{"-m", whyGraph}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := fixtureRepo(t, tc.base, tc.head)
			runCLI(t, fixtureDate, append(tc.first, "-C", dir)...)
			got := runCLI(t, fixtureDate, append([]string{cmdRequest, "-C", dir}, tc.request...)...)
			if got.code != exitUnknown || got.stderr != "" {
				t.Errorf("exit = %d, stderr %q; want 2 and nothing on stderr", got.code, got.stderr)
			}
			if !strings.HasSuffix(got.stdout, "\n") || !strings.HasPrefix(lastLine(got.stdout), askOwner) {
				t.Errorf("the request does not end with %q:\n%s", askOwner, got.stdout)
			}
			assertCLIGolden(t, requestGoldens, tc.name, got.stdout)
		})
	}
}

// TestRequestRefusesFromTheCLI: every refusal exits 2 with its reason on
// stderr and prints no request.
func TestRequestRefusesFromTheCLI(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		head    map[string]string
		first   []string
		edit    bool
		older   bool
		request []string
		want    string
	}{
		{name: "no check output", head: map[string]string{tcNetem: tcImportsUtil},
			request: []string{"-m", whyGraph}, want: checkFirst},
		{name: "an edit after the check", head: map[string]string{tcNetem: tcImportsUtil}, first: []string{cmdCheck}, edit: true,
			request: []string{"-m", whyGraph}, want: checkFirst},
		{name: "a check by another engine", head: map[string]string{tcNetem: tcImportsUtil}, first: []string{cmdCheck}, older: true,
			request: []string{"-m", whyGraph}, want: checkFirst},
		{name: "a pass", head: map[string]string{runtimeGo: fixtureFiles()[runtimeGo] + "\n// Done.\n"}, first: []string{cmdCheck},
			request: []string{"-m", whyGraph}, want: "nothing to ask: archfit: pass"},
		{name: "no reason off a terminal", head: map[string]string{tcNetem: tcImportsUtil}, first: []string{cmdCheck},
			want: `a reason is required: archfit request -m "<why>"`},
		{name: "a date in the past", head: map[string]string{tcNetem: tcImportsUtil}, first: []string{cmdCheck},
			request: []string{"-m", whyGraph, flagExpires, "2026-11-01"}, want: "--expires is not a later date"},
		{name: "a date too far ahead", head: map[string]string{tcNetem: tcImportsUtil}, first: []string{cmdCheck},
			request: []string{"-m", whyGraph, flagExpires, "2027-12-01"}, want: "--expires is too far ahead: want 2027-11-10 or earlier"},
		{name: "the flag at the end of the reason", head: map[string]string{tcNetem: tcImportsUtil}, first: []string{cmdCheck},
			request: []string{"-m", whyGraph + " --expires soon"}, want: "the reason ends like the --expires flag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := fixtureRepo(t, fixtureFiles(), tc.head)
			if tc.first != nil {
				runCLI(t, fixtureDate, append(tc.first, "-C", dir)...)
			}
			if tc.edit {
				writeFiles(t, dir, map[string]string{tcNetem: tcImportsUtil + "\n// Edited.\n"})
			}
			if tc.older {
				stampEngine(t, dir, "0.8.0")
			}
			got := runCLI(t, fixtureDate, append([]string{cmdRequest, "-C", dir}, tc.request...)...)
			if got.code != exitUnknown || got.stdout != "" {
				t.Errorf("exit = %d, stdout %q; want 2 and no request", got.code, got.stdout)
			}
			if !strings.HasPrefix(got.stderr, "archfit: ") || !strings.Contains(got.stderr, tc.want) {
				t.Errorf("stderr = %q, want archfit: ... %q", got.stderr, tc.want)
			}
		})
	}

	got := runCLI(t, fixtureDate, cmdRequest, "-m", whyGraph, "-C", t.TempDir())
	if got.code != exitUnknown || !strings.Contains(got.stderr, notARepo) {
		t.Errorf("outside a repository: exit %d, stderr %q", got.code, got.stderr)
	}
}

// stampEngine rewrites the engine of the stored check output, as if another
// version of archfit had written it.
func stampEngine(t *testing.T, dir, engine string) {
	t.Helper()
	path := filepath.Join(dir, lastFile)
	data, err := os.ReadFile(path) //nolint:gosec // controlled fixture checkout
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	var output map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(doc["output"], &output); err != nil {
		t.Fatal(err)
	}
	output["engine"] = engine
	if doc["output"], err = json.Marshal(output); err != nil {
		t.Fatal(err)
	}
	if data, err = json.Marshal(doc); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestLineReason reads the reason a terminal gives, after the question.
func TestLineReason(t *testing.T) {
	t.Parallel()
	var prompt strings.Builder
	got, err := lineReason{in: strings.NewReader(whyGraph + "\nmore\n"), prompt: &prompt}.Reason(context.Background())
	if err != nil || got != whyGraph+"\n" || prompt.String() != "reason: " {
		t.Errorf("Reason = %q, %v after %q; want the first line after %q", got, err, prompt.String(), "reason: ")
	}
	got, err = lineReason{in: strings.NewReader("no newline"), prompt: &prompt}.Reason(context.Background())
	if err != nil || got != "no newline" {
		t.Errorf("Reason at end of input = %q, %v", got, err)
	}
	boom := errors.New("boom")
	if _, err := (lineReason{in: failingReader{boom}, prompt: &prompt}).Reason(context.Background()); !errors.Is(err, boom) {
		t.Errorf("Reason on a read error = %v, want %v", err, boom)
	}
}

// TestLineReasonStopsOnCancel: a signal at the prompt ends the wait, though
// the read of the terminal never returns.
func TestLineReasonStopsOnCancel(t *testing.T) {
	t.Parallel()
	in, _ := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	var prompt strings.Builder
	done := make(chan error, 1)
	go func() {
		_, err := lineReason{in: in, prompt: &prompt}.Reason(ctx)
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Reason after cancel = %v, want %v", err, context.Canceled)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Reason still waits after its context ended")
	}
}

// TestNoTerminalReasonInTests: a command run with its own stdin never asks.
func TestNoTerminalReasonInTests(t *testing.T) {
	t.Parallel()
	if r := terminalReason(&appDeps{Stdin: strings.NewReader("")}); r != nil {
		t.Errorf("terminalReason with a given stdin = %v, want nil", r)
	}
	var _ request.ReasonReader = lineReason{}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	return lines[len(lines)-1]
}

// TestWriteRequestAsksForTheSurface: an error past the surface asks one
// question, whether its package belongs to the surface; other errors ask none.
// The question is in the printed preview only, never in the request document.
func TestWriteRequestAsksForTheSurface(t *testing.T) {
	past := request.Error{
		Path: "cmd/main.go", Line: 14, Rule: "coupling", From: "cmd", To: moduleTC, Package: "pkg/tc/netem",
		Balance: &result.Balance{Value: 2, Min: 7, Strength: result.StrengthIntrusive, Volatility: result.VolatilityHigh},
	}
	graph := request.Error{Path: "cmd/main.go", Line: 14, Rule: ruleGraph, From: "cmd", To: moduleTC}
	var b strings.Builder
	r := request.Request{Reason: "netem belongs to the surface", Errors: []request.Error{past, graph}}
	if err := writeRequest(&b, r, true, time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	want := "cmd/main.go:14: coupling: cmd -> pkg_tc\n" +
		"  balance: 2/10 (min 7): intrusive, pkg_tc is high\n" +
		"  question: should pkg/tc/netem be part of the surface of pkg_tc?\n" +
		"cmd/main.go:14: graph: cmd -> pkg_tc\n" +
		"request: netem belongs to the surface\n" + askOwner + "\n"
	if got := b.String(); got != want {
		t.Errorf("request:\n--- got\n%s--- want\n%s", got, want)
	}
}
