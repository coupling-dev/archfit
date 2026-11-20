package evaluation

import (
	"github.com/coupling-dev/archfit/internal/assessment/finding"
	"github.com/coupling-dev/archfit/internal/assessment/result"
	"github.com/coupling-dev/archfit/internal/assessment/state"
	"github.com/coupling-dev/archfit/internal/policy"
)

type stateInput struct {
	Policy policy.PolicySnapshot
	Facts  Observations
}

func classifyFindings(findings []finding.Finding) map[string][]state.FindingRef {
	out := map[string][]state.FindingRef{}
	for _, f := range findings {
		if f.Status != finding.StatusNew || f.Kind != finding.KindAdvisory {
			continue
		}
		dim := dimensionForRule(f.RuleID)
		out[dim] = append(out[dim], state.FindingRef{
			ID: f.ID, RuleID: f.RuleID, Kind: f.Kind, Severity: string(f.Severity), Status: string(f.Status),
		})
	}
	return out
}

func buildState(diag *result.Result, in stateInput) state.Architecture {
	st := state.New()
	split := classifyFindings(diag.Findings)
	st.Dimensions = buildDimensions(diag, in, split)
	return st
}
