package record_test

import (
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/model/result"
	"github.com/coupling-dev/archfit/internal/output/record"
)

func TestRecordGroupsRepeatedMissingToolPaths(t *testing.T) {
	const reason = "analyzer is missing"
	u := result.Unknown{Cause: result.CauseTool, What: reason, Next: result.Next{Text: "install the analyzer"}}
	r := result.Result{Summary: "archfit: unknown", Policy: result.Policy{Protected: true}}
	for _, path := range []string{"a.js", "b.js", "c.js", "d.js"} {
		u.Path = path
		r.Unknown = append(r.Unknown, u)
	}
	var out strings.Builder
	if err := record.Result(&out, r, record.Options{}); err != nil {
		t.Fatal(err)
	}
	want := "tool: analyzer is missing (4 paths: a.js, b.js, c.js, ...)\n" +
		"  next: install the analyzer\narchfit: unknown\n"
	if got := out.String(); got != want {
		t.Errorf("record:\n%s\nwant:\n%s", got, want)
	}
	out.Reset()
	if err := record.ResultJSON(&out, r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"path": "d.js"`) {
		t.Errorf("JSON lost a path: %s", &out)
	}
}
