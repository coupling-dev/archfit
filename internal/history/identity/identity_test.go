package identity

import (
	"context"
	"testing"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

func TestOwnerIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, gh, ident, want string
		verified              bool
	}{
		{"managed person", "mona_org User", "", "mona_org", true},
		{"person", "alice User", "A <bob@users.noreply.github.com> 1 +0000", "alice", true},
		{"org", "acme Organization", "A <bob@users.noreply.github.com> 1 +0000", "bob", false},
		{"noreply id", "", "A <42+bob@users.noreply.github.com> 1 +0000", "bob", false},
		{"no identity", "", "A <a@example.com> 1 +0000", PlaceholderOwner, false},
		{"bot", "robot[bot] Bot", "A <42+robot[bot]@users.noreply.github.com> 1 +0000", PlaceholderOwner, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &toolrun.RunnerMock{RunFunc: func(_ context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
				if cmd.Name == "gh" {
					return toolrun.Output{Stdout: []byte(tc.gh)}, nil
				}
				return toolrun.Output{Stdout: []byte(tc.ident)}, nil
			}}
			got, verified := OwnerIdentity(context.Background(), ".", r)
			if got != tc.want || verified != tc.verified {
				t.Fatalf("got %q, %v", got, verified)
			}
		})
	}
}
