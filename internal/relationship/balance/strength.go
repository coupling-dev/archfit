package balance

import "github.com/coupling-dev/archfit/internal/relationship/coupling"

// Surface says where a used path sits in the declared surface of the module
// that owns it: in its public part, in its model part, or in neither.
type Surface struct {
	Public bool
	Model  bool
}

// StrengthOf is the one rule that gives a dependency between two modules its
// strength (the surfaces design 6.1): a use of the public part is contract, a
// use of the model part is model, and a use of anything else in the module is
// intrusive. The check, `can`, the request preview and the report call it;
// nothing else sets a strength. Type facts a language extractor reads are
// evidence and never reach it. The policy rejects a path that is in both
// parts.
func StrengthOf(s Surface) coupling.Strength {
	switch {
	case s.Public:
		return coupling.StrengthContract
	case s.Model:
		return coupling.StrengthModel
	default:
		return coupling.StrengthIntrusive
	}
}
