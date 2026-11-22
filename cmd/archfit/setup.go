package main

import (
	"fmt"
	"strings"

	"github.com/coupling-dev/archfit/internal/application/setup"
)

// SetupCmd advances advisory onboarding on a GitHub repository.
type SetupCmd struct {
	NoHooks bool   `name:"no-hooks" help:"Do not install the Claude Code and Codex hooks."`
	Dir     string `short:"C" name:"directory" help:"Run in DIR instead of the current directory." placeholder:"DIR"`
}

func (*SetupCmd) Help() string {
	return `Set up advisory checks on a GitHub repository.

Run from the default branch, then follow the next: line.
Setup writes a policy and workflow, opens a setup pull request, and shows
how to install the App. Without gh, it prints a link and a pull request body.
It never overwrites a workflow or commits unrelated paths.

Exit codes:
  0  done, or waiting for a person
  2  stopped: wrong usage or a tool error`
}

func (c *SetupCmd) Run(deps *appDeps) error {
	service, err := setupDependencies(deps, c.Dir, c.NoHooks)
	if err != nil {
		_, _ = fmt.Fprintln(deps.Stdout, err)
		_, _ = fmt.Fprintln(deps.Stdout, "state: stopped")
		_, _ = fmt.Fprintln(deps.Stdout, "next: run "+c.command()+" in a git checkout")
		return exitWith(exitUnknown)
	}
	out := setup.Run(deps.runContext(), service)
	for _, line := range out.Lines {
		if _, err := fmt.Fprintln(deps.Stdout, line); err != nil {
			return noVerdict(err)
		}
	}
	if _, err := fmt.Fprintf(deps.Stdout, "state: %s\n", out.State); err != nil {
		return noVerdict(err)
	}
	if out.Next != "" {
		if _, err := fmt.Fprintf(deps.Stdout, "next: %s\n", c.continuation(strings.TrimPrefix(out.Next, "next: "))); err != nil {
			return noVerdict(err)
		}
	}
	for _, line := range out.After {
		if _, err := fmt.Fprintln(deps.Stdout, line); err != nil {
			return noVerdict(err)
		}
	}
	return exitWith(out.Exit)
}

// command preserves the selected checkout and hook mode when setup is resumed.
func (c *SetupCmd) command() string {
	command := setup.Command
	if c.NoHooks {
		command += " --no-hooks"
	}
	if c.Dir != "" {
		command += " -C '" + strings.ReplaceAll(c.Dir, "'", "'\\''") + "'"
	}
	return command
}

func (c *SetupCmd) continuation(next string) string {
	next = strings.ReplaceAll(next, setup.Command+" --no-hooks", setup.Command)
	return strings.ReplaceAll(next, setup.Command, c.command())
}
