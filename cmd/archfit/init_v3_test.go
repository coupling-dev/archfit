package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/toolrun"

	"github.com/coupling-dev/archfit/internal/initcfg/policy3"
)

// initGoldens holds the text of `archfit init`.
const initGoldens = "testdata/init"

const (
	cmdInit      = "init"
	originRemote = "https://github.com/alexei-led/pumba.git"
	agentsNotes  = "# Agents\n\nRun make test before you push.\n"
)

// archfit3Policy is a version 2 policy, the Archfit 3 format.
const archfit3Policy = "version: 2\nmodules: {}\n"

// initFixture is the fixture code without a policy: the copied literal of
// pkg/tc is an error that exists, which init writes as an exception.
func initFixture() map[string]string {
	files := fixtureFiles()
	delete(files, defaultConfigPath)
	files[tcInterface] = tcCopiesLiteral
	files[policy3.AgentsFile] = agentsNotes
	return files
}

// initRepo is fixtureRepo with an origin remote on GitHub, for the owners.
func initRepo(t *testing.T, base map[string]string) string {
	t.Helper()
	dir := fixtureRepo(t, base, nil)
	gitRun(t, dir, "remote", "add", "origin", originRemote)
	return dir
}

// commitAll commits the working tree on the branch of the change.
func commitAll(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-qm", "init", "--no-gpg-sign")
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // fixture files
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestInitAFreshRepository: init writes the policy and the block, lists the
// error that exists as an exception, writes no workflow, and the first check
// passes.
func TestInitAFreshRepository(t *testing.T) {
	t.Parallel()
	dir := initRepo(t, initFixture())
	var stdout, stderr bytes.Buffer
	runner := &toolrun.RunnerMock{DetectFunc: toolrun.New().Detect, StreamFunc: toolrun.New().Stream, RunFunc: func(ctx context.Context, cmd toolrun.ToolCmd) (toolrun.Output, error) {
		if cmd.Name == "gh" {
			return toolrun.Output{Stdout: []byte("alice User")}, nil
		}
		return toolrun.New().Run(ctx, cmd)
	}}
	got := cliRun{code: runDeps([]string{cmdInit, "-C", dir}, &appDeps{Runner: runner, Stdout: &stdout, Stderr: &stderr, now: day(fixtureDate)})}
	got.stdout, got.stderr = stdout.String(), stderr.String()
	if got.code != 0 || got.stderr != "" {
		t.Fatalf("init: exit %d, stderr %q", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "exception: duplication pkg_tc -> pkg_util until 2027-02-08") {
		t.Errorf("init must list dated decisions: %s", got.stdout)
	}
	if policy := readText(t, filepath.Join(dir, defaultConfigPath)); !strings.Contains(policy, "expires: \"2027-02-08\"") {
		t.Errorf("init must date the exception: %s", policy)
	}
	assertCLIGolden(t, initGoldens, "fresh", got.stdout)

	if policy := readText(t, filepath.Join(dir, defaultConfigPath)); !isVersion1([]byte(policy)) {
		t.Errorf("the written policy is not version 1:\n%s", policy)
	}
	if agents := readText(t, filepath.Join(dir, policy3.AgentsFile)); agents != agentsNotes+"\n"+policy3.Block {
		t.Errorf("AGENTS.md =\n%s\nwant the notes, then the block", agents)
	}
	workflows, err := os.ReadDir(filepath.Join(dir, workflowsDir))
	if err != nil || len(workflows) != 1 {
		t.Errorf("workflows = %v, %v; want only the fixture's own (D11)", workflows, err)
	}

	commitAll(t, dir)
	check := runCLI(t, fixtureDate, cmdCheck, "-C", dir)
	if check.code != 0 || check.stdout != "archfit: pass\n" {
		t.Errorf("check after init: exit %d\n%s%s", check.code, check.stdout, check.stderr)
	}
}

// TestInitWithAVersion1Policy: init leaves the policy as it is and writes
// only a missing block; a second run writes nothing. Both exit 0.
func TestInitWithAVersion1Policy(t *testing.T) {
	t.Parallel()
	dir := initRepo(t, fixtureFiles())
	before := readText(t, filepath.Join(dir, defaultConfigPath))

	for _, name := range []string{"version3_block", "version3_nothing"} {
		got := runCLI(t, fixtureDate, cmdInit, "-C", dir)
		if got.code != 0 || got.stderr != "" {
			t.Fatalf("%s: exit %d, stderr %q", name, got.code, got.stderr)
		}
		assertCLIGolden(t, initGoldens, name, got.stdout)
	}
	if after := readText(t, filepath.Join(dir, defaultConfigPath)); after != before {
		t.Errorf("init changed the policy:\n%s", after)
	}
	if agents := readText(t, filepath.Join(dir, policy3.AgentsFile)); agents != policy3.Block {
		t.Errorf("AGENTS.md =\n%s\nwant the block", agents)
	}
}

// TestInitErrorsExit2: a real error exits 2, names its cause, and writes
// nothing.
func TestInitErrorsExit2(t *testing.T) {
	t.Parallel()
	unbalanced := initFixture()
	unbalanced[policy3.AgentsFile] = "<!-- archfit:start -->\nno end\n"
	otherVersion := initFixture()
	otherVersion[defaultConfigPath] = version3
	archfit3 := initFixture()
	archfit3[defaultConfigPath] = archfit3Policy
	for _, tc := range []struct {
		name   string
		files  map[string]string
		remote bool
		want   string
	}{
		{name: "unbalanced markers", files: unbalanced, remote: true, want: "markers are unbalanced"},
		{name: "a version 3 policy", files: otherVersion, remote: true, want: "version 3 is not supported"},
		{name: "a version 2 policy", files: archfit3, remote: true, want: "version 2 is not supported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := fixtureRepo(t, tc.files, nil)
			if tc.remote {
				gitRun(t, dir, "remote", "add", "origin", originRemote)
			}
			got := runCLI(t, fixtureDate, cmdInit, "-C", dir)
			if got.code != exitUnknown || got.stdout != "" || !strings.Contains(got.stderr, tc.want) {
				t.Errorf("exit %d, stdout %q, stderr %q; want 2 and %q", got.code, got.stdout, got.stderr, tc.want)
			}
			if _, had := tc.files[defaultConfigPath]; had {
				if policy := readText(t, filepath.Join(dir, defaultConfigPath)); policy != tc.files[defaultConfigPath] {
					t.Errorf("init changed the policy:\n%s", policy)
				}
			} else if _, err := os.Stat(filepath.Join(dir, defaultConfigPath)); !os.IsNotExist(err) {
				t.Errorf("init wrote a policy: %v", err)
			}
			if agents := readText(t, filepath.Join(dir, policy3.AgentsFile)); agents != tc.files[policy3.AgentsFile] {
				t.Errorf("init changed AGENTS.md:\n%s", agents)
			}
		})
	}

	got := runCLI(t, fixtureDate, cmdInit, "-C", t.TempDir())
	if got.code != exitUnknown || !strings.Contains(got.stderr, notARepo) {
		t.Errorf("outside a repository: exit %d, stderr %q", got.code, got.stderr)
	}
}

func TestInitInstallsAgentHooks(t *testing.T) {
	dir := initRepo(t, initFixture())
	claude := harnessConfigFile(harnessClaude)
	codex := harnessConfigFile(harnessCodex)
	writeFixtureFile(t, dir, claude, userClaudeSettings)
	chdirIsolated(t, dir)
	subdir := filepath.Join(dir, "pkg", "tc")

	before := hookStates(doctorJSON(t).AgentLoop)
	if got := before["claude Stop"]; got != hookInactive+" ("+reasonNotInstalled+")" {
		t.Fatalf("doctor before init: Claude Stop = %q", got)
	}
	first := runCLI(t, fixtureDate, cmdInit, "-C", subdir)
	if first.code != 0 || !strings.Contains(first.stdout, codexTrustNotice) {
		t.Fatalf("init: %+v", first)
	}
	after := hookStates(doctorJSON(t).AgentLoop)
	for hook, want := range map[string]string{
		"claude Stop":          hookActive,
		harnessCodex + " Stop": hookInactive + " (" + reasonNotTrusted + ")",
		"git pre-commit":       hookInactive + " (" + reasonNotInstalled + ")",
	} {
		if got := after[hook]; got != want {
			t.Errorf("doctor after init: %s = %q, want %q", hook, got, want)
		}
	}
	wantClaude := readText(t, filepath.Join(dir, claude))
	if !strings.Contains(wantClaude, "opus") || !strings.Contains(wantClaude, "echo bye") {
		t.Errorf("init lost existing settings or hooks: %s", wantClaude)
	}
	wantCodex := readText(t, filepath.Join(dir, codex))
	if rerun := runCLI(t, fixtureDate, cmdInit, "-C", subdir); rerun.code != 0 {
		t.Fatalf("repeat init: %+v", rerun)
	}
	for path, want := range map[string]string{claude: wantClaude, codex: wantCodex} {
		if got := readText(t, filepath.Join(dir, path)); got != want {
			t.Errorf("repeat init changed %s", path)
		}
	}
}

func TestInitNoHooks(t *testing.T) {
	dir := initRepo(t, initFixture())
	claude := harnessConfigFile(harnessClaude)
	writeFixtureFile(t, dir, claude, userClaudeSettings)
	got := runCLI(t, fixtureDate, cmdInit, "--no-hooks", "-C", dir)
	if got.code != 0 {
		t.Fatalf("init --no-hooks: %+v", got)
	}
	if after := readText(t, filepath.Join(dir, claude)); after != userClaudeSettings {
		t.Errorf("init --no-hooks changed Claude settings: %s", after)
	}
	if _, err := os.Stat(filepath.Join(dir, harnessConfigFile(harnessCodex))); !os.IsNotExist(err) {
		t.Errorf("Codex config should not exist, stat error = %v", err)
	}
}

func TestInitDoesNotWriteHooksOutsideCheckout(t *testing.T) {
	dir := initRepo(t, initFixture())
	outside := t.TempDir()
	rel := harnessConfigFile(harnessClaude)
	target := filepath.Join(outside, filepath.Base(rel))
	writeFixtureFile(t, outside, filepath.Base(rel), userClaudeSettings)
	if err := os.Symlink(outside, filepath.Join(dir, filepath.Dir(rel))); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, fixtureDate, cmdInit, "-C", dir)
	if got.code != 0 || !strings.Contains(got.stdout, "resolves outside the repository") {
		t.Fatalf("init: %+v", got)
	}
	if after := readText(t, target); after != userClaudeSettings {
		t.Errorf("init changed external Claude settings: %s", after)
	}
	if _, err := os.Stat(filepath.Join(dir, harnessConfigFile(harnessCodex))); err != nil {
		t.Errorf("Codex config missing: %v", err)
	}
}
