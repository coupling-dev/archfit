# Changelog

History before 0.9.0 is in [docs/archived/CHANGELOG-v0-v5.md](docs/archived/CHANGELOG-v0-v5.md).

## v0.9.0

### Summary

Archfit has a new home in the `coupling-dev` organization and restarts at 0.9.0.
The engine, the image, the Action, the Homebrew formula and the App carry this one number.
The policy file starts again at `version: 1`.
A dependency now takes its strength from the surface its target declares.
Each balance names its inputs, and a use past a surface has a legal next step.
`init` gives a first run that does not end in an error.
While releases are 0.x, a minor release may break the policy, the record, the exit codes or the result JSON.
A patch release never breaks them. See [Version rule](docs/development.md#version-rule).

### Breaking changes

- **New home and install lines.** Install with one of these:

  ```sh
  go install github.com/coupling-dev/archfit/cmd/archfit@v0.9.0
  brew install coupling-dev/tap/archfit
  docker pull ghcr.io/coupling-dev/archfit:v0.9.0
  ```

  The Go module is `github.com/coupling-dev/archfit`, with no major suffix.
  The schema `$id` values point at `https://raw.githubusercontent.com/coupling-dev/archfit/main/`.
  Releases up to v5.0.0 stay in the old home. Nothing there is retagged or deleted.

- **Policy `version: 1`.** Every command refuses another version with one error.
  To upgrade, keep a copy of the old `.archfit.yaml`, remove it and run `archfit init` to write a fresh policy.
  Then carry your rules and decisions over from the copy.
  `init` no longer converts an Archfit 3 policy, and the migration guide is gone.
- **Strength from the declared surface.** One function gives every dependency its strength.
  `check`, `can`, the request preview and `report` call it.
  - A use of a `public` path is `contract`.
  - A use of a `model` path is `model`.
  - A use of any other path in the target module is `intrusive`.
  - Every strength takes the volatility of the target module.

  `balance.strength` no longer takes `functional`. Earlier, a use past the surface took the higher volatility of the two modules.

- **`report` balances change.** The report no longer guesses a strength from Go or .NET types.
  Type facts stay on the graph as evidence. On this repository, 49 of 67 seams in the report changed.
  `check` verdicts did not change on the four repositories that run Archfit.
- **New policy errors.** The same glob in `public` and `model` is an error.
  A file in both parts is an error. A `model` glob that matches no file of its module is an error.

### New

- **Policy key `model`.** `modules.<name>.model` lists the paths of a module that others may use as its model.
  When a module names `model` and not `public`, it has no public part.
  An added `model` glob loosens the policy, and a removed one tightens it.
- **The balance names its inputs.** The record, `can`, the request preview and SARIF show the strength, the target module and its volatility:

  ```text
  balance: 9/10 (min 7): contract, pkg_util is low
  ```

- **A legal next step past a surface.** A `coupling` error for a use past the surface names the package and the declared surface.
  It offers `archfit request` in both `deny` and `ask` mode:

  ```text
  pkg_tc -> pkg_util: coupling: pkg/util/retry is past the surface
    balance: 2/10 (min 7): intrusive, pkg_util is high
    surface: pkg/util (public), pkg/util/types (model)
    next: use the surface, or: archfit request -m "<why retry belongs to the surface>"
  ```

  `archfit request` asks one question for such an error: should the package be part of the surface of its module?
  The owner changes the declaration. The use is never approved as it is.
  `archfit rules` lists the surface of each allowed dependency and the module's own `model` part.

- **Dated exceptions.** `init` writes `expires` on each exception it creates, 90 days after the run.
  It ends with the list of those exceptions as decisions for the owner.
- **Agent hooks by default.** `init` installs the Claude Code and Codex repository hooks, also when a policy exists.
  Use `--no-hooks` to leave hook files alone. The git pre-commit hook stays opt-in.
- **A first run without an error.** When an analyzer is missing, `init` writes a draft from the code it can read.
  It lists the paths it could not read and prints each install line once.
  `check` stays `unknown` while a required analyzer is missing.
  `init` adds `.archfit-cache/` to `.gitignore`.
- **`doctor` matches the workflow pin.** `doctor` lists only the tools for the languages it finds.
  It compares the local engine with the `# vX` comment at the end of the Action's `uses:` line.
  A missing or conflicting comment gives "cannot tell", never a mismatch. `doctor` makes no network call.

### Changed

- The `report` text uses the words of the record: declared, assumed and seen.
  It groups repeated reasons for unread evidence, with a count and up to three paths. The JSON keeps every path.
- `archfit request` ignores a stored result of another engine version, so it never sends empty balance inputs.
- The README starts from the problem and the ask-once loop, with an animated example.

### Result JSON

- `balance.strength` (`contract`, `model` or `intrusive`) and `balance.volatility` are new and always present.
- `surface` (`public` and `model` path lists) is new on an error and on a `can` answer.
  It is set for a use past the surface.
- `package` is new on an error and on a `can` answer. It names the package past the surface.
- Exit codes do not change: `check` exits `0` for `pass`, `1` for `fail` and `2` for `unknown`.
