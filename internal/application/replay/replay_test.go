package replay_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coupling-dev/archfit/internal/application/replay"
	"github.com/coupling-dev/archfit/internal/model/result"
)

type history struct {
	src replay.Source
	err error
	n   int
}

func (h *history) Changes(_ context.Context, n int) (replay.Source, error) {
	h.n = n
	return h.src, h.err
}

// checker answers by commit; a commit with no answer is an error.
type checker struct {
	results map[string]result.Result
	errs    map[string]error
	// onCheck runs before each answer, with the commit.
	onCheck func(commit string)
	calls   []string
}

func (c *checker) Check(_ context.Context, ch replay.Change) (result.Result, error) {
	c.calls = append(c.calls, ch.Commit)
	if c.onCheck != nil {
		c.onCheck(ch.Commit)
	}
	if err := c.errs[ch.Commit]; err != nil {
		return result.Result{}, err
	}
	return c.results[ch.Commit], nil
}

func change(commit string, merge bool) replay.Change {
	return replay.Change{Commit: commit, Parent: commit + "-parent", Merge: merge, Subject: "subject " + commit}
}

func pass() result.Result { return result.Result{Status: result.StatusPass} }

func asks(what string) result.Result {
	return result.Result{Status: result.StatusFail, Errors: []result.Error{
		{What: "fix me", Next: result.Next{Kind: result.NextFix}},
		{What: what, Next: result.Next{Kind: result.NextCommand}},
	}}
}

func TestRunClassifiesEveryChange(t *testing.T) {
	fixOnly := result.Result{Status: result.StatusFail, Errors: []result.Error{{What: "x", Next: result.Next{Kind: result.NextFix}}}}
	waiting := result.Result{Status: result.StatusWaiting}
	cases := []struct {
		name string
		res  result.Result
		want replay.Result
	}{
		{"pass is quiet", pass(), replay.Result{Passed: 1}},
		{"a fail that asks an owner", asks("a -> b"), replay.Result{Asked: 1}},
		{"a fail that says to fix the code is to look at", fixOnly, replay.Result{Look: 1}},
		{"no verdict is to look at", result.Result{Status: result.StatusUnknown}, replay.Result{Look: 1}},
		{"waiting is to look at", waiting, replay.Result{Look: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := replay.Service{
				History: &history{src: replay.Source{Ref: "origin/main", Changes: []replay.Change{change("c1", true)}}},
				Checker: &checker{results: map[string]result.Result{"c1": tc.res}},
			}
			got, err := svc.Run(context.Background(), 30)
			if err != nil {
				t.Fatal(err)
			}
			if got.Passed != tc.want.Passed || got.Asked != tc.want.Asked || got.Look != tc.want.Look || got.Read != 1 || got.Stopped {
				t.Errorf("got %+v, want %+v read 1", got, tc.want)
			}
		})
	}
}

func TestRunCountsAFailedCheckAsToLookAt(t *testing.T) {
	svc := replay.Service{
		History: &history{src: replay.Source{Changes: []replay.Change{change("c1", true), change("c2", true)}}},
		Checker: &checker{
			results: map[string]result.Result{"c2": pass()},
			errs:    map[string]error{"c1": errors.New("git worktree add failed")},
		},
	}
	got, err := svc.Run(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if got.Look != 1 || got.Passed != 1 || got.Read != 2 {
		t.Errorf("got %+v, want 1 to look at, 1 passed, 2 read", got)
	}
}

func TestRunDoesNotCheckAChangeWithNoParent(t *testing.T) {
	root := replay.Change{Commit: "root", Subject: "first"}
	chk := &checker{}
	svc := replay.Service{History: &history{src: replay.Source{Changes: []replay.Change{root}}}, Checker: chk}
	got, err := svc.Run(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.Look != 1 || got.Passed != 0 || len(chk.calls) != 0 {
		t.Errorf("got %+v after %v, want one change to look at and no check", got, chk.calls)
	}
}

func TestRunNamesAtMostThreeExamples(t *testing.T) {
	changes := []replay.Change{change("c1", true), change("c2", true), change("c3", true), change("c4", true)}
	changes[1].PR, changes[1].Title = 12, "Add billing"
	results := map[string]result.Result{}
	for _, c := range changes {
		results[c.Commit] = asks("dep " + c.Commit)
	}
	svc := replay.Service{History: &history{src: replay.Source{Changes: changes}}, Checker: &checker{results: results}}
	got, err := svc.Run(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if got.Asked != 4 || len(got.Examples) != 3 {
		t.Fatalf("got %d asked and %d examples, want 4 and 3", got.Asked, len(got.Examples))
	}
	if got.Examples[0].Label != "c1 subject c1" || got.Examples[1].Label != "#12 Add billing" || got.Examples[1].What != "dep c2" {
		t.Errorf("examples %+v", got.Examples)
	}
}

func TestRunStopsWhenTheDeadlineEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes := []replay.Change{change("c1", true), change("c2", true), change("c3", true), change("c4", true)}
	results := map[string]result.Result{"c1": pass(), "c2": pass(), "c3": pass(), "c4": pass()}
	chk := &checker{results: results, errs: map[string]error{"c3": context.DeadlineExceeded}}
	chk.onCheck = func(commit string) {
		if commit == "c3" {
			cancel()
		}
	}
	svc := replay.Service{History: &history{src: replay.Source{Changes: changes}}, Checker: chk}
	got, err := svc.Run(ctx, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Stopped || got.Read != 2 || got.Total != 4 || got.Passed != 2 || got.Look != 0 {
		t.Errorf("got %+v, want stopped after 2 of 4 read, the change in flight not counted", got)
	}
	if len(chk.calls) != 3 {
		t.Errorf("checked %v, want it to stop at c3", chk.calls)
	}
}

func TestRunStopsBeforeTheFirstChangeWhenTheDeadlineHasEnded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	chk := &checker{}
	svc := replay.Service{History: &history{src: replay.Source{Changes: []replay.Change{change("c1", true)}}}, Checker: chk}
	got, err := svc.Run(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Stopped || got.Read != 0 || got.Total != 1 || len(chk.calls) != 0 {
		t.Errorf("got %+v after %v", got, chk.calls)
	}
}

func TestRunReturnsAHistoryError(t *testing.T) {
	want := errors.New("no default branch")
	svc := replay.Service{History: &history{err: want}, Checker: &checker{}}
	if _, err := svc.Run(context.Background(), 30); !errors.Is(err, want) {
		t.Errorf("got %v, want %v", err, want)
	}
}

func TestRunPassesTheCountToTheHistory(t *testing.T) {
	h := &history{}
	if _, err := (replay.Service{History: h, Checker: &checker{}}).Run(context.Background(), 17); err != nil {
		t.Fatal(err)
	}
	if h.n != 17 {
		t.Errorf("history asked for %d changes, want 17", h.n)
	}
}

func TestRunNotesARebaseOnlyWhenNoChangeIsAMergeCommit(t *testing.T) {
	cases := []struct {
		name    string
		changes []replay.Change
		want    bool
	}{
		{"linear history", []replay.Change{change("c1", false), change("c2", false)}, true},
		{"a merge commit", []replay.Change{change("c1", false), change("c2", true)}, false},
		{"no changes", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := replay.Service{History: &history{src: replay.Source{Changes: tc.changes}}, Checker: &checker{results: map[string]result.Result{}}}
			got, err := svc.Run(context.Background(), 5)
			if err != nil {
				t.Fatal(err)
			}
			if got.Rebase != tc.want {
				t.Errorf("Rebase = %v, want %v", got.Rebase, tc.want)
			}
		})
	}
}

func TestText(t *testing.T) {
	cases := []struct {
		name     string
		res      replay.Result
		deadline time.Duration
		want     string
	}{
		{
			name: "complete, with every note",
			res: replay.Result{
				Ref: "origin/main", Total: 30, Read: 30, Passed: 22, Asked: 5, Look: 3,
				Incomplete: true, Rebase: true,
				Examples: []replay.Example{{Label: "#12 Add billing", What: "billing uses reports"}},
			},
			want: "replay: 30 merged pull requests read: 22 passed, 5 would have needed an owner, 3 to look at first\n" +
				"source: first-parent history of origin/main, read locally; each change against its first parent, under the current policy\n" +
				"incomplete history: this clone is shallow and the oldest changes are not in it; run git fetch --unshallow\n" +
				"a pull request merged by rebase shows as several changes, one for each commit\n" +
				"would have needed an owner, for example:\n" +
				"  #12 Add billing: billing uses reports\n",
		},
		{
			name:     "stopped by the deadline",
			res:      replay.Result{Ref: "origin/main", Total: 30, Read: 14, Passed: 14, Stopped: true},
			deadline: 60 * time.Second,
			want: "replay: stopped after 60 s, 14 of 30 read; run archfit replay later\n" +
				"source: first-parent history of origin/main, read locally; each change against its first parent, under the current policy\n",
		},
	}
	cases = append(cases, struct {
		name     string
		res      replay.Result
		deadline time.Duration
		want     string
	}{
		name: "stopped with no deadline",
		res:  replay.Result{Total: 30, Read: 3, Stopped: true},
		want: "replay: stopped, 3 of 30 read\n",
	})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tc.res.Text(&buf, tc.deadline); err != nil {
				t.Fatal(err)
			}
			if buf.String() != tc.want {
				t.Errorf("got:\n%s\nwant:\n%s", buf.String(), tc.want)
			}
		})
	}
}
