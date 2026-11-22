package request_test

import (
	"reflect"
	"testing"

	"github.com/coupling-dev/archfit/internal/application/request"
	"github.com/coupling-dev/archfit/internal/model/result"
)

func TestStatusLines(t *testing.T) {
	const failedRow = "✓ change   ✗ check"
	story := `<!-- archfit-story v1 {"steps":{"change":"done","check":"failed","ask":"current","answer":"later","policy":"later","recheck":"later"},"request":40,"waiting_for":"alexei-led","since":"2026-11-22T09:12:00Z","next":"wait, or work on something else. The check runs again by itself when #40 is answered."} -->`
	for _, tc := range []struct {
		name string
		in   request.StatusInput
		want []string
	}{
		{"fail", request.StatusInput{Verdict: result.StatusFail, Fresh: true}, []string{failedRow, `next: archfit request -m "<why you need it>"`}},
		{"pass", request.StatusInput{Verdict: result.StatusPass, Fresh: true}, []string{"✓ change   ✓ check", "next: push and open a pull request"}},
		{"story", request.StatusInput{Verdict: result.StatusFail, Fresh: true, StoryText: story}, []string{"✓ change   ✗ check   ● ask #40   ○ answer   ○ policy   ○ re-check", "request #40 waits for @alexei-led since 09:12 (about 1 minute of their time).", "next: wait, or work on something else. The check runs again by itself when #40 is answered."}},
		{"terminal escapes", request.StatusInput{Verdict: result.StatusFail, Fresh: true, StoryText: `<!-- archfit-story v1 {"steps":{"change":"done","check":"failed"},"request":40,"waiting_for":"a\u001b[31mb\u0007","since":"2026-11-22T09:12:00Z","next":"safe \u001b[31mred\u001b[0m\ntext"} -->`}, []string{failedRow, "request #40 waits for @ab since 09:12 (about 1 minute of their time).", "next: safe redtext"}},
		{"unknown version", request.StatusInput{Verdict: result.StatusFail, Fresh: true, StoryText: `<!-- archfit-story v2 {} -->`}, []string{failedRow, "next: update archfit to read the pull request's state"}},
		{"stale", request.StatusInput{Verdict: result.StatusPass}, []string{"✓ change   ○ check", "next: run archfit check"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := request.Status(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
