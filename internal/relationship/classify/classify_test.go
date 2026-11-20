package classify_test

import (
	"slices"
	"testing"

	"github.com/coupling-dev/archfit/internal/config"
	"github.com/coupling-dev/archfit/internal/model/graph"
	"github.com/coupling-dev/archfit/internal/policy"
	"github.com/coupling-dev/archfit/internal/relationship/classify"
	"github.com/coupling-dev/archfit/internal/relationship/coupling"
)

const (
	ownerTeamX  = "team-x"
	ownerTeamY  = "team-y"
	deployUnitA = "svc-a"
	deployUnitB = "svc-b"

	// path glob constants reused across multiple test cases.
	pathsA              = "services/a/**"
	pathsB              = "services/b/**"
	publicB             = "services/b/api/**"
	publicA             = "services/a/api/**"
	modelB              = "services/b/model/**"
	fileServicesBClient = "file:services/b/api/client.go"
	sourceGoTypes       = "go/types"
	subdomainCore       = "core"
	subdomainSupporting = "supporting"
	subdomainGeneric    = "generic"

	langRust = "rust"

	// Go workspace member module paths reused across multiple test cases.
	goModuleA = "example.com/a"
	goModuleB = "example.com/b"
	relDirA   = "services/a"
	relDirB   = "services/b"

	parentModuleKey = "parent"
	rustCrateExtra  = "extra"
)

// makeGraph builds a minimal sealed Graph from a slice of edges.
func makeGraph(edges []graph.Edge) *graph.Graph {
	// Collect all unique node IDs referenced by the edges.
	seen := make(map[string]bool)
	var nodes []graph.Node
	for _, e := range edges {
		for _, id := range []string{e.From, e.To} {
			if !seen[id] {
				seen[id] = true
				// Parse "kind:path" to build the Node.
				kind, path := graph.NodeKindFile, id
				for _, k := range []graph.NodeKind{
					graph.NodeKindFile, graph.NodeKindPackage,
					graph.NodeKindModule, graph.NodeKindRepo, graph.NodeKindExternal,
				} {
					prefix := string(k) + ":"
					if len(id) > len(prefix) && id[:len(prefix)] == prefix {
						kind = k
						path = id[len(prefix):]
						break
					}
				}
				nodes = append(nodes, graph.Node{Kind: kind, Path: path})
			}
		}
	}
	return graph.Build([]graph.Facts{{
		Nodes:    nodes,
		Edges:    edges,
		Language: "go",
	}})
}

// edgeKey returns the coupling.Index key for an edge.
func edgeKey(e graph.Edge) string {
	return e.From + "\x00" + e.To + "\x00" + string(e.Kind)
}

func TestRun(t *testing.T) {
	// Shared module config used by most sub-tests.
	//   module "a": paths=["services/a/**"], public=[publicA]
	//               owner="team-x", deploy_unit="svc-a", subdomain="core"
	//   module "b": paths=["services/b/**"], public=["services/b/api/**"], model=[modelB]
	//               owner="team-y", deploy_unit="svc-b", subdomain="supporting"
	//   module "c": paths=["services/c/**"], public=["services/c/api/**"]
	//               owner="team-x", deploy_unit="svc-a", subdomain="generic"
	//   module "d": paths=["services/d/**"], public=["services/d/api/**"]
	//               owner="team-x", deploy_unit="svc-a", subdomain=""  (unknown)
	modules := map[string]policy.ModuleDef{
		"a": {
			Paths:      []string{pathsA},
			Public:     []string{publicA},
			Owner:      ownerTeamX,
			DeployUnit: deployUnitA,
			Subdomain:  subdomainCore,
		},
		"b": {
			Paths:      []string{pathsB},
			Public:     []string{publicB},
			Model:      []string{modelB},
			Owner:      ownerTeamY,
			DeployUnit: deployUnitB,
			Subdomain:  subdomainSupporting,
		},
		"c": {
			Paths:      []string{"services/c/**"},
			Public:     []string{"services/c/api/**"},
			Owner:      ownerTeamX,
			DeployUnit: deployUnitA,
			Subdomain:  subdomainGeneric,
		},
		"d": {
			Paths:      []string{"services/d/**"},
			Public:     []string{"services/d/api/**"},
			Owner:      ownerTeamX,
			DeployUnit: deployUnitA,
			Subdomain:  "",
		},
	}

	cfg := classify.Config{Modules: modules}

	// Helper to build a simple imports edge between two file paths.
	importEdge := func(from, to string) graph.Edge {
		return graph.Edge{
			From:       "file:" + from,
			To:         "file:" + to,
			Kind:       graph.EdgeKindImports,
			Language:   "go",
			Confidence: "high",
		}
	}

	tests := []struct {
		name     string
		edge     graph.Edge
		wantStr  coupling.Strength
		wantDist coupling.Distance
		wantVol  coupling.Volatility
		wantExp  coupling.Explicitness
	}{
		{
			name:     "contract edge — target matches public glob",
			edge:     importEdge("services/a/impl.go", "services/b/api/client.go"),
			wantStr:  coupling.StrengthContract,
			wantDist: coupling.DistanceCrossDeployUnit, // different owner, different deploy_unit
			wantVol:  coupling.VolatilityLow,           // b.subdomain = "supporting" → low (book Table 9.1)
			wantExp:  coupling.ExplicitnessExplicit,
		},
		{
			name:     "model edge — target matches model glob",
			edge:     importEdge("services/a/impl.go", "services/b/model/order.go"),
			wantStr:  coupling.StrengthModel,
			wantDist: coupling.DistanceCrossDeployUnit,
			wantVol:  coupling.VolatilityLow,
			wantExp:  coupling.ExplicitnessUnknown,
		},
		{
			name:     "intrusive edge — target past the surface",
			edge:     importEdge("services/a/impl.go", "services/b/internal/secret.go"),
			wantStr:  coupling.StrengthIntrusive,
			wantDist: coupling.DistanceCrossDeployUnit,
			wantVol:  coupling.VolatilityLow, // b.subdomain = "supporting" → low (book Table 9.1)
			wantExp:  coupling.ExplicitnessImplicit,
		},
		{
			name:     "intrusive edge — any path past the surface, not only internal/",
			edge:     importEdge("services/a/impl.go", "services/b/util/helper.go"),
			wantStr:  coupling.StrengthIntrusive,
			wantDist: coupling.DistanceCrossDeployUnit,
			wantVol:  coupling.VolatilityLow, // the target's: b.subdomain = "supporting" → low
			wantExp:  coupling.ExplicitnessImplicit,
		},
		{
			name:     "same-module edge — distance same_module, no strength",
			edge:     importEdge("services/a/x.go", "services/a/api/types.go"),
			wantStr:  coupling.StrengthUnknown, // the surface speaks only to other modules
			wantDist: coupling.DistanceSameModule,
			wantVol:  coupling.VolatilityHigh, // a.subdomain = "core"
			wantExp:  coupling.ExplicitnessUnknown,
		},
		{
			name:     "cross-module same-owner — different module, same owner",
			edge:     importEdge("services/a/impl.go", "services/c/api/client.go"),
			wantStr:  coupling.StrengthContract,
			wantDist: coupling.DistanceCrossModule, // both owner=team-x
			wantVol:  coupling.VolatilityLow,       // c.subdomain = "generic"
			wantExp:  coupling.ExplicitnessExplicit,
		},
		{
			name:     "cross-module different owner same deploy-unit — a and d",
			edge:     importEdge("services/a/impl.go", "services/d/api/types.go"),
			wantStr:  coupling.StrengthContract,
			wantDist: coupling.DistanceCrossModule,  // both owner=team-x
			wantVol:  coupling.VolatilityUndeclared, // d resolves but d.subdomain = ""
			wantExp:  coupling.ExplicitnessExplicit,
		},
		{
			name:     "cross-deploy-unit — different owner, different deploy_unit",
			edge:     importEdge("services/a/impl.go", "services/b/api/client.go"),
			wantStr:  coupling.StrengthContract,
			wantDist: coupling.DistanceCrossDeployUnit,
			wantVol:  coupling.VolatilityLow, // b.subdomain = "supporting" → low (book Table 9.1)
			wantExp:  coupling.ExplicitnessExplicit,
		},
		{
			name:     "undeclared subdomain — module resolves but no subdomain/volatility",
			edge:     importEdge("services/a/impl.go", "services/d/internal/impl.go"),
			wantStr:  coupling.StrengthIntrusive, // past d's public part
			wantDist: coupling.DistanceCrossModule,
			wantVol:  coupling.VolatilityUndeclared, // d resolves; no subdomain/volatility/path match
			wantExp:  coupling.ExplicitnessImplicit,
		},
		{
			name:     "unresolvable from-path — distance unknown",
			edge:     importEdge("external/foo.go", "services/b/api/client.go"),
			wantStr:  coupling.StrengthContract,
			wantDist: coupling.DistanceUnknown, // external/foo.go matches no module
			wantVol:  coupling.VolatilityLow,   // b.subdomain = "supporting" → low (book Table 9.1)
			wantExp:  coupling.ExplicitnessExplicit,
		},
		{
			name:     "unresolvable to-path — strength and volatility unknown",
			edge:     importEdge("services/a/impl.go", "external/pkg/foo.go"),
			wantStr:  coupling.StrengthUnknown,
			wantDist: coupling.DistanceUnknown,
			wantVol:  coupling.VolatilityUnknown,
			wantExp:  coupling.ExplicitnessUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := makeGraph([]graph.Edge{tc.edge})
			idx := classify.Run(g, cfg)

			key := edgeKey(tc.edge)
			cl, ok := idx[key]
			if !ok {
				t.Fatalf("edge key %q not found in index", key)
			}

			if cl.Strength != tc.wantStr {
				t.Errorf("Strength = %q, want %q", cl.Strength, tc.wantStr)
			}
			if cl.Distance != tc.wantDist {
				t.Errorf("Distance = %q, want %q", cl.Distance, tc.wantDist)
			}
			if cl.Volatility != tc.wantVol {
				t.Errorf("Volatility = %q, want %q", cl.Volatility, tc.wantVol)
			}
			if cl.Explicitness != tc.wantExp {
				t.Errorf("Explicitness = %q, want %q", cl.Explicitness, tc.wantExp)
			}
		})
	}
}

func TestRun_AttachesDeterministicConnascenceEvidence(t *testing.T) {
	edge := graph.Edge{
		From:     extFromGo,
		To:       fileServicesBClient,
		Kind:     graph.EdgeKindImports,
		Language: graph.LangGo,
		ConnascenceHints: []graph.ConnascenceHint{
			{Kind: graph.ConnascenceName, Source: sourceGoTypes, Detail: "resolved symbol"},
			{Kind: graph.ConnascenceType, Source: sourceGoTypes, Detail: "type name"},
			{Kind: "runtime_value", Source: sourceGoTypes},
		},
	}
	g := makeGraph([]graph.Edge{edge})
	idx := classify.Run(g, classify.Config{Modules: map[string]policy.ModuleDef{
		"a": {Paths: []string{pathsA}},
		"b": {Paths: []string{pathsB}},
	}})

	got := idx[edgeKey(edge)].Connascence
	if len(got) != 2 {
		t.Fatalf("Connascence len = %d, want 2: %+v", len(got), got)
	}
	if got[0].Kind != coupling.ConnascenceName || got[1].Kind != coupling.ConnascenceType {
		t.Fatalf("Connascence = %+v, want name then type", got)
	}
}

func TestRun_ConnascenceAbstainsWithoutExtractorEvidence(t *testing.T) {
	edge := graph.Edge{
		From:     extFromGo,
		To:       fileServicesBClient,
		Kind:     graph.EdgeKindImports,
		Language: graph.LangGo,
	}
	g := makeGraph([]graph.Edge{edge})
	idx := classify.Run(g, classify.Config{Modules: map[string]policy.ModuleDef{
		"a": {Paths: []string{pathsA}},
		"b": {Paths: []string{pathsB}},
	}})

	if got := idx[edgeKey(edge)].Connascence; len(got) != 0 {
		t.Fatalf("Connascence len = %d, want 0: %+v", len(got), got)
	}
}

// TestRun_EmptyGraph verifies that Run on an empty graph returns an empty index.
func TestRun_EmptyGraph(t *testing.T) {
	g := graph.Build(nil)
	cfg := classify.Config{Modules: map[string]policy.ModuleDef{}}
	idx := classify.Run(g, cfg)
	if len(idx) != 0 {
		t.Errorf("expected empty index for empty graph, got %d entries", len(idx))
	}
}

// TestRun_RegistersRustModuleGraphNodes verifies the last-mile: a Rust module-graph
// edge ("crate::mod" nodes) with NO configured modules must still classify to a
// cross-module distance, because AugmentModulesFromGraph synthesises the module
// definitions. Without it, moduleFor fails and the edge is distance-unknown (never
// counted by coupling_balance/encapsulation). Siblings under the same crate resolve
// to same-owner (the "::" segment separator).
func TestRun_RegistersRustModuleGraphNodes(t *testing.T) {
	e := graph.Edge{
		From:         "package:demo::a",
		To:           "package:demo::b",
		Kind:         graph.EdgeKindDependsOn,
		Language:     langRust,
		StrengthHint: "functional",
	}
	g := makeGraph([]graph.Edge{e})

	// Without registration: unknown distance (not counted).
	bare := classify.Run(g, classify.Config{Modules: map[string]policy.ModuleDef{}})
	if cl := bare[edgeKey(e)]; cl.Distance != coupling.DistanceUnknown {
		t.Errorf("unregistered: Distance = %q, want unknown (module nodes not in config)", cl.Distance)
	}

	// With registration: cross-module, same owner (siblings in one crate). A
	// synthetic module declares no surface, so it is public as a whole: contract,
	// whatever the type hint says.
	mods := classify.AugmentModulesFromGraph(g, map[string]policy.ModuleDef{})
	idx := classify.Run(g, classify.Config{Modules: mods})
	cl, ok := idx[edgeKey(e)]
	if !ok {
		t.Fatalf("edge not found in index")
	}
	if cl.Distance != coupling.DistanceCrossModule {
		t.Errorf("Distance = %q, want cross_module (registered sibling modules)", cl.Distance)
	}
	if cl.Strength != coupling.StrengthContract {
		t.Errorf("Strength = %q, want contract (no declared surface)", cl.Strength)
	}
}

// TestAugmentModulesFromGraph_LeavesNonRustUntouched guards the "::" gate: Go/TS/Python
// graph nodes (slash/dot separators) must NOT be auto-registered, so configured-module
// semantics for those languages are unchanged.
func TestAugmentModulesFromGraph_LeavesNonRustUntouched(t *testing.T) {
	g := makeGraph([]graph.Edge{{
		From: "package:internal/x", To: "package:internal/y", Kind: graph.EdgeKindImports, Language: "go",
	}})
	in := map[string]policy.ModuleDef{}
	out := classify.AugmentModulesFromGraph(g, in)
	if len(out) != 0 {
		t.Errorf("non-Rust nodes must not be registered, got %d modules: %v", len(out), out)
	}
}

// TestRun_IndexKeyMatchesEdge verifies that the index key format is consistent
// with edge canonical key (from + NUL + to + NUL + kind).
func TestRun_IndexKeyMatchesEdge(t *testing.T) {
	modules := map[string]policy.ModuleDef{
		"a": {Paths: []string{globPkgA}, Public: []string{"pkg/a/api/**"}},
		"b": {Paths: []string{globPkgB}, Public: []string{"pkg/b/api/**"}},
	}
	cfg := classify.Config{Modules: modules}

	e := graph.Edge{
		From:     "file:pkg/a/main.go",
		To:       "file:pkg/b/api/client.go",
		Kind:     graph.EdgeKindImports,
		Language: "go",
	}
	g := makeGraph([]graph.Edge{e})
	idx := classify.Run(g, cfg)

	want := e.From + "\x00" + e.To + "\x00" + string(e.Kind)
	if _, ok := idx[want]; !ok {
		t.Errorf("expected key %q in index; got keys: %v", want, keys(idx))
	}
}

// TestRun_ExplicitVolatilityFieldOverridesSubdomain verifies that an explicit
// Volatility field on a ModuleDef takes precedence over the Subdomain heuristic.
func TestRun_ExplicitVolatilityFieldOverridesSubdomain(t *testing.T) {
	modules := map[string]policy.ModuleDef{
		"a": {Paths: []string{globPkgA}},
		"b": {Paths: []string{globPkgB}, Subdomain: subdomainCore, Volatility: "low"},
	}
	cfg := classify.Config{Modules: modules}

	e := graph.Edge{
		From:     filePkgAXGo,
		To:       filePkgBYGo,
		Kind:     graph.EdgeKindImports,
		Language: "go",
	}
	g := makeGraph([]graph.Edge{e})
	idx := classify.Run(g, cfg)

	key := edgeKey(e)
	cl := idx[key]
	// Volatility field "low" should override subdomain "core" (which would give high).
	if cl.Volatility != coupling.VolatilityLow {
		t.Errorf("Volatility = %q, want %q (explicit field should override subdomain)", cl.Volatility, coupling.VolatilityLow)
	}
}

// TestRun_ExplicitnessHintOverridesGlob verifies that ExplicitnessHint on the
// edge takes precedence over the config-glob-derived explicitness.
func TestRun_ExplicitnessHintOverridesGlob(t *testing.T) {
	// services/a/internal/** is past the public part of module "a" → StrengthIntrusive → ExplicitnessImplicit.
	// An AST signal can flip this to "explicit" via ExplicitnessHint.
	modules := map[string]policy.ModuleDef{
		"a": {
			Paths:  []string{pathsA},
			Public: []string{publicA},
			Owner:  ownerTeamX, DeployUnit: deployUnitA,
		},
		"b": {
			Paths:  []string{pathsB},
			Public: []string{publicB},
			Owner:  ownerTeamY, DeployUnit: deployUnitB,
		},
	}
	cfg := classify.Config{Modules: modules}

	tests := []struct {
		name    string
		hint    string
		wantExp coupling.Explicitness
	}{
		{
			name:    "hint=explicit overrides intrusive-derived implicit",
			hint:    "explicit",
			wantExp: coupling.ExplicitnessExplicit,
		},
		{
			name:    "hint=implicit overrides contract-derived explicit",
			hint:    "implicit",
			wantExp: coupling.ExplicitnessImplicit,
		},
		{
			name:    "hint empty — glob result used",
			hint:    "",
			wantExp: coupling.ExplicitnessImplicit, // intrusive → implicit
		},
	}

	// Base edge: b→a/internal → StrengthIntrusive → ExplicitnessImplicit without hint.
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := graph.Edge{
				From:             "file:services/b/impl.go",
				To:               "file:services/a/internal/secret.go",
				Kind:             graph.EdgeKindImports,
				Language:         "go",
				ExplicitnessHint: tc.hint,
			}
			g := makeGraph([]graph.Edge{e})
			idx := classify.Run(g, cfg)
			key := edgeKey(e)
			cl, ok := idx[key]
			if !ok {
				t.Fatalf("edge key %q not found in index", key)
			}
			if cl.Explicitness != tc.wantExp {
				t.Errorf("Explicitness = %q, want %q", cl.Explicitness, tc.wantExp)
			}
		})
	}
}

// TestRun_Severity verifies that book-formula severity (cl.Score.Band) is stored
// on the Classification for cross-boundary edges.
func TestRun_Severity(t *testing.T) {
	// Modules:
	//   "a": paths=services/a/**, owner=team-x, deploy=svc-a, subdomain=core (high volatility)
	//   "b": paths=services/b/**, public=services/b/api/**, internal=services/b/internal/**,
	//        owner=team-y, deploy=svc-b, subdomain=supporting (low volatility)
	//   "c": paths=services/c/**, public=services/c/api/**, owner=team-x, deploy=svc-a, subdomain=generic (low vol)
	modules := map[string]policy.ModuleDef{
		"a": {
			Paths:      []string{pathsA},
			Public:     []string{publicA},
			Owner:      ownerTeamX,
			DeployUnit: deployUnitA,
			Subdomain:  subdomainCore,
		},
		"b": {
			Paths:      []string{pathsB},
			Public:     []string{publicB},
			Internal:   []string{"services/b/internal/**"},
			Owner:      ownerTeamY,
			DeployUnit: deployUnitB,
			Subdomain:  subdomainSupporting,
		},
		"c": {
			Paths:      []string{"services/c/**"},
			Public:     []string{"services/c/api/**"},
			Owner:      ownerTeamX,
			DeployUnit: deployUnitA,
			Subdomain:  subdomainGeneric,
		},
	}
	cfg := classify.Config{Modules: modules}

	importEdge := func(from, to string) graph.Edge {
		return graph.Edge{
			From: "file:" + from, To: "file:" + to,
			Kind: graph.EdgeKindImports, Language: "go",
		}
	}

	tests := []struct {
		name         string
		edge         graph.Edge
		wantSeverity coupling.Severity
	}{
		{
			// contract (S=1) + cross_deploy_unit (D=9) + supporting/low (V=3):
			// max(|1-9|=8, 10-6=4)+1=9 → none (loose XOR quadrant, book-correct).
			name:         "contract cross-deploy medium-vol → none (XOR loose quadrant)",
			edge:         importEdge("services/a/impl.go", "services/b/api/client.go"),
			wantSeverity: coupling.SeverityNone,
		},
		{
			// intrusive (S=10) + cross_deploy_unit (D=9) + supporting/low (V=3, book Table 9.1):
			// max(|10-9|=1, 10-3=7)+1=8 → low.
			// (BalanceResult returned critical; book formula correctly scores low —
			// stable/low-volatility target neutralizes even a tight cross-deploy edge.)
			name:         "intrusive cross-deploy low-vol → low (book formula)",
			edge:         importEdge("services/a/impl.go", "services/b/internal/secret.go"),
			wantSeverity: coupling.SeverityLow,
		},
		{
			// contract (S=1) + cross_module (D=9) + generic/low (V=3):
			// max(|1-9|=8, 10-3=7)+1=9 → none (a published contract across a boundary
			// is the loose quadrant, whatever the owner).
			name:         "contract cross-module low-vol → none (loose quadrant)",
			edge:         importEdge("services/a/impl.go", "services/c/api/client.go"),
			wantSeverity: coupling.SeverityNone,
		},
		{
			// same-module edge: no severity computed
			name:         "same-module edge — no severity",
			edge:         importEdge("services/a/impl.go", "services/a/api/types.go"),
			wantSeverity: coupling.SeverityNone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := makeGraph([]graph.Edge{tc.edge})
			idx := classify.Run(g, cfg)
			key := edgeKey(tc.edge)
			cl, ok := idx[key]
			if !ok {
				t.Fatalf("edge key %q not found in index", key)
			}
			if cl.Severity != tc.wantSeverity {
				t.Errorf("Severity = %q, want %q (str=%q dist=%q vol=%q)",
					cl.Severity, tc.wantSeverity, cl.Strength, cl.Distance, cl.Volatility)
			}
		})
	}
}

// TestRun_SurfaceDecidesStrength is the classification rule of the surfaces
// design 6.1, cell by cell: the declared surface of the module that owns the
// target gives the strength, and no type fact of an extractor changes it. Each
// surface is crossed with every strength hint and data hint an extractor
// emits; the hints stay on the edge as evidence.
func TestRun_SurfaceDecidesStrength(t *testing.T) {
	const (
		toAPI   = "file:services/b/api/client.go"
		toModel = "file:services/b/model/order.go"
		toImpl  = "file:services/b/impl/store.go"
		toNone  = "file:vendor/x/x.go"
	)
	declared := policy.ModuleDef{Paths: []string{pathsB}, Public: []string{publicB}, Model: []string{modelB}}
	modelOnly := policy.ModuleDef{Paths: []string{pathsB}, Model: []string{modelB}}
	undeclared := policy.ModuleDef{Paths: []string{pathsB}}
	surfaces := []struct {
		name string
		b    policy.ModuleDef
		to   string
		want coupling.Strength
	}{
		{"the public part", declared, toAPI, coupling.StrengthContract},
		{"the model part", declared, toModel, coupling.StrengthModel},
		{"past the surface", declared, toImpl, coupling.StrengthIntrusive},
		{"model only: the model part", modelOnly, toModel, coupling.StrengthModel},
		{"model only: no public part", modelOnly, toAPI, coupling.StrengthIntrusive},
		{"no surface declared: the whole module is public", undeclared, toImpl, coupling.StrengthContract},
		{"a target in no module", declared, toNone, coupling.StrengthUnknown},
	}
	hints := []string{
		"", string(coupling.StrengthContract), string(coupling.StrengthModel), hintFunctional,
		string(coupling.StrengthSymmetric), hintIntrusive, graph.StrengthHintDTO, "garbage",
	}
	dataHints := []string{"", string(coupling.StrengthModel), graph.StrengthHintDTO}
	for _, sf := range surfaces {
		for _, hint := range hints {
			for _, data := range dataHints {
				t.Run(sf.name+" hint="+hint+" data="+data, func(t *testing.T) {
					e := graph.Edge{
						From: "file:services/a/impl.go", To: sf.to, Kind: graph.EdgeKindImports,
						Language: graph.LangGo, StrengthHint: hint, DataStrengthHint: data,
					}
					modules := map[string]policy.ModuleDef{"a": {Paths: []string{pathsA}}, "b": sf.b}
					cl := classify.Run(makeGraph([]graph.Edge{e}), classify.Config{Modules: modules})[edgeKey(e)]
					if cl.Strength != sf.want {
						t.Errorf("Strength = %q, want %q", cl.Strength, sf.want)
					}
				})
			}
		}
	}
}

// TestRun_DotNetInternalMemberThroughPublicFile is the CleanArchitecture V5
// case: Infrastructure calls Application's internal Result constructor through
// InternalsVisibleTo, in a file a public: glob covers. Roslyn reports the use
// as intrusive. The surfaces design 6.1 keeps such a use as evidence: one
// dependency has one number, from the declared surface, so it stays contract.
func TestRun_DotNetInternalMemberThroughPublicFile(t *testing.T) {
	modules := map[string]policy.ModuleDef{
		"Infrastructure": {Paths: []string{"src/Infrastructure/**"}},
		"Application":    {Paths: []string{"src/Application/**"}, Public: []string{"src/Application/Common/Models/**"}},
	}
	e := graph.Edge{
		From:             "file:src/Infrastructure/Leaks/InfrastructureUsesInternalCtor.cs",
		To:               "file:src/Application/Common/Models/Result.cs",
		Kind:             graph.EdgeKindImports,
		Language:         graph.LangDotNet,
		StrengthHint:     string(coupling.StrengthIntrusive),
		DataStrengthHint: string(coupling.StrengthModel),
	}
	cl := classify.Run(makeGraph([]graph.Edge{e}), classify.Config{Modules: modules})[edgeKey(e)]
	if cl.Strength != coupling.StrengthContract {
		t.Fatalf("strength = %q, want contract", cl.Strength)
	}
}

// keys returns all keys of the index as a slice (for error messages).
func keys(idx coupling.Index) []string {
	ks := make([]string, 0, len(idx))
	for k := range idx {
		ks = append(ks, k)
	}
	return ks
}

// TestRun_ApprovedLabelPrecedence verifies the strength precedence chain:
// internal glob (intrusive) is authoritative; a public glob is a not-intrusive
// floor whose KIND is resolved by approved label > extractor hint > contract.
const (
	globPkgA    = "pkg/a/**"
	globPkgB    = "pkg/b/**"
	pinnedModel = "model"
	labelKeyAB  = "a\x00b"

	modKeyPkgA  = "pkg/a"
	modKeyPkgB  = "pkg/b"
	pathPkgAGo  = "pkg/a/a.go"
	pathPkgBGo  = "pkg/b/b.go"
	filePkgAAGo = "file:pkg/a/a.go"
	filePkgBBGo = "file:pkg/b/b.go"
	filePkgAXGo = "file:pkg/a/x.go"
	fileAX      = "pkg/a/x.go"
	filePkgBYGo = "file:pkg/b/y.go"

	modKeySvcX = "services/x"
	globSvcX   = "services/x/**"
)

// Connascence is report-only evidence: whatever kinds an extractor reports, it
// never sets or raises strength (bc_score.v7 S3).
func TestRun_ConnascenceNeverSetsStrength(t *testing.T) {
	build := func(hints []graph.ConnascenceHint) (*graph.Graph, string) {
		e := graph.Edge{
			From: filePkgAAGo, To: filePkgBBGo,
			Kind: graph.EdgeKindImports, ConnascenceHints: hints,
		}
		g := graph.Build([]graph.Facts{{
			Language: "go",
			Nodes: []graph.Node{
				{Kind: graph.NodeKindFile, Path: pathPkgAGo},
				{Kind: graph.NodeKindFile, Path: pathPkgBGo},
			},
			Edges: []graph.Edge{e},
		}})
		return g, e.From + "\x00" + e.To + "\x00" + string(e.Kind)
	}
	publicB := map[string]policy.ModuleDef{
		"a": {Paths: []string{globPkgA}},
		"b": {Paths: []string{globPkgB}, Public: []string{globPkgB}},
	}
	pastB := map[string]policy.ModuleDef{
		"a": {Paths: []string{globPkgA}},
		"b": {Paths: []string{globPkgB}, Public: []string{"pkg/b/api/**"}},
	}
	cases := []struct {
		name    string
		modules map[string]policy.ModuleDef
		kind    string
		want    coupling.Strength
	}{
		{"past the surface + algorithm stays intrusive", pastB, graph.ConnascenceAlgorithm, coupling.StrengthIntrusive},
		{"past the surface + meaning stays intrusive", pastB, graph.ConnascenceMeaning, coupling.StrengthIntrusive},
		{"past the surface + position stays intrusive", pastB, graph.ConnascencePosition, coupling.StrengthIntrusive},
		{"public floor + algorithm stays contract", publicB, graph.ConnascenceAlgorithm, coupling.StrengthContract},
		{"public floor + meaning stays contract", publicB, graph.ConnascenceMeaning, coupling.StrengthContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, key := build([]graph.ConnascenceHint{{Kind: tc.kind, Source: sourceGoTypes}})
			idx := classify.Run(g, classify.Config{Modules: tc.modules})
			if got := idx[key].Strength; got != tc.want {
				t.Fatalf("strength = %q, want %q", got, tc.want)
			}
			if len(idx[key].Connascence) == 0 {
				t.Fatal("connascence evidence was dropped; it must stay report-only evidence")
			}
		})
	}
}

// TestRun_ContractRecommended verifies the generic-subdomain contract advisory:
// ContractRecommended is set when the to-module is a generic subdomain and the
// strength is non-contract; it is NOT set when strength is contract, the edge is
// same-module, or the to-module is not a generic subdomain.
func TestRun_ContractRecommended(t *testing.T) {
	modules := map[string]policy.ModuleDef{
		subdomainCore: {
			Paths:      []string{"services/core/**"},
			Public:     []string{"services/core/api/**"},
			Owner:      ownerTeamX,
			DeployUnit: deployUnitA,
			Subdomain:  subdomainCore,
		},
		subdomainGeneric: {
			Paths:      []string{"services/generic/**"},
			Public:     []string{"services/generic/api/**"},
			Owner:      ownerTeamY,
			DeployUnit: deployUnitB,
			Subdomain:  subdomainGeneric,
		},
		subdomainSupporting: {
			Paths:      []string{"services/supporting/**"},
			Public:     []string{"services/supporting/api/**"},
			Owner:      ownerTeamY,
			DeployUnit: deployUnitB,
			Subdomain:  subdomainSupporting,
		},
		// Module with no explicit subdomain — must NOT be inferred as generic
		// from its path name (the path heuristic was removed; no guessing).
		"util": {
			Paths:      []string{"util/**"},
			Owner:      ownerTeamX,
			DeployUnit: deployUnitA,
		},
	}
	cfg := classify.Config{Modules: modules}

	edge := func(from, to, hint string) graph.Edge {
		return graph.Edge{
			From:         "file:" + from,
			To:           "file:" + to,
			Kind:         graph.EdgeKindImports,
			Language:     "go",
			StrengthHint: hint,
		}
	}

	tests := []struct {
		name               string
		e                  graph.Edge
		wantContractRecomm bool
	}{
		{
			// Non-contract strength → generic subdomain: advisory fires.
			name:               "functional to generic → contract recommended",
			e:                  edge("services/core/impl.go", "services/generic/impl.go", "functional"),
			wantContractRecomm: true,
		},
		{
			// Unknown strength → generic subdomain: advisory fires.
			name:               "unknown strength to generic → contract recommended",
			e:                  edge("services/core/impl.go", "services/generic/impl.go", ""),
			wantContractRecomm: true,
		},
		{
			// Contract strength → generic subdomain: no advisory (already contracted).
			name:               "contract to generic api → no advisory",
			e:                  edge("services/core/impl.go", "services/generic/api/client.go", ""),
			wantContractRecomm: false,
		},
		{
			// Non-contract to non-generic (supporting): advisory does not fire.
			name:               "functional to supporting → no advisory",
			e:                  edge("services/core/impl.go", "services/supporting/impl.go", "functional"),
			wantContractRecomm: false,
		},
		{
			// Undeclared subdomain (util/, no subdomain): NOT inferred as generic
			// from the path name — no path guessing, so no advisory fires.
			name:               "functional to undeclared util/ → no advisory (no path guessing)",
			e:                  edge("services/core/impl.go", "util/parser.go", "functional"),
			wantContractRecomm: false,
		},
		{
			// Contract to undeclared util/: no advisory either way.
			name:               "contract to undeclared util/ → no advisory",
			e:                  edge("services/core/impl.go", "util/parser.go", "contract"),
			wantContractRecomm: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := makeGraph([]graph.Edge{tc.e})
			idx := classify.Run(g, cfg)
			key := edgeKey(tc.e)
			cl, ok := idx[key]
			if !ok {
				t.Fatalf("edge key %q not found in index", key)
			}
			if cl.ContractRecommended != tc.wantContractRecomm {
				t.Errorf("ContractRecommended = %v, want %v (str=%q dist=%q vol=%q)",
					cl.ContractRecommended, tc.wantContractRecomm,
					cl.Strength, cl.Distance, cl.Volatility)
			}
		})
	}
}

// TestRun_VolatilityUndeclaredWithoutConfig verifies classify.Run produces
// VolatilityUndeclared (not Unknown) for a resolved module with no explicit
// volatility or subdomain: the module is known, only its volatility is a config
// gap. The gate never derives volatility from git churn — there is no churn path
// into classification at all.
func TestRun_VolatilityUndeclaredWithoutConfig(t *testing.T) {
	cfg := config.Config{
		Modules: map[string]policy.ModuleDef{
			"a": {Paths: []string{globPkgA}, Owner: ownerTeamX},
			"b": {Paths: []string{globPkgB}, Owner: ownerTeamY}, // no volatility, no subdomain
		},
	}
	classifyCfg := cfg.ForClassify()
	if got := classifyCfg.Modules["b"].Volatility; got != "" {
		t.Fatalf("precondition: ForClassify().Modules[b].Volatility = %q, want \"\"", got)
	}

	e := graph.Edge{
		From:     "file:pkg/a/x.go",
		To:       "file:pkg/b/y.go",
		Kind:     graph.EdgeKindImports,
		Language: "go",
	}
	g := makeGraph([]graph.Edge{e})
	idx := classify.Run(g, classifyCfg)
	cl := idx[edgeKey(e)]

	if cl.Volatility != coupling.VolatilityUndeclared {
		t.Errorf("Volatility = %q, want undeclared", cl.Volatility)
	}
}

// TestRun_BoundaryTokenFollowsOwnerNotSpelling: every module boundary scores at
// the same rung, and the token only records an owner change. The names of the
// modules and how far apart they sit in the tree decide nothing.
func TestRun_BoundaryTokenFollowsOwnerNotSpelling(t *testing.T) {
	importEdge := func(from, to string) graph.Edge {
		return graph.Edge{From: "file:" + from, To: "file:" + to, Kind: graph.EdgeKindImports, Language: "go"}
	}
	tests := []struct {
		name     string
		owners   [3]string // pkg/a, pkg/b, services/x
		from, to string
		want     coupling.Distance
	}{
		{"siblings, one owner", [3]string{ownerTeamX, ownerTeamX, ownerTeamX}, fileAX, "pkg/b/y.go", coupling.DistanceCrossModule},
		{"distant subtrees, one owner stay cross_module", [3]string{ownerTeamX, ownerTeamX, ownerTeamX}, fileAX, modKeySvcX + "/y.go", coupling.DistanceCrossModule},
		{"siblings, two owners", [3]string{ownerTeamX, ownerTeamY, ownerTeamX}, fileAX, "pkg/b/y.go", coupling.DistanceCrossModuleDiffOwner},
		{"no owners declared", [3]string{}, fileAX, modKeySvcX + "/y.go", coupling.DistanceCrossModule},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			modules := map[string]policy.ModuleDef{
				modKeyPkgA: {Paths: []string{globPkgA}, Owner: tc.owners[0]},
				modKeyPkgB: {Paths: []string{globPkgB}, Owner: tc.owners[1]},
				modKeySvcX: {Paths: []string{globSvcX}, Owner: tc.owners[2]},
			}
			e := importEdge(tc.from, tc.to)
			cl := classify.Run(makeGraph([]graph.Edge{e}), classify.Config{Modules: modules})[edgeKey(e)]
			if cl.Distance != tc.want {
				t.Errorf("Distance = %q, want %q", cl.Distance, tc.want)
			}
		})
	}
}

// TestRun_MultiOwnerSiblingsLifted verifies that in a multi-owner repo, distinct
// ownership lifts sibling modules from SameOwner to DiffOwner — ownership is
// authoritative (not degenerate) and the max() composite picks it up.
func TestRun_MultiOwnerSiblingsLifted(t *testing.T) {
	// Two distinct owners → not degenerate → ownership applies.
	// pkg/a and pkg/b are code-structure siblings (same_owner by structure),
	// but owned by different teams → ownership lifts to DiffOwner.
	modules := map[string]policy.ModuleDef{
		modKeyPkgA: {Paths: []string{globPkgA}, Owner: ownerTeamX},
		modKeyPkgB: {Paths: []string{globPkgB}, Owner: ownerTeamY},
	}
	// Both modules carry explicit hand-authored owners (distinct) → Step 2 fires.
	cfg := config.Config{Modules: modules}.ForClassify()

	e := graph.Edge{
		From: "file:pkg/a/x.go", To: "file:pkg/b/y.go",
		Kind: graph.EdgeKindImports, Language: "go",
	}
	cl := classify.Run(makeGraph([]graph.Edge{e}), cfg)[edgeKey(e)]
	if cl.Distance != coupling.DistanceCrossModuleDiffOwner {
		t.Errorf("siblings distinct owners: Distance = %q, want cross_module_different_owner", cl.Distance)
	}
	if cl.DistanceBasis != coupling.DistanceBasisOwnership {
		t.Errorf("siblings distinct owners: DistanceBasis = %q, want ownership", cl.DistanceBasis)
	}
}

// TestRun_DistanceBasisPopulated verifies that DistanceBasis is set correctly
// on Classification after classify.Run for the three signal sources.
func TestRun_DistanceBasisPopulated(t *testing.T) {
	importEdge := func(from, to string) graph.Edge {
		return graph.Edge{
			From: "file:" + from, To: "file:" + to,
			Kind: graph.EdgeKindImports, Language: "go",
		}
	}

	t.Run("degenerate owner → code_structure basis", func(t *testing.T) {
		modules := map[string]policy.ModuleDef{
			modKeyPkgA: {Paths: []string{globPkgA}, Owner: ownerTeamX},
			modKeyPkgB: {Paths: []string{globPkgB}, Owner: ownerTeamX},
		}
		// No explicit owners — degenerate, structure fallback fires.
		cfg := config.Config{Modules: modules}.ForClassify()
		e := importEdge(fileAX, "pkg/b/y.go")
		cl := classify.Run(makeGraph([]graph.Edge{e}), cfg)[edgeKey(e)]
		if cl.DistanceBasis != coupling.DistanceBasisModule {
			t.Errorf("DistanceBasis = %q, want code_structure", cl.DistanceBasis)
		}
	})

	t.Run("explicit owner → ownership basis", func(t *testing.T) {
		modules := map[string]policy.ModuleDef{
			modKeyPkgA: {Paths: []string{globPkgA}, Owner: ownerTeamX},
			modKeyPkgB: {Paths: []string{globPkgB}, Owner: ownerTeamY},
		}
		// Mark both as explicit via production path so Step 2 fires (non-degenerate).
		cfg := config.Config{Modules: modules}.ForClassify()
		e := importEdge(fileAX, "pkg/b/y.go")
		cl := classify.Run(makeGraph([]graph.Edge{e}), cfg)[edgeKey(e)]
		if cl.DistanceBasis != coupling.DistanceBasisOwnership {
			t.Errorf("DistanceBasis = %q, want ownership", cl.DistanceBasis)
		}
	})

	t.Run("deploy unit difference → deploy_unit basis", func(t *testing.T) {
		modules := map[string]policy.ModuleDef{
			modKeyPkgA: {Paths: []string{globPkgA}, Owner: ownerTeamX, DeployUnit: deployUnitA},
			modKeyPkgB: {Paths: []string{globPkgB}, Owner: ownerTeamX, DeployUnit: deployUnitB},
		}
		// Deploy unit is Step 1 (absolute); ExplicitOwners irrelevant here.
		cfg := config.Config{Modules: modules}.ForClassify()
		e := importEdge(fileAX, "pkg/b/y.go")
		cl := classify.Run(makeGraph([]graph.Edge{e}), cfg)[edgeKey(e)]
		if cl.DistanceBasis != coupling.DistanceBasisDeployUnit {
			t.Errorf("DistanceBasis = %q, want deploy_unit", cl.DistanceBasis)
		}
		if cl.Distance != coupling.DistanceCrossDeployUnit {
			t.Errorf("Distance = %q, want cross_deploy_unit", cl.Distance)
		}
	})

	t.Run("same module → unknown basis (omits from JSON)", func(t *testing.T) {
		modules := map[string]policy.ModuleDef{
			modKeyPkgA: {Paths: []string{globPkgA}, Owner: ownerTeamX},
		}
		// Same-module edge; basis is unknown regardless of ownership config.
		cfg := config.Config{Modules: modules}.ForClassify()
		e := importEdge(fileAX, "pkg/a/y.go")
		cl := classify.Run(makeGraph([]graph.Edge{e}), cfg)[edgeKey(e)]
		if cl.DistanceBasis != coupling.DistanceBasisUnknown {
			t.Errorf("DistanceBasis = %q, want empty (unknown)", cl.DistanceBasis)
		}
	})
}

// TestRun_CohesiveCloseModules proves that intra-module coupling (same module)
// produces DistanceSameModule and an empty DistanceBasis. Strong coupling within
// a module is cohesion — not a defect — and must never be flagged as a problem.
func TestRun_CohesiveCloseModules(t *testing.T) {
	// Single module "internal/core". Two files within it.
	const soleOwner = "alice"
	modules := map[string]policy.ModuleDef{
		"internal/core": {Paths: []string{"internal/core/**"}, Owner: soleOwner},
	}
	cfg := classify.Config{Modules: modules}

	e := graph.Edge{
		From:     "file:internal/core/domain.go",
		To:       "file:internal/core/service.go",
		Kind:     graph.EdgeKindImports,
		Language: "go",
	}
	g := makeGraph([]graph.Edge{e})
	idx := classify.Run(g, cfg)

	cl, ok := idx[edgeKey(e)]
	if !ok {
		t.Fatalf("edge key not found in index")
	}
	if cl.Distance != coupling.DistanceSameModule {
		t.Errorf("Distance = %q, want same_module (intra-module coupling is cohesion, not a defect)", cl.Distance)
	}
	// DistanceBasis is unknown/empty for same-module edges — no cross-boundary signal.
	if cl.DistanceBasis != coupling.DistanceBasisUnknown {
		t.Errorf("DistanceBasis = %q, want empty (same_module case has no distance basis)", cl.DistanceBasis)
	}
}

// TestRun_SmallOSSDeployUnitBoundaryStaysFar proves that even in a single-owner
// (degenerate) repo, modules with different deploy units stay at CrossDeployUnit
// distance. Deploy unit is absolute: it overrides code-structure and ownership.
func TestRun_SmallOSSDeployUnitBoundaryStaysFar(t *testing.T) {
	// Single owner "alice" everywhere (degenerate → ownership suppressed).
	// But modules declare distinct deploy units, which is absolute.
	modules := map[string]policy.ModuleDef{
		modKeyPkgA: {Paths: []string{globPkgA}, Owner: "alice", DeployUnit: deployUnitA},
		modKeyPkgB: {Paths: []string{globPkgB}, Owner: "alice", DeployUnit: deployUnitB},
	}
	cfg := classify.Config{Modules: modules}

	e := graph.Edge{
		From:     filePkgAXGo,
		To:       filePkgBYGo,
		Kind:     graph.EdgeKindImports,
		Language: "go",
	}
	g := makeGraph([]graph.Edge{e})
	cl := classify.Run(g, cfg)[edgeKey(e)]

	if cl.Distance != coupling.DistanceCrossDeployUnit {
		t.Errorf("Distance = %q, want cross_deploy_unit (deploy boundary is absolute even in single-owner repos)", cl.Distance)
	}
	if cl.DistanceBasis != coupling.DistanceBasisDeployUnit {
		t.Errorf("DistanceBasis = %q, want deploy_unit", cl.DistanceBasis)
	}
}

// TestRun_VolatilityUnknownWhenModuleUnresolved verifies that an edge whose
// to-path matches no configured module yields VolatilityUnknown — genuinely
// indeterminate, distinct from the undeclared config-gap case above.
func TestRun_VolatilityUnknownWhenModuleUnresolved(t *testing.T) {
	cfg := config.Config{
		Modules: map[string]policy.ModuleDef{
			"a": {Paths: []string{globPkgA}, Owner: ownerTeamX},
		},
	}
	classifyCfg := cfg.ForClassify()

	e := graph.Edge{
		From:     "file:pkg/a/main.go",
		To:       "file:external/pkg/foo.go", // matches no module
		Kind:     graph.EdgeKindImports,
		Language: "go",
	}
	g := makeGraph([]graph.Edge{e})
	idx := classify.Run(g, classifyCfg)
	cl := idx[edgeKey(e)]

	if cl.Volatility != coupling.VolatilityUnknown {
		t.Errorf("Volatility = %q, want unknown", cl.Volatility)
	}
}

// buildGoWorkspaceGraph builds a Graph with the given GoModules and edges, suitable
// for testing AugmentGoWorkspaceModules.
// buildCrateGraph builds a Rust crate-level graph (package:<crate> nodes) plus
// the CrateRoots that map each crate to its repo-relative directory.
func buildCrateGraph(crateRoots []graph.CrateRoot, edges []graph.Edge) *graph.Graph {
	seen := make(map[string]bool)
	var nodes []graph.Node
	for _, e := range edges {
		for _, id := range []string{e.From, e.To} {
			if !seen[id] {
				seen[id] = true
				nodes = append(nodes, graph.Node{Kind: graph.NodeKindPackage, Path: graph.NodePath(id)})
			}
		}
	}
	return graph.Build([]graph.Facts{{Nodes: nodes, Edges: edges, Language: graph.LangRust, CrateRoots: crateRoots}})
}

// TestAugmentCargoCrateNodes covers the three cases of Rust crate-node binding:
// a directory-path config (ruff-like) binds the bare-name node to the dir-covering
// module while preserving its attributes; a bare-name config (tokio/yazi-like) is a
// no-op; and a single-crate graph is left untouched by the ≥2 gate.
func TestAugmentCargoCrateNodes(t *testing.T) {
	const (
		crateAst   = "ruff_python_ast"
		crateLint  = "ruff_linter"
		dirAst     = "crates/ruff_python_ast"
		dirLint    = "crates/ruff_linter"
		crateTokio = "tokio"
		crateUtil  = "tokio-util"
		crateSolo  = "solo"
	)
	crateEdge := func(from, to string) graph.Edge {
		return graph.Edge{
			From: "package:" + from, To: "package:" + to,
			Kind: graph.EdgeKindImports, Language: graph.LangRust, StrengthHint: hintFunctional,
		}
	}

	t.Run("path-glob config binds crate node and preserves attributes (ruff-like)", func(t *testing.T) {
		e := crateEdge(crateLint, crateAst)
		g := buildCrateGraph([]graph.CrateRoot{
			{Dir: dirLint, Name: crateLint},
			{Dir: dirAst, Name: crateAst},
		}, []graph.Edge{e})
		cfg := map[string]policy.ModuleDef{
			crateLint: {Paths: []string{dirLint + "/**"}, Volatility: "high", Subdomain: subdomainCore},
			crateAst:  {Paths: []string{dirAst + "/**"}, Volatility: "medium", Subdomain: "generic"},
		}
		mods := classify.AugmentCargoCrateNodes(g, cfg)
		// Bare crate name appended to the dir-covering module, attributes preserved.
		if !slices.Contains(mods[crateAst].Paths, crateAst) {
			t.Errorf("bare crate glob not bound: %v", mods[crateAst].Paths)
		}
		if mods[crateAst].Volatility != "medium" || mods[crateAst].Subdomain != "generic" {
			t.Errorf("module attributes lost: vol=%q sub=%q", mods[crateAst].Volatility, mods[crateAst].Subdomain)
		}
		// The cross-crate edge now classifies with a real Distance (was Unknown → external).
		idx := classify.Run(g, classify.Config{Modules: mods})
		cl, ok := idx[edgeKey(e)]
		if !ok {
			t.Fatal("cross-crate edge missing from coupling index")
		}
		if cl.Distance == coupling.DistanceUnknown {
			t.Error("cross-crate edge still distance-unknown after augmentation")
		}
	})

	t.Run("a bound crate node joins the part of the surface that holds its directory", func(t *testing.T) {
		e := crateEdge(crateLint, crateAst)
		g := buildCrateGraph([]graph.CrateRoot{
			{Dir: dirLint, Name: crateLint},
			{Dir: dirAst, Name: crateAst},
		}, []graph.Edge{e})
		for _, tc := range []struct {
			name          string
			public, model []string
			want          coupling.Strength
		}{
			// The resolved default: public is the module's paths.
			{"public is the whole module", []string{dirAst + "/**"}, nil, coupling.StrengthContract},
			{"public names the crate directory", []string{dirAst}, nil, coupling.StrengthContract},
			{"model names the crate directory", nil, []string{dirAst}, coupling.StrengthModel},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cfg := map[string]policy.ModuleDef{
					crateLint: {Paths: []string{dirLint + "/**"}},
					crateAst:  {Paths: []string{dirAst + "/**"}, Public: tc.public, Model: tc.model},
				}
				mods := classify.AugmentCargoCrateNodes(g, cfg)
				if got := classify.Run(g, classify.Config{Modules: mods})[edgeKey(e)].Strength; got != tc.want {
					t.Errorf("strength = %q, want %q: the bare crate node is in the declared part", got, tc.want)
				}
			})
		}
	})

	t.Run("bare-name config is a no-op (tokio/yazi-like)", func(t *testing.T) {
		e := crateEdge(crateTokio, crateUtil)
		g := buildCrateGraph([]graph.CrateRoot{
			{Dir: crateTokio, Name: crateTokio},
			{Dir: crateUtil, Name: crateUtil},
		}, []graph.Edge{e})
		cfg := map[string]policy.ModuleDef{
			crateTokio: {Paths: []string{crateTokio}},
			crateUtil:  {Paths: []string{crateUtil}},
		}
		mods := classify.AugmentCargoCrateNodes(g, cfg)
		if len(mods) != len(cfg) {
			t.Errorf("bare-name config not a no-op: module count %d→%d", len(cfg), len(mods))
		}
		if len(mods[crateTokio].Paths) != 1 {
			t.Errorf("tokio paths mutated: %v", mods[crateTokio].Paths)
		}
	})

	t.Run("single crate untouched (degenerate gate)", func(t *testing.T) {
		g := graph.Build([]graph.Facts{{
			Nodes:      []graph.Node{{Kind: graph.NodeKindPackage, Path: crateSolo}},
			Language:   graph.LangRust,
			CrateRoots: []graph.CrateRoot{{Dir: "crates/solo", Name: crateSolo}},
		}})
		cfg := map[string]policy.ModuleDef{crateSolo: {Paths: []string{"crates/solo/**"}}}
		mods := classify.AugmentCargoCrateNodes(g, cfg)
		if len(mods[crateSolo].Paths) != 1 || mods[crateSolo].Paths[0] != "crates/solo/**" {
			t.Errorf("single-crate repo should be untouched, got %v", mods[crateSolo].Paths)
		}
	})
}

func buildGoWorkspaceGraph(goMods []graph.GoModule, edges []graph.Edge) *graph.Graph {
	seen := make(map[string]bool)
	var nodes []graph.Node
	for _, e := range edges {
		for _, id := range []string{e.From, e.To} {
			if !seen[id] {
				seen[id] = true
				kind, path := graph.NodeKindFile, id
				for _, k := range []graph.NodeKind{
					graph.NodeKindFile, graph.NodeKindPackage,
					graph.NodeKindModule, graph.NodeKindRepo, graph.NodeKindExternal,
				} {
					prefix := string(k) + ":"
					if len(id) > len(prefix) && id[:len(prefix)] == prefix {
						kind = k
						path = id[len(prefix):]
						break
					}
				}
				nodes = append(nodes, graph.Node{Kind: kind, Path: path})
			}
		}
	}
	return graph.Build([]graph.Facts{{
		Nodes:     nodes,
		Edges:     edges,
		Language:  "go",
		GoModules: goMods,
	}})
}

// TestAugmentGoWorkspaceModules_TwoMembersAutoRegister verifies that when ≥2 Go
// workspace members are loaded, each gets a synthetic module registered (one per
// member), and that a cross-member edge with a StrengthHint classifies with a real
// Distance and a scored EdgeScore — the direct proxy for "coupling_balance measures."
func TestAugmentGoWorkspaceModules_TwoMembersAutoRegister(t *testing.T) {
	e := graph.Edge{
		From:         "package:services/a/foo",
		To:           "package:services/b/bar",
		Kind:         graph.EdgeKindImports,
		Language:     "go",
		StrengthHint: "functional",
	}
	goMods := []graph.GoModule{
		{Path: goModuleA, RelDir: relDirA},
		{Path: goModuleB, RelDir: relDirB},
	}
	g := buildGoWorkspaceGraph(goMods, []graph.Edge{e})

	mods := classify.AugmentGoWorkspaceModules(g, map[string]policy.ModuleDef{})
	if len(mods) != 2 {
		t.Fatalf("want 2 synthetic modules, got %d: %v", len(mods), mods)
	}
	// Each registered module must carry the member's glob.
	for _, m := range goMods {
		found := false
		for _, def := range mods {
			for _, p := range def.Paths {
				if p == m.RelDir+"/**" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("no synthetic module with glob %q for member %q", m.RelDir+"/**", m.Path)
		}
	}

	// Cross-member edge must get a real Distance and score.
	idx := classify.Run(g, classify.Config{Modules: mods})
	cl, ok := idx[edgeKey(e)]
	if !ok {
		t.Fatalf("edge not found in coupling index")
	}
	if cl.Distance == coupling.DistanceUnknown {
		t.Errorf("Distance = unknown; want a real distance (auto-registration broken)")
	}
	// With strength=functional + a real distance, the scorer must produce a scored result
	// — this is the unit-level proxy for "coupling_balance measures."
	if !cl.Score.Scored {
		t.Errorf("Score.Scored = false; want true (coupling_balance must measure when strength+distance are known)")
	}
}

// TestAugmentGoWorkspaceModules_OneMemberNoOp guards the ≥2 gate: a single loaded
// module must not trigger auto-registration, preserving byte-identical output for
// single-module repos and archfit's own self-scan.
func TestAugmentGoWorkspaceModules_OneMemberNoOp(t *testing.T) {
	g := buildGoWorkspaceGraph(
		[]graph.GoModule{{Path: "example.com/myapp", RelDir: "cmd/myapp"}},
		nil,
	)
	in := map[string]policy.ModuleDef{"mymod": {Paths: []string{pathsA}}}
	out := classify.AugmentGoWorkspaceModules(g, in)
	if len(out) != len(in) {
		t.Errorf("1-member: want map unchanged (len %d), got len %d: %v", len(in), len(out), out)
	}
}

// TestAugmentGoWorkspaceModules_ConfigGlobWins verifies that an already-configured
// module covering a member's directory takes precedence over auto-registration.
func TestAugmentGoWorkspaceModules_ConfigGlobWins(t *testing.T) {
	goMods := []graph.GoModule{
		{Path: goModuleA, RelDir: relDirA},
		{Path: goModuleB, RelDir: relDirB},
	}
	g := buildGoWorkspaceGraph(goMods, nil)
	// Both member directories already covered by explicit config modules.
	// Use the deployUnitA/B constants ("svc-a"/"svc-b") as map keys to satisfy goconst.
	in := map[string]policy.ModuleDef{
		deployUnitA: {Paths: []string{pathsA}},
		deployUnitB: {Paths: []string{pathsB}},
	}
	out := classify.AugmentGoWorkspaceModules(g, in)
	if len(out) != len(in) {
		t.Errorf("config globs win: want map unchanged (len %d), got len %d; extra: %v",
			len(in), len(out), out)
	}
}

// TestAugmentModulesFromGraph_OwnerInheritance verifies that a synthetic Rust
// submodule inherits the owner of its nearest config-declared ancestor (the crate
// module). Without this fix, inter-submodule edges classify as different_owner
// instead of cross_module (the herdr regression).
func TestAugmentModulesFromGraph_OwnerInheritance(t *testing.T) {
	// classify.Config declares the crate-level module with owner="team-x".
	// cargo-modules graph produces submodule nodes "mycrate::a" and "mycrate::b"
	// which are NOT in config — they should be synthesised and inherit owner "team-x".
	configMods := map[string]policy.ModuleDef{
		crateName: {Paths: []string{crateName + "/**"}, Owner: ownerTeamX},
	}
	e := graph.Edge{
		From:         "package:mycrate::a",
		To:           "package:mycrate::b",
		Kind:         graph.EdgeKindDependsOn,
		Language:     langRust,
		StrengthHint: hintFunctional,
	}
	g := makeGraph([]graph.Edge{e})

	augmented := classify.AugmentModulesFromGraph(g, configMods)

	// Both synthetic submodules must carry owner="team-x".
	for _, key := range []string{"mycrate::a", "mycrate::b"} {
		def, ok := augmented[key]
		if !ok {
			t.Fatalf("synthetic module %q not registered", key)
		}
		if def.Owner != ownerTeamX {
			t.Errorf("module %q: Owner = %q, want %q (inherited from ancestor)", key, def.Owner, ownerTeamX)
		}
	}

	// The inter-submodule edge must classify as cross_module, not different_owner.
	idx := classify.Run(g, classify.Config{Modules: augmented})
	cl, ok := idx[edgeKey(e)]
	if !ok {
		t.Fatalf("edge not found in index after augmentation")
	}
	if cl.Distance != coupling.DistanceCrossModule {
		t.Errorf("Distance = %q, want cross_module (submodules share inherited owner)", cl.Distance)
	}
}

// TestAugmentCargoCrateNodes_LeavesGoUntouched guards Fix group 0 Task 0.1
// (reproduces A1): AugmentCargoCrateNodes gates on
// "n.Kind != graph.NodeKindPackage", but graph.NodeKindPackage is emitted by
// the Go extractor too — nothing in the gate is Rust-specific. On a Go-only
// graph (no "::" module-graph nodes, no CrateRoots) with package nodes not
// covered by any configured module glob, the function must be a no-op: it
// must never register a synthetic module for a Go package. Today it does
// (the omni bug: a phantom "assets/dal"-style module gets registered with no
// owner), so this test is RED until Fix group 2 adds an explicit
// Rust-language gate.
func TestAugmentCargoCrateNodes_LeavesGoUntouched(t *testing.T) {
	const (
		goPkgUncovered1 = "internal/assets/dal"
		goPkgUncovered2 = "internal/assets/pkg"
	)
	g := graph.Build([]graph.Facts{{
		Nodes: []graph.Node{
			{Kind: graph.NodeKindPackage, Path: goPkgUncovered1},
			{Kind: graph.NodeKindPackage, Path: goPkgUncovered2},
		},
		Language: "go",
		// No CrateRoots: this is a pure Go graph, never a Rust one.
	}})
	// One declared module that does NOT cover either uncovered package —
	// mirrors omni's "intentionally-partial module coverage" shape.
	in := map[string]policy.ModuleDef{
		"declared": {Paths: []string{"cmd/**"}, Owner: ownerTeamX},
	}

	out := classify.AugmentCargoCrateNodes(g, in)

	if len(out) != len(in) {
		t.Fatalf("Go-only graph: want modules map unchanged (len %d), got len %d: %v", len(in), len(out), out)
	}
	for _, uncovered := range []string{goPkgUncovered1, goPkgUncovered2} {
		if _, exists := out[uncovered]; exists {
			t.Errorf("Go package %q must not be registered as a synthetic Rust crate module; out = %v", uncovered, out)
		}
	}
}

// wantInherited lists the ModuleDef fields a synthetic module must inherit
// from its nearest config-declared ancestor. Deliberately explicit (not
// reflect-based) so a future new ModuleDef field must be added here by a
// reviewer, rather than silently passing without coverage.
type wantInherited struct {
	Owner      string
	Volatility string
	Subdomain  string
	Layer      string
	DeployUnit string
}

// assertInherited fails t for every field of got that does not match want,
// so a single run reports every dropped attribute at once.
func assertInherited(t *testing.T, label string, got policy.ModuleDef, want wantInherited) {
	t.Helper()
	if got.Owner != want.Owner {
		t.Errorf("%s: Owner = %q, want %q (ancestor inheritance)", label, got.Owner, want.Owner)
	}
	if got.Volatility != want.Volatility {
		t.Errorf("%s: Volatility = %q, want %q (ancestor inheritance)", label, got.Volatility, want.Volatility)
	}
	if got.Subdomain != want.Subdomain {
		t.Errorf("%s: Subdomain = %q, want %q (ancestor inheritance)", label, got.Subdomain, want.Subdomain)
	}
	if got.Layer != want.Layer {
		t.Errorf("%s: Layer = %q, want %q (ancestor inheritance)", label, got.Layer, want.Layer)
	}
	if got.DeployUnit != want.DeployUnit {
		t.Errorf("%s: DeployUnit = %q, want %q (ancestor inheritance)", label, got.DeployUnit, want.DeployUnit)
	}
}

// TestAugmentFunctions_InheritAllAncestorAttributes guards Fix group 0 Task
// 0.4 (reproduces B3): every Augment* function that registers a synthetic
// module hand-writes its own ancestor lookup, and every one of them copies
// only Owner — Volatility, Subdomain, Layer, and DeployUnit are dropped for
// every synthetic module, in all three functions. Undeclared Volatility
// scores the conservative worst case (V=10), which is the root cause of
// tokio's 917-finding flood. RED today for Volatility/Subdomain/Layer/
// DeployUnit on all three functions.
func TestAugmentFunctions_InheritAllAncestorAttributes(t *testing.T) {
	want := wantInherited{
		Owner:      "team-inherit",
		Volatility: "low",
		Subdomain:  "supporting",
		Layer:      "domain",
		DeployUnit: "svc-inherit",
	}
	parent := policy.ModuleDef{
		Owner:      want.Owner,
		Volatility: want.Volatility,
		Subdomain:  want.Subdomain,
		Layer:      want.Layer,
		DeployUnit: want.DeployUnit,
	}

	t.Run("AugmentModulesFromGraph (Rust submodule)", func(t *testing.T) {
		// classify.Config declares only the crate-level module "mycrate"; the
		// cargo-modules graph produces submodule node "mycrate::child" that
		// is not in config and must be synthesised as a child of "mycrate".
		configMods := map[string]policy.ModuleDef{"mycrate": parent}
		e := graph.Edge{
			From: "package:mycrate", To: "package:mycrate::child",
			Kind: graph.EdgeKindDependsOn, Language: langRust, StrengthHint: hintFunctional,
		}
		g := makeGraph([]graph.Edge{e})

		out := classify.AugmentModulesFromGraph(g, configMods)
		child, ok := out["mycrate::child"]
		if !ok {
			t.Fatalf("synthetic module %q not registered", "mycrate::child")
		}
		assertInherited(t, "mycrate::child", child, want)
	})

	t.Run("AugmentGoWorkspaceModules (workspace member)", func(t *testing.T) {
		// Parent config module uses a literal (non-wildcard) path so it does
		// NOT fully cover the member directory via ModuleFor (so the member
		// still gets auto-registered), but its stripped directory IS a path
		// prefix of the member's RelDir, so ancestorByPath can donate —
		// mirrors the documented "partial ancestor donates owner" fallback.
		configMods := map[string]policy.ModuleDef{parentModuleKey: {
			Paths: []string{"services"}, Owner: want.Owner, Volatility: want.Volatility,
			Subdomain: want.Subdomain, Layer: want.Layer, DeployUnit: want.DeployUnit,
		}}
		goMods := []graph.GoModule{
			{Path: goModuleA, RelDir: relDirA},
			{Path: goModuleB, RelDir: relDirB},
		}
		g := buildGoWorkspaceGraph(goMods, nil)

		out := classify.AugmentGoWorkspaceModules(g, configMods)
		member, ok := out[goModuleA]
		if !ok {
			t.Fatalf("synthetic module for member %q not registered; out = %v", goModuleA, out)
		}
		assertInherited(t, goModuleA, member, want)
	})

	t.Run("AugmentCargoCrateNodes (uncovered crate)", func(t *testing.T) {
		// Same literal-parent-path trick as above: "crates" is a directory
		// prefix of the crate's root ("crates/extra") but does not itself
		// cover "crates/extra/x" via ModuleFor, so the crate falls through to
		// synthetic registration and donation via ancestorByPath.
		configMods := map[string]policy.ModuleDef{parentModuleKey: {
			Paths: []string{"crates"}, Owner: want.Owner, Volatility: want.Volatility,
			Subdomain: want.Subdomain, Layer: want.Layer, DeployUnit: want.DeployUnit,
		}}
		g := graph.Build([]graph.Facts{{
			Nodes: []graph.Node{
				{Kind: graph.NodeKindPackage, Path: rustCrateExtra},
				{Kind: graph.NodeKindPackage, Path: "other"},
			},
			Language:   langRust,
			CrateRoots: []graph.CrateRoot{{Dir: "crates/extra", Name: rustCrateExtra}},
		}})

		out := classify.AugmentCargoCrateNodes(g, configMods)
		crateMod, ok := out[rustCrateExtra]
		if !ok {
			t.Fatalf("synthetic module for crate %q not registered; out = %v", rustCrateExtra, out)
		}
		assertInherited(t, rustCrateExtra, crateMod, want)
	})
}

func TestAugmentFunctions_InheritOwnerlessAncestorAttributes(t *testing.T) {
	want := wantInherited{
		Volatility: extVolMedium,
		Subdomain:  subdomainCore,
		Layer:      "domain",
		DeployUnit: "svc-ownerless",
	}
	parent := policy.ModuleDef{
		Volatility: want.Volatility,
		Subdomain:  want.Subdomain,
		Layer:      want.Layer,
		DeployUnit: want.DeployUnit,
	}

	t.Run("AugmentModulesFromGraph (Rust submodule)", func(t *testing.T) {
		configMods := map[string]policy.ModuleDef{crateName: parent}
		e := graph.Edge{
			From: "package:" + crateName, To: "package:" + crateName + "::child",
			Kind: graph.EdgeKindDependsOn, Language: langRust, StrengthHint: hintFunctional,
		}
		g := makeGraph([]graph.Edge{e})

		out := classify.AugmentModulesFromGraph(g, configMods)
		child, ok := out[crateName+"::child"]
		if !ok {
			t.Fatalf("synthetic module %q not registered", crateName+"::child")
		}
		assertInherited(t, crateName+"::child", child, want)
	})

	t.Run("AugmentGoWorkspaceModules (workspace member)", func(t *testing.T) {
		configMods := map[string]policy.ModuleDef{parentModuleKey: {
			Paths:      []string{"services"},
			Volatility: want.Volatility,
			Subdomain:  want.Subdomain,
			Layer:      want.Layer,
			DeployUnit: want.DeployUnit,
		}}
		goMods := []graph.GoModule{
			{Path: goModuleA, RelDir: relDirA},
			{Path: goModuleB, RelDir: relDirB},
		}
		g := buildGoWorkspaceGraph(goMods, nil)

		out := classify.AugmentGoWorkspaceModules(g, configMods)
		member, ok := out[goModuleA]
		if !ok {
			t.Fatalf("synthetic module for member %q not registered; out = %v", goModuleA, out)
		}
		assertInherited(t, goModuleA, member, want)
	})

	t.Run("AugmentCargoCrateNodes (uncovered crate)", func(t *testing.T) {
		configMods := map[string]policy.ModuleDef{parentModuleKey: {
			Paths:      []string{"crates"},
			Volatility: want.Volatility,
			Subdomain:  want.Subdomain,
			Layer:      want.Layer,
			DeployUnit: want.DeployUnit,
		}}
		g := graph.Build([]graph.Facts{{
			Nodes: []graph.Node{
				{Kind: graph.NodeKindPackage, Path: rustCrateExtra},
				{Kind: graph.NodeKindPackage, Path: "other"},
			},
			Language:   langRust,
			CrateRoots: []graph.CrateRoot{{Dir: "crates/extra", Name: rustCrateExtra}},
		}})

		out := classify.AugmentCargoCrateNodes(g, configMods)
		crateMod, ok := out[rustCrateExtra]
		if !ok {
			t.Fatalf("synthetic module for crate %q not registered; out = %v", rustCrateExtra, out)
		}
		assertInherited(t, rustCrateExtra, crateMod, want)
	})
}

// TestRun_SameModuleEdgesAbstain: the declared surface speaks only to other
// modules, so an edge inside one module has no strength, whatever type hint
// the extractor read, and its score abstains (missing evidence is not zero).
// It keeps SeverityNone, so the advisory pipeline never sees it.
func TestRun_SameModuleEdgesAbstain(t *testing.T) {
	modules := map[string]policy.ModuleDef{
		"a": {Paths: []string{pathsA}, Public: []string{publicA}, Subdomain: subdomainCore},
	}
	cfg := classify.Config{Modules: modules}
	for _, hint := range []string{"", string(coupling.StrengthModel), hintFunctional, hintIntrusive} {
		e := graph.Edge{
			From: "file:services/a/mud.go", To: "file:services/a/y.go",
			Kind: graph.EdgeKindImports, Language: "go", StrengthHint: hint,
		}
		cl := classify.Run(makeGraph([]graph.Edge{e}), cfg)[edgeKey(e)]
		if cl.Distance != coupling.DistanceSameModule {
			t.Fatalf("hint %q: Distance = %q, want same_module", hint, cl.Distance)
		}
		if cl.Strength != coupling.StrengthUnknown || cl.Score.Scored || cl.Severity != coupling.SeverityNone {
			t.Errorf("hint %q: strength %q, scored %v, severity %q; want unknown, abstained, none",
				hint, cl.Strength, cl.Score.Scored, cl.Severity)
		}
	}
}
