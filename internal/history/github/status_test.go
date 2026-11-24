package requestsend_test

import (
	"context"
	"testing"
	"time"

	requestsend "github.com/coupling-dev/archfit/internal/history/github"
	"github.com/coupling-dev/archfit/internal/toolrun"
)

func TestStoryText(t *testing.T) {
	calls := 0
	runner := &toolrun.RunnerMock{RunFunc: func(_ context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		calls++
		if cmd.Name != "gh" || cmd.Timeout != 10*time.Second {
			t.Errorf("command: %+v", cmd)
		}
		if calls == 1 {
			return toolrun.Output{Stdout: []byte(`{"number":1,"headRefOid":"abc"}`)}, nil
		}
		return toolrun.Output{Stdout: []byte(`{"check_runs":[{"app":{"slug":"archfit-review"},"started_at":"2026-01-01T00:00:00Z","output":{"text":"old"}},{"app":{"slug":"archfit-review"},"started_at":"2026-01-02T00:00:00Z","output":{"text":"new"}}]}`)}, nil
	}}
	got, state := requestsend.StoryText(context.Background(), ".", runner, "archfit-review")
	if got != "new" || state != "" || calls != 2 {
		t.Errorf("got %q, %q, %d calls", got, state, calls)
	}
}

// TestStoryTextReadsOnlyTheBuildsApp: status reads the check of the App this
// build trusts, staging included, and no other App's text (N1).
func TestStoryTextReadsOnlyTheBuildsApp(t *testing.T) {
	const runs = `{"check_runs":[{"app":{"slug":"other"},"started_at":"2026-01-03T00:00:00Z","output":{"text":"malicious"}},` +
		`{"app":{"slug":"archfit-review"},"started_at":"2026-01-02T00:00:00Z","output":{"text":"production"}},` +
		`{"app":{"slug":"archfit-review-staging"},"started_at":"2026-01-01T00:00:00Z","output":{"text":"staging"}},` +
		`{"app":{"slug":""},"started_at":"2026-01-04T00:00:00Z","output":{"text":"no app"}}]}`
	for _, tc := range []struct{ slug, want string }{
		{"archfit-review", "production"},
		{"archfit-review-staging", "staging"},
		{"", ""},
	} {
		t.Run(tc.slug, func(t *testing.T) {
			calls := 0
			runner := &toolrun.RunnerMock{RunFunc: func(_ context.Context, _ toolrun.ToolCmd) (toolrun.Output, error) {
				calls++
				if calls == 1 {
					return toolrun.Output{Stdout: []byte(`{"number":1,"headRefOid":"abc"}`)}, nil
				}
				return toolrun.Output{Stdout: []byte(runs)}, nil
			}}
			text, state := requestsend.StoryText(context.Background(), ".", runner, tc.slug)
			if text != tc.want || state != "" {
				t.Errorf("got %q, %q; want %q", text, state, tc.want)
			}
		})
	}
}
