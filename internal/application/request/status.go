package request

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coupling-dev/archfit/internal/model/result"
)

type StatusInput struct {
	Verdict   result.Status
	Fresh     bool
	StoryText string
	Now       time.Time
}

type Story struct {
	Steps      map[string]string `json:"steps"`
	Request    int               `json:"request"`
	WaitingFor string            `json:"waiting_for"`
	Since      string            `json:"since"`
	Next       string            `json:"next"`
}

func ParseStory(text string) (Story, bool) {
	const prefix = "<!-- archfit-story v1 "
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) && strings.HasSuffix(line, " -->") {
			var story Story
			err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(line, prefix), " -->")), &story)
			return story, err == nil
		}
	}
	return Story{}, false
}

func Status(in StatusInput) []string {
	row := "✓ change   ○ check"
	if !in.Fresh {
		return []string{row, "next: run archfit check"}
	}
	switch in.Verdict {
	case result.StatusFail:
		row = "✓ change   ✗ check"
	case result.StatusPass:
		row = "✓ change   ✓ check"
	}
	if story, ok := ParseStory(in.StoryText); ok {
		var steps []string
		for _, step := range []struct{ key, label string }{{"change", "change"}, {"check", "check"}, {"ask", "ask"}, {"answer", "answer"}, {"policy", "policy"}, {"recheck", "re-check"}} {
			value := story.Steps[step.key]
			mark := map[string]string{"done": "✓", "failed": "✗", "current": "●", "later": "○"}[value]
			if mark == "" {
				continue
			}
			label := step.label
			if step.key == "ask" && story.Request > 0 {
				label += fmt.Sprintf(" #%d", story.Request)
			}
			steps = append(steps, mark+" "+label)
		}
		lines := []string{strings.Join(steps, "   ")}
		if since, err := time.Parse(time.RFC3339, story.Since); err == nil && story.Request > 0 && story.WaitingFor != "" {
			minutes := int(in.Now.Sub(since).Minutes())
			if minutes < 1 {
				minutes = 1
			}
			unit := "minutes"
			if minutes == 1 {
				unit = "minute"
			}
			lines = append(lines, fmt.Sprintf("request #%d waits for @%s since %s (about %d %s of their time).", story.Request, story.WaitingFor, since.Format("15:04"), minutes, unit))
		}
		return append(lines, "next: "+story.Next)
	}
	if strings.Contains(in.StoryText, "<!-- archfit-story v") && !strings.Contains(in.StoryText, "<!-- archfit-story v1 ") {
		return []string{row, "next: update archfit to read the pull request's state"}
	}
	switch in.Verdict {
	case result.StatusFail:
		return []string{row, `next: archfit request -m "<why you need it>"`}
	case result.StatusPass:
		return []string{row, "next: push and open a pull request"}
	default:
		return []string{row, "next: run archfit check"}
	}
}
