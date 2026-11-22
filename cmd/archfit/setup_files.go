package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/coupling-dev/archfit/internal/config/policyfile"
	"github.com/coupling-dev/archfit/internal/toolrun"
)

const (
	setupCheckIgnore = "check-ignore"
	workflowYAML     = ".yaml"
	workflowYML      = ".yml"
)

type setupFiles struct {
	co             checkout
	workflow, body string
	noHooks        bool
}

func (f setupFiles) SafePaths() (string, error) {
	for _, rel := range f.ValidationPaths() {
		path := filepath.Join(f.co.root, rel)
		if !within(f.co.root, path) {
			return rel + " is a symbolic link; replace it with a file, then archfit setup", fmt.Errorf("%s is a symbolic link or resolves outside the checkout", rel)
		}
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return rel, err
		}
		if !info.Mode().IsRegular() {
			return rel + " is a symbolic link; replace it with a file, then archfit setup", fmt.Errorf("%s is a symbolic link or not a regular file", rel)
		}
		out, err := f.co.runner.Run(context.Background(), toolrun.ToolCmd{
			Name: gitBinary, WorkDir: f.co.root, Args: []string{setupCheckIgnore, "--", rel},
		})
		if err != nil {
			return "check git ignore rules, then archfit setup", err
		}
		if out.ExitCode == 0 {
			return rel + " is ignored by git; setup will not publish it. Remove it from .gitignore or move it, then archfit setup", fmt.Errorf("%s is ignored by git", rel)
		}
		if out.ExitCode != 1 {
			return "check git ignore rules, then archfit setup", fmt.Errorf("git check-ignore exited %d", out.ExitCode)
		}
	}
	return "", nil
}

func (f setupFiles) Policy() (bool, error) {
	data, found, err := f.co.policyAt("")
	if err != nil || !found {
		return found, err
	}
	if !isVersion1(data) {
		_, err := policyfile.Decode(data)
		return false, fmt.Errorf("%s: %w", defaultConfigPath, err)
	}
	return true, nil
}

func (f setupFiles) Workflow() (bool, error) {
	path := filepath.Join(f.co.root, f.workflow)
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		if err != nil {
			return false, err
		}
		if !within(f.co.root, path) {
			return false, fmt.Errorf("%s resolves outside the checkout", f.workflow)
		}
		data, err := os.ReadFile(path) //nolint:gosec // setup workflow inside this checkout
		if err != nil {
			return false, err
		}
		if !setupUsesAction(data) {
			return false, fmt.Errorf("%s exists and does not use %s; setup does not overwrite it", f.workflow, actionRepo)
		}
		return true, nil
	}
	entries, err := os.ReadDir(filepath.Join(f.co.root, workflowsDir))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() || (filepath.Ext(entry.Name()) != workflowYAML && filepath.Ext(entry.Name()) != workflowYML) {
			continue
		}
		name := filepath.ToSlash(filepath.Join(workflowsDir, entry.Name()))
		// Only a workflow committed on HEAD can accompany the setup commit.
		out, err := f.co.runner.Run(context.Background(), toolrun.ToolCmd{
			Name: gitBinary, WorkDir: f.co.root, Args: []string{"show", "HEAD:" + name},
		})
		if err != nil {
			return false, err
		}
		if out.ExitCode == 0 && setupUsesAction(out.Stdout) {
			return true, nil
		}
	}
	return false, nil
}

func (f setupFiles) WriteWorkflow(branch, tag, sha string) error {
	path := filepath.Join(f.co.root, f.workflow)
	if !within(f.co.root, path) {
		return fmt.Errorf("%s resolves outside the checkout", f.workflow)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec // a new repository workflow
	if err != nil {
		return err
	}
	_, err = file.WriteString(setupWorkflow(branch, tag, sha))
	return errors.Join(err, file.Close())
}

func setupWorkflow(branch, tag, sha string) string {
	return fmt.Sprintf(`name: archfit
on:
  pull_request:
  push:
    branches: ['%s']
  merge_group:
    types: [checks_requested]

permissions:
  contents: read
  id-token: write

jobs:
  archfit:
    runs-on: ubuntu-latest
    steps:
      - uses: %s@%s # %s
        with:
          endpoint: https://app.coupling.dev
`, strings.ReplaceAll(branch, "'", "''"), actionRepo, sha, tag)
}

func (f setupFiles) ValidPolicy(data []byte) bool {
	_, err := policyfile.Decode(data)
	return err == nil
}

func (f setupFiles) PinNote() string { return doctorPin(f.co.root, version) }

func (f setupFiles) Exceptions() ([]string, error) {
	data, _, err := f.co.policyAt("")
	if err != nil {
		return nil, err
	}
	policy, err := policyfile.Decode(data)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, x := range policy.Exceptions {
		place := x.From
		if x.To != "" {
			place += " -> " + x.To
		}
		line := "exception " + x.Rule
		if place != "" {
			line += " (" + place + ")"
		}
		if x.Expires != "" {
			line += ": expires " + x.Expires
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func (f setupFiles) WriteBody(body string) error {
	path := filepath.Join(f.co.root, f.body)
	if !within(f.co.root, path) {
		return fmt.Errorf("%s resolves outside the checkout", f.body)
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", f.body)
	}
	if prior, err := os.ReadFile(path); err == nil && string(prior) == body { //nolint:gosec // setup's own cache path
		return nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return writeFileAtomic(path, []byte(body), 0o600)
}

func (f setupFiles) ValidationPaths() []string {
	return []string{defaultConfigPath, f.workflow, "AGENTS.md", ".gitignore", harnessConfigFile(harnessClaude), harnessConfigFile(harnessCodex)}
}

func (f setupFiles) Paths() []string {
	paths := f.ValidationPaths()
	if f.noHooks {
		return paths[:len(paths)-2]
	}
	return paths
}

func setupUsesAction(data []byte) bool {
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Uses string `yaml:"uses"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if yaml.Unmarshal(data, &workflow) != nil {
		return false
	}
	for _, job := range workflow.Jobs {
		for _, step := range job.Steps {
			if ref, ok := strings.CutPrefix(step.Uses, actionRepo+"@"); ok && ref != "" && !strings.ContainsAny(ref, " \t\r\n") {
				return true
			}
		}
	}
	return false
}
