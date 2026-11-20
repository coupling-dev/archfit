# Archfit onboarding design

Status: proposed, 2026-11-20. The owner accepts it before task 3.8 of the beta plan. Epic 8
(release 0.10.0) is built from it.

Sources:

- The beta plan, `docs/plans/20261120-archfit-0-9-new-home-and-beta.md`: task 2.4, epic 8,
  sections 2 and 3.
- The change-control design, `docs/plans/20261110-archfit-4-architecture-change-control.md`,
  sections 9 and 10. Section 7 of this document lists where this design changes it.
- The surfaces design, `docs/plans/20261117-archfit-4-1-surfaces-and-suggestions.md`, sections
  6.1, 6.6 and 6.9.
- The cold run of task 0.4, terminal part, measured on 2026-11-20 with `v0.9.0-rc.1` on a fresh
  clone of `nektos/act` (team report `archfit-76-coldrun-terminal`).
- The beta readiness review v0.4 of 2026-11-19, section 5 (private page, not in this repository).
- The code on `main` at `3b9fe5c6`, pull request #7 (task 3.0), and the App on `main` at
  `499131d`.

**Numbers.** A number marked _measured_ comes from the cold run. A number marked _provisional_
comes from the readiness review and is replaced when the App part of task 0.4 is measured. A
number marked _target_ is a goal of the beta plan, not a measurement.

## 1. Today

The terminal part, _measured_:

| Step                                    | Place    | Time      | What happened                                                                                                                                         |
| --------------------------------------- | -------- | --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| Install with `go install …@v0.9.0-rc.1` | terminal | seconds   | Works. `--version` shows no commit for a `go install` build.                                                                                          |
| `archfit doctor`                        | terminal | under 1 s | Lists `cargo`, `cargo-modules`, `archfit-roslyn` as missing and `dotnet` as needed in a Go-only repository. Every hook is "inactive (not installed)". |
| `archfit init`                          | terminal | 1 s       | A 151-line policy: 14 modules, 23 allowed dependencies, 3 rules, 3 exceptions, none with `expires`. Also edits `AGENTS.md`.                           |
| `archfit check`                         | terminal | 1 s       | `pass`, with "origin/main has no .archfit.yaml: judged with the local policy" and `policy: not protected`.                                            |
| Left behind                             | —        | —         | `.archfit-cache/` is untracked and not in `.gitignore`.                                                                                               |

The reward of the terminal part today is a pass on a tree with no change, and a 151-line YAML file
with no list of decisions.

The whole path, _provisional_ (review v0.4, section 5): **11 steps before the first verdict on a
pull request, in 4 places** (terminal, GitHub, the App, waiting on the operator):

1. install the CLI; 2. install ast-grep; 3. `archfit init`; 4. write the workflow by hand; 5. open
   the setup pull request; 6. install the App; 7. wait for admission; 8. sign in; 9. register a
   passkey; 10. approve the first policy; 11. merge. The first verdict comes after step 11. The
   ruleset is a twelfth step after it.

The minutes of steps 6 to 11 and the admission wait are not measured.

## 2. The route

The _target_ (beta plan, epic 8): a person who has never used Archfit gets a verdict on a pull
request in **under ten minutes** and in **at most four steps**:

1. Install the CLI: `brew install coupling-dev/tap/archfit`.
2. `archfit setup`.
3. Install the App, from the link `setup` prints.
4. Merge the setup pull request.

Where each step of today goes:

| Today (provisional)                        | On the route                                                                                                      | Task     |
| ------------------------------------------ | ----------------------------------------------------------------------------------------------------------------- | -------- |
| 1. Install the CLI                         | Step 1.                                                                                                           | —        |
| 2. Install ast-grep                        | Off the route. `init` writes the policy for the code it can read and lists the other paths with one install line. | 3.6      |
| 3. `archfit init`                          | Inside `setup`.                                                                                                   | 8.2      |
| 4. Write the workflow by hand              | `setup` writes it, with the pins of its own version.                                                              | 8.2      |
| 5. Open the setup pull request             | `setup` opens it.                                                                                                 | 8.2      |
| 6. Install the App                         | Step 3.                                                                                                           | —        |
| 7. Wait for admission                      | Gone: the account is admitted before the invite is sent.                                                          | 8.8      |
| 8. Sign in, 9. passkey, 10. first approval | Moved to rung 3, "Make it required".                                                                              | 8.7, 8.9 |
| 11. Merge                                  | Step 4.                                                                                                           | —        |
| Ruleset                                    | Moved to rung 3, with a ruleset file.                                                                             | 8.6      |

## 3. Three rungs

Each rung gives a reward before it asks for more. A team can stop on any rung.

|        | Rung 1: try                                                                                                    | Rung 2: advisory                                                                    | Rung 3: required                                                      |
| ------ | -------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| Who    | One developer or agent                                                                                         | The team, on one repository                                                         | The owners, when the team asks                                        |
| Needs  | The CLI. No account, no network.                                                                               | The App installed, the setup pull request merged. No sign-in, no passkey.           | Sign-in, a passkey, a repository administrator.                       |
| Runs   | `init`, `check`, `replay --last 30`, `can`, `rules`                                                            | The workflow on each pull request; the App comments.                                | The same, and the check blocks.                                       |
| Reward | How noisy the policy would have been on the last 30 merged pull requests, and the open decisions of the owner. | A verdict on every pull request, a welcome comment, a digest after seven days.      | No change merges past the policy without an owner's answer.           |
| Policy | Local, `policy: not protected`.                                                                                | On the default branch, `gate` absent (advisory). A `fail` shows as a neutral check. | `gate: required`. Approved with a passkey. Loosening needs a passkey. |
| Exit   | `archfit setup`                                                                                                | "Make it required" on the App setup page                                            | —                                                                     |

Rung 3 is also the success signal of the beta: a team that makes one boundary required by its own
choice (beta plan, task 9.5).

## 4. Commands and pages on the route

### 4.1 Commands

| Command                                 | Rung | What it does                                                                                                                                                                                                  | Task         |
| --------------------------------------- | ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| `brew install coupling-dev/tap/archfit` | 1    | Installs the CLI. Other ways: `go install`, a release binary checked against `SHA256SUMS`. No script piped into a shell: a stranger's agent runs this line.                                                   | —            |
| `archfit init`                          | 1    | Writes the policy and the agent hooks. Section 5.                                                                                                                                                             | 3.4–3.6, 8.5 |
| `archfit check`                         | 1–3  | The verdict on the change. Unchanged.                                                                                                                                                                         | —            |
| `archfit replay --last N`               | 1    | Runs the policy over the last `N` merged pull requests: how many stay quiet, how many would have asked an owner, which three an owner would have wanted to see. Its summary goes into the setup pull request. | 8.4          |
| `archfit setup`                         | 1→2  | One resumable command; section 4.2.                                                                                                                                                                           | 8.2          |
| `archfit setup --agent`                 | 1→2  | Prints the runbook for a coding agent, matched to the installed version. The prompt a user copies is one line: "Install archfit, run `archfit setup --agent` and follow it."                                  | 8.3          |
| `archfit doctor`                        | 1–3  | Shows only the tools of the languages found, the hooks, and one line: the local engine matches the workflow pin.                                                                                              | 3.5, 3.6     |
| `archfit can`, `archfit request`        | 2–3  | Unchanged. A use past a surface may ask one question (surfaces design 6.9).                                                                                                                                   | 3.3          |
| `archfit why <a> <b>`                   | 2–3  | Who approved a dependency, when, and the reason, read from the history of `.archfit.yaml`.                                                                                                                    | 8.12         |

### 4.2 `archfit setup`

`setup` is a state machine. Each run reads the state from the repository and GitHub, does the
next step, and prints where the user is and one `next:` line. A second run never repeats a done
step. A person and an agent follow the same path.

| State                                       | `setup` does                                                                                                                                                                                                                                                                            | `next:`                                         |
| ------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| No policy                                   | Runs `init` (section 5) and `replay --last 30`. Prints the open decisions.                                                                                                                                                                                                              | `archfit setup` again after you read the policy |
| Policy, no workflow                         | Writes `.github/workflows/archfit.yaml`: `coupling-dev/archfit-action@<commit> # vX`, where `vX` is the version of the running CLI. It finds the commit by the Action tag `vX` (`git ls-remote`).                                                                                       | — (same run)                                    |
| Files not on a branch                       | Creates the branch `archfit/setup`, commits the policy, the workflow and the `AGENTS.md` block with the user's git identity, pushes, and opens the pull request with `gh` when it is signed in, else prints the compare link. The body holds the replay summary and the open decisions. | install the App: `<install link>`               |
| Pull request open, no App check on its head | Prints the App install link and the setup page link.                                                                                                                                                                                                                                    | install the App, then `archfit setup`           |
| App check present                           | Prints the result of the check on the setup pull request.                                                                                                                                                                                                                               | merge #N                                        |
| Merged                                      | Prints "rung 2: advisory" and the App setup page.                                                                                                                                                                                                                                       | —                                               |

`setup` stops at each human step (install the App, merge) and never does it for the user.
`setup` never writes the policy over a version 1 file, as `init` does not.

**Decision: `setup` writes the workflow, `init` never does.** The change-control design (10.1)
says `init` writes the workflow. `init` in 0.9.0 does not (`docs/commands.md`, `init`), and this
design keeps it so. Reasons: `init` is rung 1 and needs no account; a workflow that runs before the App is
installed has its upload refused, and the Action fails the job (`scripts/report.sh` in the Action); and one
command, `setup`, owns every pin, so the engine, the Action and the workflow line carry one number.

**The agent runbook.** `setup --agent` tells the agent to: run `setup`; propose module merges and
names; draft `why` and `instead` from the written rules of the repository; ask the owner which
existing dependency the team only tolerates and which next one it must see in review; put the
replay in the pull request; stop at every human step. The `AGENTS.md` block says "never edit
`.archfit.yaml`". Onboarding is the one exception, and the runbook says it ends when the setup pull
request merges.

### 4.3 Pages

| Page                           | Rung | Shows                                                                                                                                                  | Task          |
| ------------------------------ | ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------- |
| GitHub App install page        | 2    | GitHub's own page, for the repository `setup` named.                                                                                                   | —             |
| App setup page                 | 2–3  | The state of one repository and one next action. Never asks the user to copy a workflow by hand. The last action is "Make it required".                | 8.9           |
| Pull request check and comment | 2–3  | Today's record. With `gate` absent or `advisory`, a `fail` is a neutral check. From the first merge of the setup pull request, with no first approval. | 8.7           |
| Welcome comment                | 2    | On the setup pull request: how to ask, what happens next.                                                                                              | 8.10          |
| Digest                         | 2    | After seven days: pull requests checked, quiet, asked.                                                                                                 | 8.10          |
| Request page                   | 2–3  | The request, the balance with its inputs, the surface answers (declare as public, declare as model, allow until a date, reject with `instead`).        | 5.2           |
| "Make it required"             | 3    | The first approval with a passkey (D6 default), the policy pull request that sets `gate: required`, the ruleset file and its `gh api` line.            | 8.1, 8.6, 8.9 |

One page in the documents, "Try it on your pull request", tells the route once: the Pumba story,
two expected results, the fork limit. The engine guide, the Action README and the App guide link
to it (task 8.11).

## 5. Defaults

### 5.1 `init` in 0.9.0 today

Unchanged by this design until epic 3 lands (`docs/commands.md`, `init`; checked on a scratch
repository with `main` at `3b9fe5c6`):

- Writes a version 1 policy from the code: modules, `public`, `depends_on`, `tests`, `generated`,
  `owners` from the GitHub `origin` remote, `only` rules, and an exception for each error that
  exists today. No `expires`. No `gate` key, so the gate is `advisory`.
- Writes the five lines for agents in `AGENTS.md`.
- Writes no workflow and installs no hook.
- Stops with exit `2` before it writes anything when a tool is missing, the `AGENTS.md` markers are
  unbalanced, or there is no GitHub `origin` remote.
- Leaves `.archfit-cache/` untracked.

### 5.2 `init` after epic 3 (0.9.0) and epic 8 (0.10.0)

| Default                                                                                                                                                                                                      | Task                  |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------- |
| Writes the policy for the code it can read. Each path it cannot read is listed with one install line. A missing tool never ends `init` in an error.                                                          | 3.6                   |
| Every exception it writes has `expires`. It ends with the list of them as decisions for the owner.                                                                                                           | 3.4                   |
| Adds `.archfit-cache/` to `.gitignore`.                                                                                                                                                                      | 3.6                   |
| Installs the agent hooks unless `--no-hooks`.                                                                                                                                                                | 3.5                   |
| Writes no `gate` key: the repository starts advisory.                                                                                                                                                        | today                 |
| Writes no workflow.                                                                                                                                                                                          | today; `setup` in 8.2 |
| Ends with the open decisions of the policy: exceptions that need a date, modules with no declared surface, packages used past a surface, volatility nobody set. Also in the report. Never on a pull request. | 8.5                   |

**The `expires` date** is a proposal of this design: 90 days after `init`. Task 3.4 has not
started; its worker follows the value the owner accepts here.

### 5.3 Agent hooks

| Hook                                            | Default after task 3.5                                                               | Reason                                                                                                                                                               |
| ----------------------------------------------- | ------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Claude Code: session-start, post-tool-use, stop | Installed by `init` when Claude Code is found.                                       | The trap run in review v0.4 (one small model, 4.0.0 build): without hooks 4 of 4 runs kept the forbidden import; with hooks 4 of 4 asked. A signal, not a benchmark. |
| Codex: session-start, post-tool-use, stop       | Installed by `init` when Codex is found. Codex runs them after the user trusts them. | The same.                                                                                                                                                            |
| git pre-commit                                  | Not installed by `init`. `archfit doctor --fix` installs it.                         | A `fail` stops a person's commit. On rung 1 nobody has agreed to that yet.                                                                                           |

`init --no-hooks` installs none. `doctor` reports each missing hook (3.5). `doctor --fix` stays the
way to install them in another checkout.

## 6. What 0.9.0 must already carry

0.10.0 adds commands and pages. It must not make a repository on 0.9.0 change its policy or its
workflow, and it must not make the App reject a 0.9.0 result. The App decodes results, envelopes
and the manifest with `DisallowUnknownFields` (`internal/wire/result/decode.go`,
`internal/wire/envelope/decode_v2.go`, `internal/wire/manifest/engine4.go`), and the result schema
sets `additionalProperties: false`. So every field 0.10.0 needs exists in 0.9.0.

| Item                                                                                                 | 0.10.0 needs it for                                                                                                            | In 0.9.0                                                                                                                                                                                                      |
| ---------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Policy `version: 1`                                                                                  | Every command.                                                                                                                 | Carried: `archfit.schema.json` (#3).                                                                                                                                                                          |
| Policy key `gate`, absent means `advisory`                                                           | Rung 2 starts with no key; rung 3 writes `gate: required`.                                                                     | Carried: `archfit.schema.json` (`default: advisory`), `internal/config/policyfile/file.go`. The App reads it from the policy on the target branch and shows a `fail` as neutral (`internal/gate/upload4.go`). |
| Policy key `model`                                                                                   | The request page answer "declare as model".                                                                                    | Task 3.0 (#7) adds the key; 3.1 gives it meaning; 5.2 in the App.                                                                                                                                             |
| `expires` on exceptions                                                                              | The open decisions; dated exceptions from `init`.                                                                              | Carried by the schema. Task 3.4 makes `init` write it.                                                                                                                                                        |
| `# request #N` on each policy line the App writes                                                    | `archfit why` reads who and why from the history of the file.                                                                  | Carried by the App's policy writer (`internal/wire/policy3/write.go`). Task 5.2 keeps it in the version 1 writer.                                                                                             |
| Result: `strength` and `volatility` in `balance`, optional `surface`                                 | The balance with its inputs on the request and pull-request pages; the surface request.                                        | Task 3.0 (#7); filled by 3.1 to 3.3; decoded by the App in 5.2. Contract id stays `archfit.result.v1`.                                                                                                        |
| No new result field in 0.10.0                                                                        | A 0.10.0 App accepts 0.9.0 results, and a 0.9.0 App is not sent fields it rejects.                                             | A rule for epic 8. A field it needs later gets a new contract id, and the App accepts both for a window.                                                                                                      |
| Manifest with several entries, `v0.MINOR.PATCH(-rc.N)` versions, Action commits of both repositories | The 0.10.0 App lists 0.9.x and 0.10.x side by side.                                                                            | Task 5.3. Today the manifest accepts only `v4.*` (`internal/wire/manifest/module.md`).                                                                                                                        |
| One number: Action `vX` pins engine `vX`                                                             | `setup` finds the Action commit by the tag of its own version.                                                                 | Tasks 4.2 and 4.3.                                                                                                                                                                                            |
| The `uses:` line ends with `# vX`                                                                    | `doctor` compares the pinned version with its own, with no network call. A missing comment is "cannot tell", never a mismatch. | Task 3.6 (`doctor`) and task 5.3 (the App's onboarding template writes the comment). Today the template writes none (`internal/control/onboarding/templates/archfit.yaml.tmpl`).                              |
| `policy.protected` is true when a workflow uses `coupling-dev/archfit-action`                        | The workflow `setup` writes is recognized.                                                                                     | Carried: `cmd/archfit/change_wiring.go` (`actionRepo`, #3).                                                                                                                                                   |
| Upload envelope 2                                                                                    | The Action uploads as today.                                                                                                   | Carried, unchanged.                                                                                                                                                                                           |

Not a contract, so not needed in 0.9.0: advisory before the first approval (task 8.7) changes how
the App treats an upload whose policy no owner approved yet (today the reason `first_approval`
makes it `unknown`). It is App behaviour on the same result.

## 7. Changes to the change-control design

The owner accepts them with this document.

| Change-control design                                                          | This design                                                                                                         | Reason                                                                                                                                       |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| 10.1: `init` writes the policy, the workflow and the agent block.              | `init` writes the policy, the agent block and the agent hooks. `setup` writes the workflow.                         | Section 4.2.                                                                                                                                 |
| 9.2: the first policy is approved with a passkey before change control starts. | The first merge of the setup pull request starts advisory checks. The first approval comes with "Make it required". | Decision D2 of the beta plan. Nothing blocks in advisory, so there is nothing to forge. The protected branch stays the source of the policy. |
| 10.7: setup is 6 human or agent actions.                                       | 4 steps.                                                                                                            | Section 2.                                                                                                                                   |

## 8. Open points

| Id  | Question                                                                      | Default                                                                                                                                                                                                                              |
| --- | ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| O1  | How does an owner answer a request in rung 2, before any passkey?             | The request page prepares the one-line policy pull request; the owner merges it on GitHub with no passkey, because nothing is enforced yet. The first approval in rung 3 covers the policy as it is then. Linked to D6 and task 8.1. |
| O2  | The `expires` date `init` writes.                                             | 90 days. Task 3.4 follows the owner's answer.                                                                                                                                                                                        |
| O3  | The minutes and steps of the App part of task 0.4.                            | Not measured. The numbers marked provisional above are replaced when they are.                                                                                                                                                       |
| O4  | Does `setup` open the pull request with `gh`, or only print the compare link? | `gh` when it is signed in; else the link. No GitHub token is read by Archfit.                                                                                                                                                        |
| O5  | A repository with a policy written by `init` before `setup` exists.           | `setup` keeps it and continues from "Policy, no workflow".                                                                                                                                                                           |
