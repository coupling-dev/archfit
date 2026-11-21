package reporttext

import (
	"strconv"
	"strings"

	"github.com/coupling-dev/archfit/internal/model/report"
)

// groupUnread groups only path-level copies of the same reason and fix.
func groupUnread(items []report.UnreadPlace) []report.UnreadPlace {
	const maxPaths = 3
	type key struct{ reason, fix string }
	type group struct {
		index, count int
		paths        [maxPaths]string
	}
	groups := map[key]group{}
	out := make([]report.UnreadPlace, 0, len(items))
	for _, item := range items {
		if item.Path == "" {
			out = append(out, item)
			continue
		}
		k := key{item.Reason, item.Fix}
		g, ok := groups[k]
		if !ok {
			g.index = len(out)
			out = append(out, item)
		}
		if g.count < maxPaths {
			g.paths[g.count] = location(item.Path, item.Line)
		}
		g.count++
		groups[k] = g
	}
	for k, g := range groups {
		if g.count < 2 {
			continue
		}
		paths := strings.Join(g.paths[:min(maxPaths, g.count)], ", ")
		if g.count > maxPaths {
			paths += ", ..."
		}
		out[g.index] = report.UnreadPlace{Reason: k.reason + " (" + strconv.Itoa(g.count) + " paths: " + paths + ")", Fix: k.fix}
	}
	return out
}
