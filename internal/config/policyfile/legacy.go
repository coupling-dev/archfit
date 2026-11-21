package policyfile

import "github.com/goccy/go-yaml"

// LegacyError says that a version 1 file has the shape of the policy that
// archfit 1.x and older wrote, under the same version number. The check shows
// Why and Next in place of its generic invalid-policy lines.
type LegacyError struct{}

func (LegacyError) Error() string {
	return "this is a policy from archfit 1.x or older, which this engine does not read"
}

// Why is the reason line of the record.
func (LegacyError) Why() string {
	return "archfit 1.x wrote version 1 too, with other keys, and this engine cannot convert it"
}

// Next is the next line of the record.
func (LegacyError) Next() string {
	return "move it aside and run archfit init"
}

// checkLegacy finds a key that only the archfit 1.x format had: coupling and ai
// at the top, type in a rule, scip and clones under analyzers. The current
// schema has none of them. One such key is enough: the file is not a typo away
// from a valid policy. A module owner is not on the list: schema 1 has owners
// at the top only, and a new policy can mistake the two. Other unknown keys
// keep the strict decoder's errors.
func checkLegacy(data []byte) error {
	var f struct {
		Coupling  any              `yaml:"coupling"`
		AI        any              `yaml:"ai"`
		Rules     []map[string]any `yaml:"rules"`
		Analyzers map[string]any   `yaml:"analyzers"`
	}
	if yaml.Unmarshal(data, &f) != nil {
		return nil
	}
	legacy := f.Coupling != nil || f.AI != nil
	for _, r := range f.Rules {
		_, typed := r["type"]
		legacy = legacy || typed
	}
	for _, key := range []string{"scip", "clones"} {
		_, old := f.Analyzers[key]
		legacy = legacy || old
	}
	if legacy {
		return LegacyError{}
	}
	return nil
}
