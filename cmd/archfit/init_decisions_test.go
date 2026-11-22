package main

import (
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/config/policyfile"
)

func TestOwnerDecisionLines(t *testing.T) {
	public := []string{"api/**"}
	tests := []struct {
		name string
		file policyfile.File
		want string
	}{
		{"exception", policyfile.File{Exceptions: []policyfile.Exception{{Rule: "graph", From: "a", To: "b", Expires: "2027-01-01"}}}, "for the owner to decide:\n  exception: graph a -> b until 2027-01-01\n"},
		{"no public", policyfile.File{Modules: map[string]policyfile.Module{"a": {Volatility: "high"}}}, "for the owner to decide:\n  module a: no public part declared; other modules may use all of it\n"},
		{"volatility set", policyfile.File{Modules: map[string]policyfile.Module{"a": {Public: public, Volatility: "high"}}}, ""},
		{"subdomain set", policyfile.File{Modules: map[string]policyfile.Module{"a": {Public: public, Subdomain: "core"}}}, ""},
		{"empty", policyfile.File{}, ""},
		{"order", policyfile.File{Exceptions: []policyfile.Exception{{Rule: "graph", From: "a", To: "b"}}, Modules: map[string]policyfile.Module{"z": {}, "a": {}}}, "for the owner to decide:\n  exception: graph a -> b\n  module a: no public part declared; other modules may use all of it\n  module z: no public part declared; other modules may use all of it\n  module a: volatility not set; taken as high\n  module z: volatility not set; taken as high\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			writeOwnerDecisions(&b, tt.file)
			if got := b.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
