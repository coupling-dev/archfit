package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

// TestStatusRunsThroughTheCLI: status runs through the real entry point (N2).
func TestStatusRunsThroughTheCLI(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := runArchfit(t, "status", "-C", t.TempDir())
	if code != 0 || stderr != "" || stdout != "✓ change   ○ check\nnext: run archfit check\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// TestEveryCommandRunReturnsError: kong checks a Run method only when it calls
// it, so a Run that returns no error crashes the command at run time. Running
// every command for real would push, post or install; the signature check
// stands in for that, and --help proves each command parses.
func TestEveryCommandRunReturnsError(t *testing.T) {
	t.Parallel()
	parser, err := kong.New(&cli{}, kong.Name("archfit"))
	if err != nil {
		t.Fatal(err)
	}
	errorType := reflect.TypeFor[error]()
	var leaves int
	var walk func(n *kong.Node, path []string)
	walk = func(n *kong.Node, path []string) {
		for _, child := range n.Children {
			if child.Type != kong.CommandNode {
				continue
			}
			args := append(append([]string{}, path...), child.Name)
			if !child.Leaf() {
				walk(child, args)
				continue
			}
			leaves++
			target := child.Target
			if target.Kind() != reflect.Pointer {
				target = target.Addr()
			}
			run := target.MethodByName("Run")
			if !run.IsValid() || run.Type().NumOut() != 1 || run.Type().Out(0) != errorType {
				t.Errorf("%s: Run must return exactly error, got %v", strings.Join(args, " "), run)
			}
			code, stdout, stderr := runArchfit(t, append(args, flagHelp)...)
			if code != 0 || stderr != "" || stdout == "" {
				t.Errorf("%s --help: exit %d, stderr %q", strings.Join(args, " "), code, stderr)
			}
		}
	}
	walk(parser.Model.Node, nil)
	if leaves < 10 {
		t.Fatalf("walked %d commands; the walk missed the command tree", leaves)
	}
}
