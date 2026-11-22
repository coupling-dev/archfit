package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func TestSetupFilesDoNotOverwriteForeignWorkflow(t *testing.T) {
	root := t.TempDir()
	f := setupFiles{co: checkout{root: root}, workflow: ".github/workflows/archfit.yaml", body: ".archfit-cache/setup-pull-request.md"}
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
	f := setupFiles{co: checkout{root: root}, workflow: ".github/workflows/archfit.yaml", body: ".github/body.md"}
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
