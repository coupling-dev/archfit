# Getting started

This guide installs archfit, sets up a repository, and shows the daily loop: `rules`, `can`, `check`, and `request`. For full tool requirements, see [Language support](languages.md).

There are two ways to set up a repository:

- **Checks on GitHub pull requests:** run `archfit setup` and follow [Try it on your pull request](try-it-on-your-pull-request.md). Setup writes the policy, the workflow and the setup pull request.
- **Local only:** run `archfit init`, as below. It writes the policy and no workflow; you can run `archfit setup` later.

## Install archfit

On macOS or Linux, install the latest release with Homebrew:

```sh
brew install coupling-dev/tap/archfit
```

Or install with Go 1.27 or newer:

```sh
go install github.com/coupling-dev/archfit/cmd/archfit@latest
```

Each command installs the latest release. Setup pins the Action release that matches the archfit you installed, so install the release you want before you run `archfit setup`. The Homebrew formula and the release binary need no Go toolchain. You can also download a release binary from the [release page](https://github.com/coupling-dev/archfit/releases). Check the file against `SHA256SUMS` before you install it.

## Check the tools

Run:

```sh
archfit doctor
```

This command reports tools for the languages it finds, in two groups: needed for the check, and optional, for the report. The check always needs `git`. Other tools depend on the languages. Install the tools of the first group for the languages in your repository. A missing tool makes the check `unknown` for the code it cannot read. See [Language support](languages.md#analyzer-tools).

`doctor` compares the local engine with the version comment on the workflow pin. It also reports missing or inactive hooks. See [Hooks](agents.md#hooks).

## Set up the repository locally

For checks on GitHub pull requests, run `archfit setup` instead: see [Try it on your pull request](try-it-on-your-pull-request.md). For the local-only path, run from your repository:

```sh
archfit init
```

`init` writes the policy, `.archfit.yaml`, from the code: the modules, the part of each module that other modules use today (`public`), the dependencies between modules that exist (`depends_on`), the test and generated paths, and the authenticated GitHub user as owner, or an unverified login from a GitHub noreply author email. If neither identity is available, replace the placeholder with your GitHub login. Owners approve requests, so name an owner who does not write most of the changes: `init` and `setup` say so when they name you as the only owner. Each error that exists today becomes an exception dated 90 days from the run, so the first check passes when the evidence is complete. The final list names these decisions for the owner. `init` also writes five lines for agents between markers in `AGENTS.md`.

```text
$ archfit init
wrote .archfit.yaml: 4 modules, 0 allowed dependencies, 0 rules, 1 exception
wrote AGENTS.md: the instruction block for agents
installed hooks: .claude/settings.json
installed hooks: .codex/hooks.json
  codex runs these hooks only after you trust them: start codex and approve the hooks
exceptions for the owner to review:
  exception: duplication pkg_tc -> pkg_util until 2027-02-08
owner: alice. Owners approve requests: name an owner who does not write most of the changes
next: read .archfit.yaml, then open a pull request with it
```

`init` installs repository hooks for Claude Code and Codex by default. To skip them, run `archfit init --no-hooks`. The git pre-commit hook is opt-in.

If a tool is missing, `init` writes a draft policy and lists what it could not read. Install the named tool and review the draft before opening a pull request.

`init` adds `.archfit-cache/` to `.gitignore`.

Read the policy, then open one pull request with it. An owner removes an exception when the code is fixed, and writes rules with `why` and `instead`. Read [Configuration](configuration.md) for the fields and [Rules](rules.md) for the built-in rules, custom rules, and exceptions.

`init` never changes a version 1 policy. A version 2 policy (`.archfit.yaml` from Archfit 3) is an error: remove it, then run `archfit init` for a fresh version 1 policy.

## Work on a change

1. Before you start, show the rules for the files you change:

   ```sh
   archfit rules
   archfit rules pkg/tc
   ```

2. Before a new dependency between modules, ask:

   ```text
   $ archfit can pkg/tc pkg/util
   pkg_tc -> pkg_util: graph: not an allowed dependency
     balance: 9/10 (min 7): contract, pkg_util is low
     next: archfit request -m "<why you need it>"
   ```

3. Before you finish, check the change against the target branch:

   ```sh
   archfit check
   ```

   The status line comes after the errors: `archfit: pass`, `archfit: fail, N errors`, or `archfit: unknown, no verdict`. The exit code is `0`, `1`, or `2`. Each error says where, which rule, what is wrong, and `next`: do what `next` says. `unknown` is never a pass: the record names what to restore. See [Results](results.md).

When no workflow uses the Archfit Action, every result carries the line `policy: not protected`: the result is a local guide. The protected check on pull requests comes with the Archfit App and Action 4.

## When a rule is in your way

Some rules need an owner: a new dependency between modules (`graph`), or a copy of a literal another module holds (`duplication`). A dependency past a declared surface can also ask an owner. Its request asks whether the package belongs to that surface. Their `next` is `archfit request`. After `archfit check` (or `archfit can`), run:

```sh
archfit request -m "netem needs the shared retry helper"
```

The request takes the errors of the last check, so you write only the reason. Add `--expires 2026-12-10` to ask for an exception until a date. Without the App, `archfit request` prints the request and ends with `next: ask an owner`: show it to an owner. Do not copy the code to get around the rule.

## For owners: the state of the repository

```sh
archfit report
```

`report` judges the whole tree, not one change. Its state is `clean`, `drift` (an error no exception covers), or `expired` (an exception passed its date and the code still breaks the rule). It also lists the exceptions close to their date, the rules nothing checks, the balance of each allowed dependency, and the measurements of the code. It exits `0` whatever it shows.

## Run the check in CI

A pull request is checked against its target branch. Fetch the history and name the branch:

```sh
archfit check --base "origin/$TARGET_BRANCH"
```

Fail the job on any exit code except `0`. See [CI](ci.md) for the workflow.

## Use Docker

The release image runs archfit with a bundled toolchain:

```sh
docker run --rm -v "$(pwd):/src" ghcr.io/coupling-dev/archfit:latest \
  check
```

The complete image includes Node, Python, Rust, the .NET SDK, ast-grep, cargo-modules, and the Roslyn helper (`archfit-roslyn`).
Go is build-only and is not present in the final image.
No analyzer package installation is needed at run time.
See [Languages and tools](languages.md) for exact versions, optional execution, and limits.

The image runs as a non-root user and supports an arbitrary numeric UID with `HOME=/tmp`.
Target dependencies can still need network access. The image is not a network-isolated sandbox:
optional indexers and Rust build tools can execute target code. Do not forward credentials into it.

## More help

- [Commands](commands.md) — flags and exit codes.
- [Configuration](configuration.md) — the policy fields and their defaults.
- [Troubleshooting](troubleshooting.md) — common setup and analysis issues.
