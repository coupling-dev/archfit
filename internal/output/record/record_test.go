package record_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coupling-dev/archfit/internal/model/result"
	"github.com/coupling-dev/archfit/internal/output/record"
	"github.com/coupling-dev/archfit/internal/testutil"
)

// contractExamples holds the JSON of every example of design 8.1, the goldens
// of the result contract. The text goldens here are written from the design,
// not generated: the renderer must print the design's text from the
// contract's JSON.
const contractExamples = "../../model/result/testdata"

const (
	ruleGraph = "graph"
	fixCode   = "fix the code"
	failOne   = "archfit: fail, 1 error"
	file      = "x.go"
	engine    = "4.0.0"
	passLine  = "archfit: pass"
	pkgTC     = "pkg_tc"
	repair    = "assessment-repair"
)

// TestGoldensMatchDesignExamples renders every example of design 8.1 and
// compares it byte for byte with its golden.
func TestGoldensMatchDesignExamples(t *testing.T) {
	for _, name := range []string{"pass", "deny", ruleGraph, ruleCoupling, "duplication", "policy_differs", "expired_exception"} {
		t.Run(name, func(t *testing.T) {
			var r result.Result
			decodeExample(t, name, &r)
			var buf bytes.Buffer
			if err := record.Result(&buf, r, record.Options{}); err != nil {
				t.Fatalf("Result: %v", err)
			}
			assertGolden(t, name, buf.String())
		})
	}
	t.Run("can", func(t *testing.T) {
		var a result.CanAnswer
		decodeExample(t, "can", &a)
		var buf bytes.Buffer
		if err := record.Answer(&buf, a, record.Options{}); err != nil {
			t.Fatalf("Answer: %v", err)
		}
		assertGolden(t, "can", buf.String())
	})
}

// TestGoldensUseNoNotUsedWord holds every golden of this package to the words
// of design section 5.
func TestGoldensUseNoNotUsedWord(t *testing.T) {
	goldens, err := filepath.Glob(filepath.Join("testdata", "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(goldens) == 0 {
		t.Fatal("no golden found")
	}
	for _, path := range goldens {
		raw, err := os.ReadFile(path) //nolint:gosec // golden files of this package
		if err != nil {
			t.Fatal(err)
		}
		if words := testutil.NotUsedIn(string(raw)); len(words) > 0 {
			t.Errorf("%s uses %q (design 5, the Not used column)", path, words)
		}
	}
}

func TestResultLines(t *testing.T) {
	policy := result.Policy{Path: ".archfit.yaml", Digest: "sha256:abc"}
	for _, tc := range []struct {
		name string
		r    result.Result
		opts record.Options
		// unprotected keeps Policy.Protected false; every other case is
		// protected, so it shows no "policy: not protected" line.
		unprotected bool
		want        string
	}{
		{
			name: "errors in the given order, then unknown reasons, then the status line",
			r: result.Result{
				Summary: "archfit: unknown, no verdict",
				Errors: []result.Error{
					{Path: "b.go", Line: 2, Rule: ruleGraph, What: "second in file order, first here", Next: result.Next{Text: fixCode}},
					{Path: "a.go", Line: 1, Rule: "cycle", What: "first in file order", Next: result.Next{Text: fixCode}},
				},
				Unknown: []result.Unknown{
					{Path: "c.go", Line: 3, Cause: result.CauseImport, What: "an import cannot be resolved", Next: result.Next{Text: "fix the import"}},
				},
			},
			want: "b.go:2: graph: second in file order, first here\n  next: fix the code\n" +
				"a.go:1: cycle: first in file order\n  next: fix the code\n" +
				"c.go:3: import: an import cannot be resolved\n  next: fix the import\n" +
				"archfit: unknown, no verdict\n",
		},
		{
			name: "fields of an error in record order",
			r: result.Result{
				Summary: failOne,
				Errors: []result.Error{{
					Path: file, Line: 5, Rule: ruleCoupling, What: "w", To: pkgTC,
					Why: "y", Instead: "i", Balance: &result.Balance{Value: 3, Min: 7, Strength: result.StrengthModel, Volatility: result.VolatilityHigh},
					Next: result.Next{Text: fixCode},
				}},
			},
			want: "x.go:5: coupling: w\n  why: y\n  instead: i\n  balance: 3/10 (min 7): model, pkg_tc is high\n  next: fix the code\narchfit: fail, 1 error\n",
		},
		{
			name: "balance at the ends of the scale",
			r: result.Result{
				Summary: failOne,
				Errors: []result.Error{{
					Path: file, Line: 5, Rule: ruleCoupling, What: "w", To: pkgTC,
					Balance: &result.Balance{Value: 10, Min: 1, Strength: result.StrengthContract, Volatility: result.VolatilityLow}, Next: result.Next{Text: "n"},
				}},
			},
			want: "x.go:5: coupling: w\n  balance: 10/10 (min 1): contract, pkg_tc is low\n  next: n\narchfit: fail, 1 error\n",
		},
		{
			name: "a use past the surface names the surface after the balance",
			r: result.Result{
				Summary: failOne,
				Errors: []result.Error{{
					Path: file, Line: 5, Rule: ruleCoupling, What: "cmd -> pkg/tc/netem is past the surface", To: pkgTC,
					Balance: &result.Balance{Value: 2, Min: 7, Strength: result.StrengthIntrusive, Volatility: result.VolatilityHigh},
					Surface: &result.Surface{Public: []string{"pkg/tc", "pkg/tc/api.go"}, Model: []string{"pkg/tc/model"}},
					Package: "pkg/tc/netem", Next: result.Next{Text: "n"},
				}},
			},
			want: "x.go:5: coupling: cmd -> pkg/tc/netem is past the surface\n  balance: 2/10 (min 7): intrusive, pkg_tc is high\n" +
				"  surface: pkg/tc (public), pkg/tc/api.go (public), pkg/tc/model (model)\n  next: n\narchfit: fail, 1 error\n",
		},
		{
			name: "a surface with no declared part",
			r: result.Result{
				Summary: failOne,
				Errors: []result.Error{{
					Path: file, Line: 5, Rule: ruleCoupling, What: "w", To: pkgTC,
					Surface: &result.Surface{Public: []string{}, Model: []string{}}, Next: result.Next{Text: "n"},
				}},
			},
			want: "x.go:5: coupling: w\n  surface: none declared\n  next: n\narchfit: fail, 1 error\n",
		},
		{
			name: "a note in parentheses after the next step",
			r: result.Result{
				Summary: failOne,
				Errors:  []result.Error{{Path: file, Line: 5, Rule: ruleGraph, What: "w", Next: result.Next{Text: "archfit request", Note: "a hint"}}},
			},
			want: "x.go:5: graph: w\n  next: archfit request     (a hint)\narchfit: fail, 1 error\n",
		},
		{
			name: "an unknown reason without a path",
			r: result.Result{
				Summary: "archfit: unknown, no verdict",
				Unknown: []result.Unknown{{Cause: result.CauseBase, What: "no merge base", Why: "nothing to compare with", Next: result.Next{Text: "git fetch --unshallow"}}},
			},
			want: "base: no merge base\n  why: nothing to compare with\n  next: git fetch --unshallow\narchfit: unknown, no verdict\n",
		},
		{
			name: "verbose adds the policy digest and the engine version after the status line",
			r:    result.Result{Summary: passLine, Policy: policy, Engine: engine, Base: "b1", Head: "h1", Date: "2026-11-10"},
			opts: record.Options{Verbose: true},
			want: "archfit: pass\npolicy: .archfit.yaml sha256:abc\nengine: 4.0.0\n",
		},
		{
			name: "without verbose there is no digest or version",
			r:    result.Result{Summary: passLine, Policy: policy, Engine: engine},
			want: "archfit: pass\n",
		},
		{
			name:        "a policy no workflow protects adds a line after the status line",
			r:           result.Result{Summary: failOne, Errors: []result.Error{{Path: file, Line: 5, Rule: ruleGraph, What: "w", Next: result.Next{Text: fixCode}}}},
			unprotected: true,
			want:        "x.go:5: graph: w\n  next: fix the code\narchfit: fail, 1 error\npolicy: not protected\n",
		},
		{
			name:        "the not protected line comes before the verbose lines",
			r:           result.Result{Summary: passLine, Policy: policy, Engine: engine},
			opts:        record.Options{Verbose: true},
			unprotected: true,
			want:        "archfit: pass\npolicy: not protected\npolicy: .archfit.yaml sha256:abc\nengine: 4.0.0\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.r
			r.Policy.Protected = !tc.unprotected
			var buf bytes.Buffer
			if err := record.Result(&buf, r, tc.opts); err != nil {
				t.Fatalf("Result: %v", err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("record:\n--- got\n%s--- want\n%s", got, tc.want)
			}
		})
	}
}

func TestAnswerLines(t *testing.T) {
	for _, tc := range []struct {
		name        string
		a           result.CanAnswer
		opts        record.Options
		unprotected bool
		want        string
	}{
		{
			name: "yes with the balance it would have",
			a:    result.CanAnswer{From: "a", To: "b", Answer: result.ReplyYes, Balance: &result.Balance{Value: 8, Min: 7, Strength: result.StrengthModel, Volatility: result.VolatilityLow}},
			want: "a -> b: yes\n  balance: 8/10 (min 7): model, b is low\n",
		},
		{
			name: "no past the surface, as design 6.9 shows it",
			a: result.CanAnswer{
				From: "analysis-application", To: repair, Answer: result.ReplyNo, Rule: ruleCoupling,
				What:    "internal/assessment/score is past the surface",
				Balance: &result.Balance{Value: 2, Min: 7, Strength: result.StrengthIntrusive, Volatility: result.VolatilityHigh},
				Surface: &result.Surface{Public: []string{"internal/assessment/finding"}, Model: []string{}},
				Package: "internal/assessment/score",
				Next:    &result.Next{Kind: result.NextCommand, Text: `use the surface, or: archfit request -m "<why score belongs to the surface>"`},
			},
			want: "analysis-application -> assessment-repair: coupling: internal/assessment/score is past the surface\n" +
				"  balance: 2/10 (min 7): intrusive, assessment-repair is high\n" +
				"  surface: internal/assessment/finding (public)\n" +
				"  next: use the surface, or: archfit request -m \"<why score belongs to the surface>\"\n",
		},
		{
			name: "no with every field",
			a: result.CanAnswer{
				From: "a", To: "b", Answer: result.ReplyNo, Rule: "no-a-to-b", What: "a must not depend on b",
				Why: "y", Instead: "i", Next: &result.Next{Text: fixCode},
			},
			want: "a -> b: no-a-to-b: a must not depend on b\n  why: y\n  instead: i\n  next: fix the code\n",
		},
		{
			name: "unknown without a rule",
			a: result.CanAnswer{
				From: "tools/x", To: "b", Answer: result.ReplyUnknown, What: "tools/x is in no module",
				Next: &result.Next{Text: "archfit rules tools/x"},
			},
			want: "tools/x -> b: unknown: tools/x is in no module\n  next: archfit rules tools/x\n",
		},
		{
			name: "a note prints as it does for an error",
			a:    result.CanAnswer{From: "a", To: "b", Answer: result.ReplyNo, Rule: ruleGraph, What: "w", Next: &result.Next{Text: "archfit request", Note: "a hint"}},
			want: "a -> b: graph: w\n  next: archfit request     (a hint)\n",
		},
		{
			name: "verbose adds the policy digest and the engine version",
			a: result.CanAnswer{
				From: "a", To: "b", Answer: result.ReplyYes,
				Policy: result.Policy{Path: ".archfit.yaml", Digest: "sha256:abc"}, Engine: engine,
			},
			opts: record.Options{Verbose: true},
			want: "a -> b: yes\npolicy: .archfit.yaml sha256:abc\nengine: 4.0.0\n",
		},
		{
			name:        "a policy no workflow protects adds a line after the answer",
			a:           result.CanAnswer{From: "a", To: "b", Answer: result.ReplyYes, Balance: &result.Balance{Value: 8, Min: 7, Strength: result.StrengthModel, Volatility: result.VolatilityLow}},
			unprotected: true,
			want:        "a -> b: yes\n  balance: 8/10 (min 7): model, b is low\npolicy: not protected\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := tc.a
			a.Policy.Protected = !tc.unprotected
			var buf bytes.Buffer
			if err := record.Answer(&buf, a, tc.opts); err != nil {
				t.Fatalf("Answer: %v", err)
			}
			got := buf.String()
			if got != tc.want {
				t.Errorf("answer:\n--- got\n%s--- want\n%s", got, tc.want)
			}
			if strings.Contains(got, "archfit: ") {
				t.Errorf("an answer has no status line:\n%s", got)
			}
		})
	}
}

func TestWriteErrorIsReturned(t *testing.T) {
	if err := record.Result(failingWriter{}, result.Result{Summary: passLine}, record.Options{}); !errors.Is(err, errWrite) {
		t.Errorf("Result error = %v, want %v", err, errWrite)
	}
	if err := record.Answer(failingWriter{}, result.CanAnswer{Answer: result.ReplyYes}, record.Options{}); !errors.Is(err, errWrite) {
		t.Errorf("Answer error = %v, want %v", err, errWrite)
	}
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

func decodeExample(t *testing.T, name string, v any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(contractExamples, name+".json")) //nolint:gosec // fixed example names
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("decode example %s: %v", name, err)
	}
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", name+".txt")) //nolint:gosec // fixed golden names
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("record differs from design 8.1:\n--- got\n%s--- golden\n%s", got, want)
	}
}

// TestBalanceLine names the inputs of a balance: the strength, then the
// target module and its volatility (the surfaces design 6.1).
func TestBalanceLine(t *testing.T) {
	for _, tc := range []struct {
		b    result.Balance
		to   string
		want string
	}{
		{result.Balance{Value: 9, Min: 7, Strength: result.StrengthContract, Volatility: result.VolatilityHigh}, "architecture-policy",
			"balance: 9/10 (min 7): contract, architecture-policy is high"},
		{result.Balance{Value: 7, Min: 7, Strength: result.StrengthModel, Volatility: result.VolatilityMedium}, repair,
			"balance: 7/10 (min 7): model, assessment-repair is medium"},
		{result.Balance{Value: 2, Min: 7, Strength: result.StrengthIntrusive, Volatility: result.VolatilityHigh}, "assessment-repair",
			"balance: 2/10 (min 7): intrusive, assessment-repair is high"},
		{result.Balance{Value: 10, Min: 1, Strength: result.StrengthContract, Volatility: result.VolatilityLow}, "pkg_util",
			"balance: 10/10 (min 1): contract, pkg_util is low"},
	} {
		if got := record.BalanceLine(tc.b, tc.to); got != tc.want {
			t.Errorf("BalanceLine(%+v, %q) = %q, want %q", tc.b, tc.to, got, tc.want)
		}
	}
}
