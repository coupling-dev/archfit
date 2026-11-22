package requestsend_test

import (
	"context"
	"errors"
	"io"
	"testing"

	github "github.com/coupling-dev/archfit/internal/history/github"
	"github.com/coupling-dev/archfit/internal/toolrun"
)

type ghRunner struct {
	out  toolrun.Output
	err  error
	args []string
}

func (r *ghRunner) Detect(context.Context, string) (toolrun.ToolInfo, bool) {
	return toolrun.ToolInfo{}, false
}

func (r *ghRunner) Run(_ context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
	r.args = append([]string{cmd.Name}, cmd.Args...)
	return r.out, r.err
}

func (r *ghRunner) Stream(context.Context, toolrun.ToolCmd, func(io.Reader) error) (toolrun.Output, error) {
	return toolrun.Output{}, errors.New("not used")
}

func TestMergedPulls(t *testing.T) {
	const list = `[{"number":12,"title":"Add billing","mergeCommit":{"oid":"aaa"}},
		{"number":13,"title":"No merge commit","mergeCommit":null},
		{"number":14,"title":"Fix","mergeCommit":{"oid":"bbb"}}]`
	cases := []struct {
		name string
		out  toolrun.Output
		err  error
		want map[string]github.Pull
	}{
		{"a list", toolrun.Output{Stdout: []byte(list)}, nil,
			map[string]github.Pull{"aaa": {Number: 12, Title: "Add billing"}, "bbb": {Number: 14, Title: "Fix"}}},
		{"gh missing", toolrun.Output{}, errors.New("executable file not found"), nil},
		{"not signed in", toolrun.Output{ExitCode: 4, Stderr: []byte("gh auth login")}, nil, nil},
		{"not json", toolrun.Output{Stdout: []byte("oops")}, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := github.MergedPulls(context.Background(), t.TempDir(), &ghRunner{out: tc.out, err: tc.err})
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("%s: got %v, want %v", k, got[k], v)
				}
			}
		})
	}
}
