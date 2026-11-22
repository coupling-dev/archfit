# The story of a change and the four-part message (task 8.18)

Status: accepted by the owner, 2026-11-22 ("I like the prototype — make it real"), with the public-part answers of section 5. Plan task 8.18
of `docs/plans/20261122-archfit-0-10-onboarding.md`. It extends the accepted `docs/plans/20261122-archfit-onboarding-texts.md`
(8.14) and does not change it. Where this file and 8.14 name the same thing, 8.14 wins.

The owner said, on 2026-11-22: "these actions: reload, edit, approve — are not that obvious; I am not sure I would know
what to do without AI working on these." This file is the answer for the words and the order of events. Tasks 8.19 (answer
pages), 8.20 (the App owns git), 8.21 (`archfit status`) and 8.22 (notifications) build what it shows.

Sources: `reports/archfit-76-ux-brainstorm.md` (team directory), the App editor plan for 8.20
(`reports/b10-app-editor-plan-8-20.md`), `docs/commands.md`, `docs/results.md`, and the current App texts
(`internal/control/ui/wording.go`, `internal/gate/feedback/view.go`).

## 1. The story of a change

Every change that touches the policy goes through the same six steps. All three places show the same row, with the
current step marked.

```text
change → check → ask → answer → policy → re-check
```

One running example is used in this file: pull request #412 makes `billing` use `reports`. The policy does not allow it.
The worker is an agent working for `@dev`. The owner of `reports` is `@alexei-led`. The request is #40. The App's policy
pull request is #44.

| Step         | What happens                                                                                                 | Who acts             | Terminal                                                                        | Pull request                                                                   | App                                                   |
| ------------ | ------------------------------------------------------------------------------------------------------------ | -------------------- | ------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | ----------------------------------------------------- |
| **change**   | A worker changes code and pushes.                                                                            | worker               | `archfit can`, then `archfit check` before finishing.                           | The `archfit/gate` check starts.                                               | Nothing.                                              |
| **check**    | The check finds an error: the change breaks a rule an owner may allow.                                       | the check, by itself | `archfit check` prints the error with `why:` and `next:`.                       | Check `fail`. Comment with the story row, **Next:** line and the error.        | The setup page counts the pull request as checked.    |
| **ask**      | The worker asks the owner of the module.                                                                     | worker               | `archfit request -m "<why>"` prints who answers, where, and what happens after. | Check `waiting`. Comment names `@alexei-led`. Request #40 opens as a question. | The owner's list "Needs you" shows #40.               |
| **answer**   | The owner reads the question and answers with one passkey.                                                   | owner                | `archfit status` shows `answer` as the current step, then `approved`.           | Comment: "#40 approved by @alexei-led".                                        | The answer page. Then a history line.                 |
| **policy**   | The App changes the policy in its own pull request, keeps it up to date, and merges it when its checks pass. | the App              | `archfit status`: "the App is merging #44".                                     | Comment: "policy updated in #44".                                              | History line. The row sits under "Moving on its own". |
| **re-check** | The check runs again with the new policy and passes.                                                         | the check, by itself | The agent hook says "#40 approved: run archfit check".                          | Check `pass`.                                                                  | History line: "PR #412 passes".                       |

Rules for the story:

- A step is **done**, **current**, **failed** (only `check`), or **later**. A path that ends early marks the rest as not
  needed. A check that passes at once has no `ask`, `answer` or `policy` step.
- A rejection ends at `answer`. The worker gets the owner's `instead:` text and starts again at `change`.
- Exactly one person or system is named as the next actor. A place never shows two equal next actions.
- The worker's state is never "wait" without a reason and an end: the message says what the wait is for.

Four statuses are shown to people: `pass`, `fail`, `waiting`, `unknown` (8.14). `waiting` is set by the App on a pull
request only, when an open request covers every error; the command line never prints it. In the terminal, `archfit status`
shows the same fact as the `ask` step with the request number.

## 2. The four-part message

Every message that stops or asks a person says four things, in this order. A message that needs only a few words still
holds all four; a part may be one clause.

1. **What is happening.** The fact, with the place: file and line, pull request, request number.
2. **Why.** The rule or the reason in plain words. Not the rule's internal name alone.
3. **Next, and who.** One action and the person or system that does it, with the time it takes when a person acts.
4. **After.** What happens once that action is done.

Where the four parts go, per surface:

| Surface              | What                     | Why                                | Next, and who                       | After                                                                  |
| -------------------- | ------------------------ | ---------------------------------- | ----------------------------------- | ---------------------------------------------------------------------- |
| Terminal record      | the error line           | `why:`                             | `next:`                             | `after:` (new line, only on a `next:` that waits for a person or asks) |
| Pull request check   | check title              | first line of the summary          | **Next:** line                      | line under **Next:**                                                   |
| Pull request comment | top line (story row)     | the error, folded details under it | **Next:** line with the `@`-mention | "After" line                                                           |
| Request issue        | the title, as a question | evidence                           | "Answer in the App" and the choices | the line under the choices                                             |
| App page             | page title               | evidence block                     | the answer buttons                  | "What happens" under each button                                       |

### 2.1 Terminal: `archfit check`

```text
$ archfit check
internal/billing/invoice.go:42: graph: billing -> reports is not an allowed dependency
  why: billing does not read reports; reports reads billing
  next: archfit request -m "<why you need it>"     (do not copy the code)
  after: an owner answers; the policy updates by itself; run archfit check again
archfit: fail, 1 error
```

### 2.2 Terminal: `archfit request`

```text
$ archfit request -m "invoice needs the report totals"
asked: request #40 for 1 error, billing -> reports
who answers: @alexei-led, owner of reports (about 1 minute, passkey)
where: <answer page link>
after: the policy updates by itself and this branch passes archfit check; archfit status shows the steps
next: wait, or work on something else
```

Without the App, the request is printed and the last line is `next: ask an owner` (as today); `after:` is left out.

### 2.3 Agent hook

On the end of a turn, and on the start of a session, while a request of the branch is open:

```text
archfit: request #40 waits for @alexei-led (since 09:12). Do not work around the rule.
Work on something else. The check runs again when #40 is answered. archfit status shows the steps.
```

On approval:

```text
archfit: request #40 is approved and the policy is updated. Run archfit check.
```

On rejection:

```text
archfit: request #40 is rejected by @alexei-led. Instead: move the shared type to shared. Then run archfit check.
```

### 2.4 Pull request: check and comment

Check `archfit/gate`, conclusion neutral in advisory, `action_required` in required mode. Title:

```text
waiting for @alexei-led: request #40
```

Comment (one comment per pull request, edited in place; the top line is the story row):

```markdown
✓ change → ✗ check → ● **ask #40** → ○ answer → ○ policy → ○ re-check

**Next: @alexei-led answers request #40** (1 minute, passkey): <answer page link>
After the answer, the App updates the policy and this check runs again. Nothing for the author to do.

`billing` uses `reports` at `internal/billing/invoice.go:42`. The policy does not allow it: billing does not read reports.

<details><summary>Details</summary>
rule graph · balance 9/10 (min 7) · request #40 · policy version 3f9a1c2 · result sha256:…
</details>
```

### 2.5 Request issue (the question)

```markdown
**May `billing` depend on `reports`?**

@alexei-led, you own `reports`. Pull request #412 makes `billing` use it at `internal/billing/invoice.go:42`.
The policy does not allow that. The author says: "invoice needs the report totals".

Evidence: `billing` → `reports`: 1 use. `reports` → `billing`: none. Balance 9/10 (min 7).

**Answer in the App** (1 minute, passkey): <answer page link>

- Allow: adds 1 line to the policy.
- Allow until 2027-02-20: adds an exception that expires on that date.
- Reject, and say what to do instead.

After your answer the App updates the policy and checks #412 again. You can undo an answer later.
```

### 2.6 App: the answer page

The page is `§5 mockup 1`. Its four parts: the title is the question (what); the evidence block is why; the buttons are
next, with the passkey; each button's effect line is after.

## 3. Vocabulary

8.14 section 1 is the base. This section adds the words the story needs and the words that must go. A **banned** word is
banned in user-facing text, which section 4 defines. Folded details are the one place where a version, fingerprint or
`digest` may show.

### 3.1 Allowed (the story words)

| Word                                                                                                                   | Means                                                                               | Notes                                                                                |
| ---------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| **change**                                                                                                             | What a worker pushes. Step 1.                                                       | "pull request" when the pull request itself is meant.                                |
| **check**                                                                                                              | The command and `archfit/gate`. Step 2 and step 6 ("re-check").                     |                                                                                      |
| **ask** / **request**                                                                                                  | A worker asks an owner to change the policy. The request is the GitHub issue `#40`. | "ask" is the verb and the step name; "request" is the noun.                          |
| **answer**                                                                                                             | What the owner gives: approve, approve until a date, make public, or reject.        | Use "answer" for the act and the page; "approve" and "reject" name the two outcomes. |
| **policy**                                                                                                             | `.archfit.yaml`. Step 5 is "policy".                                                | "policy pull request" is the App's pull request that changes it.                     |
| **owner**, **worker**, **module**, **public**, **allowed dependency**, **rule**, **exception**, **error**, **balance** | 8.14                                                                                |                                                                                      |
| **pass, fail, waiting, unknown**                                                                                       | 8.14                                                                                |                                                                                      |
| **next**, **after**, **why**                                                                                           | The labels of the four-part message.                                                |                                                                                      |

### 3.2 Banned in user-facing text

| Banned                                             | Use                                                                  | Why it goes                                                                                                                              |
| -------------------------------------------------- | -------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| proposal                                           | request (the question), policy pull request (the App's pull request) | Two words for one thing.                                                                                                                 |
| void, voided                                       | "no longer applies", "needs your approval again"                     | Names a hidden state.                                                                                                                    |
| outside the flow                                   | "changed without a request"                                          | The flow is the design team's word.                                                                                                      |
| YAML edit, edit the YAML                           | "change the policy", "answer the request"                            | The normal path is an answer, not an edit. The advanced editor is labelled "Advanced: edit the policy file" (an allowed exception, 4.3). |
| envelope                                           | result                                                               | The Action's upload is a "result".                                                                                                       |
| digest, hash, sha                                  | version, fingerprint                                                 | Only inside folded details, the `--verbose` output and the uploaded file.                                                                |
| reload, refresh (the page)                         | nothing: the page updates itself                                     | If something changed, the page says what changed and shows the new diff.                                                                 |
| behind, update branch, admin merge, squash, rebase | nothing: the App does it                                             | Git mechanics belong to the App (8.20).                                                                                                  |
| stale                                              | "changed since"                                                      |                                                                                                                                          |

The words of 8.14 and design section 5 stay banned (rung, surface, edge, gate, scan, finding, violation, decision,
accept, decline, record as a verb, PR, and the others). One conflict to settle in the first run of the test: today's
`archfit can` prints a `surface:` line, which 8.14 bans. The test finds it; the engine slice changes it to `public:` or
adds a marked exception.

Hook texts for agents use the same words. An agent is told to run `archfit check`, never to "re-trigger" or "refresh".

## 4. The vocabulary test (spec for the three slices)

One list, one matcher, three repositories. The engine already has both: `testutil.NotUsedIn` and its word list in
`internal/testutil/wording.go`; the App vendors it (`internal/wire/wording/wording.go.txt`, with a copy and a drift check
in `web/e2e/not-used.json`). The slices extend that mechanism; they do not write a second one.

### 4.1 The list

- The engine's `notUsedWords` is the source. The slice adds the words of 3.2 (as whole words and phrases, any case,
  across white space: the matcher's current rule) and the 8.14 words it lacks (`rung`, `surface`, `pr`).
- The App re-vendors the file. The Action takes a copy in `tests/vocabulary/not-used.txt` with the same drift check the
  App has: a test fails when the copy differs from the engine's list at the pinned release.
- `archfit/gate` and `gate:` stay names, not words (the existing `gateNames` rule). No other name is exempt by default.

### 4.2 Two kinds of test, in each repository

**Rendered text (authoritative).** One table-driven test per repository renders every user-facing state and runs the
matcher on the whole text. One row per state of the story: check `pass`, `fail` with and without a request, `waiting`,
answered, policy updating, re-check passed, rejected, approval no longer applies, unknown, plus the setup states of 8.14.
A new state without a row fails a test that counts the states.

| Repository       | What is rendered                                                                                                                                                                                                                                                                                   |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| archfit (engine) | the text of every command in every state: `check`, `can`, `request`, `setup`, `status`, `init`, the hook texts. The existing tests (`hook_test.go`, `changecheck_test.go`, `request_test.go`, `rules_test.go`, `canquery_test.go`) already do this for part of them.                               |
| archfit-action   | the job summary of `scripts/report.sh` for each result and each refusal, run with fixtures (the Action's `tests/` already run it).                                                                                                                                                                 |
| archfit-app      | every check title, summary and comment (`internal/gate/feedback`, `internal/gate/approvals/texts.go`), every error sentence of `internal/control/ui/wording.go`, and the rendered pages (`web/e2e`, over the state fixtures of `docs/ui-state-fixtures.md`: `notUsedIn(document.body.innerText)`). |

**Static text (catches what no fixture reaches).** A file list, not a directory sweep: only files whose text is for
people are named, and each has an extractor. Code identifiers, wire formats and schemas are never scanned: the App's
`ErrOutsideFlow`, the `envelope` schema and `digest` fields are not user-facing and stay.

| Repository     | Files                                                                                                                                               | Extractor                                                                                                                                                                                                          |
| -------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| archfit        | user pages: `README.md` and `docs/*.md` except `architecture.md`, `development.md`; `.github/ISSUE_TEMPLATE/*.yml`                                  | the whole file, code fences included (the examples are what users copy)                                                                                                                                            |
| archfit-action | `README.md`, `action.yml` (input and output descriptions), `scripts/texts.sh`                                                                       | whole file; the slice moves every message that `report.sh` prints into `scripts/texts.sh`, so the scan is one file and not a shell parser                                                                          |
| archfit-app    | `internal/control/ui/wording.go`, `internal/gate/feedback/view.go`, `internal/gate/approvals/texts.go`, `docs/user-guide.md`; `web/src/**/*.svelte` | Go: string literals only, read with `go/parser` (comments and identifiers skipped). Svelte: template text and the `title`, `aria-label`, `placeholder` and `alt` attributes, with `<script>` and `<style>` removed |

The slice fixes the file list in the test, so a new text file that nobody added fails review, not the test. A file in the
list that does not exist fails the test.

### 4.3 Marking an allowed exception

An exception is visible next to the text and has a reason. There is no central ignore file.

- One line: a comment on the same line or the line above, in the file's comment style:
  `// wording: allow digest: shown only in the verbose record` (Go, TS), `# wording: allow …` (shell, YAML),
  `<!-- wording: allow … -->` (Markdown, Svelte).
- A block in Markdown or a template: `<!-- wording: allow digest begin: reason -->` … `<!-- wording: allow digest end -->`.
- Folded details: text inside `<details>` is skipped, for `digest`, `hash` and `sha` only. In the terminal the same rule
  covers the `--verbose` output.
- A whole file that is a reference (for example `docs/evidence.md`, which explains digests): an entry in the test's table,
  `file → words → reason`. Each entry shows in review.
- The advanced policy editor page may use "edit the policy file" under a heading "Advanced". Its marker names the word and
  says "advanced tool, never on the normal path".

A marker for a word that the line does not use fails the test, so markers do not outlive the text.

### 4.4 What a failure prints

```text
internal/control/ui/wording.go:34: "outside the flow" is not used. Use: "changed without a request".
  to allow it here: // wording: allow outside the flow: <reason>
```

The first run in each repository lists the existing hits. The slice fixes the texts it owns or marks an exception; it
does not delete a word from the list to pass. Known hits: the App's `outside the flow` and `Reload the page` sentences;
the engine's `surface:` line.

## 5. Mockups

The private page (`story-and-messages mockups`) shows each state in the App's style. This list names what it shows, with
the text that matters. Task numbers are the tasks that build each part.

1. **Answer page (8.19).** Question as title, evidence, three answers as large buttons (four for a public-part request), each with its effect, one passkey.
2. **Batch answers (8.19).** "3 requests wait for you": choose per request, one passkey, one policy pull request.
3. **Undo (8.19).** "Revert this answer" on an answered request, with its effect.
4. **Main moved (8.20): nothing to do.** The history line and the sentence that says the App handled it.
5. **Re-approve needed (8.20).** The policy on main changed after the answer; the new diff; one button.
6. **`archfit status` (8.21)** for every step and for the rejected and offline cases.
7. **Pull request comment (8.22)** at each step, with the `@`-mention.
8. **"Needs you" (8.22)**, with the story row on each row and a second list for what moves on its own.

Text of the answer page, for the vocabulary check (button labels and effect lines):

```text
May billing depend on reports?
  [Allow]                 Adds 1 line to the policy. Pull request #412 passes when the policy is updated, about 2 minutes. You can undo it later.
  [Allow until a date]    Adds an exception for this one place that ends on the date you choose. After it, the check fails again.
  [Reject…]               Changes nothing in the policy. The author sees what you say to do instead.
Confirm with your passkey.
```

For a request that asks "should this package be public?" (owner decision, 2026-11-22), three primary answers:
**Make it public**, **Allow until a date** and **Reject…** (with `instead`). Below them, a smaller fourth option:
**Declare it as shared model**, with one line: other modules may use it as a shared model, a lower balance than a public part (7 or 8 of 10; `docs/configuration.md`). None approves the use as it is (`docs/commands.md`, `can`):
two change the declaration, one is an exception with an end date.

When something changed while the owner looked: a banner above the page, "Request #40 was answered by @maria 1 minute ago",
or "The policy on main changed: the diff below is the new one". The page never says "reload".

When main moved and the answer still applies (8.20 B: the policy bytes are the same):

```text
approved by you 09:14 → the App moved the policy change onto the new main (#44) → merged 09:19 → PR #412 passes
```

When the policy bytes changed, the answer no longer applies (8.20 B):

```text
The policy on main changed after you approved: @maria changed lines 12 to 14 in 3f9a1c2.
Your answer no longer applies as written. This is the change now: +1 line.      [Approve again]
```

`archfit status`:

```text
$ archfit status
✓ change   ✗ check   ● ask #40   ○ answer   ○ policy   ○ re-check
request #40 waits for @alexei-led since 09:12 (about 1 minute of their time).
next: wait, or work on something else. The check runs again by itself when #40 is answered.
```

## 6. What the owner and the worker never see again

| Hidden state today                    | What replaces it                                                                                                      |
| ------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| "Reload" after a change on the page   | A banner that names the change.                                                                                       |
| A branch behind the default branch    | Nothing. The App moves its policy pull request itself.                                                                |
| "Approval void"                       | "The policy on main changed after you approved" and one **Approve again** button, only when the policy bytes changed. |
| "The policy changed outside the flow" | "The policy on main changed without a request: <who>, <lines>. An owner confirms it or reverts it."                   |
| A policy editor as the way to answer  | The answer page. The editor stays as an advanced tool, never linked from a request.                                   |

## 7. Open points

- **Re-check trigger.** The brainstorm says the check runs again by itself when the policy updates. If the App cannot do
  that for a given pull request, the **After** line says "run archfit check" and `status` shows `re-check` as the worker's
  step. 8.20 confirms which.
- **Who is `@`-mentioned.** The owners of the module the request touches, from the policy (8.22 decides the lookup). One
  mention per owner per request, and one more when the owner must act again (approve again). Edits of the comment do not
  mention again.
- **`archfit status` needs the App.** Without GitHub access it shows the local part and says so: `unknown: cannot read
request #40 (no GitHub access)`, with the `gh auth login` next line (8.21).
- **Undo (8.19).** The mockup assumes "Revert this answer" opens a policy pull request that removes the line and merges it
  when green, and says that open pull requests that rely on it fail again. 8.19 confirms.
- **Exception date default.** The mockup offers 90 days as in 8.14's exception example. The owner may type another date.

## 8. Done when

The owner accepts the mockups. The three vocabulary tests exist: engine (list and docs scan), Action (copy, texts file),
App (re-vendor, rendered and static). 8.19 to 8.22 use the texts of this file; a change to a text starts here.
