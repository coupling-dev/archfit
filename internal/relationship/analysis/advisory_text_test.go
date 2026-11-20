package analysis_test

import (
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/model/graph"
	"github.com/coupling-dev/archfit/internal/policy"
	"github.com/coupling-dev/archfit/internal/relationship"
	"github.com/coupling-dev/archfit/internal/relationship/analysis"
)

func TestCouplingAdvisoryNeverCallsACriticalEdgeCheap(t *testing.T) {
	for _, sev := range []relationship.Severity{relationship.SeverityCritical, relationship.SeverityHigh, relationship.SeverityMedium} {
		for _, dist := range []relationship.Distance{relationship.DistanceCrossModule, relationship.DistanceCrossDeployUnit} {
			why := analysis.BCAdvisoryWhy(relationship.Edge{
				Strength: relationship.StrengthModel, Distance: dist, Volatility: relationship.VolatilityHigh, Severity: sev,
			})
			if strings.Contains(why, "cheap") {
				t.Errorf("%s at %s: why calls the edge cheap: %s", sev, dist, why)
			}
		}
	}
}

// A use past the surface takes the target's volatility (the surfaces design
// 6.1): a high source never raises it, and the why names the volatility that
// applied without attributing it to the source.
func TestCouplingAdvisoryExplainsEffectiveVolatility(t *testing.T) {
	modules := twoModules()
	source := modules[moduleA]
	source.Volatility = volLow
	modules[moduleA] = source
	got := analysis.Analyze(analysis.Input{
		Graph: graphTo(fileB), Policy: relationshipPolicy(modules),
	})
	if edge := onlyEdge(t, got); edge.Volatility != relationship.VolatilityHigh {
		t.Fatalf("effective volatility = %s, want high from the target", edge.Volatility)
	}
	if len(got.Assessment.AdvisoryCandidates) != 1 {
		t.Fatalf("advisories = %d, want 1", len(got.Assessment.AdvisoryCandidates))
	}
	why := got.Assessment.AdvisoryCandidates[0].Why
	if !strings.Contains(why, "high effective volatility") || strings.Contains(why, "higher of source and target") {
		t.Errorf("why = %q, want the target's high effective volatility only", why)
	}
	if strings.Contains(why, "volatile target") || strings.Contains(why, "forces a change") {
		t.Errorf("why overclaims target volatility or certain propagation: %s", why)
	}
}

func TestCouplingAdvisoryExternalEdgeAbstains(t *testing.T) {
	g := graph.Build([]graph.Facts{{
		Nodes: []graph.Node{
			{Kind: graph.NodeKindFile, Path: fileA, Language: graph.LangGo},
			{Kind: graph.NodeKindPackage, Path: "vendor.example/sdk", Language: graph.LangGo},
		},
		Edges: []graph.Edge{{From: nodeA, To: "package:vendor.example/sdk", Kind: graph.EdgeKindImports,
			Language: graph.LangGo, StrengthHint: string(relationship.StrengthFunctional)}},
	}})
	modules := twoModules()
	source := modules[moduleA]
	source.Volatility = volLow
	modules[moduleA] = source
	p := relationshipPolicy(modules)
	p.Topology.ExternalSystems = map[string]policy.ExternalSystemDef{
		"vendor": {Targets: []string{"vendor.example/**"}, Volatility: volHigh},
	}
	got := analysis.Analyze(analysis.Input{Graph: g, Policy: p})
	edge := onlyEdge(t, got)
	if edge.Distance != relationship.DistanceExternal || edge.Volatility != relationship.VolatilityHigh {
		t.Fatalf("external distance/volatility = %s/%s, want declared_external/high", edge.Distance, edge.Volatility)
	}
	// A declared external target has no declared surface, so it has no
	// strength: the edge abstains and raises no advisory (not a clean score).
	if edge.Strength != relationship.StrengthUnknown || edge.Classified.Score.Scored {
		t.Fatalf("external strength %q, scored %v: want unknown and abstained", edge.Strength, edge.Classified.Score.Scored)
	}
	if len(got.Assessment.AdvisoryCandidates) != 0 {
		t.Fatalf("advisories = %d, want 0 for an abstained edge", len(got.Assessment.AdvisoryCandidates))
	}
}
