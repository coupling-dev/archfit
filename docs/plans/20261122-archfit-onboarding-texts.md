# Onboarding texts: one vocabulary (task 8.14)

Status: proposal, 2026-11-22. Plan task 8.14 (C7) of `docs/plans/20261122-archfit-0-10-onboarding.md`. The owner
reviews the private page built from this file before 8.9 and 8.10 start. Texts for 8.2, 8.7, 8.9, 8.10 and 8.15
are built from this file once it merges.

Sources:

- The setup proposal, `reports/b10-setup-design-plan.md`, sections 2, 6, 10 and 11 (section 11 wins).
- The onboarding design, `docs/plans/20261120-archfit-onboarding-design.md`, sections 2 to 4 and 8 (O1 amended).
- The approval threat model, `docs/plans/20261122-archfit-approval-threat-model.md` (D6 open).
- The cold run of 0.9.0, `reports/b9-coldrun-0-9-0.md`, stumbles 7, 8, 12 and 14.
- The App's current texts: `internal/control/ui/wording.go`, `internal/gate/feedback/view.go`,
  `web/src/components/Setup.svelte`. The Action's current texts: `scripts/report.sh`.
- The "Not used" column of the change-control design, section 5 (`docs/plans/20261110-archfit-4-architecture-change-control.md`).

How to read this file:

- Every text in sections 2 to 5 is the wording the product should show. A line marked **(changed)** differs from the
  proposal or from 8.7's wording; section 6 gives the old text, the new one and the reason.
- `<angle brackets>` are filled in at run time or by a later task. Placeholders still open: `<install link>` (8.8
  verifies it), `<setup page link>` (8.9 names the path; Q2), `<gh api line>` (8.6), `<operator contact>` (owner).
- No text here uses a word from the "Not used" column of design section 5. Section 7 lists the checks.

## 1. Vocabulary

One word per thing, in the CLI, the check, the pull request, the App and the documents. The rows are the terms
users see; the last rows are words not to use.

| Term                                | Means                                                                                                                       | Use instead of                              | Where it shows                      |
| ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------- | ----------------------------------- |
| **policy**                          | The file `.archfit.yaml`.                                                                                                   | config, architecture file                   | CLI, App, pull requests             |
| **module**                          | A named set of paths.                                                                                                       | component                                   | CLI, App                            |
| **public**                          | The part of a module that other modules may use.                                                                            | surface                                     | CLI, App (`declare as public`)      |
| **allowed dependency**              | Module A may depend on module B.                                                                                            | edge, relation, approved pair               | CLI, App, check text                |
| **rule**                            | Anything the check enforces.                                                                                                | law, intent, guardrail                      | CLI, check text                     |
| **exception**                       | A permission to break one rule in one place, with a reason and a date.                                                      | waiver, baseline entry, debt                | CLI, setup pull request             |
| **error**                           | One place where a change breaks a rule.                                                                                     | finding, violation, blocker, problem        | check, pull request comment         |
| **check**                           | The command, and the pull request check `archfit/gate`.                                                                     | gate, scan                                  | CLI, check, App                     |
| **pass / fail / waiting / unknown** | The four statuses of a change.                                                                                              | blocked, denied, healthy, needs attention   | everywhere                          |
| **advisory**                        | The policy has no `gate: required`. A `fail` does not block a merge.                                                        | rung 2                                      | CLI, check, App, setup pull request |
| **required**                        | The policy has `gate: required` and the ruleset requires `archfit/gate`. A `fail` blocks.                                   | rung 3, enforced                            | CLI, App                            |
| **setup pull request**              | The pull request from branch `archfit/setup` that adds the policy and the workflow.                                         | PR, onboarding pull request                 | CLI, App, pull request              |
| **request**                         | A GitHub issue that asks an owner to change the policy. Its id is the issue number, `#431`.                                 | decision request, CR, ticket                | CLI, App, pull request              |
| **approve / reject**                | An owner's answer to a request.                                                                                             | accept, decline, record                     | App                                 |
| **admit**                           | An operator lets an account or installation use Archfit. Only an owner's answer is "approve".                               | approve an installation, approve an account | App                                 |
| **owner**                           | A person who writes the policy and answers requests.                                                                        | architect, approver                         | everywhere                          |
| **administrator**                   | The GitHub role on an organization or repository. Used only where GitHub requires it (the first approval, the App install). | —                                           | App, install pages                  |
| **worker**                          | A person or agent who changes code.                                                                                         | —                                           | pull request                        |
| **balance**                         | The coupling number, shown as `9/10 (min 7)`.                                                                               | rating, score                               | App, CLI                            |
| **report**                          | The repository-wide measurements for the owner.                                                                             | analysis, insight                           | CLI, App                            |
| **quiet**                           | A pull request with no error. Count word only.                                                                              | —                                           | setup page, replay                  |
| **asked**                           | A pull request that opened a request for an owner. Count word only.                                                         | —                                           | setup page, replay                  |
| **for the owner to decide**         | The open points of a policy, listed by `init` and `setup`.                                                                  | open decisions                              | CLI, setup pull request             |
| **setup page**                      | The App's page for one repository: its state and its next step.                                                             | dashboard                                   | App                                 |
| **the App**                         | The Archfit GitHub App. In full: "the Archfit App".                                                                         | the bot                                     | everywhere                          |
| **make it required**                | The action that starts rung 3 on the setup page.                                                                            | approve and enforce                         | App                                 |

Not shown to users, so not used in any text: **rung** (the design's name for advisory, required and try; the
design team uses it, the product does not), **surface**, **edge**, **gate** (except in the names `archfit/gate` and
`gate:`), **scan**, **finding**, **violation**, **blocker**, **task**, **decision** (use "to decide" or "question"),
**accept**, **decline**, **record** as a verb, **review** as the name of the report, **PR** (write "pull request"),
**score**, **rating**, **architect**, **approver**, **insight**, **analysis**, **healthy**, **blocked**, **denied**.

**Where the vocabulary changes the proposal's words:** section 6.

## 2. `archfit setup` in the terminal, step by step

Each run prints what it did, then `state:` and one `next:` line. A `next:` line holds one action. Information lines
that are not an action have no `next:` prefix. A second run does nothing that the first run did.

Exit codes: `0` when the step is done or setup waits for a person; `2` when setup stopped.

### 2.1 No policy (first run)

```text
$ archfit setup
wrote .archfit.yaml: 14 modules, 23 allowed dependencies, 3 rules, 3 exceptions
wrote AGENTS.md: the agent block
installed hooks: Claude Code, Codex
replay: 30 merged pull requests read: 22 quiet, 5 asked, 3 to show an owner
for the owner to decide:
  exception billing-legacy: expires 2027-02-20 (90 days)
  module reports has no public part
  volatility not set: 2 modules
state: no workflow yet
next: archfit setup
```

The next line is the second run. Read the policy first; setup does not wait for you. Exit `0`.

### 2.2 Replay stopped at 60 seconds (C5)

Same as 2.1, with this line in place of the replay line:

```text
replay: stopped after 60 s, 14 of 30 read; run archfit replay later
```

Setup goes on. The setup pull request body then has no replay section.

### 2.3 Policy, no workflow: files pushed and pull request opened

Second run, with the `workflow` scope present (C4).

```text
kept .archfit.yaml
hooks: active
wrote .github/workflows/archfit.yaml: coupling-dev/archfit-action@<40-hex> # v0.10.0
check: pass against origin/main
created branch archfit/setup
committed .archfit.yaml, AGENTS.md, .github/workflows/archfit.yaml as <your name> <your email>
pushed archfit/setup
opened setup pull request #12: <pull request url>
state: setup pull request open; the App is not installed
next: install the App: <install link>
```

Exit `0`. The `40-hex` commit is written in full in the file; the output shows the first 12 characters.

### 2.4 `gh` missing `workflow` scope (C4)

Stops before the push. Written in the same run as 2.3, in place of the push and pull request lines:

```text
wrote .github/workflows/archfit.yaml: coupling-dev/archfit-action@<40-hex> # v0.10.0
the gh login has no workflow scope, so GitHub would refuse the workflow file
state: stopped before the push
next: gh auth refresh -s workflow
```

Exit `0`: setup waits for you. Run `archfit setup` again after the refresh.

Without `gh`, the push fails instead, and setup prints git's first error line with the same `next:` line.

### 2.5 No `gh`: compare link

Same as 2.3 up to the push. Then:

```text
pushed archfit/setup
pull request body: .archfit-cache/setup-pull-request.md
state: setup pull request not opened; gh is not signed in
next: open the pull request: https://github.com/<owner>/<repo>/compare/main...archfit/setup?expand=1
the App, once the pull request is open: <install link>
```

Exit `0`. The last line is information. The second run cannot tell the pull request from the App check, so it says so
(see 2.6).

### 2.6 Setup pull request open, App not installed

Second run, or a run after 2.5:

```text
setup pull request #12 is open: <pull request url>
App check: none yet on 3f9a1c2
setup page: <setup page link>
state: waiting for the App
next: install the App: <install link>
```

With no `gh`, the same state reads: `pull request: open it, or check on GitHub; gh cannot show it` (a single
information line, no second run needed).

Exit `0`.

### 2.7 Setup pull request open, App check present (merge to start)

```text
setup pull request #12: App check "merge to start" (neutral) <check url>
state: waiting for the merge
next: merge #12: <pull request url>
```

Exit `0`.

### 2.8 Merged: advisory

```text
setup pull request #12 is merged
advisory: checks report on each pull request and do not block a merge (changed; see 6.1)
setup page: <setup page link>
state: done
```

No `next:` line. Exit `0`.

### 2.9 Stop states (exit 2)

Dev build (row 0):

```text
archfit 0.10.0-dev (commit 4b1e0c9) is a development build: it has no Action release to pin
state: stopped
next: install a released archfit: brew install coupling-dev/tap/archfit
```

Wrong branch (row 0). Setup starts on the default branch only:

```text
HEAD is feature/billing, not main
state: stopped
next: git switch main
```

Foreign workflow file (row 0). A file that does not use the Action is not overwritten:

```text
.github/workflows/archfit.yaml exists and does not use coupling-dev/archfit-action; setup does not overwrite it
state: stopped
next: rename .github/workflows/archfit.yaml
```

Closed pull request (row "closed"):

```text
setup pull request #12 was closed without a merge: <pull request url>
state: stopped
next: reopen #12: <pull request url>
```

Also: "or delete the branch `archfit/setup` and run `archfit setup` again" as an information line below the `next:` line.

Missing origin head (row 0). Setup passes `--base` itself, so this is the only case left:

```text
origin/HEAD is not set, so setup does not know the default branch
state: stopped
next: git remote set-head origin --auto
```

Default branch with commits not on origin (row 0; the proposal gives no text, this is new):

```text
main has 2 commits that origin/main does not have
state: stopped
next: git push origin main
```

## 3. Pull request texts

Check names: `archfit/gate` for the App's check; `archfit` for the Action's job.

### 3.1 The Action job before the App is installed (C1, 8.15)

Job conclusion: neutral. Job summary:

```text
archfit: not checked yet
Install the App: <install link>
This job did not check the change: the Archfit App is not installed on this repository.
After you install it, the App runs the check on this pull request again. Nothing to re-run.
```

Replaces the current `- App: refused the upload: HTTP 403 ...` line for this one refusal only. Every other refusal
still fails the job.

### 3.2 The App's check on the setup pull request (C2, 8.7)

Check `archfit/gate`, conclusion neutral, never `action_required`, never a passkey wait. Title:

```text
merge to start
```

Summary:

```text
This pull request sets up Archfit. Merge it to start the checks.
Nothing is checked, and no merge is stopped, before then.
```

### 3.3 Advisory pass on an ordinary pull request (8.7)

Check conclusion: success. Title:

```text
pass, advisory
```

Summary:

```text
archfit: pass
Advisory: this check does not block the merge.
```

### 3.4 Advisory fail on an ordinary pull request (8.7)

Check conclusion: neutral (the same as the current advisory mapping). Title:

```text
fail, 1 error, advisory
```

Summary (the error text is the App's existing text, in the same shape; the rule text is an example):

```text
archfit: fail, 1 error
internal/billing/invoice.go:42 **reports is not an allowed dependency of billing**
- why: billing does not read reports; reports reads billing
- instead: move the shared type to shared
Advisory: this check does not block the merge. To make it required, open the setup page: <setup page link>
```

The how-to line (the last line) replaces the proposal's advisory line (section 6.2).

### 3.5 Unknown on an ordinary pull request

Check conclusion: as the App decides today for unknown (6.4 asks whether it stays action_required in advisory).
Title:

```text
unknown, no verdict
```

Summary, one example per reason (two shown; each reason has its own `next:` line):

```text
archfit: unknown, no verdict
Archfit cannot read a valid policy on the default branch.
next: merge a valid .archfit.yaml to the default branch
```

```text
archfit: unknown, no verdict
The policy changed outside the flow. An owner must confirm or revert it.
next: ask an owner to confirm the policy change: <setup page link>
```

### 3.6 Welcome comment on the setup pull request (8.10)

```markdown
## Archfit is set up on this repository

Merge this pull request to start the checks. Each pull request then gets one verdict: pass, fail or unknown.
Until an owner makes the check required, a fail does not block a merge.

- To ask for a change to the policy, run `archfit request -m "<why>"` on your branch.
- The owners are <owner list>. They answer requests in the App: <setup page link>.
- The setup page shows the counts for this week and the next step.
```

Owner list: the owners from the policy, as written there.

### 3.7 Setup pull request body (8.2)

```markdown
This pull request sets up archfit 0.10.0: the policy, the workflow and the instructions for agents.
Checks are advisory: nothing blocks a merge until an owner makes the check required.

## Last 30 merged pull requests

22 quiet, 5 asked, 3 to show an owner. Read them with `archfit replay --last 30`.

## For the owner to decide

- exception billing-legacy: expires 2027-02-20 (90 days)
- module reports has no public part
- volatility not set: 2 modules

## Next

1. Install the Archfit App: <install link>
2. Merge this pull request.
```

The replay section is left out when replay is not available or was stopped. The owner section is left out when the
policy has nothing for the owner to decide. Both missing: the first and the last part only.

## 4. The App setup page (8.9)

One page per repository. One state, one next action. The page never asks anyone to copy a workflow by hand.
The page keeps the App's current style: sentence case headings, one primary button, a muted line for
"why", and no marketing text.

### 4.1 Not installed

Heading: `Set up Archfit for <owner>/<repo>`

```text
The Archfit App is not installed on this repository.
Install it on this repository only.
```

Button: **Install the App** (link to GitHub's install page; the link selects this repository if GitHub allows it,
8.8 verifies).

Below the button, one line for each edge case in section 5.

### 4.2 Installed, setup pull request open

Heading: `Set up Archfit for <owner>/<repo>`

```text
Merge the setup pull request to start the checks.
Setup pull request #12: open. No merge is stopped before you merge it.
```

Button: **Open pull request #12**.

### 4.3 Advisory (after the setup pull request merged)

Heading: `Checks are advisory on <owner>/<repo>`

```text
This week: 12 checked, 9 quiet, 3 asked.
Checks report on each pull request and do not block a merge.
```

Button (primary, beside the counts): **Make it required**.

Muted line: `Make it required when the owners agree. It starts with one approval of the policy and the workflow.`

Link: **Requests** (the open requests, by number).

### 4.4 "Make it required" (before the first approval)

Heading: `Make the check required`

Section **Changes since the setup pull request**. Listed before anything else. Nothing here is enforced yet (O1
amended, onboarding design section 8):

```text
#41  exception reports-shared, expires 2027-02-20   merged by @ada   request #37
#44  module billing, public part set               merged by @ben   request #39
```

Muted line: `Read each change before you approve. The first approval covers all of them.`

Section **What happens**, one line each:

1. Approve the policy and the workflow with your passkey. One approval covers both files.
2. The App opens a pull request that sets `gate: required` in the policy. Merge it.
3. Add the branch ruleset that requires `archfit/gate`. Import the file in GitHub under Settings → Rules → Rulesets,
   or run `<gh api line>`.

Warning line (above the steps, always shown): `Without the ruleset, a fail does not block a merge, even when the policy says required.`

Button: **Approve with your passkey** (D6 default: the App passkey, pending the owner's choice; section 7).

If no passkey is registered: `You need a passkey to approve.` and button **Register a passkey**.

Disclosure **Why a passkey?**: `A passkey proves that a person made this approval now. A token or a saved session does not.`

Errors shown on this page (each one sentence, with the next step):

- Not an owner: `Only an owner named in the policy can approve.`
- First approval, not an administrator: `Only an owner who is also a repository administrator can approve the first policy. Ask one: <owner list>.`
- Policy changed since the page loaded: `The change moved since you looked. Reload the page and approve again with your passkey.`

### 4.5 Required

Heading: `Required on <owner>/<repo>`

```text
The check blocks a merge that the policy does not allow.
Approved by @ada on 2026-11-24 with a passkey, for request #37.
Ruleset: archfit/gate is required on the default branch.
```

Button: **Review the policy** (the current policy on the default branch).

Muted line: `This week: 12 checked, 9 quiet, 3 asked.`

## 5. Install edge cases (C3)

Each case has one message and one action. Shown on the setup page in place of the install button, and as the install
page's error text when GitHub shows one.

### 5.1 The account is not admitted

Heading: `This account is not admitted to Archfit yet.`

```text
Archfit admits accounts by hand. An operator must admit yours before the App can be installed.
Send this to <operator contact>:
  Please admit the GitHub account <login> to Archfit, for the repository <owner>/<repo>.
```

Button: **Copy the message** (selectable text, too).

### 5.2 The installer is not an administrator of the organization

Heading: `Only an administrator of <owner> can install the App.`

```text
Ask an administrator of <owner> to open the install link and install the App on <repo>:
  <install link>
Send them this page: <setup page link>
The administrators of <owner> are on its People page on GitHub.
```

Button: **Copy the message** with the link and the page address.

## 6. Changes to the proposal's wording

### 6.1 Terminal lines (proposal section 6 and owner changes in section 11)

| Where             | Proposal says                                                                        | This text says                                                                         | Why                                                                           |
| ----------------- | ------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| merged            | `rung 2: advisory`                                                                   | `advisory: checks report on each pull request and do not block a merge`                | "rung" is the design's name; users see the state, not the rung                |
| no policy, replay | `open decisions:` and its list                                                       | `for the owner to decide:`                                                             | "decision" is on the Not used list (design 5); the test would fail the golden |
| no policy, replay | (no replay line shape given)                                                         | `replay: 30 merged pull requests read: 22 quiet, 5 asked, 3 to show an owner`          | "quiet" and "asked" as in design 4.1                                          |
| install step      | `install the App: <link>, then archfit setup`                                        | `next: install the App: <link>`; the rerun is the user's                               | one action per `next:` line (the brief); the proposal's rerun is dropped (C1) |
| no `gh`           | `open the pull request: <compare link>, install the App: <link>, then archfit setup` | `next: open the pull request: <compare link>`; the install link on an information line | one action per `next:` line                                                   |
| PR open, no check | `App check: none on <head>`                                                          | `App check: none yet`                                                                  | short and plain                                                               |
| PR open, no check | `re-ran the archfit job: <run url>`                                                  | removed                                                                                | C1: the App re-runs; setup does not                                           |
| advisory replay   | `which three an owner would have wanted to see`                                      | `3 to show an owner`                                                                   | short                                                                         |
| no policy         | `read .archfit.yaml, then archfit setup`                                             | `next: archfit setup` with the line "read the policy first" above                      | one action per `next:` line                                                   |

### 6.2 Pull request texts (8.7's proposed wording)

The 8.7 worker proposed the title `Archfit advisory: pass/fail` and this advisory line:

> Advisory only. To make this check required, have a repository administrator approve the policy and workflow in the App and add the branch ruleset.

| Proposed                                     | New                                                  | Why                                                                                                                            |
| -------------------------------------------- | ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| title `Archfit advisory: pass/fail`          | title `pass, advisory` and `fail, 1 error, advisory` | The other titles put the status first (`fail, 1 error`, `waiting, request #431`). "Archfit" is the check's name already.       |
| "advisory" only in the title                 | "advisory" kept, after the status                    | A neutral icon on a fail reads as skipped to a stranger (cold run, stumble 14). The word says what the grey icon means.        |
| "have a repository administrator approve"    | "open the setup page"                                | The owner answers, not the administrator. The administrator is needed only for the first approval (section 4.4 says so there). |
| "approve the policy and workflow"            | one approval, named on the page                      | "Approve the policy and the workflow" reads as two decisions. The cold run (stumble 12) took it as two.                        |
| "add the branch ruleset" in the pull request | on the page, step 3                                  | The ruleset comes after the approval and the policy change. One action per line.                                               |

New line (advisory, with the link): `Advisory: this check does not block the merge. To make it required, open the setup page: <setup page link>`

### 6.3 App page

No proposal text existed for the App page (8.9 writes it). The rules in section 4 apply.

### 6.4 Open for the owner or 8.7

- **Unknown in advisory.** `checkState` maps unknown to `action_required`, which blocks a merge (`internal/gate/feedback/publish_result.go`). Advisory is non-blocking by design (D2). The text in 3.5 assumes unknown does not block in advisory. Decide: neutral in advisory too (recommended), or keep action_required.
- **"Make it required" in advisory.** The proposal lists "policy changes since setup" only before the first approval. Section 4.4 follows that.

## 7. Checks and open points

### 7.1 Words

Each text in sections 2 to 5 was checked against the "Not used" list of design section 5 (the same list the engine
and App test). The check ran on the text of these sections, not on the table in section 1 or section 6.

### 7.2 Open for the owner

| Id                  | Question                                                                      | Default in this file                                              |
| ------------------- | ----------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| D6                  | Which approval does "Make it required" use in 0.10.0?                         | The App passkey, shown in 4.4 as the default and marked pending.  |
| D6-G1               | The threat model's G1 (first registration): does the page need a step for it? | 8.9 decides how; the page shows the registration issue as a line. |
| install link        | Does GitHub's install page pre-select the repository?                         | `<install link>`, verified by 8.8.                                |
| operator contact    | Who admits accounts (5.1)?                                                    | `<operator contact>`: the owner names the person or address.      |
| counts              | Is "asked" the right word for the count (C9 uses it)?                         | Yes; section 1 defines it. Alternative: "needed an owner".        |
| unknown in advisory | See 6.4.                                                                      | Neutral in advisory (recommended).                                |
| rung names          | Do users ever see "rung" or the numbers 1 to 3?                               | No. Section 1 keeps them out.                                     |

### 7.3 Done when

- Plain words, short lines, terms used the same way everywhere (section 1).
- Every `next:` line is one action (sections 2 and 6.1).
- The private page built from this file is published, and the owner's review is requested by the manager.
