// Package classify assigns Balanced Coupling classifications to graph edges:
// strength, distance, volatility, and explicitness.
package classify

import (
	"maps"
	gopath "path"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/coupling-dev/archfit/internal/model/graph"
	"github.com/coupling-dev/archfit/internal/policy"
	"github.com/coupling-dev/archfit/internal/policy/resolved"
	"github.com/coupling-dev/archfit/internal/relationship/balance"
	"github.com/coupling-dev/archfit/internal/relationship/coupling"
	"github.com/coupling-dev/archfit/internal/relationship/scoring"
)

// subdomain constants are the accepted Khononov subdomain values used throughout
// the classify package to derive volatility and detect generic targets.
const (
	subdomainCore       = "core"
	subdomainSupporting = "supporting"
	subdomainGeneric    = "generic"
)

// volatility level literals accepted in config (`volatility:` on modules and
// external_systems entries; "legacy" is a module-only alias for frozen).
const (
	volatilityHigh   = "high"
	volatilityMedium = "medium"
	volatilityLow    = "low"
	volatilityFrozen = "frozen"
	volatilityLegacy = "legacy"
)

// Run classifies every edge in g and returns a coupling.Index keyed by the
// edge canonical key (from + "\x00" + to + "\x00" + kind).
//
// For each edge:
//   - Strength: balance.StrengthOf over the declared surface of the
//     module that owns the target (contract, model or intrusive); unknown when
//     no module owns it or the edge stays inside one module.
//   - Distance: same_module, or one of three tokens for a module boundary
//     (cross_module / cross_module_different_owner / cross_deploy_unit) that all
//     score at D=9. When either endpoint cannot be resolved to a module,
//     distance is unknown.
//   - Volatility: derived from the to-module's subdomain field
//     (core→high, supporting→low, generic→low, ""/"unknown"→unknown).
//   - Explicitness: explicit when strength=contract; implicit when strength=intrusive;
//     unknown otherwise.
//   - Score: continuous EdgeScore from the production BookScorer.
//     Applied to every known-distance edge; unknown-distance edges are zero.
//     A same-module edge has no strength, so its score abstains and it keeps
//     SeverityNone — the advisory pipeline stays cross-boundary.
func Run(g *graph.Graph, c Config) coupling.Index {
	mm := buildModuleIndex(c.Modules)
	idx := make(coupling.Index)
	scorer := scoring.DefaultScorer()

	effectiveVol := computeEffectiveVolatility(g, mm, c)
	extSystems := buildExternalSystemIndex(c.ExternalSystems)

	for _, e := range g.Edges() {
		cl := classify(e, mm, c, effectiveVol, extSystems)
		// Score every edge whose distance is known; unknown-distance edges are
		// not scored (zero EdgeScore). A same-module edge has no strength
		// (resolveStrength), so the scorer abstains on it and the local_coupling
		// block lists it as abstained.
		// Severity is derived from cl.Score.Band so the book formula and the
		// advisory severity are always identical — the single source of truth.
		if cl.Distance != coupling.DistanceUnknown {
			cl.Score = scorer.Score(cl)
			// Set Severity from the book score band for cross-boundary edges only.
			// Same-module edges keep SeverityNone: the bc/imbalanced_coupling
			// advisory pipeline and coupling_balance stay cross-module (fractal
			// level separation); local complexity is report-only. Abstained edges
			// (Scored=false, Band="") remain SeverityNone — abstain-not-fake.
			if cl.Distance != coupling.DistanceSameModule {
				cl.Severity = cl.Score.Band
			}
		}
		idx[edgeKey(e)] = cl
	}

	return idx
}

// edgeKey returns the canonical coupling.Index key for an edge.
// Matches the format documented in coupling.Index: from + "\x00" + to + "\x00" + kind.
func edgeKey(e graph.Edge) string {
	return e.From + "\x00" + e.To + "\x00" + string(e.Kind)
}

// pathFromID extracts the path component from a node ID of the form "kind:path".
// If there is no ":", the entire ID is returned.
func pathFromID(id string) string {
	_, after, ok := strings.Cut(id, ":")
	if ok {
		return after
	}
	return id
}

// moduleIndex holds the module definitions for classification. Path→module
// resolution delegates to the shared policy.ModuleMap so there is a single
// most-specific-match implementation; modules gives the owner's declared
// surface to resolveStrength.
type moduleIndex struct {
	modules map[string]policy.ModuleDef
	mm      policy.ModuleMap
}

// buildModuleIndex builds the index from the Modules map.
func buildModuleIndex(modules map[string]policy.ModuleDef) moduleIndex {
	return moduleIndex{modules: modules, mm: policy.BuildModuleMap(modules)}
}

// moduleFor returns the most-specific module name whose Paths globs match path,
// delegating to policy.ModuleMap (single source of truth for resolution).
// Returns ("", false) if no module matches.
func (mi moduleIndex) moduleFor(path string) (string, bool) {
	return mi.mm.ModuleFor(path)
}

// AugmentModulesFromGraph returns a Modules map extended with a synthetic module
// for every first-party module-graph node not already covered by a configured
// module's path globs. Rust intra-crate module nodes ("<crate>::<mod>", produced by
// the cargo-modules extractor) are otherwise unknown to moduleFor, so their edges
// classify as distance-unknown and never count toward coupling_balance or
// encapsulation. The gate is the "::" separator, which only the Rust module-graph
// convention uses (Go/TS use "/", Python "."), so Go/TS/Python graphs and their
// configured modules are untouched. Existing config modules keep precedence — a
// synthetic module is only added when nothing already covers the node. The input
// map is not mutated; a copy is returned only if something is added.
func AugmentModulesFromGraph(g *graph.Graph, modules map[string]policy.ModuleDef) map[string]policy.ModuleDef {
	if g == nil {
		return modules
	}
	mi := buildModuleIndex(modules)
	out := modules
	cloned := false
	for _, n := range g.Nodes() {
		path := n.Path
		if n.Kind == graph.NodeKindExternal || !strings.Contains(path, "::") {
			continue // external dep, or not a Rust module-graph node
		}
		if _, ok := mi.moduleFor(path); ok {
			continue // already covered by a configured module
		}
		if !cloned {
			out = make(map[string]policy.ModuleDef, len(modules)+8)
			maps.Copy(out, modules)
			cloned = true
		}
		if _, exists := out[path]; !exists {
			out[path] = inheritAncestorAttrs(ancestorByKey(path, modules), []string{path})
		}
	}
	return out
}

// AugmentGoWorkspaceModules returns a Modules map extended with a synthetic module
// for each Go workspace member whose directory is not already covered by a configured
// module's path globs. Called when ≥2 workspace members were loaded so cross-member
// edges can classify with a real Distance for coupling_balance.
//
// The gate is len(GoModules) >= 2 — single-module repos and archfit's own self-scan
// (which collapses to 1 surviving member after exclusion) are byte-identical to
// before, mirroring the Rust "::" gate in AugmentModulesFromGraph.
//
// Members at the repo root (RelDir == ".") are skipped: a root-relative glob would
// over-match sibling members (abstain-not-fake). Existing config modules keep
// precedence — a synthetic module is only added when nothing already covers the
// member's directory. The input map is not mutated; a copy is returned only if
// something is added.
func AugmentGoWorkspaceModules(g *graph.Graph, modules map[string]policy.ModuleDef) map[string]policy.ModuleDef {
	if g == nil {
		return modules
	}
	goMods := g.GoModules()
	if len(goMods) < 2 {
		return modules // ≥2 gate: single-module repos and self-scan are untouched
	}
	mi := buildModuleIndex(modules)
	out := modules
	cloned := false
	for _, m := range goMods {
		if m.RelDir == "." {
			continue // root member: no precise glob boundary; abstain-not-fake
		}
		// "already covered" check: does any configured module glob match a
		// representative path inside this member's directory?
		if _, ok := mi.moduleFor(m.RelDir + "/x"); ok {
			continue
		}
		if !cloned {
			out = make(map[string]policy.ModuleDef, len(modules)+len(goMods))
			maps.Copy(out, modules)
			cloned = true
		}
		if _, exists := out[m.Path]; !exists {
			// Inherit attributes from the nearest config-declared ancestor module.
			// For Go workspace members the "already covered" check above uses
			// mi.moduleFor(m.RelDir+"/x") — if there were a covering ancestor
			// we'd have skipped this member. Instead we do a direct prefix scan
			// on the member's RelDir so partial ancestors (e.g. a module whose
			// glob covers a parent dir) can still donate their attributes.
			out[m.Path] = inheritAncestorAttrs(ancestorByPath(m.RelDir, modules), []string{m.RelDir + "/**"})
		}
	}
	return out
}

// AugmentCargoCrateNodes binds crate-level Rust nodes (a `package:<crate>` node
// whose path is the bare crate name, no "::") to a declared module when the config
// used a directory-path glob (e.g. "crates/ruff_python_ast/**") that matches the
// crate's source dir but NOT the bare crate-name node the cargo-metadata extractor
// emits. Without this, such edges classify distance-unknown → external → 0 scored,
// and coupling_balance reports n/a even though the architect declared the crates.
//
// For each crate node not already covered by a configured glob: if a configured
// module covers the crate's repo-relative directory (from graph CrateRoots), the
// bare crate name is appended to THAT module's Paths so the node binds while keeping
// the module's owner/subdomain/volatility/layer. If no module covers the dir, a
// synthetic module is registered (bare name, ancestor-inherited owner).
//
// Gate: ≥2 first-party crate nodes — a single-crate repo is a degenerate graph
// (coupling unmeasurable → n/a) and is left untouched. Configs that already use
// bare crate-name globs (tokio, yazi) bind on the first check and are no-ops.
// Intra-crate "<crate>::<mod>" nodes are handled by AugmentModulesFromGraph.
// The input map is not mutated; a copy is returned only if something is added.
func AugmentCargoCrateNodes(g *graph.Graph, modules map[string]policy.ModuleDef) map[string]policy.ModuleDef {
	if g == nil {
		return modules
	}
	crateNodes := make([]string, 0)
	for _, n := range g.Nodes() {
		if n.Kind != graph.NodeKindPackage || n.Language != graph.LangRust || strings.Contains(n.Path, "::") {
			continue // non-package, non-Rust (e.g. Go), or intra-crate module node (handled elsewhere)
		}
		crateNodes = append(crateNodes, n.Path)
	}
	if len(crateNodes) < 2 {
		return modules // <2 crates: degenerate graph → coupling unmeasurable (n/a)
	}
	dirByName := make(map[string]string, len(crateNodes))
	for _, cr := range g.CrateRoots() {
		dirByName[cr.Name] = cr.Dir
	}
	mi := buildModuleIndex(modules)
	out := modules
	cloned := false
	ensureClone := func() {
		if !cloned {
			out = make(map[string]policy.ModuleDef, len(modules)+len(crateNodes))
			maps.Copy(out, modules)
			cloned = true
		}
	}
	for _, name := range crateNodes {
		if _, ok := mi.moduleFor(name); ok {
			continue // already bound by a bare crate-name glob (e.g. tokio, yazi)
		}
		dir := dirByName[name]
		if dir != "" {
			if modName, ok := mi.moduleFor(dir + "/x"); ok {
				// A configured module covers the crate dir but not its bare-name node;
				// bind the node to it, preserving owner/subdomain/volatility/layer.
				// The bare node is the crate, so it joins the part of the
				// surface that holds the crate's directory.
				ensureClone()
				def := out[modName]
				def.Paths = append(append([]string{}, def.Paths...), name)
				if resolved.InPart(def.Public, dir, false) || namesFilesIn(def.Public, dir) {
					def.Public = append(append([]string{}, def.Public...), name)
				}
				if resolved.InPart(def.Model, dir, false) || namesFilesIn(def.Model, dir) {
					def.Model = append(append([]string{}, def.Model...), name)
				}
				out[modName] = def
				continue
			}
		}
		// No configured module covers the crate: register a synthetic one.
		ensureClone()
		if _, exists := out[name]; !exists {
			out[name] = inheritAncestorAttrs(ancestorByPath(dir, modules), []string{name})
		}
	}
	return out
}

// ancestorByKey finds the nearest config-declared ancestor of a Rust
// module-graph node (key uses "::" separator). It returns the ModuleDef of the
// config module whose key is the longest "::"-prefix of path, or the zero
// ModuleDef if none. Owner is not required: an ownerless parent can still donate
// volatility, subdomain, layer, and deploy-unit metadata.
func ancestorByKey(path string, modules map[string]policy.ModuleDef) policy.ModuleDef {
	var best policy.ModuleDef
	bestLen := 0
	for name, def := range modules {
		// A module is an ancestor when path starts with name+"::" or equals name.
		prefix := name + "::"
		if path == name || strings.HasPrefix(path, prefix) {
			if len(name) > bestLen {
				bestLen = len(name)
				best = def
			}
		}
	}
	return best
}

// ancestorByPath finds the nearest config-declared ancestor for a Go workspace
// member or Rust crate directory, matching by directory path prefix. It
// returns the ModuleDef of the config module whose glob paths share the
// longest directory prefix with relDir, or the zero ModuleDef if none. This is
// a fallback for the case where no module glob fully covers the child
// (otherwise the caller would have skipped it as already-covered), but a
// parent-directory module may still donate its attributes. Owner is not
// required: an ownerless parent can still donate volatility, subdomain, layer,
// and deploy-unit metadata.
func ancestorByPath(relDir string, modules map[string]policy.ModuleDef) policy.ModuleDef {
	var best policy.ModuleDef
	bestLen := 0
	for _, def := range modules {
		for _, p := range def.Paths {
			// Strip trailing glob suffixes to get the directory root.
			dir := strings.TrimRight(strings.TrimSuffix(strings.TrimSuffix(p, "**"), "/"), "/")
			if dir == "" {
				continue
			}
			if relDir == dir || strings.HasPrefix(relDir, dir+"/") {
				if len(dir) > bestLen {
					bestLen = len(dir)
					best = def
				}
			}
		}
	}
	return best
}

// inheritAncestorAttrs builds a synthetic ModuleDef for a newly registered
// module, carrying paths plus every inheritable attribute — Owner, Volatility,
// Subdomain, Layer, DeployUnit — from the nearest config-declared ancestor.
// The single shared helper for all three Augment* functions, so a synthetic
// module never silently drops Volatility/Subdomain/Layer/DeployUnit the way an
// Owner-only copy would (undeclared Volatility scores the conservative worst
// case, V=10 — the root cause of the tokio finding flood).
func inheritAncestorAttrs(ancestor policy.ModuleDef, paths []string) policy.ModuleDef {
	return policy.ModuleDef{
		Paths:      paths,
		Owner:      ancestor.Owner,
		Volatility: ancestor.Volatility,
		Subdomain:  ancestor.Subdomain,
		Layer:      ancestor.Layer,
		DeployUnit: ancestor.DeployUnit,
	}
}

// matchesAnyGlob reports whether path matches any of the given glob patterns.
func matchesAnyGlob(path string, globs []string) bool {
	for _, pattern := range globs {
		if matched, _ := doublestar.Match(pattern, path); matched {
			return true
		}
	}
	return false
}

// classify computes a Classification for a single edge.
//
// ExplicitnessHint on the edge overrides the config-glob-derived explicitness
// when non-empty ("explicit" or "implicit"). Severity is set in Run after the
// book score is computed (cl.Score.Band → cl.Severity).
func classify(e graph.Edge, mi moduleIndex, c Config, effectiveVol map[string]coupling.Volatility, extSystems externalSystemIndex) coupling.Classification {
	modules := c.Modules
	fromPath := pathFromID(e.From)
	toPath := pathFromID(e.To)

	str := resolveStrength(e, mi)

	// --- Distance & volatility ---
	// Every strength balance.StrengthOf gives takes the target's volatility: the
	// upstream module's volatility decides (the surfaces design 6.1). A
	// declared external target keeps its own declared volatility.
	dist, distBasis, vol := resolveDistanceVolatility(fromPath, toPath, mi, c, effectiveVol, extSystems)

	// --- Explicitness ---
	// ExplicitnessHint from the extractor (AST signal) takes precedence over the
	// config-glob heuristic when it is set.
	exp := classifyExplicitness(str)
	switch e.ExplicitnessHint {
	case "explicit":
		exp = coupling.ExplicitnessExplicit
	case "implicit":
		exp = coupling.ExplicitnessImplicit
	}

	// --- Contract-recommended advisory ---
	// When a generic-subdomain target is reached via non-contract strength (model,
	// functional, intrusive, or unknown), BC's anti-corruption-layer guidance applies:
	// introduce a contract (interface/adapter) so the caller is decoupled from the
	// provider's implementation volatility. This flag is carried on the Classification
	// so the engine can emit a dedicated advisory finding.
	contractRecommended := str != coupling.StrengthContract &&
		dist != coupling.DistanceSameModule &&
		isGenericSubdomain(toPath, mi, modules)

	return coupling.Classification{
		Strength:            str,
		Distance:            dist,
		Volatility:          vol,
		Explicitness:        exp,
		ContractRecommended: contractRecommended,
		DistanceBasis:       distBasis,
		Connascence:         connascenceFromHints(e.ConnascenceHints),
	}
}

// connascenceFromHints maps extractor edge hints into typed coupling evidence,
// deduplicated and sorted for deterministic output. Unknown kinds abstain.
func connascenceFromHints(hints []graph.ConnascenceHint) []coupling.ConnascenceEvidence {
	if len(hints) == 0 {
		return nil
	}
	seen := make(map[coupling.ConnascenceEvidence]struct{}, len(hints))
	out := make([]coupling.ConnascenceEvidence, 0, len(hints))
	for _, h := range hints {
		kind, ok := connascenceKind(h.Kind)
		if !ok || h.Source == "" {
			continue
		}
		ev := coupling.ConnascenceEvidence{Kind: kind, Source: h.Source, Detail: h.Detail}
		if _, exists := seen[ev]; exists {
			continue
		}
		seen[ev] = struct{}{}
		out = append(out, ev)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Detail < out[j].Detail
	})
	return out
}

func connascenceKind(kind string) (coupling.ConnascenceKind, bool) {
	switch kind {
	case graph.ConnascenceName:
		return coupling.ConnascenceName, true
	case graph.ConnascenceType:
		return coupling.ConnascenceType, true
	case graph.ConnascenceMeaning:
		return coupling.ConnascenceMeaning, true
	case graph.ConnascenceAlgorithm:
		return coupling.ConnascenceAlgorithm, true
	case graph.ConnascencePosition:
		return coupling.ConnascencePosition, true
	default:
		return "", false
	}
}

// resolveDistanceVolatility computes the distance token and the effective
// volatility for an edge.
//
// Declared external system (`external_systems:`), book Ch10 Example 1: a
// declared cross-vendor integration seam sits at the distance ladder's far end
// and ENTERS scoring, carrying the declared volatility (default low). Only a
// DECLARED external target gets D=10; an undeclared external edge keeps the
// disclosed exclusion (DistanceUnknown → classified_edges.external) — scoring
// every library import at D=10 would flood the metric with vendor noise. The
// match is gated on the TARGET's own resolution, not the composite distance:
// classifyDistance also returns DistanceUnknown when only the SOURCE is
// unresolved, and an edge into a real declared module must never be re-labelled
// external just because an external glob overlaps that module's path space.
func resolveDistanceVolatility(fromPath, toPath string, mi moduleIndex, c Config, effectiveVol map[string]coupling.Volatility, extSystems externalSystemIndex) (coupling.Distance, coupling.DistanceBasis, coupling.Volatility) {
	modules := c.Modules
	dist, distBasis := classifyDistance(fromPath, toPath, mi, modules)
	if dist == coupling.DistanceUnknown {
		if _, toOK := mi.moduleFor(toPath); !toOK {
			if v, ok := extSystems.match(toPath); ok {
				return coupling.DistanceExternal, coupling.DistanceBasisExternal, v
			}
		}
	}
	return dist, distBasis, classifyVolatilityEffective(toPath, mi, modules, effectiveVol)
}

// resolveStrength gives an edge the strength of the surfaces design 6.1: the
// module that owns the target, and where the target sits in that module's
// declared surface, decide it through balance.StrengthOf, as they do for
// the check. The surface speaks only to other modules, so an edge inside one
// module, or to a target in no module, has no strength (unknown) and its score
// abstains. The extractors' type facts (StrengthHint, DataStrengthHint) and
// connascence are evidence; they never set a strength.
func resolveStrength(e graph.Edge, mi moduleIndex) coupling.Strength {
	toPath := pathFromID(e.To)
	owner, ok := mi.moduleFor(toPath)
	if !ok {
		return coupling.StrengthUnknown
	}
	if from, ok := mi.moduleFor(pathFromID(e.From)); ok && from == owner {
		return coupling.StrengthUnknown
	}
	return balance.StrengthOf(surfaceOf(mi.modules[owner], toPath, isFileNode(e.To)))
}

// surfaceOf places a graph-node path in the declared surface of def, with
// the matcher the check uses (resolved.InPart). A module that declares
// neither public nor model is public as a whole, as the policy defaults say
// (resolved.Policy.WithDefaults); so is a module the report adds on its own,
// a Go workspace member or a Rust module, which declares nothing. A module
// that declares only model has no public part.
func surfaceOf(def policy.ModuleDef, path string, isFile bool) balance.Surface {
	if len(def.Public) == 0 && len(def.Model) == 0 {
		return balance.Surface{Public: true}
	}
	in := func(globs []string) bool {
		return resolved.InPart(globs, path, isFile) || !isFile && namesFilesIn(globs, path)
	}
	return balance.Surface{Public: in(def.Public), Model: in(def.Model)}
}

// namesFilesIn reports whether a glob names files directly in dir, such as
// pkg/api/*.go or pkg/api/api.go for the package pkg/api. The check counts a
// package as in a part when one of its files is; the graph has no file list
// for a package node, so the report reads the glob's directory instead. A
// glob whose last segment has no dot and no wildcard names a directory
// (pkg/api), not files in its parent. Limit: a wildcard directory
// (pkg/*/api.go) counts for every package it names, also one without that
// file.
func namesFilesIn(globs []string, dir string) bool {
	for _, g := range globs {
		if !strings.ContainsAny(gopath.Base(g), ".*?[{") {
			continue
		}
		if ok, _ := doublestar.Match(gopath.Dir(g), dir); ok {
			return true
		}
	}
	return false
}

// isFileNode reports whether a graph node id names a file. A package or a
// project node names a directory or a unit, which the surface matches as a
// whole.
func isFileNode(id string) bool {
	kind, _, _ := strings.Cut(id, ":")
	return graph.NodeKind(kind) == graph.NodeKindFile
}

// classifyDistance names the boundary of an edge. A module boundary is the far
// end of the in-house ladder (D=9), so the token only records what else changes
// across it, in this order:
//
//  1. A differing deploy unit → cross_deploy_unit (basis deploy_unit).
//  2. Two non-empty, differing owners → cross_module_different_owner
//     (basis ownership).
//  3. Otherwise → cross_module (basis module_boundary).
//
// The token never moves severity. An owner change can relabel a seam; it cannot
// make one qualify.
func classifyDistance(fromPath, toPath string, mi moduleIndex, modules map[string]policy.ModuleDef) (coupling.Distance, coupling.DistanceBasis) {
	fromMod, fromOK := mi.moduleFor(fromPath)
	toMod, toOK := mi.moduleFor(toPath)

	if !fromOK || !toOK {
		return coupling.DistanceUnknown, coupling.DistanceBasisUnknown
	}

	if fromMod == toMod {
		return coupling.DistanceSameModule, coupling.DistanceBasisUnknown
	}

	return moduleDistance(fromMod, toMod, modules)
}

// moduleDistance names the boundary between two RESOLVED, distinct modules.
func moduleDistance(fromMod, toMod string, modules map[string]policy.ModuleDef) (coupling.Distance, coupling.DistanceBasis) {
	fromDef, toDef := modules[fromMod], modules[toMod]
	if fromDef.DeployUnit != "" && toDef.DeployUnit != "" && fromDef.DeployUnit != toDef.DeployUnit {
		return coupling.DistanceCrossDeployUnit, coupling.DistanceBasisDeployUnit
	}
	if fromDef.Owner != "" && toDef.Owner != "" && fromDef.Owner != toDef.Owner {
		return coupling.DistanceCrossModuleDiffOwner, coupling.DistanceBasisOwnership
	}
	return coupling.DistanceCrossModule, coupling.DistanceBasisModule
}

// classifyVolatility derives domain volatility for the to-module from declared
// metadata only, per Khononov's volatility-from-subdomain mapping (core→high,
// supporting→low, generic→low) with an explicit per-module override:
//
//  1. Explicit `volatility` field on the module definition (hand-authored override).
//  2. Subdomain mapping: core→high, supporting→low, generic→low.
//
// There is NO path/name guessing: archfit does not infer volatility from a
// directory name (a fragile, surprising heuristic). A module that declares
// neither is reported as VolatilityUndeclared so the result is honest and the
// user is nudged to declare it.
//
// Resolution outcomes are deliberately three-valued:
//   - to-module unresolved → VolatilityUnknown (genuinely indeterminate).
//   - to-module resolved but neither volatility nor subdomain declared →
//     VolatilityUndeclared (a config gap; scored conservatively as worst-case,
//     and Lint() surfaces it).
//   - otherwise → high/medium/low/frozen.
//
// No churn or git history is consulted here — volatility is config-declared only.
func classifyVolatility(toPath string, mi moduleIndex, modules map[string]policy.ModuleDef) coupling.Volatility {
	toMod, ok := mi.moduleFor(toPath)
	if !ok {
		return coupling.VolatilityUnknown
	}
	def := modules[toMod]

	// Priority 1: explicit Volatility field.
	// Accepted values: high, medium, low, frozen, legacy.
	switch strings.ToLower(def.Volatility) {
	case volatilityHigh:
		return coupling.VolatilityHigh
	case volatilityMedium:
		return coupling.VolatilityMedium
	case volatilityLow:
		return coupling.VolatilityLow
	case volatilityFrozen, volatilityLegacy:
		return coupling.VolatilityFrozen
	}

	// Priority 2: subdomain mapping.
	switch strings.ToLower(def.Subdomain) {
	case subdomainCore:
		return coupling.VolatilityHigh
	case subdomainSupporting:
		return coupling.VolatilityLow
	case subdomainGeneric:
		return coupling.VolatilityLow
	}

	// Nothing declared — a closable config gap, reported honestly (no guessing).
	return coupling.VolatilityUndeclared
}

// isGenericSubdomain reports whether the to-module is classified as a generic
// subdomain — true only when the module explicitly declares `subdomain: generic`.
// (No path/name guessing: archfit does not infer "generic" from a directory name.)
//
// This is used to trigger the contract-recommended advisory when a generic
// target is reached via non-contract strength (BC's anti-corruption-layer guidance).
func isGenericSubdomain(toPath string, mi moduleIndex, modules map[string]policy.ModuleDef) bool {
	toMod, ok := mi.moduleFor(toPath)
	if !ok {
		return false
	}
	return strings.ToLower(modules[toMod].Subdomain) == subdomainGeneric
}

// computeEffectiveVolatility computes per-module effective volatility after an
// inferred-volatility cascade (book Ch9). When cascade is disabled, returns the
// base (config-declared) volatility for each module unchanged.
//
// Propagation rule: if module A is strongly coupled (strength ≥ functional) to
// module B and B's effective volatility is high, A's effective volatility is
// raised to high. The pass runs to a deterministic fixpoint, so volatility can
// propagate across a chain of deliberate strong integrations instead of stopping
// at one hop. Values are only raised, never lowered.
//
// Strong strength set for propagation: Functional, Symmetric, Intrusive.
func computeEffectiveVolatility(g *graph.Graph, mi moduleIndex, c Config) map[string]coupling.Volatility {
	modules := c.Modules
	// Seed effective map from config-declared volatility.
	effective := make(map[string]coupling.Volatility, len(modules))
	for name, def := range modules {
		effective[name] = volatilityFromDef(def)
	}
	if !c.VolatilityCascadeEnabled || g == nil {
		return effective
	}

	type cascadeEdge struct{ from, to string }
	edges := make([]cascadeEdge, 0)
	for _, e := range g.Edges() {
		resolved := resolveStrength(e, mi)
		if !isStrongStrength(resolved) {
			continue
		}
		fromPath := pathFromID(e.From)
		toPath := pathFromID(e.To)
		fromMod, okFrom := mi.moduleFor(fromPath)
		toMod, okTo := mi.moduleFor(toPath)
		if !okFrom || !okTo || fromMod == toMod {
			continue
		}
		edges = append(edges, cascadeEdge{from: fromMod, to: toMod})
	}

	for changed := true; changed; {
		changed = false
		for _, e := range edges {
			if effective[e.to] != coupling.VolatilityHigh || effective[e.from] == coupling.VolatilityHigh {
				continue
			}
			effective[e.from] = coupling.VolatilityHigh
			changed = true
		}
	}
	return effective
}

// volatilityFromDef derives base volatility from a ModuleDef using the same
// priority logic as classifyVolatility but without the path heuristic (the
// module is already resolved; we want the config-declared level only).
func volatilityFromDef(def policy.ModuleDef) coupling.Volatility {
	switch strings.ToLower(def.Volatility) {
	case volatilityHigh:
		return coupling.VolatilityHigh
	case volatilityMedium:
		return coupling.VolatilityMedium
	case volatilityLow:
		return coupling.VolatilityLow
	case volatilityFrozen, volatilityLegacy:
		return coupling.VolatilityFrozen
	}
	switch strings.ToLower(def.Subdomain) {
	case subdomainCore:
		return coupling.VolatilityHigh
	case subdomainSupporting:
		return coupling.VolatilityLow
	case subdomainGeneric:
		return coupling.VolatilityLow
	}
	return coupling.VolatilityUndeclared
}

// isStrongStrength reports whether str is at or above the functional threshold
// for the inferred-volatility cascade. Contract and model are below the threshold.
func isStrongStrength(str coupling.Strength) bool {
	return str == coupling.StrengthFunctional ||
		str == coupling.StrengthSymmetric ||
		str == coupling.StrengthIntrusive
}

// classifyVolatilityEffective derives effective volatility for the to-module
// using the pre-computed effectiveVol map (post-cascade). Falls back to
// classifyVolatility when the map is nil or the module is not found.
func classifyVolatilityEffective(toPath string, mi moduleIndex, modules map[string]policy.ModuleDef, effectiveVol map[string]coupling.Volatility) coupling.Volatility {
	toMod, ok := mi.moduleFor(toPath)
	if !ok {
		return coupling.VolatilityUnknown
	}
	if effectiveVol != nil {
		if v, found := effectiveVol[toMod]; found {
			return v
		}
	}
	return classifyVolatility(toPath, mi, modules)
}

// classifyExplicitness derives explicitness from strength.
func classifyExplicitness(str coupling.Strength) coupling.Explicitness {
	switch str {
	case coupling.StrengthContract:
		return coupling.ExplicitnessExplicit
	case coupling.StrengthIntrusive:
		return coupling.ExplicitnessImplicit
	default:
		return coupling.ExplicitnessUnknown
	}
}
