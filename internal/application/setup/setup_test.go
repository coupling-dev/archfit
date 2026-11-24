package setup

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coupling-dev/archfit/internal/testutil"
)

func TestNext(t *testing.T) {
	for _, tc := range []struct {
		state State
		want  string
	}{
		{NoPolicy, "archfit setup"},
		{NoWorkflow, "install the App: " + InstallURL},
		{Unpublished, "install the App: " + InstallURL},
		{NeedsScope, "gh auth refresh -s workflow"},
		{ManualPR, "open the pull request: https://github.com/o/r/compare/release%2Fmain...archfit/setup?expand=1"},
		{OfflinePR, "open the pull request: https://github.com/o/r/compare/release%2Fmain...archfit/setup?expand=1"},
		{NoCheck, "install the App: " + InstallURL},
		{CheckPresent, "merge #7: https://github.com/o/r/pull/7"},
		{Closed, "pull request #7 was closed; reopen it, or delete the branch archfit/setup, then archfit setup"},
		{Advisory, "open the setup page: " + AppURL},
		{Required, "check that a branch ruleset requires archfit/gate: https://github.com/o/r/settings/rules"},
		{Blocked, ""},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			p := Progress{State: tc.state, Repo: "o/r", Default: "release/main", PR: PullRequest{Number: 7, URL: "https://github.com/o/r/pull/7"}}
			if got := Next(p); got != tc.want {
				t.Errorf("next: %q, want %q", got, tc.want)
			}
		})
	}
}

// TestNextUsesTheInjectedAppURL: a candidate build sets AppURL with -X, and
// the setup page it names must be that App, not the production one.
func TestNextUsesTheInjectedAppURL(t *testing.T) {
	old := AppURL
	AppURL = "https://candidate.example.test"
	t.Cleanup(func() { AppURL = old })
	if got := Next(Progress{State: Advisory}); got != "open the setup page: https://candidate.example.test" {
		t.Errorf("next: %q", got)
	}
}

func TestTag(t *testing.T) {
	const release = "v0.10.0"
	for version, want := range map[string]string{
		"0.10.0": release, release: release, "v0.10.0-rc.1": "v0.10.0-rc.1",
		devBuild: "", "v0.10.0-0.20261122101010-abcdefabcdef": "", "v0.10.0+dirty": "",
	} {
		t.Run(version, func(t *testing.T) {
			got, err := Tag(version)
			if got != want || (err != nil) != (want == "") {
				t.Errorf("Tag(%q) = %q, %v; want %q", version, got, err, want)
			}
		})
	}
}

func TestBody(t *testing.T) {
	bare := Body("v0.10.0", InstallURL, nil, nil)
	for _, line := range []string{
		"This pull request sets up archfit 0.10.0:",
		"Checks are advisory: nothing blocks a merge until an owner makes the check required.",
		"1. Install the Archfit App: " + InstallURL,
		"2. Merge this pull request.",
	} {
		if !strings.Contains(bare, line) {
			t.Errorf("missing %q in %q", line, bare)
		}
	}
	if strings.Contains(bare, "## Last 30") || strings.Contains(bare, "## For the owner") {
		t.Errorf("empty sections rendered: %q", bare)
	}
	full := Body("0.10.0", InstallURL, &ReplaySummary{Passed: 22, NeededOwner: 5, ToShowOwner: 3}, []string{"exception legacy: expires 2027-02-20"})
	for _, section := range []string{"## Last 30 merged pull requests\n\n22 passed, 5 needed an owner", "## For the owner to decide\n\n- exception legacy: expires 2027-02-20"} {
		if !strings.Contains(full, section) {
			t.Errorf("missing %q in %q", section, full)
		}
	}
}

func TestReplayTimeout(t *testing.T) {
	var out Outcome
	got := out.replay(t.Context(), "main", func(ctx context.Context, _ string) (ReplaySummary, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > time.Minute {
			t.Errorf("replay is not bounded to a minute")
		}
		return ReplaySummary{Read: 14}, context.DeadlineExceeded
	})
	if words := testutil.StoryNotUsedIn(strings.Join(out.Lines, "\n")); len(words) > 0 {
		t.Errorf("replay timeout vocabulary %v in %q", words, out.Lines)
	}
	if got != nil || strings.Join(out.Lines, "\n") != "replay: stopped after 60 s, 14 of 30 read; run archfit replay later" {
		t.Errorf("summary %v, lines %q", got, out.Lines)
	}
}

func TestShellQuote(t *testing.T) {
	if got := ShellQuote("team's branch; echo bad"); got != "'team'\\''s branch; echo bad'" {
		t.Fatalf("unsafe quoting: %q", got)
	}
}

func TestAppSlugComesFromTheInstallURL(t *testing.T) {
	for _, tc := range []struct{ url, want string }{
		{"https://github.com/apps/archfit-review/installations/new", "archfit-review"},
		{"https://github.com/apps/archfit-review-staging/installations/new", "archfit-review-staging"},
		{"https://github.com/apps/archfit-review-staging", "archfit-review-staging"},
		{"https://github.com/apps//installations/new", ""},
		{"https://example.com/apps/x/installations/new", ""},
		{"", ""},
	} {
		if got := appSlug(tc.url); got != tc.want {
			t.Errorf("appSlug(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
	if AppSlug() != "archfit-review" {
		t.Errorf("default build trusts %q", AppSlug())
	}
}
