package setup

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNext(t *testing.T) {
	for _, tc := range []struct {
		state State
		want  string
	}{
		{NoPolicy, "archfit setup"},
		{NoWorkflow, "install the App: " + installURL},
		{Unpublished, "install the App: " + installURL},
		{NeedsScope, "gh auth refresh -s workflow"},
		{ManualPR, "open the pull request: https://github.com/o/r/compare/release%2Fmain...archfit/setup?expand=1"},
		{NoCheck, "install the App: " + installURL},
		{CheckPresent, "merge #7: https://github.com/o/r/pull/7"},
		{Closed, "pull request #7 was closed; reopen it, or delete the branch archfit/setup, then archfit setup"},
		{Merged, ""},
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
	bare := Body("v0.10.0", installURL, nil, nil)
	for _, line := range []string{
		"This pull request sets up archfit 0.10.0:",
		"Checks are advisory: nothing blocks a merge until an owner makes the check required.",
		"1. Install the Archfit App: " + installURL,
		"2. Merge this pull request.",
	} {
		if !strings.Contains(bare, line) {
			t.Errorf("missing %q in %q", line, bare)
		}
	}
	if strings.Contains(bare, "## Last 30") || strings.Contains(bare, "## For the owner") {
		t.Errorf("empty sections rendered: %q", bare)
	}
	full := Body("0.10.0", installURL, &ReplaySummary{Passed: 22, NeededOwner: 5, ToShowOwner: 3}, []string{"exception legacy: expires 2027-02-20"})
	for _, section := range []string{"## Last 30 merged pull requests\n\n22 passed, 5 needed an owner", "## For the owner to decide\n\n- exception legacy: expires 2027-02-20"} {
		if !strings.Contains(full, section) {
			t.Errorf("missing %q in %q", section, full)
		}
	}
}

func TestReplayTimeout(t *testing.T) {
	var out Outcome
	got := out.replay(t.Context(), func(ctx context.Context) (ReplaySummary, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > time.Minute {
			t.Errorf("replay is not bounded to a minute")
		}
		return ReplaySummary{Read: 14}, context.DeadlineExceeded
	})
	if got != nil || strings.Join(out.Lines, "\n") != "replay: stopped after 60 s, 14 of 30 read; run archfit replay later" {
		t.Errorf("summary %v, lines %q", got, out.Lines)
	}
}

func TestShellQuote(t *testing.T) {
	if got := ShellQuote("team's branch; echo bad"); got != "'team'\\''s branch; echo bad'" {
		t.Fatalf("unsafe quoting: %q", got)
	}
}
