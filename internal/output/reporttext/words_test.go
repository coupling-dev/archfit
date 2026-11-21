package reporttext_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/model/report"
	"github.com/coupling-dev/archfit/internal/output/reporttext"
)

func TestReportTextNamesOriginsAndGroupsUnread(t *testing.T) {
	d := state(empty(report.RepositoryClean))
	d.State.Evidence.Snapshot.Unknowns = nil
	for _, origin := range []string{"asserted", "assessed", "observed", "assumed", "unknown"} {
		d.State.Evidence.Components = append(d.State.Evidence.Components, report.EvidenceComponent{Volatility: report.EvidenceStringFact{Origin: origin}})
	}
	for _, path := range []string{"a.js", "b.js", "c.js", "d.js"} {
		d.State.Repository.Unread = append(d.State.Repository.Unread, report.UnreadPlace{Path: path, Reason: "ast-grep missing", Fix: "install ast-grep"})
	}
	var b bytes.Buffer
	if err := reporttext.New().Render(d, &b); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"volatility origins: declared 1, seen 2, assumed 1, unknown 1",
		"ast-grep missing (4 paths: a.js, b.js, c.js, ...)",
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, &b)
		}
	}
	if strings.Count(b.String(), "install ast-grep") != 1 {
		t.Errorf("repeated install hint:\n%s", &b)
	}
	data, err := json.Marshal(d.State.Repository)
	if err != nil || !bytes.Contains(data, []byte("d.js")) {
		t.Errorf("JSON must retain all paths: %s, %v", data, err)
	}
}

func TestReportKeepsDistinctUnreadReasons(t *testing.T) {
	const installA = "install a"
	const missing = "missing"
	r := empty(report.RepositoryClean)
	r.Unread = []report.UnreadPlace{
		{Path: "a.js", Reason: missing, Fix: installA},
		{Path: "b.js", Reason: missing, Fix: "install b"},
		{Reason: missing, Fix: installA},
	}
	d := state(r)
	var b bytes.Buffer
	if err := reporttext.New().Render(d, &b); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"a.js: missing", "b.js: missing", installA, "install b"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("report lacks %q: %s", want, &b)
		}
	}
	if strings.Contains(b.String(), "paths:") {
		t.Errorf("grouped different fixes: %s", &b)
	}
}

func TestReportOriginLabelsLeaveJSONUnchanged(t *testing.T) {
	d := state(empty(report.RepositoryClean))
	d.State.Evidence.Components = []report.EvidenceComponent{{Volatility: report.EvidenceStringFact{Origin: "assessed"}}}
	before, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := reporttext.New().Render(d, &b); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(d)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("report text changed JSON: %v", err)
	}
}
