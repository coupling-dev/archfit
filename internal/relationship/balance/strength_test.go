package balance_test

import (
	"testing"

	"github.com/coupling-dev/archfit/internal/relationship"
	"github.com/coupling-dev/archfit/internal/relationship/balance"
)

func TestStrengthOf(t *testing.T) {
	tests := []struct {
		name    string
		surface balance.Surface
		want    relationship.Strength
	}{
		{"the public part", balance.Surface{Public: true}, relationship.StrengthContract},
		{"the model part", balance.Surface{Model: true}, relationship.StrengthModel},
		{"past the surface", balance.Surface{}, relationship.StrengthIntrusive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := balance.StrengthOf(tt.surface); got != tt.want {
				t.Errorf("StrengthOf(%+v) = %q, want %q", tt.surface, got, tt.want)
			}
		})
	}
}
