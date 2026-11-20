// Package record renders a check result and a `can` answer as the text
// record of design section 8: where, which rule, what; then why, instead,
// balance, next; the status line last, as in a linter.
//
// The grammar:
//
//	record      = { error } { unknown } status-line [ verbose ]
//	error       = path [ ":" line ] ": " rule-id ": " what NL fields next-line
//	unknown     = [ path [ ":" line ] ": " ] cause ": " what NL fields next-line
//	fields      = [ "  why: " text NL ] [ "  instead: " text NL ]
//	              [ "  balance: " n "/10 (min " m ")" NL ]
//	next-line   = "  next: " text [ "     (" note ")" ] NL
//	status-line = "archfit: " status [ ", " count ] [ " (not from this change)" ] NL
//	              [ unprotected ]
//	unprotected = "policy: not protected" NL
//	verbose     = "policy: " path " " digest NL "engine: " version NL
//	answer      = from " -> " to ": " ( "yes" | ( rule-id | "no" | "unknown" ) ": " what ) NL
//	              fields [ next-line ] [ unprotected ] [ verbose ]
//
// A path-less error, such as an expired exception, uses .archfit.yaml as the
// path and no line. `next` is "fix the code", a command, "wait for an owner:
// #n", or a short instruction. A `can` answer has no status line.
//
// The renderer prints the producer's verdict and steps as given. Repeated
// tool reasons may share a text entry; JSON retains every location.
// Digests and the engine version appear only with Options.Verbose (design 5).
// A policy no workflow protects adds the line "policy: not protected" to every
// result and answer (design 9.4).
//
// The same result has two more forms (design 8.3): ResultJSON and AnswerJSON
// write the documents of archfit.result.schema.json, and ResultSARIF writes a
// SARIF 2.1.0 log for code scanning whose messages hold the record's lines.
package record

import (
	"io"
	"strconv"
	"strings"

	"github.com/coupling-dev/archfit/internal/model/result"
)

// Options changes what a record prints.
type Options struct {
	// Verbose adds the policy digest and the engine version after the
	// status line, or after the answer.
	Verbose bool
}

// noteGap separates a next step from its note, as in design 8.1.
const noteGap = "     "

// Result writes the record of a check result: its errors in the order
// given, its unknown reasons, then its status line.
func Result(w io.Writer, r result.Result, opts Options) error {
	var b strings.Builder
	for _, e := range r.Errors {
		writeHead(&b, location(e.Path, e.Line), e.Rule, e.What)
		writeDetails(&b, e.Why, e.Instead, e.Balance, &e.Next)
	}
	for _, u := range groupToolReasons(r.Unknown) {
		writeHead(&b, location(u.Path, u.Line), string(u.Cause), u.What)
		writeDetails(&b, u.Why, "", nil, &u.Next)
	}
	b.WriteString(r.Summary + "\n")
	writeUnprotected(&b, r.Policy)
	if opts.Verbose {
		writeVerbose(&b, r.Policy, r.Engine)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// groupToolReasons keeps the first occurrence of each equivalent tool reason
// in place. Each group retains its next step and shows at most three paths.
func groupToolReasons(unknown []result.Unknown) []result.Unknown {
	if len(unknown) < 2 {
		return unknown
	}
	const maxPaths = 3
	type group struct {
		index int
		count int
		paths [maxPaths]string
	}
	groups := map[result.Unknown]group{}
	out := make([]result.Unknown, 0, len(unknown))
	for _, u := range unknown {
		if u.Cause != result.CauseTool || u.Path == "" {
			out = append(out, u)
			continue
		}
		key := u
		key.Path, key.Line = "", 0
		g, ok := groups[key]
		if !ok {
			g.index = len(out)
			out = append(out, u)
		}
		if g.count < maxPaths {
			g.paths[g.count] = location(u.Path, u.Line)
		}
		g.count++
		groups[key] = g
	}
	for reason, g := range groups {
		if g.count < 2 {
			continue
		}
		paths := strings.Join(g.paths[:min(maxPaths, g.count)], ", ")
		if g.count > maxPaths {
			paths += ", ..."
		}
		reason.What += " (" + strconv.Itoa(g.count) + " paths: " + paths + ")"
		out[g.index] = reason
	}
	return out
}

// Answer writes the answer of `archfit can`: one line, then its details.
func Answer(w io.Writer, a result.CanAnswer, opts Options) error {
	var b strings.Builder
	label := a.Rule
	if label == "" {
		label = string(a.Answer)
	}
	writeHead(&b, a.From+" -> "+a.To, label, a.What)
	writeDetails(&b, a.Why, a.Instead, a.Balance, a.Next)
	writeUnprotected(&b, a.Policy)
	if opts.Verbose {
		writeVerbose(&b, a.Policy, a.Engine)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func location(path string, line int) string {
	if line == 0 {
		return path
	}
	return path + ":" + strconv.Itoa(line)
}

// writeHead writes "where: label: what". An empty where or what drops its
// part, so a yes prints "from -> to: yes".
func writeHead(b *strings.Builder, where, label, what string) {
	if where != "" {
		b.WriteString(where + ": ")
	}
	b.WriteString(label)
	if what != "" {
		b.WriteString(": " + what)
	}
	b.WriteString("\n")
}

func writeDetails(b *strings.Builder, why, instead string, balance *result.Balance, next *result.Next) {
	if why != "" {
		b.WriteString("  why: " + why + "\n")
	}
	if instead != "" {
		b.WriteString("  instead: " + instead + "\n")
	}
	if balance != nil {
		b.WriteString("  balance: " + strconv.Itoa(balance.Value) + "/10 (min " + strconv.Itoa(balance.Min) + ")\n")
	}
	if next != nil {
		b.WriteString("  next: " + next.Text)
		if next.Note != "" {
			b.WriteString(noteGap + "(" + next.Note + ")")
		}
		b.WriteString("\n")
	}
}

// notProtected is the line of design 9.4: no check on pull requests guards
// the policy, so the result is a local guide.
const notProtected = "policy: not protected"

func writeUnprotected(b *strings.Builder, policy result.Policy) {
	if !policy.Protected {
		b.WriteString(notProtected + "\n")
	}
}

func writeVerbose(b *strings.Builder, policy result.Policy, engine string) {
	b.WriteString("policy: " + policy.Path + " " + policy.Digest + "\n")
	b.WriteString("engine: " + engine + "\n")
}
