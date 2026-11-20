package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/toolrun"
)

func TestInitIgnoresCacheIdempotently(t *testing.T) {
	dir := initRepo(t, initFixture())
	writeFixtureFile(t, dir, ".gitignore", "vendor/\n")
	for range 2 {
		got := runCLI(t, fixtureDate, cmdInit, "--no-hooks", "-C", dir)
		if got.code != 0 {
			t.Fatalf("init: %+v", got)
		}
	}
	got := readText(t, filepath.Join(dir, ".gitignore"))
	if got != "vendor/\n.archfit-cache/\n" {
		t.Fatalf(".gitignore = %q", got)
	}
}

func TestDoctorRelevantToolsAndWorkflowPin(t *testing.T) {
	dir := fixRepo(t, map[string]string{
		markerGoMod:                   "module example.com/a\n\ngo 1.27\n",
		"main.go":                     "package main\n",
		".github/workflows/check.yml": "jobs:\n  check:\n    steps:\n      - uses: coupling-dev/archfit-action@abc # v0.9.0\n",
	})
	old := version
	version = "v0.9.0"
	t.Cleanup(func() { version = old })
	out := runDoctorCmd(t, dir)
	if strings.Contains(out, "cargo") || strings.Contains(out, "dotnet") || !strings.Contains(out, "local engine matches the workflow pin") {
		t.Fatalf("doctor: %s", out)
	}
	if err := os.WriteFile(filepath.Join(dir, ".github/workflows/check.yml"), []byte("jobs:\n  x:\n    steps:\n      - uses: coupling-dev/archfit-action@abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out = runDoctorCmd(t, dir)
	if !strings.Contains(out, "cannot tell") {
		t.Fatalf("doctor missing pin comment: %s", out)
	}
}

func TestDoctorPinFromLocalWorkflow(t *testing.T) {
	const (
		pinned = "      - uses: coupling-dev/archfit-action@abc"
		match  = "the local engine matches the workflow pin"
	)
	for _, tt := range []struct{ name, lines, want string }{
		{"match", pinned + " # v0.9.0\n", match},
		{"quoted", "      - uses: 'coupling-dev/archfit-action@abc' # v0.9.0\n", match},
		{"mismatch", pinned + " # v0.8.0\n", "the local engine does not match"},
		{"conflict", pinned + " # v0.9.0\n" + pinned + " # v0.8.0\n", unknownWorkflowPin},
		{"missing", pinned + "\n", unknownWorkflowPin},
		{"commented", "      # - uses: coupling-dev/archfit-action@abc # v0.9.0\n", unknownWorkflowPin},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeFixtureFile(t, root, ".github/workflows/test.yaml", "jobs:\n  check:\n    steps:\n"+tt.lines)
			if got := doctorPin(root, "v0.9.0"); !strings.HasPrefix(got, tt.want) {
				t.Errorf("pin status = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInitIgnoresCacheDespiteEarlierNegation(t *testing.T) {
	dir := initRepo(t, initFixture())
	const before = ".archfit-cache/\n!.*/\n"
	writeFixtureFile(t, dir, ".gitignore", before)
	for range 2 {
		if got := runCLI(t, fixtureDate, cmdInit, "--no-hooks", "-C", dir); got.code != 0 {
			t.Fatalf("init: %+v", got)
		}
	}
	if got := readText(t, filepath.Join(dir, ".gitignore")); got != before+".archfit-cache/\n" {
		t.Fatalf(".gitignore = %q", got)
	}
}

func TestDoctorDoesNotRequireAnalyzerForVendoredOrCachedScripts(t *testing.T) {
	dir := fixRepo(t, map[string]string{
		markerGoMod:                             "module example.com/go-only\n\ngo 1.27\n",
		"vendor/example/app.js":                 "export {}\n",
		cacheDirName + "/worktrees/base/app.js": "export {}\n",
	})
	out := runDoctorCmd(t, dir)
	if doctorRow(testAstGrep, "missing", "").MatchString(out) {
		t.Errorf("doctor reported ast-grep for a vendored or cached script:\n%s", out)
	}
}

func TestInitReportsUnreadPathsAndOneInstallHint(t *testing.T) {
	files := initFixture()
	paths := []string{"scripts/a.js", "scripts/b.js"}
	for _, path := range paths {
		files[path] = "export {}"
	}
	dir := initRepo(t, files)
	runner := toolrun.New()
	withoutAstGrep := &toolrun.RunnerMock{
		DetectFunc: func(ctx context.Context, name string) (toolrun.ToolInfo, bool) {
			if name == testAstGrep {
				return toolrun.ToolInfo{}, false
			}
			return runner.Detect(ctx, name)
		},
		RunFunc: runner.Run,
	}
	var stdout, stderr bytes.Buffer
	code := runDeps([]string{cmdInit, "--no-hooks", "-C", dir}, &appDeps{
		Runner: withoutAstGrep, Stdout: &stdout, Stderr: &stderr, now: day(fixtureDate),
	})
	if code != 0 {
		t.Fatalf("init exit %d: %s\n%s", code, &stdout, &stderr)
	}
	out := stdout.String()
	if !strings.Contains(out, "draft policy: some evidence was not read") || strings.Count(out, "fix: install ast-grep:") != 1 {
		t.Errorf("init did not show partial evidence and one install hint:\n%s", out)
	}
	for _, path := range paths {
		if !strings.Contains(out, "not read: "+path+":") {
			t.Errorf("init omitted %s:\n%s", path, out)
		}
	}
}

func TestDoctorDoesNotRequirePythonToolsForExcludedPackages(t *testing.T) {
	dir := fixRepo(t, map[string]string{
		"pkg/__init__.py": "",
		defaultConfigPath: "version: 1\nowners: [owner]\nmodules:\n  app: {paths: ['**']}\nexclude: ['pkg/**']\n",
	})
	out := runDoctorCmd(t, dir)
	if doctorRow(testAstGrep, "missing", "").MatchString(out) {
		t.Errorf("doctor reported ast-grep for an excluded Python package:\n%s", out)
	}
}
