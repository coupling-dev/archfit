// Worked examples of bc_score.v7 (docs/coupling.md). Each test
// builds the scenario with the names the example uses and checks the score the
// design promises, so a change to distance or strength rules fails here first.
package analysis_test

import (
	"testing"

	"github.com/coupling-dev/archfit/internal/model/graph"
	"github.com/coupling-dev/archfit/internal/policy"
	"github.com/coupling-dev/archfit/internal/relationship"
	"github.com/coupling-dev/archfit/internal/relationship/analysis"
)

// exampleEdge is one import. The declared surface of the target's module
// gives its strength; an edge carries no type hint, which could not change it.
type exampleEdge struct {
	fromFile, toFile string
}

func exampleGraph(edges ...exampleEdge) *graph.Graph {
	var nodes []graph.Node
	es := make([]graph.Edge, 0, len(edges))
	seen := map[string]bool{}
	node := func(path string) {
		if !seen[path] {
			seen[path] = true
			nodes = append(nodes, graph.Node{Kind: graph.NodeKindFile, Path: path, Language: graph.LangGo})
		}
	}
	for _, e := range edges {
		node(e.fromFile)
		node(e.toFile)
		es = append(es, graph.Edge{
			From: "file:" + e.fromFile, To: "file:" + e.toFile, Kind: graph.EdgeKindImports, Language: graph.LangGo,
			Locations: []graph.Location{{File: e.fromFile, Line: 1}},
		})
	}
	return graph.Build([]graph.Facts{{Nodes: nodes, Edges: es}})
}

func analyze(modules map[string]policy.ModuleDef, edges ...exampleEdge) relationship.AnalysisResult {
	return analysis.Analyze(analysis.Input{Graph: exampleGraph(edges...), Policy: relationshipPolicy(modules)})
}

func seamBetween(t *testing.T, res relationship.AnalysisResult, from, to string) relationship.Seam {
	t.Helper()
	for _, s := range res.Assessment.Seams {
		if s.FromModule == from && s.ToModule == to {
			return s
		}
	}
	t.Fatalf("seam %s -> %s not found", from, to)
	return relationship.Seam{}
}

func requireScore(t *testing.T, res relationship.AnalysisResult, wantStrength relationship.Strength, wantBalance int, wantBand relationship.Severity) relationship.Edge {
	t.Helper()
	e := onlyEdge(t, res)
	if e.Strength != wantStrength || e.Classified.Score.Balance != wantBalance || e.Severity != wantBand {
		t.Fatalf("edge = %s balance %d band %q (distance %s, volatility %s), want %s balance %d band %q",
			e.Strength, e.Classified.Score.Balance, e.Severity, e.Distance, e.Volatility, wantStrength, wantBalance, wantBand)
	}
	return e
}

const (
	fileAPIX            = "a/api/x.go"
	fileAPIY            = "b/api/y.go"
	fileOrders          = "orders/o.go"
	subdomainSupporting = "supporting"
	volMedium           = "medium"
	fileCart            = "sales/cart/c.go"
)

func core(owner, deploy string, paths ...string) policy.ModuleDef {
	return policy.ModuleDef{Paths: paths, Owner: owner, DeployUnit: deploy, Subdomain: subdomainCore}
}

// Example 1: one owner, one deploy unit. `orders` calls `billing`, which
// declares no surface, so the whole module is its public part. The call is
// contract coupling at D=9: balanced, for any spelling of the module keys.
func TestV7Example1_PortCallInOneService(t *testing.T) {
	for name, keys := range map[string][2]string{
		"flat keys":   {"orders", "billing"},
		"nested keys": {"internal/shop/orders", "internal/shop/deep/billing"},
	} {
		t.Run(name, func(t *testing.T) {
			modules := map[string]policy.ModuleDef{
				keys[0]: core("team", "svc", "orders/**"),
				keys[1]: core("team", "svc", "billing/**"),
			}
			res := analyze(modules, exampleEdge{fromFile: fileOrders, toFile: "billing/b.go"})
			requireScore(t, res, relationship.StrengthContract, 9, relationship.SeverityNone)
		})
	}
}

// Example 2: an adapter reads `billing/store`, past the public part `billing`
// declares. Intrusive at D=9 into a core module is critical; into a supporting
// module the target's low volatility rescues it.
func TestV7Example2_InternalReachInOneService(t *testing.T) {
	billing := func(subdomain string) policy.ModuleDef {
		return policy.ModuleDef{Paths: []string{"billing/**"}, Public: []string{"billing/api/**"}, Owner: "team", DeployUnit: "svc", Subdomain: subdomain}
	}
	tests := []struct {
		subdomain string
		balance   int
		band      relationship.Severity
	}{
		{"core", 2, relationship.SeverityCritical},
		{subdomainSupporting, 8, relationship.SeverityLow},
	}
	for _, tt := range tests {
		t.Run(tt.subdomain, func(t *testing.T) {
			modules := map[string]policy.ModuleDef{
				"adapter": core("team", "svc", "adapter/**"),
				"billing": billing(tt.subdomain),
			}
			res := analyze(modules, exampleEdge{fromFile: "adapter/a.go", toFile: "billing/store/s.go"})
			requireScore(t, res, relationship.StrengthIntrusive, tt.balance, tt.band)
		})
	}
}

// Example 4: two deploy units. `web` calls the public part of `inventory`. A
// call through the declared surface is contract and never reads as a
// distributed monolith.
func TestV7Example4_PortCallAcrossDeployUnits(t *testing.T) {
	modules := map[string]policy.ModuleDef{
		"web":       core("team-a", "web", "web/**"),
		"inventory": core("team-b", "inventory", "inventory/**"),
	}
	modules["inventory"] = policy.ModuleDef{Paths: []string{"inventory/**"}, Public: []string{"inventory/api/**"}, Owner: "team-b", DeployUnit: "inventory", Subdomain: subdomainCore}
	res := analyze(modules, exampleEdge{fromFile: "web/w.go", toFile: "inventory/api/r.go"})
	e := requireScore(t, res, relationship.StrengthContract, 9, relationship.SeverityNone)
	if e.Distance != relationship.DistanceCrossDeployUnit {
		t.Errorf("distance = %s, want cross_deploy_unit", e.Distance)
	}
	if seamBetween(t, res, "web", "inventory").DistributedMonolith {
		t.Error("a port call across deploy units must never qualify as a distributed monolith")
	}
}

// Example 5: nested modules. `sales/cart` reaches past the declared public
// part of its sibling `sales/invoice` and of `stock/lookup` on another branch.
// Both uses are intrusive into core modules, so both are critical, and the
// seam names the container each pair shares.
func TestV7Example5_NestedModulesScoreAlike(t *testing.T) {
	withPublic := func(def policy.ModuleDef, public string) policy.ModuleDef {
		def.Public = []string{public}
		return def
	}
	modules := map[string]policy.ModuleDef{
		"sales":   core("team", "svc", "sales/**"),
		"cart":    core("team", "svc", "sales/cart/**"),
		"invoice": withPublic(core("team", "svc", "sales/invoice/**"), "sales/invoice/api/**"),
		"stock":   core("team", "svc", "stock/**"),
		"lookup":  withPublic(core("team", "svc", "stock/lookup/**"), "stock/lookup/api/**"),
	}
	res := analyze(modules,
		exampleEdge{fromFile: fileCart, toFile: "sales/invoice/i.go"},
		exampleEdge{fromFile: fileCart, toFile: "stock/lookup/l.go"},
	)
	sibling := seamBetween(t, res, "cart", "invoice")
	cross := seamBetween(t, res, "cart", "lookup")
	for name, s := range map[string]relationship.Seam{"sibling": sibling, "other branch": cross} {
		if s.Severity != relationship.SeverityCritical {
			t.Errorf("%s seam severity = %q, want critical", name, s.Severity)
		}
	}
	if got, want := sibling.RawDistance.Basis, "module_boundary@sales"; got != want {
		t.Errorf("sibling basis = %q, want %q", got, want)
	}
	if got, want := cross.RawDistance.Basis, "module_boundary@system"; got != want {
		t.Errorf("other-branch basis = %q, want %q", got, want)
	}
	if sibling.RawDistance.BoundaryCrossings != 2 || cross.RawDistance.BoundaryCrossings != 4 {
		t.Errorf("boundary crossings = %d and %d, want 2 and 4", sibling.RawDistance.BoundaryCrossings, cross.RawDistance.BoundaryCrossings)
	}
}

// Renaming module keys changes no score, no distance and no boundary count: the
// containment tree reads declared paths only.
func TestV7KeyRenameChangesNothing(t *testing.T) {
	build := func(sales, cart string) relationship.Seam {
		modules := map[string]policy.ModuleDef{
			sales: core("team", "svc", "sales/**"),
			cart:  core("team", "svc", "sales/cart/**"),
			"x":   core("team", "svc", "x/**"),
		}
		res := analyze(modules, exampleEdge{fromFile: fileCart, toFile: "x/x.go"})
		return seamBetween(t, res, cart, "x")
	}
	a, b := build("sales", "cart"), build("deep/er/sales", "very/deep/er/cart")
	if a.Severity != b.Severity || a.Distance != b.Distance || a.RawDistance.BoundaryCrossings != b.RawDistance.BoundaryCrossings || a.RawDistance.Basis != b.RawDistance.Basis {
		t.Errorf("rename changed the seam: %+v vs %+v", a.RawDistance, b.RawDistance)
	}
}

// Example 3: two teams, one deploy unit. `orders` (core) reaches past the
// public part of `pricing` (supporting) into `pricing/engine`. The use is
// intrusive, and the upstream module's volatility decides (the surfaces design
// 6.1): low, so the balance is 8 and the seam does not qualify. Through
// `pricing/api`, the declared public part, the same call is contract.
func TestV7Example3_UpstreamVolatilityDecides(t *testing.T) {
	modules := map[string]policy.ModuleDef{
		"orders":  core("team-a", "svc", "orders/**"),
		"pricing": {Paths: []string{"pricing/**"}, Public: []string{"pricing/api/**"}, Owner: "team-b", DeployUnit: "svc", Subdomain: subdomainSupporting},
	}
	res := analyze(modules, exampleEdge{fromFile: fileOrders, toFile: "pricing/engine/e.go"})
	e := requireScore(t, res, relationship.StrengthIntrusive, 8, relationship.SeverityLow)
	if e.Volatility != relationship.VolatilityLow {
		t.Errorf("volatility = %s, want low: the target's, not the core source's", e.Volatility)
	}
	if seamBetween(t, res, "orders", "pricing").DistributedMonolith {
		t.Error("an intrusive use of a low-volatility module must not qualify the seam")
	}

	through := analyze(modules, exampleEdge{fromFile: fileOrders, toFile: "pricing/api/p.go"})
	requireScore(t, through, relationship.StrengthContract, 9, relationship.SeverityNone)
}

// Example 6: no volatility declared anywhere. A call to a module that declares
// no surface is contract and balanced. A use past the public part of `rules`
// scores critical, but nobody declared the volatility, so the seam is unrated:
// it never qualifies, it says declare_volatility, and the coupling summary
// counts the unrated edges and their target modules.
func TestV7Example6_UndeclaredVolatilityIsUnrated(t *testing.T) {
	modules := map[string]policy.ModuleDef{
		"app":      {Paths: []string{"app/**"}},
		"adapters": {Paths: []string{"adapters/**"}},
		"rules":    {Paths: []string{"rules/**"}, Public: []string{"rules/api/**"}},
	}
	res := analyze(modules,
		exampleEdge{fromFile: "app/a.go", toFile: "adapters/s.go"},
		exampleEdge{fromFile: "app/a.go", toFile: "rules/d.go"},
	)
	port := seamBetween(t, res, "app", "adapters")
	if port.Severity != relationship.SeverityNone || port.DistributedMonolith {
		t.Errorf("port seam = %q qualifying %t, want none and not qualifying", port.Severity, port.DistributedMonolith)
	}
	concrete := seamBetween(t, res, "app", "rules")
	if concrete.Severity != relationship.SeverityCritical {
		t.Errorf("concrete seam severity = %q, want critical", concrete.Severity)
	}
	if concrete.DistributedMonolith {
		t.Error("an unrated seam must not qualify")
	}
	if concrete.Hypothesis != relationship.SeamHypothesisDeclareVolatility {
		t.Errorf("hypothesis = %q, want declare_volatility", concrete.Hypothesis)
	}
	s := res.Assessment.ClassifiedEdges
	if s.UnratedVolatilityEdges != 2 {
		t.Errorf("unrated edges = %d, want 2 (both edges end in undeclared volatility)", s.UnratedVolatilityEdges)
	}
	if got := s.UnratedVolatilityModules; len(got) != 2 {
		t.Errorf("unrated modules = %v, want adapters and rules: the target's volatility applies", got)
	}
}

// The qualification rule: intrusive strength, a module boundary, the target's
// declared high volatility. Contract and model never qualify, and a cohesive
// source role never excuses an intrusive use. Every qualifying edge is
// critical, so the gate set is a subset of the severity set.
func TestV7QualificationRule(t *testing.T) {
	const (
		toPublic = "dst/api/d.go"
		toModel  = "dst/model/d.go"
		toPast   = "dst/d.go"
	)
	type tc struct {
		name     string
		toFile   string
		srcVol   string
		dstVol   string
		role     policy.Role
		want     bool
		wantBand relationship.Severity
	}
	tests := []tc{
		{"intrusive, high", toPast, volHigh, volHigh, "", true, relationship.SeverityCritical},
		{"intrusive, source high, target low", toPast, volHigh, volLow, "", false, relationship.SeverityLow},
		{"intrusive, both low", toPast, volLow, volLow, "", false, relationship.SeverityLow},
		{"intrusive, medium never qualifies", toPast, volMedium, volMedium, "", false, relationship.SeverityMedium},
		{"intrusive, undeclared is unrated", toPast, "", "", "", false, relationship.SeverityCritical},
		{"intrusive from a composition root still qualifies", toPast, volHigh, volHigh, policy.RoleCompositionRoot, true, relationship.SeverityCritical},
		{"intrusive from generated code still qualifies", toPast, volHigh, volHigh, policy.RoleGenerated, true, relationship.SeverityCritical},
		{"the public part is contract", toPublic, volHigh, volHigh, "", false, relationship.SeverityNone},
		{"model coupling is below the bar", toModel, volHigh, volHigh, "", false, relationship.SeverityLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modules := map[string]policy.ModuleDef{
				"src": {Paths: []string{"src/**"}, Volatility: tt.srcVol, Role: tt.role},
				"dst": {Paths: []string{"dst/**"}, Public: []string{"dst/api/**"}, Model: []string{"dst/model/**"}, Volatility: tt.dstVol},
			}
			res := analyze(modules, exampleEdge{fromFile: "src/s.go", toFile: tt.toFile})
			s := seamBetween(t, res, "src", "dst")
			if s.DistributedMonolith != tt.want {
				t.Errorf("qualifies = %t, want %t (severity %q)", s.DistributedMonolith, tt.want, s.Severity)
			}
			if s.Severity != tt.wantBand {
				t.Errorf("severity = %q, want %q", s.Severity, tt.wantBand)
			}
			if s.DistributedMonolith && s.Severity != relationship.SeverityCritical {
				t.Errorf("a qualifying seam is critical, got %q", s.Severity)
			}
		})
	}
}
