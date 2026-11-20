package classify_test

import (
	"testing"

	"github.com/coupling-dev/archfit/internal/model/graph"
	"github.com/coupling-dev/archfit/internal/policy"
	"github.com/coupling-dev/archfit/internal/relationship/classify"
	"github.com/coupling-dev/archfit/internal/relationship/coupling"
)

// Every strength the declared surface gives takes the target's volatility:
// the upstream module's volatility decides (the surfaces design 6.1). A high
// source never raises it, and a type hint never changes the strength.
func TestEdgeVolatilityIsTheTargets(t *testing.T) {
	t.Parallel()

	const high, low = "high", "low"
	tests := []struct {
		name     string
		to       string
		strength coupling.Strength
		src, dst string // declared volatility ("" = undeclared)
		want     coupling.Volatility
	}{
		{"contract: high source, low target", "file:services/b/api/c.go", coupling.StrengthContract, high, low, coupling.VolatilityLow},
		{"model: high source, low target", "file:services/b/model/m.go", coupling.StrengthModel, high, low, coupling.VolatilityLow},
		{"intrusive: high source, low target", fileToB, coupling.StrengthIntrusive, high, low, coupling.VolatilityLow},
		{"intrusive: low source, high target", fileToB, coupling.StrengthIntrusive, low, high, coupling.VolatilityHigh},
		{"intrusive: declared high source, undeclared target", fileToB, coupling.StrengthIntrusive, high, "", coupling.VolatilityUndeclared},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := classify.Config{Modules: map[string]policy.ModuleDef{
				modNameA: {Paths: []string{pathsA}, Volatility: tt.src},
				modNameB: {
					Paths: []string{pathsB}, Public: []string{publicB}, Model: []string{modelB},
					Volatility: tt.dst,
				},
			}}
			edge := graph.Edge{From: fileFromA, To: tt.to, Kind: graph.EdgeKindImports, StrengthHint: hintFunctional}
			cl := classify.Run(makeGraph([]graph.Edge{edge}), cfg)[edgeKey(edge)]
			if cl.Strength != tt.strength {
				t.Fatalf("Strength = %q, want %q", cl.Strength, tt.strength)
			}
			if cl.Volatility != tt.want {
				t.Errorf("Volatility = %q, want %q", cl.Volatility, tt.want)
			}
		})
	}
}
