package main

import (
	"errors"
	"fmt"
)

// ReplayCmd is `archfit replay` (internal/application/replay): the policy run
// over the last changes of the default branch. It judges every change under
// the policy of the working tree and reads this clone only.
type ReplayCmd struct {
	Last int    `default:"30" help:"How many of the last changes of the default branch to read." placeholder:"N"`
	Dir  string `short:"C" name:"directory" help:"Run in DIR instead of the current directory." placeholder:"DIR"`
}

func (*ReplayCmd) Help() string {
	return `Run the policy over the last changes of the default branch, to see how many
would have passed and how many would have needed an owner.

The changes are the first-parent commits of the default branch, as this clone
holds them: archfit reads no remote. A merge commit and a squash merge are one
change; a pull request merged by rebase shows as several. Each change is
checked against its first parent, under the policy in the working tree. When gh
is signed in, it only adds the pull request number and title to a change.

Exit codes:
  0  the replay ran
  2  no default branch, no valid policy, wrong usage, or a tool error

Example:
  archfit replay --last 30`
}

// errNoReplayChanges is the usage error of --last.
var errNoReplayChanges = errors.New("--last must be at least 1")

// Run has no deadline of its own. `archfit setup` runs the same service
// (checkout.replay) under a context of 60 seconds (plan C5).
func (c *ReplayCmd) Run(deps *appDeps) error {
	if c.Last < 1 {
		return noVerdict(errNoReplayChanges)
	}
	ctx := deps.runContext()
	co, err := openCheckout(ctx, deps, dirOr(c.Dir))
	if err != nil {
		return noVerdict(err)
	}
	// The policy is read before any change is checked out: a missing or
	// invalid one would make every change unknown.
	if _, err := (localPolicy{co}).Policy(ctx); err != nil {
		return noVerdict(err)
	}
	svc := co.replay(deps)
	res, err := svc.Run(ctx, c.Last)
	if err != nil {
		return noVerdict(fmt.Errorf("replay: %w", err))
	}
	if err := res.Text(deps.Stdout, 0); err != nil {
		return noVerdict(err)
	}
	return nil
}
