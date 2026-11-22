# Commands

Archfit has nine public commands. Run `archfit <command> --help` for their flags. Bare `archfit` prints a usage error; it runs no check or report.

## Policy version

`check`, `can`, `rules`, `request`, `report`, and `replay` need a version 1 `.archfit.yaml`. A version 2 file (Archfit 3) is an error: remove it and run `archfit init` to write a fresh version 1 policy.

## Choose a command

| Task | Command |
| --- | --- |
| Check a change: pass, fail, or unknown | `archfit check` |
| Ask whether a dependency or library may be used | `archfit can <from> <to>` |
| Show the rules that apply | `archfit rules [PATH... \| RULE] [--graph]` |
| Ask an owner to change the policy | `archfit request -m "<why>" [--expires DATE]` |
| Set up advisory checks on GitHub | `archfit setup` |
| Write the first policy and agent instructions locally | `archfit init` |
| See the state of the repository and its measurements | `archfit report` |
| See how the last changes of the default branch fare under the policy | `archfit replay [--last N]` |
| Check tools and hooks, or install what is missing | `archfit doctor` or `archfit doctor --fix` |

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | `pass`, yes, or done. `report` exits 0 whatever the repository state shows. A sent `request` prints its comment or issue URL. |
| `1` | `fail` or no. |
| `2` | `unknown`, wrong usage, a tool error, or a run interrupted by `SIGINT` or `SIGTERM`. A request was not sent, or its send outcome is unknown. |

Unknown commands and flags also exit `2`. Diagnostics go to stderr; normal output goes to stdout. The hidden hooks use their harness's protocol, described below.

## Command details

### `check`

`archfit check [flags]` gives the status of the change in the working tree: `pass`, `fail`, or `unknown`. It needs a version 1 policy. If the policy is missing, run `archfit init`. A version 2 policy is an error.

```sh
archfit check
archfit check --base origin/main
archfit check --policy candidate.yaml
archfit check --format sarif > archfit.sarif
```

| Flag | Meaning |
| --- | --- |
| `--base <ref>` | The target branch. Default: the remote's default branch, `origin/HEAD`. |
| `--policy <file>` | Judge the change with this policy file instead of `.archfit.yaml`, and do not compare it with the policy of the target branch. A preview: nothing is kept for `archfit request`. A relative path is read from the directory of `-C`, else from the current directory. |
| `--format <name>` | `text` (default), `json`, or `sarif`. |
| `--verbose` | Add the policy digest and the engine version after the status line. |
| `-C, --directory <dir>` | Run in `<dir>` instead of the current directory. |

The run:

1. The target branch is `--base`, else `origin/HEAD`. With neither, the status is `unknown` and the record says to pass `--base`. A CI checkout often has no `origin/HEAD`: see [CI](ci.md).
2. The base is the merge base of `HEAD` and the target branch. When `HEAD` is the tip of the target branch, the base is the first parent of `HEAD`: the change is the last commit. A clone without the merge base, such as a shallow clone, is `unknown`, and the record names the fetch.
3. The policy on the target branch is the protected one. When `.archfit.yaml` differs from it, the status is `unknown`, the record names the command that restores the file, and nothing else is checked. When the target branch has no `.archfit.yaml`, the check uses the local policy and says so on stderr.
4. The policy must fit the tree. A path glob that matches no file, or another policy error, is `unknown` with the message.
5. The check reads the base and the working tree and counts the errors of each by rule, modules, and symbol. An error the base does not have, or has fewer times, is added by the change: the status is `fail`. An expired exception fails every change, and the status line ends with `(not from this change)`. See [results](results.md).
6. On a push (`HEAD` is the tip of the target branch) with a `pass` or `fail` status, the JSON result adds the state of the repository. See [results](results.md#json).

The default text record is also the agent workflow: run `archfit check` without a format flag and follow `next:`. Exit `0` means `pass`, `1` means `fail`, and `2` means `unknown`, wrong usage, or a tool error. Repeated tool errors share a line with a path count and up to three examples. The JSON retains each path.

The record goes to stdout. Notes, such as a target branch with no policy, and tool errors go to stderr. When no workflow in `.github/workflows/` has a step that uses `coupling-dev/archfit-action` (`uses: coupling-dev/archfit-action@<ref>`), every result carries the line `policy: not protected`, after the status line: the result is a local guide, not change control.

Each run except a `--policy` preview keeps its JSON output and the git tree id of the working tree in `.archfit-cache/last.json`. `archfit request` reads it while the tree and `HEAD` are the same. The same tree, policy, engine version, and date give the same output.

### `can <from> <to>`

`archfit can <from> <to> [flags]` answers before you write code: may `<from>` depend on `<to>`? It reads `.archfit.yaml` (version 1) and applies the rules `check` applies to one new dependency: `graph`, `coupling`, `layers`, and the custom `deny` and `only` rules.

```text
$ archfit can pkg/tc pkg/util
pkg_tc -> pkg_util: graph: not an allowed dependency
  balance: 9/10 (min 7): contract, pkg_util is low
  next: archfit request -m "<why you need it>"
```

A use past the surface of the target names the surface and asks one question: should this package be part of the surface? This is the answer on archfit's own repository:

```text
$ archfit can internal/application/analysis.go internal/assessment/score
analysis-application -> assessment-repair: coupling: internal/assessment/score is past the surface
  balance: 2/10 (min 7): intrusive, assessment-repair is high
  surface: internal/assessment/finding (public)
  next: use the surface, or: archfit request -m "<why score belongs to the surface>"
```

Use what the `surface:` line lists, or ask an owner to declare the package `public` or `model`. A use past the surface is never approved as it is: only the declaration changes.

Each side is a module name, a file or a directory relative to the repository root, or a Go import path that a `go.mod` of the checkout maps. `<to>` can also be a library, such as `github.com/spf13/cobra`. A module name as `<to>` means the public part of the module. A path that does not exist yet is judged by the module its glob gives.

| Answer | Exit | The record shows |
| --- | --- | --- |
| yes | `0` | The balance, when the two sides are different modules. |
| no | `1` | The rule, its `why` for a custom rule, `instead`, the balance, the surface for a use past it, and `next`. |
| unknown | `2` | A path in no module, or a question only a file can answer, and the command to run next. |

A directory is judged one file at a time, as `check` judges it. When its files answer differently, the answer is `unknown`.

The answer is approximate in two cases. It applies no exception, and it does not see a cycle that the new dependency would close. `check` judges both: a `yes` can still fail `check` when the dependency closes a cycle, and a `no` can pass `check` when an exception that has not expired covers it.

`can` keeps its JSON answer in `.archfit-cache/last.json`, as `check` does, for `archfit request`.

| Flag | Meaning |
| --- | --- |
| `--format <name>` | `text` (default) or `json`. |
| `--verbose` | Add the policy digest and the engine version. |
| `-C, --directory <dir>` | Run in `<dir>` instead of the current directory. |

### `rules [PATH... | RULE]`

`archfit rules` shows the rules of `.archfit.yaml` (version 1) in plain words. Paths are relative to the repository root.

| Form | Shows |
| --- | --- |
| `archfit rules` | The paths form for the files the change adds lines to, against the base `check` uses. |
| `archfit rules <path>...` | For each path: its module, the public part, volatility, layer, and allowed dependencies with the surface of each (its `public` and `model` globs), then the rules that apply with their modes and the exceptions that apply with their dates. A path in no module, a declared test, or a generated path says so. |
| `archfit rules <rule-id>` | One rule in full, with its exceptions. Built-in rule ids work too. |
| `archfit rules --graph` | The modules by layer, inner first, with their allowed dependencies. |

The notes of the policy are listed as "not checked". Dates show the days left, such as `2026-11-19 (9 days)`.

| Flag | Meaning |
| --- | --- |
| `--graph` | The module graph. It takes no path or rule id. |
| `-C, --directory <dir>` | Run in `<dir>` instead of the current directory. |

### `request`

`archfit request -m "<why>" [--expires DATE] [flags]` asks an owner to change the policy. It takes the errors of the last `archfit check`, or the dependency of the last `archfit can` answer, from `.archfit-cache/last.json`, so you write only the reason. The output is used only while `HEAD` and the working tree are the ones it was produced on: after an edit, run `archfit check` again.

Only an error whose `next` is `archfit request` joins the request: an error of a rule an owner may allow, or a use past the surface. An error that says `fix the code`, or `wait for an owner: #n`, does not.

A use past the surface asks one question: should its package be part of the surface? The printed request shows it on a `question:` line; the App asks the owner to declare the package `public` or `model`, or to reject the request.

With a GitHub remote, `archfit request` posts `/archfit request <why> [--expires DATE]` as a comment on the one open pull request for the current branch. The target repository is `origin`, or a configured GitHub `upstream` for a fork. The head uses the push remote's owner, falling back to the `origin` owner. Several matching pull requests means nothing is sent: the error names their numbers. A detached HEAD or a branch without an upstream has no matching pull request.

Without a pull request, only a request from `archfit can` opens an issue. Its body holds the request line and the `can` record; the App marks the numbers "as reported" until a pull request confirms them. With `check` output, nothing is opened: `next: open a pull request first`.

Credentials are tried in order: `GH_TOKEN`, `GITHUB_TOKEN`, then `gh auth token`. No credentials means exit `2` and `next: gh auth login`. Tokens are never printed. Success exits `0` and prints the comment or issue URL. API errors exit `2` without printing a successful request or changing the last result. HTTP calls time out after 30 seconds. A POST is never retried; when the send outcome is unknown (for example, a lost connection after sending), check GitHub before trying again.

Without a GitHub remote, the request is printed and exits `2`; show it to an owner. The last line is `next: ask an owner`:

```text
$ archfit request -m "netem needs the shared retry helper"
pkg/tc/netem.go:12: graph: pkg_tc -> pkg_util
  balance: 9/10 (min 7): contract, pkg_util is low
request: netem needs the shared retry helper
next: ask an owner     (open a pull request first if there is none)
```

The request for a use past the surface, on archfit's own repository:

```text
$ archfit request -m "views needs the score of a seam"
internal/config/views.go:4: coupling: policy-config-adapter -> assessment-repair
  balance: 2/10 (min 7): intrusive, assessment-repair is high
  question: should internal/assessment/score be part of the surface of assessment-repair?
request: views needs the score of a seam
policy: not protected
next: ask an owner     (open a pull request first if there is none)
```

| Flag                    | Meaning                                                                                                                                                                                                                       |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `-m <why>`              | The reason. Required. On a terminal, `archfit request` asks for it when `-m` is not given. Use one line of at most 500 characters. The reason must not end in `--expires` or `--expires <word>`: give the date with the flag. |
| `--expires <date>`      | Ask for an exception until this day, `YYYY-MM-DD`: a date after today and after the `check`, at most 365 days ahead. Without it the request asks for a permanent change.                                                      |
| `-C, --directory <dir>` | Run in `<dir>` instead of the current directory.                                                                                                                                                                              |

`archfit request` exits `2` with the reason on stderr when there is no `check` or `can` output for this tree (`run archfit check first`), when the output is `pass` or has nothing an owner may allow (`nothing to ask`), when it is `unknown`, or when `-m` or `--expires` is wrong.

### `init`

`archfit init [flags]` sets up a repository. It writes what is missing and never changes a version 1 policy. It writes no workflow file. Use [`setup`](#setup) to add the workflow and open the setup pull request.

| The repository has | `archfit init` |
| --- | --- |
| No `.archfit.yaml` | Writes a version 1 policy from the code: a module for each directory of code cut to two levels, such as `pkg/chaos` (for Python, below `src/`), with `paths` and `public` (the packages other modules use today); `depends_on` as the dependencies that exist; `tests` and `generated`; `owners` from the local GitHub identity; an `only` rule for network, process, or database code that only some modules use; and an exception for each error that exists today, so the first `check` passes when the evidence is complete. |
| Any other `.archfit.yaml`, such as version 2 (Archfit 3) | Fails with the version error and changes nothing. Remove the file, then run `archfit init`. |
| A version 1 `.archfit.yaml` | Leaves it as it is. |

`init` adds `.archfit-cache/` to the root `.gitignore`. A missing tool leaves a draft based on available evidence. The output lists unread paths and prints each install hint once.

For the owner, init first uses the authenticated GitHub user. Otherwise it uses a GitHub noreply author email as an unverified login. If neither is available, it writes a placeholder and asks you to replace it. No origin remote is required.

New exceptions expire 90 days after the run. The final list names them as decisions for the owner.

In each case it writes the five lines for agents between the `<!-- archfit:start -->` and `<!-- archfit:end -->` markers in `AGENTS.md`, and creates the file when it is missing. Text outside the markers stays as it is. When the block is already there, it stays as it is.

`init` installs the Claude Code and Codex repository hooks unless you pass `--no-hooks`. It leaves the git pre-commit hook alone. If it cannot install a hook, it reports why and keeps the repository setup.

```text
$ archfit init
wrote .archfit.yaml: 4 modules, 0 allowed dependencies, 0 rules, 1 exception
wrote AGENTS.md: the instruction block for agents
installed hooks: .claude/settings.json
installed hooks: .codex/hooks.json
  codex runs these hooks only after you trust them: start codex and approve the hooks
for the owner to decide:
  exception: duplication pkg_tc -> pkg_util until 2027-02-08
  module pkg_chaos: volatility not set; taken as high
  module pkg_runtime: volatility not set; taken as high
  module pkg_tc: volatility not set; taken as high
  module pkg_util: volatility not set; taken as high
next: read .archfit.yaml, then open a pull request with it
```

The list identifies exceptions, modules with no public part declared, and modules whose volatility defaults to high. Read the policy before you open the pull request. A place in the code that `init` could not read is listed with `not read:` and its fix. Unbalanced markers in `AGENTS.md` stop `init` before it writes anything, with exit `2`.

| Flag | Meaning |
| --- | --- |
| `-C, --directory <dir>` | Run in `<dir>` instead of the current directory. |

### `setup`

Run `archfit setup` from the default branch of a checkout with a GitHub `origin`.
Run `archfit setup` again when the output asks you to continue.
After setup creates `archfit/setup`, continue on that branch.
Use `--no-hooks` to skip Claude Code and Codex hooks, or `-C DIR` to select a checkout.

| State | What setup does | What to do next |
| --- | --- | --- |
| No policy | Writes a policy and the agent instructions. | Read `.archfit.yaml`, then run `archfit setup` again. |
| No workflow | Keeps the policy and writes a [pinned workflow](ci.md#the-workflow-archfit-setup-writes). | Follow the next step after the push. |
| Stopped before the push | The HTTPS push uses a `gh` login with known classic scopes, without `workflow`. | Run `gh auth refresh -s workflow`, then run setup again. |
| Pull request not opened | Prints a compare link and saves the body to `.archfit-cache/setup-pull-request.md`. | Open the pull request on GitHub with the saved body. |
| Pull request open, no App check | Prints the App install link. Setup does not re-run the workflow. | Install the App. Use the printed re-run link if the check does not start. |
| App check present | Prints the status of `archfit/gate`. | Review and merge the setup pull request. |
| Done | The default branch has the policy and an Archfit workflow. | Checks are advisory. Setup prints no `next:` line. |

After writing a new policy, setup shows how it would judge up to 30 recent first-parent changes. It also includes the counts in the first setup pull request body. Replay uses the local remote-tracking default branch, does not fetch PR labels or start CI, and stops after 60 seconds. If the local ref is unavailable, run `archfit replay` later.

Before a commit, setup runs the check against the remote default branch. A `fail` does not stop setup.
An `unknown` stops setup before the commit and shows the check's next step.
Setup commits only changed setup paths, but includes any existing edits in those paths.
Review the pull request before merging.

SSH pushes use the SSH key and do not require the `gh` `workflow` scope.
When `gh` does not report classic scopes, setup attempts the push and reports a permission error if Git rejects it.
Without a signed-in `gh`, setup still pushes via Git. It cannot see the pull request or App check.
A later run does not push an unchanged branch. If Git refuses to update the workflow, fix your Git credentials.
When setup can detect a merged policy and workflow, it works without `gh`.

Setup exits `0` when it completes a step or waits for you. It exits `2` when it stops.
Follow the single `next:` line to continue.

### `report`

`archfit report [flags]` gives owners the repository-wide picture. It decides nothing: it exits `0` whatever it shows, and `2` only on wrong usage or a tool error. It never runs on the path of `check`.

On a version 1 policy, it first judges the whole working tree with the rules and exceptions of `.archfit.yaml` on today's date. The state is one of:

| State | Meaning |
| --- | --- |
| `clean` | No error outside the exceptions. |
| `drift` | An error exists that no exception covers. The report lists each one. |
| `expired` | An exception passed its date and the code still breaks the rule. Every change fails until an owner extends the exception or the code is fixed. |

Beside the state, the report lists the places the check could not read (the state does not cover them), the exceptions within 14 days of their date, the rules nothing checks, and the balance of each allowed dependency. Then come the measurements of the code. A tool whose evidence is partial or timed out, or that the code needs and is not installed, is listed first under `evidence not complete:`: a measurement it feeds can show a zero that was not measured. A policy that does not fit the tree, such as a path glob that matches no file, stops the report with exit `2` before the measurements.

```text
$ archfit report
repository: drift, 1 error no exception covers
pkg/tc/netem.go:12: graph: pkg_tc -> pkg_util is not an allowed dependency
  why: a new dependency between modules needs an owner
  balance: 9/10 (min 7)
exceptions close to their date:
  duplication pkg_tc -> pkg_util: 2026-11-19 (9 days)
not checked:
  keep-docs: Each module has a README.
allowed dependencies:
  pkg_chaos -> pkg_runtime: 9/10 (min 7)
measurements:
  cycle: 0 import cycles (strong)
policy: not protected
```

A version 2 policy stops the report with exit `2` and guidance to run `archfit init`.

`--format json` writes the `archfit.architecture-state.v2` document of `archfit.report.schema.json`. On a version 1 policy, its `repository` object holds the state and each list above.

| Flag | Meaning |
| --- | --- |
| `--format <name>` | `text` (default) or `json`. |
| `-C, --directory <dir>` | Run in `<dir>` instead of the current directory. |

### `replay`

`archfit replay [--last N]` runs the policy over the last `N` changes of the default branch (30 by default), to show how many would have passed and how many would have needed an owner. It decides nothing about the working tree and exits `0` when it ran; it exits `2` on wrong usage, a missing or invalid policy, no default branch, or a tool error.

The changes are the first-parent commits of the default branch that origin/HEAD names, as this clone holds them. Archfit fetches nothing. A merge commit and a squash merge are one change each. A pull request merged by rebase shows as several changes, one for each commit, and the output says so when no change in the list is a merge commit. Each change is checked against its first parent, in a temporary worktree of its commit that is removed when the change is done, whatever the outcome. The working tree, the index, `git worktree list` and `.archfit-cache/last.json` stay as they were.

Every change is judged under the policy file of the working tree, not the policy the change had. A policy edit you have not committed counts.

| Class | Meaning |
| --- | --- |
| passed | The status of the change is `pass`. |
| would have needed an owner | The status is `fail` and an error says `archfit request`: only an owner can allow it. |
| to look at first | Everything else: a `fail` that only says to fix the code, `unknown`, a change with no parent in this clone, or a check that could not run. Missing evidence is never counted as a pass. |

When `gh` is signed in, it only labels a change with its pull request number and title. The counts are the same without it.

```text
$ archfit replay --last 30
replay: 30 merged pull requests read: 22 passed, 5 would have needed an owner, 3 to look at first
source: first-parent history of origin/main, read locally; each change against its first parent, under the current policy
would have needed an owner, for example:
  #431 Add the tc shaper: pkg_tc -> pkg_util is not an allowed dependency
```

A shallow clone holds fewer changes than asked for: the oldest change it holds has no parent, so it is counted under "to look at first", and the output adds `incomplete history` and the command that fetches the rest. Text is the only output form.

A run you interrupt with `SIGINT` stops, prints how many changes it has read (`replay: stopped, 14 of 30 read`), and exits `2`. The change being read at that moment is not counted.

| Flag | Meaning |
| --- | --- |
| `--last <N>` | How many changes to read. Default 30. |
| `-C, --directory <dir>` | Run in `<dir>` instead of the current directory. |

### `doctor`

Doctor checks whether the policy names a person as an owner. A placeholder or an organization-only list fails the text check. Without GitHub access, doctor reports that it cannot verify the owner. The JSON report does not include this check.

`archfit doctor [flags]` checks this checkout: the tools archfit uses and the hooks of the agent loop. `--fix` installs the hooks.

The text report lists tools for languages found in the checkout. It checks the workflow pin using the Action step’s trailing version comment, such as `# v0.9.0`. A missing or conflicting comment gives “cannot tell.” Doctor does not fetch the version. The JSON format still lists all known tools.

The tools come in two tables:

| Group | Tools | Without the tool |
| --- | --- | --- |
| Needed for the check | `git` for every language. `ast-grep` for TypeScript, JavaScript and Python imports, and for the literals of every language but Go. `cargo` for Rust. `dotnet` for C#. | The check is `unknown` for the code it cannot read. |
| Optional, for the report | `go`, `cargo-modules`, `archfit-roslyn` | `report` measures less. The check never runs these tools. |

A Go repository needs only `git`. Each row shows the tool, its status (`ok`, `missing`, or `invalid`), the languages that use it, and its path or install hint. For .NET, doctor prints the SDK and helper versions that the current directory's `global.json` selects, and fails when that pin cannot be satisfied.

After the tools and the LLM settings, doctor shows the hooks. For each harness (Claude Code, Codex, git), it shows whether the harness is here and whether each archfit hook runs: `active`, or `inactive` with the reason. Reasons are: not installed; for Claude Code, `disableAllHooks` in `.claude/settings.json` or `.claude/settings.local.json`; for Codex, not trusted yet. Codex keeps hook trust in `~/.codex/config.toml` (`CODEX_HOME`). doctor cannot recompute the trusted hash, so a trusted Codex hook is `active (hash not verified)`. When git ignores the settings file of a harness it found, doctor warns: other checkouts and worktrees do not get its hooks. A tracked settings file reaches every clone, so it gets no warning. Then doctor shows the target branch, the scope of the check, and whether a policy file differs from the target.

`doctor --fix` writes the hooks of each harness it finds, at the root of the repository:

| Harness | Found when | File | Events |
| --- | --- | --- | --- |
| Claude Code | `.claude/` is in the repository, or `claude` is on `PATH` | `.claude/settings.json` | `SessionStart`, `PostToolUse` (after `Edit`, `Write`, `MultiEdit`, `NotebookEdit`, `Bash`), `Stop`, `SubagentStop` |
| Codex | `codex` is on `PATH`, or `CODEX_HOME` (default `~/.codex`) exists | `.codex/hooks.json` | `SessionStart`, `PostToolUse` (after `apply_patch`, `Bash`), `Stop` |
| git | the directory is in a git repository | `pre-commit` in the hooks directory git uses (`core.hooksPath` when set) | the pre-commit hook |

It keeps every other setting and hook in a settings file and replaces its own earlier entries, so a second run changes nothing. It never replaces a pre-commit hook that archfit did not write, and never writes into a hooks directory outside the repository, such as a global `core.hooksPath`: it prints a warning with the line to add to that hook. The hooks read `.archfit.yaml` at the repository root; doctor warns when it does not exist. Codex runs the hooks only after you trust them. Outside a git repository, `--fix` writes no hook and says so. After the hooks, `--fix` installs the analyzer toolchains that archfit can install; `--hooks-only` stops after the hooks. A failed toolchain install keeps the hooks and makes doctor exit non-zero.

| Flag | Meaning |
| --- | --- |
| `--fix` | Install the hooks of the harnesses found here, and the missing toolchains archfit can install. `--install` is an alias. |
| `--lang <name>` | Limit the toolchain install to `go`, `ts`, `py`, or `rust`. Repeatable. Default: all. |
| `--hooks-only` | With `--fix`: write the hooks only; the toolchains are left as they are. |
| `-n, --dry-run` | With `--fix`: print the install commands and the hook files, and change nothing. |
| `--format <name>` | `text` (default) or `json`. `json` prints the tool rows and the hooks as `archfit.doctor.v1`. Each tool row has `group` (`check` or `report`), `languages` (absent: every language), and `status` (`ok`, `missing`, or `invalid` with its `error`). A harness whose settings file git ignores has `ignored: true`. `json` cannot be combined with `--fix`. An `invalid` tool fails `doctor` in both formats, after the full report. |

## Internal hooks

`archfit doctor --fix` installs these entry points. They are not public setup commands.

### `hook run <event>`

`archfit hook run <event> [flags]` is what the installed hooks call. Events are `session-start`, `post-tool-use`, `stop`, and `pre-commit`. Every hook reads `.archfit.yaml` at the repository root, as `check` does. The first three read the hook event (`session_id`, `cwd`, `hook_event_name`, `stop_hook_active`) on stdin and keep the session in `.archfit-cache/session/<session_id>.json`.

| Event | Runs | Stops when |
| --- | --- | --- |
| `session-start` | Records `HEAD` as the session base and prints the five lines that `init` writes for agents. | never |
| `post-tool-use` | `can` on each import on a line added since the session base, when the working tree changed, with the errors that exceptions cover left out. It stores the answer of the first `no` for `archfit request`. | `can` answers `no`, and the session has not shown it yet. |
| `stop` | `check` | The change fails, with errors other than the set it stopped on last. A pass or an `unknown` forgets that set. |
| `pre-commit` | `check` on the staged content against `HEAD` | The commit adds an error, or an exception is past its date. |

| Flag | Meaning |
| --- | --- |
| `--harness <name>` | `claude` (default) or `codex`. Both use the same protocol. |

`pre-commit` also accepts hidden `--index-file` for Git hook integration. `GIT_INDEX_FILE` supplies its default. `archfit hook claude` and `archfit hook git` are removed, and so is `-c`: run `archfit doctor --fix` again to rewrite installed hooks.

## Version and help

Every command accepts `-h, --help`. The top-level command accepts `-v, --version`.

See [configuration](configuration.md) for config fields, [results](results.md) for output contracts, and [CI](ci.md) for gate setup.

## `archfit status`

Show the local check and, when available, the pull request's current story. Status is informational and exits 0.

```text
$ archfit status
✓ change   ✗ check   ● ask #40   ○ answer   ○ policy   ○ re-check
request #40 waits for @alexei-led since 09:12 (about 1 minute of their time).
next: wait, or work on something else. The check runs again by itself when #40 is answered.
```

Without a pull request or GitHub access, status shows the local check. Run `archfit check` again after changing the tree.
