package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coupling-dev/archfit/internal/history/identity"
	"github.com/coupling-dev/archfit/internal/toolrun"
)

func TestDoctorOwners(t *testing.T) {
	for _, tc := range []struct {
		name, owners, result string
		fail                 bool
		calls                int
	}{
		{"placeholder", identity.PlaceholderOwner, "", true, 0}, {"org", "acme", "Organization", true, 1}, {"person", testOwnerLogin, "User", false, 1}, {"managed", "mona_org", "User", false, 1}, {"offline", testOwnerLogin, "", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, defaultConfigPath), []byte("version: 1\nowners: [\""+tc.owners+"\"]\nmodules:\n  app: {paths: ['**']}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			runner := &toolrun.RunnerMock{RunFunc: func(_ context.Context, c toolrun.ToolCmd) (toolrun.Output, error) {
				calls++
				if c.Name != "gh" || c.Timeout != 5*time.Second {
					t.Fatalf("unexpected command: %+v", c)
				}
				if tc.result == "" {
					return toolrun.Output{ExitCode: 1}, nil
				}
				return toolrun.Output{Stdout: []byte(tc.result)}, nil
			}}
			out, err := doctorOwners(context.Background(), root, runner)
			if (err != nil) != tc.fail || calls != tc.calls {
				t.Fatalf("out %q err %v calls %d", out, err, calls)
			}
			if tc.name != "person" && tc.name != "managed" && !strings.Contains(out, "next:") {
				t.Fatalf("missing next: %s", out)
			}
		})
	}
}
