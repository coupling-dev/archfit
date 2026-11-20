// Package balance gives the strength of a dependency between two modules
// (StrengthOf, from the declared surface of the target) and its balance: a
// use of the public part, of the model part, past the surface, or a copy. It
// serves the report's classifier, `can`, `request`, the preview, and the
// `coupling` rule.
//
// The balance is the book formula (bc_score.v7), computed by
// scoring.BookScorer at the cross-module distance. This package never copies
// the formula or its ordinals.
//
// It uses the declared volatility of the two modules and no volatility
// cascade: with a cascade, one new dependency would change the balance of code
// the change did not touch. The volatility that applies is the target's for a
// use (contract, model or intrusive strength): the upstream module's
// volatility decides, as in the book and in relationship/classify. A copy
// (symmetric) and functional coupling tie both sides to each other's changes,
// so they take the higher of the two.
//
// Assessment may import this package. It is deliberately absent from the
// package list of TestAssessmentConsumesOnlyThePublicRelationshipContract and
// of the assessment_no_relationship_internals rule; do not add it there. Its
// signature uses the relationship aliases and plain values, so a caller never
// names a coupling type.
package balance
