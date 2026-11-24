// Package replay is `archfit replay --last N` (plan 8.4, C5): the policy run
// over the last N changes of the default branch, to say how many would have
// passed and how many would have needed an owner.
//
// # The changes
//
// The changes are the first-parent commits of the default branch, newest
// first. Each is compared with its first parent: a merge commit and a squash
// merge are one change each, and a pull request merged by rebase shows as
// several. GitHub only labels a change with its pull request; it never
// changes which changes are counted, so the result is the same with or
// without it.
//
// # The classes
//
// Every change is judged under the policy of the working tree, not the
// policy the change had.
//
//	passed      the status is pass
//	would ask   the status is fail and an error asks an owner: its next step
//	            is `archfit request`, as in internal/application/request
//	to look at  everything else: a fail that only says to fix the code, no
//	            verdict, a change with no parent in this clone, a check that
//	            failed. Missing evidence is never counted as a pass.
//
// # The deadline
//
// Run stops when its context ends. The result then holds the changes read so
// far and Stopped; the change that was being read is not counted.
//
// The package reads no process and no file: cmd/archfit wires the ports.
package replay

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/coupling-dev/archfit/internal/model/result"
)

// maxExamples is how many changes of each named class the result names.
const maxExamples = 3

// subjectNumber finds the pull request number GitHub writes into the subject
// of a squash merge, "Title (#12)", or of a merge commit, "Merge pull request
// #12 from ...".
var subjectNumber = regexp.MustCompile(`\(#([0-9]+)\)\s*$|^Merge pull request #([0-9]+)\b`)

// Change is one first-parent commit of the default branch.
type Change struct {
	// Commit is the full commit id.
	Commit string
	// Parent is the first parent; empty for the first commit of the
	// repository and for the oldest commit of a shallow clone.
	Parent string
	// Merge is true for a commit with more than one parent.
	Merge bool
	// Subject is the first line of the commit message.
	Subject string
	// PR is the number of the merged pull request, 0 when not known.
	PR int
	// Title is the title of that pull request.
	Title string
}

// Label names the change for a person: "#12 title", or the short commit id and
// subject when no pull request is known.
func (c Change) Label() string {
	if c.PR > 0 {
		return fmt.Sprintf("#%d %s", c.PR, c.Title)
	}
	return c.short() + " " + c.Subject
}

// Name names the change in a list: "#12", the number GitHub wrote into the
// subject when no pull request is known, or else the short commit id.
func (c Change) Name() string {
	if c.PR > 0 {
		return "#" + strconv.Itoa(c.PR)
	}
	if m := subjectNumber.FindStringSubmatch(c.Subject); m != nil {
		return "#" + m[1] + m[2]
	}
	return c.short()
}

func (c Change) short() string {
	if len(c.Commit) > 7 {
		return c.Commit[:7]
	}
	return c.Commit
}

// Source is the history a replay reads.
type Source struct {
	// Ref is the default branch the changes come from, such as origin/main.
	Ref string
	// Changes are newest first, at most the number asked for.
	Changes []Change
	// Short is true when the oldest change has no parent because the clone
	// is shallow.
	Short bool
}

// History lists the changes of the default branch.
type History interface {
	// Changes returns the last n first-parent changes of the default branch
	// as this clone holds them. It does not fetch.
	Changes(ctx context.Context, n int) (Source, error)
}

// Checker judges one change under the current policy.
type Checker interface {
	// Check returns the result of the change: the commit against its first
	// parent. An error is a failure to run the check; a ended context must
	// come back as an error.
	Check(ctx context.Context, c Change) (result.Result, error)
}

// Service is the `archfit replay` use case. Both ports are required.
type Service struct {
	History History
	Checker Checker
}

// Example is a change that would have needed an owner.
type Example struct {
	// Label is Change.Label.
	Label string
	// What is what the first error that asks an owner says.
	What string
}

// Look is a change to look at first.
type Look struct {
	// Name is Change.Name.
	Name string
	// Status is what an owner would see on it: fail, waiting or unknown.
	// A change with no parent, or a check that could not run, is unknown.
	Status string
}

// Result is what a replay found.
type Result struct {
	// Source is the branch the changes were read from.
	Ref string
	// Total is the number of changes the replay was to read.
	Total int
	// Read is the number of changes judged. Read < Total only when Stopped.
	Read int
	// Passed, Asked and Look are the three classes; they add up to Read.
	Passed, Asked, Look int
	// Examples are up to three of the changes that would ask, newest first.
	Examples []Example
	// LookAt are up to three of the changes to look at first, newest first.
	LookAt []Look
	// Stopped is true when the context ended before every change was read.
	Stopped bool
	// Incomplete is true when the clone is shallow and the history it holds
	// ends in the changes read.
	Incomplete bool
	// Rebase is true when no change is a merge commit: a pull request merged
	// by rebase can then show as several changes.
	Rebase bool
}

// Run reads the last n changes. An error is a failure of the history; a
// failed check is a change to look at, and an ended context stops the run.
func (s Service) Run(ctx context.Context, n int) (Result, error) {
	src, err := s.History.Changes(ctx, n)
	if err != nil {
		if ctx.Err() != nil {
			return Result{Stopped: true}, nil
		}
		return Result{}, err
	}
	res := Result{Ref: src.Ref, Total: len(src.Changes), Incomplete: src.Short, Rebase: linear(src.Changes)}
	for _, c := range src.Changes {
		if ctx.Err() != nil {
			res.Stopped = true
			break
		}
		class, what, err := s.judge(ctx, c)
		if err != nil && ctx.Err() != nil {
			res.Stopped = true
			break
		}
		res.Read++
		switch class {
		case classPassed:
			res.Passed++
		case classAsked:
			res.Asked++
			if len(res.Examples) < maxExamples {
				res.Examples = append(res.Examples, Example{Label: c.Label(), What: what})
			}
		default:
			res.Look++
			if len(res.LookAt) < maxExamples {
				res.LookAt = append(res.LookAt, Look{Name: c.Name(), Status: what})
			}
		}
	}
	return res, nil
}

type class int

const (
	classLook class = iota
	classPassed
	classAsked
)

// judge classifies one change. what is the first error that asks an owner,
// or the status of a change to look at.
func (s Service) judge(ctx context.Context, c Change) (cl class, what string, err error) {
	unknown := string(result.StatusUnknown)
	if c.Parent == "" {
		return classLook, unknown, nil
	}
	r, err := s.Checker.Check(ctx, c)
	if err != nil {
		return classLook, unknown, err
	}
	switch r.Status {
	case result.StatusPass:
		return classPassed, "", nil
	case result.StatusFail:
		for _, e := range r.Errors {
			if e.Next.Kind == result.NextCommand {
				return classAsked, e.What, nil
			}
		}
	}
	if r.Status == "" {
		return classLook, unknown, nil
	}
	return classLook, string(r.Status), nil
}

// linear reports whether the changes hold no merge commit.
func linear(changes []Change) bool {
	if len(changes) == 0 {
		return false
	}
	for _, c := range changes {
		if c.Merge {
			return false
		}
	}
	return true
}
