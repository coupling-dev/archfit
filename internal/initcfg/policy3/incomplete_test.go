package policy3_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/model/evidence"
)

func TestInitKeepsMissingToolEvidence(t *testing.T) {
	const unread = "scripts/run.js"
	for _, tt := range []struct {
		name string
		read []string
	}{
		{"mixed evidence", []string{"main.go"}},
		{"inventory only", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			facts := fixed{
				Sources: tt.read,
				Unknown: []evidence.UnknownItem{{File: unread, Cause: evidence.CauseTool, Reason: "ast-grep is not installed", Fix: "install ast-grep"}},
			}
			root := repo(t, withSources(append(slices.Clone(tt.read), unread), map[string]string{
				gitattrs: "scripts/** linguist-generated=true\n",
			}), origin)
			got, err := initWith(t, facts, root)
			if err != nil {
				t.Fatalf("init with missing tool: %v", err)
			}
			if len(got.Unknown) != 1 || got.Unknown[0].File != unread {
				t.Fatalf("unread evidence = %+v", got.Unknown)
			}
			if _, ok := got.File.Modules["scripts"]; !ok {
				t.Fatal("init did not write the inventoried module")
			}
			if !slices.Equal(got.File.Generated, []string{"scripts/**"}) {
				t.Errorf("generated = %v, want scripts/**", got.File.Generated)
			}
			if policy := readFile(t, filepath.Join(root, ".archfit.yaml")); !strings.Contains(string(policy), "# Draft:") {
				t.Errorf("incomplete evidence is not disclosed in the draft: %s", policy)
			}
		})
	}
}
