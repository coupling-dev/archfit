package main

import (
	"fmt"

	"github.com/coupling-dev/archfit/internal/application/request"
	"github.com/coupling-dev/archfit/internal/application/setup"
	requestsend "github.com/coupling-dev/archfit/internal/history/github"
)

type StatusCmd struct {
	Dir string `short:"C" name:"directory" help:"Run in DIR instead of the current directory." placeholder:"DIR"`
}

func (c *StatusCmd) Run(deps *appDeps) error {
	ctx := deps.runContext()
	in := request.StatusInput{}
	co, err := openCheckout(ctx, deps, dirOr(c.Dir))
	if err == nil {
		last, ok, lastErr := (lastReader{co}).Last(ctx)
		current, checkoutErr := (checkoutReader{co}).Checkout(ctx)
		if lastErr == nil && checkoutErr == nil && ok && last.Check != nil && last.Tree != "" && last.Tree == current.Tree {
			in.Fresh = true
			in.Verdict = last.Check.Status
		}
		in.StoryText, _ = requestsend.StoryText(ctx, co.root, co.runner, setup.AppSlug())
	}
	for _, line := range request.Status(in) {
		_, _ = fmt.Fprintln(deps.Stdout, line)
	}
	return nil
}
