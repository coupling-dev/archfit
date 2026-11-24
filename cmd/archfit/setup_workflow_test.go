package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coupling-dev/archfit/internal/application/setup"
	"github.com/coupling-dev/archfit/internal/toolrun"
)

const fixtureSetupWorkflow = //nolint:gosec // test fixture path, not a credential
".github/workflows/archfit.yaml"

func TestSetupWorkflow(t *testing.T) {
	const sha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	got := setupWorkflow("main", "v0.10.0", sha)
	want, err := os.ReadFile("testdata/setup/archfit.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("workflow differs from actionlint's fixture:\n%s", got)
	}
	if strings.Contains(got, "$"+"{{") {
		t.Error("setup workflow must not contain GitHub expressions")
	}
	if odd := setupWorkflow("release/can't", "v0.10.0", sha); !strings.Contains(odd, "branches: ['release/can''t']") {
		t.Errorf("branch was not quoted:\n%s", odd)
	}
}

func TestSetupWorkflowUsesInjectedAppURL(t *testing.T) {
	old := setup.AppURL
	setup.AppURL = "https://candidate-823.coupling.dev"
	t.Cleanup(func() { setup.AppURL = old })
	got := setupWorkflow("main", "candidate 8.23", strings.Repeat("b", 40))
	if !strings.Contains(got, "endpoint: https://candidate-823.coupling.dev") {
		t.Errorf("endpoint was not injected:\n%s", got)
	}
	if strings.Contains(got, "app.coupling.dev") {
		t.Errorf("production endpoint survived the override:\n%s", got)
	}
}

func TestSetupFilesDoNotOverwriteForeignWorkflow(t *testing.T) {
	root := t.TempDir()
	f := setupFiles{co: checkout{root: root}, workflow: fixtureSetupWorkflow, body: ".archfit-cache/setup-pull-request.md"}
	path := filepath.Join(root, f.workflow)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	original := []byte("name: existing\non: push\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Workflow(); err == nil {
		t.Fatal("foreign workflow was accepted")
	}
	if err := f.WriteWorkflow("main", "v0.10.0", strings.Repeat("a", 40)); err == nil {
		t.Fatal("existing workflow overwritten")
	}
	got, err := os.ReadFile(path) //nolint:gosec // fixture path under t.TempDir
	if err != nil || string(got) != string(original) {
		t.Fatalf("workflow changed: %s, %v", got, err)
	}
}

func TestSetupFilesRejectWritesOutsideCheckout(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".github")); err != nil {
		t.Fatal(err)
	}
	f := setupFiles{co: checkout{root: root}, workflow: fixtureSetupWorkflow, body: ".github/body.md"}
	if err := f.WriteWorkflow("main", "v0.10.0", strings.Repeat("a", 40)); err == nil {
		t.Error("workflow escaped checkout")
	}
	if err := f.WriteBody("body"); err == nil {
		t.Error("body escaped checkout")
	}
}

func TestSetupBodyDoesNotRewriteIdenticalContents(t *testing.T) {
	root := t.TempDir()
	f := setupFiles{co: checkout{root: root}, body: ".archfit-cache/setup-pull-request.md"}
	if err := f.WriteBody("body"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, f.body)
	stamp := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteBody("body"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.ModTime().Equal(stamp) {
		t.Fatalf("body rewritten: %v %v", info, err)
	}
}

func TestSetupIgnoresUncommittedAlternateWorkflow(t *testing.T) {
	files := fixtureFiles()
	delete(files, ".github/workflows/archfit.yml")
	dir := fixtureRepo(t, files, nil)
	f := setupFiles{co: checkout{root: dir, runner: toolrun.New()}, workflow: fixtureSetupWorkflow}
	writeFixtureFile(t, dir, ".github/workflows/custom.yml", "jobs:\n  archfit:\n    steps:\n      - uses: coupling-dev/archfit-action@v1\n")
	found, err := f.Workflow()
	if err != nil || found {
		t.Fatalf("untracked alternate workflow: found %v, error %v", found, err)
	}
	gitFixture(t, dir, "add", ".github/workflows/custom.yml")
	gitRun(t, dir, "commit", "-m", "workflow", "--no-gpg-sign")
	found, err = f.Workflow()
	if err != nil || !found {
		t.Fatalf("committed alternate workflow: found %v, error %v", found, err)
	}
}

func TestSetupRejectsInvalidMergedPolicy(t *testing.T) {
	f := setupFiles{}
	if valid, _ := f.ValidPolicy([]byte("version: 1\ninvalid_key: true\n")); valid {
		t.Fatal("invalid version 1 policy accepted")
	}
}

func TestSetupNoHooksExcludesHookFiles(t *testing.T) {
	f := setupFiles{workflow: fixtureSetupWorkflow, noHooks: true}
	for _, path := range f.Paths() {
		if path == harnessConfigFile(harnessClaude) || path == harnessConfigFile(harnessCodex) {
			t.Errorf("disabled hook path included: %s", path)
		}
	}
}

func TestSetupUsesActionRequiresRealWorkflowStep(t *testing.T) {
	for _, tc := range []struct {
		name, yaml string
		want       bool
	}{
		{"step", "jobs:\n  check:\n    steps:\n      - uses: coupling-dev/archfit-action@v1\n", true},
		{"quoted action step", "jobs:\n  check:\n    steps:\n      - uses: 'coupling-dev/archfit-action@v1'\n", true},
		{"script sample", "jobs:\n  check:\n    steps:\n      - run: |\n          uses: coupling-dev/archfit-action@v1\n", false},
		{"bad yaml", "jobs: [", false},
		{"empty ref", "jobs:\n  x:\n    steps:\n      - uses: coupling-dev/archfit-action@\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := setupUsesAction([]byte(tc.yaml)); got != tc.want {
				t.Errorf("got %t want %t", got, tc.want)
			}
		})
	}
}

func TestSetupNoHooksStillValidatesHistoricalHooks(t *testing.T) {
	f := setupFiles{workflow: fixtureSetupWorkflow, noHooks: true}
	for _, hook := range []string{harnessConfigFile(harnessClaude), harnessConfigFile(harnessCodex)} {
		if !slices.Contains(f.ValidationPaths(), hook) {
			t.Errorf("missing historical path %s", hook)
		}
	}
}

func TestSetupPathReadFailureSuggestsRegularFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".github"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := setupFiles{co: checkout{root: root, runner: toolrun.New()}, workflow: fixtureSetupWorkflow, noHooks: true}
	next, err := f.SafePaths()
	if err == nil || next != "replace "+fixtureSetupWorkflow+" with a regular file, then archfit setup" {
		t.Fatalf("next %q error %v", next, err)
	}
}

func TestSetupHookPathsFollowNoHooks(t *testing.T) {
	hooks := []string{harnessConfigFile(harnessClaude), harnessConfigFile(harnessCodex)}
	if got := (setupFiles{workflow: fixtureSetupWorkflow}).HookPaths(); !slices.Equal(got, hooks) {
		t.Errorf("HookPaths() = %v, want %v", got, hooks)
	}
	if got := (setupFiles{workflow: fixtureSetupWorkflow, noHooks: true}).HookPaths(); got != nil {
		t.Errorf("HookPaths() with --no-hooks = %v, want none", got)
	}
}

// TestSetupPinNoteLeavesOutCannotTell: setup prints the pin line only when it
// can compare; a candidate pin is "cannot tell", which only doctor reports.
// setupRelease is the engine version these setup tests pin.
const setupRelease = "v0.10.0"

func TestSetupPinNoteLeavesOutCannotTell(t *testing.T) {
	old := version
	version = setupRelease
	t.Cleanup(func() { version = old })
	sha := strings.Repeat("b", 40)
	for _, tc := range []struct {
		name, comment, want string
	}{
		{"candidate", "candidate", ""},
		{"release", setupRelease, "the local engine matches the workflow pin"},
		{"other release", "v0.9.0", "the local engine does not match the workflow pin (v0.10.0 vs v0.9.0)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, fixtureSetupWorkflow)
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(setupWorkflow("main", tc.comment, sha)), 0o600); err != nil {
				t.Fatal(err)
			}
			f := setupFiles{co: checkout{root: root}, workflow: fixtureSetupWorkflow}
			if got := f.PinNote(); got != tc.want {
				t.Errorf("PinNote() = %q, want %q", got, tc.want)
			}
		})
	}
}
