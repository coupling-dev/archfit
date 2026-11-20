// Package evaluation calculates report measurements from acquired observations.
package evaluation

import (
	"time"

	"github.com/coupling-dev/archfit/internal/assessment/finding"
	"github.com/coupling-dev/archfit/internal/assessment/result"
	signal "github.com/coupling-dev/archfit/internal/assessment/signals"
	"github.com/coupling-dev/archfit/internal/assessment/staleness"
	"github.com/coupling-dev/archfit/internal/model/evidence"
	"github.com/coupling-dev/archfit/internal/model/symbol"
	"github.com/coupling-dev/archfit/internal/policy"
	"github.com/coupling-dev/archfit/internal/relationship"
)

// Input carries the measurements and relationship advisories of one report.
type Input struct {
	Relationships      relationship.Set
	Metrics            Metricset
	Signals            signal.RunSignals
	Symbols            symbol.Graph
	Coverage           []evidence.Coverage
	ChangedFiles       []string
	Policy             policy.AssessmentPolicy
	Now                time.Time
	AdvisoryCandidates []relationship.AdvisoryCandidate
	IncludeAdvisories  bool
}

// Result contains measurements and visible advisories, without a verdict.
type Result struct {
	Findings []finding.Finding
	Metrics  []result.MetricResult
	Warnings int
}

func evaluate(in Input) Result {
	collected := signal.CollectedSignals{
		Common: signal.CommonInput{Relationships: in.Relationships, Coverage: signal.NewCoverageView(in.Coverage), ChangedFiles: in.ChangedFiles, Symbols: signal.SymbolSignals{Graph: in.Symbols}},
		Symbol: signal.SymbolSignals{Graph: in.Symbols}, Size: in.Signals.Size,
	}
	calculated := make([]result.MetricResult, 0, in.Metrics.Len())
	for _, metric := range in.Metrics.metrics {
		calculated = append(calculated, metric.Calculate(collected))
	}
	visible := []finding.Finding{}
	if in.IncludeAdvisories {
		visible = candidateFindings(in.AdvisoryCandidates)
		visible = append(visible, staleness.Check(in.Relationships, in.Policy, in.Now)...)
		visible = groupBCAdvisories(visible)
	}
	return Result{Findings: visible, Metrics: calculated, Warnings: len(visible)}
}
