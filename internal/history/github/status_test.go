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
		return toolrun.Output{Stdout: []byte(`{"check_runs":[{"started_at":"2026-01-01T00:00:00Z","output":{"text":"old"}},{"started_at":"2026-01-02T00:00:00Z","output":{"text":"new"}}]}`)}, nil
	}}
	got, state := requestsend.StoryText(context.Background(), ".", runner)
	if got != "new" || state != "" || calls != 2 {
		t.Errorf("got %q, %q, %d calls", got, state, calls)
	}
}
