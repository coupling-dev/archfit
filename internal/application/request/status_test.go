package request_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/coupling-dev/archfit/internal/application/request"
	"github.com/coupling-dev/archfit/internal/model/result"
)

func TestStatusLines(t *testing.T) {
	story := `<!-- archfit-story v1 {"steps":{"change":"done","check":"failed","ask":"current","answer":"later","policy":"later","recheck":"later"},"request":40,"waiting_for":"alexei-led","since":"2026-11-22T09:12:00Z","next":"wait, or work on something else. The check runs again by itself when #40 is answered."} -->`
	now := time.Date(2026, 11, 22, 9, 13, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		in   request.StatusInput
		want []string
	}{
		{"fail", request.StatusInput{Verdict: result.StatusFail, Fresh: true}, []string{"✓ change   ✗ check", `next: archfit request -m "<why you need it>"`}},
		{"pass", request.StatusInput{Verdict: result.StatusPass, Fresh: true}, []string{"✓ change   ✓ check", "next: push and open a pull request"}},
		{"story", request.StatusInput{Verdict: result.StatusFail, Fresh: true, StoryText: story, Now: now}, []string{"✓ change   ✗ check   ● ask #40   ○ answer   ○ policy   ○ re-check", "request #40 waits for @alexei-led since 09:12 (about 1 minute of their time).", "next: wait, or work on something else. The check runs again by itself when #40 is answered."}},
		{"unknown version", request.StatusInput{Verdict: result.StatusFail, Fresh: true, StoryText: `<!-- archfit-story v2 {} -->`}, []string{"✓ change   ✗ check", "next: update archfit to read the pull request's state"}},
		{"stale", request.StatusInput{Verdict: result.StatusPass}, []string{"✓ change   ○ check", "next: run archfit check"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := request.Status(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
