package replay

import (
	"fmt"
	"io"
	"time"
)

// Text writes the result of a replay. deadline is the time the run was given;
// it is named only when the run stopped, and 0 means the run had none. The
// first line is the one `archfit setup` shows (onboarding texts, section 2).
func (r Result) Text(w io.Writer, deadline time.Duration) error {
	var err error
	line := func(format string, args ...any) {
		if err == nil {
			_, err = fmt.Fprintf(w, format+"\n", args...)
		}
	}
	switch {
	case r.Stopped && deadline == 0:
		line("replay: stopped, %d of %d read", r.Read, r.Total)
	case r.Stopped:
		line("replay: stopped after %s, %d of %d read; run archfit replay later", duration(deadline), r.Read, r.Total)
	default:
		line("replay: %d merged pull requests read: %d passed, %d would have needed an owner, %d to look at first",
			r.Read, r.Passed, r.Asked, r.Look)
	}
	if r.Ref != "" {
		line("source: first-parent history of %s, read locally; each change against its first parent, under the current policy", r.Ref)
	}
	if r.Incomplete {
		line("incomplete history: this clone is shallow and the oldest changes are not in it; run git fetch --unshallow")
	}
	if r.Rebase {
		line("a pull request merged by rebase shows as several changes, one for each commit")
	}
	if len(r.Examples) > 0 {
		line("would have needed an owner, for example:")
		for _, e := range r.Examples {
			line("  %s: %s", e.Label, e.What)
		}
	}
	return err
}

// duration writes a deadline the way the texts do: "60 s".
func duration(d time.Duration) string {
	if d >= time.Second && d%time.Second == 0 {
		return fmt.Sprintf("%d s", d/time.Second)
	}
	return d.String()
}
