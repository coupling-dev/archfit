package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/coupling-dev/archfit/internal/config/policyfile"
	"github.com/coupling-dev/archfit/internal/history/identity"
	"github.com/coupling-dev/archfit/internal/initcfg/policy3"
	"github.com/coupling-dev/archfit/internal/model/evidence"
)

// agentLines are the five lines for agents that init writes into AGENTS.md.
// The session-start hook prints them too.
const agentLines = policy3.Lines

// RepoInitCmd is `archfit init` (internal/initcfg/policy3): set up a
// repository. It writes what is missing and never changes a version 1
// policy: a fresh repository gets a version 1 policy. A repository with any
// other policy version is an error. Successful setup writes the instruction
// block and, unless disabled, the agent hooks. It writes no workflow file
// (D11: the workflow waits for Action 4).
type RepoInitCmd struct {
	NoHooks bool   `name:"no-hooks" help:"Do not install the Claude Code and Codex hooks."`
	Dir     string `short:"C" name:"directory" help:"Run in DIR instead of the current directory." placeholder:"DIR"`
}

func (*RepoInitCmd) Help() string {
	return `Set up a repository for archfit. It writes what is missing and never
changes a version 1 policy.

  No .archfit.yaml      writes a version 1 policy from the code: modules,
                        public, depends_on as the dependencies that exist,
                        tests, generated, owners from the GitHub identity,
                        and a dated exception for each error that exists today,
                        when the evidence is complete
  Version 1 policy      leaves it as it is
  Any other policy      fails: archfit reads version 1 only

Each case that succeeds writes the five lines for agents between the archfit markers in
AGENTS.md; text outside the markers stays as it is.

Init writes a draft if some evidence is missing and lists what it could not read.
It adds .archfit-cache/ to .gitignore.

Init installs Claude Code and Codex hooks unless you use --no-hooks.
It does not install the git pre-commit hook.

Then read .archfit.yaml. Run archfit setup to add the workflow and open the setup pull request.

Exit codes:
  0  done, also when nothing was missing
  2  wrong usage or a tool error`
}

// initNext is the last line of init: the owner's step.
const initNext = "next: read " + policy3.PolicyFile + ", then open a pull request with it"

const initAgentHooksByDefault = true

func (c *RepoInitCmd) Run(deps *appDeps) error {
	ctx := deps.runContext()
	co, err := openCheckout(ctx, deps, dirOr(c.Dir))
	if err != nil {
		return noVerdict(err)
	}
	text, err := runInit(ctx, deps, co, c.NoHooks, false)
	if err != nil {
		return noVerdict(err)
	}
	if _, err := io.WriteString(deps.Stdout, text); err != nil {
		return noVerdict(err)
	}
	return nil
}

// runInit shares the file-writing part of init with setup. Setup owns its
// continuation and prints its exception list after replay.
func runInit(ctx context.Context, deps *appDeps, co checkout, noHooks, forSetup bool) (string, error) {
	data, found, err := co.policyAt("")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	var draft policy3.Result
	switch {
	case !found:
		draft, err = co.initFresh(ctx, &b)
	case isVersion1(data):
		err = co.initBlock(&b)
	default:
		_, err = policyfile.Decode(data)
		err = fmt.Errorf("%s: %w", policy3.PolicyFile, err)
	}
	if err != nil {
		return "", err
	}
	if err := ignoreInitCache(ctx, co.runner, co.root); err != nil {
		fmt.Fprintf(&b, "warning: could not ignore %s/: %v\n", cacheDirName, err)
	}
	if initAgentHooksByDefault && !noHooks {
		installInitHooks(ctx, deps, co.root, &b)
	}
	if !found {
		writeUnread(&b, draft.Unknown)
		if !forSetup {
			writeOwnerDecisions(&b, draft.File)
		}
		if draft.File.Owners[0] == identity.PlaceholderOwner {
			b.WriteString("no person is named as an owner yet\n")
			if forSetup {
				b.WriteString("put your GitHub login in owners in .archfit.yaml\n")
			} else {
				b.WriteString("next: put your GitHub login in owners in .archfit.yaml\n")
			}
		} else if !forSetup {
			b.WriteString(initNext + "\n")
		}
	}
	return b.String(), nil
}

// initFresh writes the first policy of a repository.
func (co checkout) initFresh(ctx context.Context, b *strings.Builder) (policy3.Result, error) {
	owner, verified := identity.OwnerIdentity(ctx, co.root, co.runner)
	res, err := policy3.Init(ctx, policy3.Deps{Facts: co.facts(policy3.FactsConfig()), Now: co.now}, co.root, owner)
	if err != nil {
		return policy3.Result{}, err
	}
	writePolicySummary(b, "wrote", res.File)
	b.WriteString("wrote " + policy3.AgentsFile + ": the instruction block for agents\n")
	if !verified && owner != identity.PlaceholderOwner {
		b.WriteString("owner from git noreply email is not verified\n")
	}
	return res, nil
}

// installInitHooks is best effort: the repository setup is usable even when
// hook settings need manual repair. Init only writes inside the checkout.
func installInitHooks(ctx context.Context, deps *appDeps, root string, b *strings.Builder) {
	for _, harness := range []string{harnessClaude, harnessCodex} {
		rel := harnessConfigFile(harness)
		if !within(root, filepath.Join(root, rel)) {
			fmt.Fprintf(b, "warning: skipped %s: resolves outside the repository\n", rel)
			continue
		}
		if _, err := installHarnessHooks(root, harness); err != nil {
			fmt.Fprintf(b, "warning: could not install hooks in %s: %v\n", rel, err)
			continue
		}
		fmt.Fprintf(b, "installed hooks: %s\n", rel)
		if gitIgnores(ctx, deps, root, rel) {
			b.WriteString("  " + ignoredWarning(rel) + "\n")
		}
		if harness == harnessCodex {
			b.WriteString("  " + codexTrustNotice + "\n")
		}
	}
}

// initBlock adds the agent instructions without changing an existing policy.
func (co checkout) initBlock(b *strings.Builder) error {
	wrote, err := policy3.WriteBlock(co.root)
	if err != nil {
		return err
	}
	b.WriteString("left " + policy3.PolicyFile + " as it is: archfit init never changes a version 1 policy\n")
	if wrote {
		b.WriteString("wrote " + policy3.AgentsFile + ": the instruction block for agents\n")
	} else {
		b.WriteString("left " + policy3.AgentsFile + " as it is: it has the instruction block\n")
	}
	return nil
}

// writePolicySummary counts what the policy holds.
func writePolicySummary(b *strings.Builder, verb string, f policyfile.File) {
	deps := 0
	for _, m := range f.Modules {
		deps += len(m.DependsOn)
	}
	fmt.Fprintf(b, "%s %s: %s, %s, %s, %s\n", verb, policy3.PolicyFile,
		count(len(f.Modules), "module"), count(deps, "allowed dependency"),
		count(len(f.Rules), "rule"), count(len(f.Exceptions), "exception"))
}

func writeOwnerDecisions(b *strings.Builder, f policyfile.File) {
	var lines strings.Builder
	for _, x := range f.Exceptions {
		place := x.From
		if x.To != "" {
			place += " -> " + x.To
		}
		lines.WriteString("  exception: " + x.Rule + " " + place)
		if x.Expires != "" {
			lines.WriteString(" until " + x.Expires)
		}
		lines.WriteString("\n")
	}
	names := make([]string, 0, len(f.Modules))
	for name := range f.Modules {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if len(f.Modules[name].Public) == 0 && len(f.Modules[name].Model) == 0 {
			fmt.Fprintf(&lines, "  module %s: no public part declared; other modules may use all of it\n", name)
		}
	}
	for _, name := range names {
		m := f.Modules[name]
		if m.Volatility == "" && m.Subdomain == "" {
			fmt.Fprintf(&lines, "  module %s: volatility not set; taken as high\n", name)
		}
	}
	if lines.Len() > 0 {
		b.WriteString("for the owner to decide:\n")
		b.WriteString(lines.String())
	}
}

// writeUnread discloses unavailable evidence. Source paths found by inventory
// can still appear in the draft policy.
func writeUnread(b *strings.Builder, items []evidence.UnknownItem) {
	if len(items) == 0 {
		return
	}
	b.WriteString("draft policy: some evidence was not read\n")
	seenFixes := map[string]bool{}
	var fixes []string
	for _, u := range items {
		b.WriteString("not read: ")
		if u.File != "" {
			b.WriteString(location(u.File, u.Line) + ": ")
		}
		b.WriteString(u.Reason + "\n")
		if u.Fix != "" && !seenFixes[u.Fix] {
			seenFixes[u.Fix] = true
			fixes = append(fixes, u.Fix)
		}
	}
	for _, fix := range fixes {
		b.WriteString("  fix: " + fix + "\n")
	}
}

// count is "n noun" with the noun in the plural for any n but 1.
func count(n int, noun string) string {
	if n != 1 {
		if strings.HasSuffix(noun, "y") {
			noun = strings.TrimSuffix(noun, "y") + "ies"
		} else {
			noun += "s"
		}
	}
	return strconv.Itoa(n) + " " + noun
}
