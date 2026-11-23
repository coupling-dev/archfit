# Archfit 0.10.0: onboarding

Status: accepted by the owner on 2026-11-22 with changes (section 2, "Owner changes"). Owner: Alexei Ledenev.
Plan owner: archfit-76. Team: b10 (manager `b10-manager`).

Sources:

- The beta plan, `docs/plans/20261120-archfit-0-9-new-home-and-beta.md`: epic 8, section 8 (record).
- The onboarding design, `docs/plans/20261120-archfit-onboarding-design.md`, accepted 2026-11-20 with open
  point O1 amended. Sections 2, 4, 5 and 6 are used here.
- The approval threat model, `docs/plans/20261122-archfit-approval-threat-model.md` (task 8.1, D6 open).
- The cold run of 0.9.0 (task 0.4 terminal part and task 7.3), 2026-11-21: 1 h 44 min to the first
  verdict, 9 stranger steps to the first pass. Its stumbles are listed in section 4.

One release, **0.10.0**: epic 8 and the fixes first planned as a 0.9.1 hotfix. One release cycle, one App
deploy, one round of pin pull requests.

## 1. Outcome

1. A stranger, on the route of section 3.2 and using only the published documents, reaches a verdict on a
   pull request in under ten minutes and in at most four steps. The cold run of task 8.13 measures it on a
   new scratch repository.
2. A repository set up with 0.10.0 never has an organization as its only owner. The docs give the exact
   Action commit of the release and say how to find it.
3. No 0.10.0 change breaks a 0.9.0 result in the App (section 5). The four onboarded repositories move their
   pin to 0.10.0 in one round.

## 2. Decisions

Already made:

| Decision | Answer |
| --- | --- |
| D1 Policy key `model` | In 0.9.0. |
| D2 Advisory before the first passkey | Yes. Rung 2 starts with no passkey and no `gate` key. |
| D3 Policy file version | `version: 1`. |
| Route | Four steps: install the CLI, `archfit setup`, install the App from the link `setup` prints, merge the setup pull request. |
| Workflow owner | `archfit setup` writes the workflow. `archfit init` never does (design section 4.2). |
| Pin | The Action tag `vX` pins engine `vX`. `setup` finds the Action commit by `git ls-remote`. The workflow line ends with `# vX`. |
| Result contract | `archfit.result.v1` gets no new field in 0.10.0 (design section 6). A field needed later gets a new contract id, and the App accepts both for a window. |

Owner changes of 2026-11-22 (relayed by archfit-76), part of this plan:

| # | Change | Task |
| --- | --- | --- |
| C1 | Before the App is installed, the Action treats "upload refused: no installation" as a **neutral** result, not a failed job. Its summary says "Install the App: <pre-filled install link>". When the App is installed, it brings the check on the open setup pull request up to date, with no new permission (P1): a check-suite re-request if GitHub allows it for an Actions suite, else a comment with the direct re-run link. | 8.15 (Action), 8.8 (App) |
| C2 | The setup pull request always gets a neutral advisory check with a short "merge to start" message: never `action_required`, never a passkey wait. | 8.7 |
| C3 | App install edge cases, each with one clear message: the account is not admitted; the installer is not an organization administrator (who to ask, what to send them). The install link pre-selects the repository if GitHub supports it (verify). | 8.8 |
| C4 | `archfit setup` checks that the `gh` login has the `workflow` scope before it pushes `.github/workflows/`. If not, its `next:` is `gh auth refresh -s workflow`. | 8.2 |
| C5 | `replay --last 30` inside `setup` stops after 60 seconds and says "run archfit replay later". The manager chose the time limit over a CI run: it keeps rung 1 offline and adds no workflow step. | 8.2, 8.4 |
| C6 | The route page has a short "how to leave" section: uninstall the App, delete the two files. | 8.11 |
| C7 | A design task before 8.9 and 8.10: mockups of every state of the App setup page, the `archfit setup` output step by step, and every pull-request text (check titles, comments), in one vocabulary and the App's existing style. A private page; the owner reviews it before the App pages are built. | 8.14 |
| C8 | No separate 0.9.1 release. H1, H2, H3, #33 and the Go 1.27 note ship in 0.10.0. | section 3 |
| C10 | Owner and developer UX (owner, 2026-11-22, after the policy-editor round of PR #44): "these actions: reload, edit, approve — are not that obvious; I am not sure I would know what to do without AI working on these." Principles: every message says what is happening, why, what you do next (who) and what happens after; one next action with its owner; one story everywhere (Change → Check → Ask → Owner answers → Policy updated → Re-check passes); the App owns git mechanics; the effect is shown before the click; a small vocabulary (never in user-facing text: proposal, void, outside the flow, YAML edit, envelope, digest). | 8.18 to 8.23 |
| C9 | `archfit why` (8.12) and `setup --agent` (8.3) move to the end of the engine lane and may slip past 0.10.0. The 7-day digest comment is dropped: the setup page shows "this week: N checked, N quiet, N asked" next to "Make it required". No new infrastructure. | 8.3, 8.9, 8.10, 8.12 |

Owner decisions of 2026-11-22 on the open points:

| Id | Decision | Task |
| --- | --- | --- |
| D6 | The App passkey for "Make it required" in 0.10.0. | 8.9 |
| D6-G1 | The first-passkey enrolment gap is closed in 8.9; its proposal picks how and says why it stops an agent that drives a signed-in browser. | 8.9 |
| D6-G2 | The amended O1 listing is enough for the beta. | 8.9 |
| D6-2b | Hardware-key signed policy commits: roadmap after 0.10.0, as an alternative to the passkey, no date. | after 0.10.0 |
| P1 | No new App permission in 0.10.0 (least privilege: no `actions: write`). 8.8 first tests on a scratch repository whether the App can re-request the setup pull request's Actions check suite with its existing permissions (`POST /repos/{o}/{r}/check-suites/{id}/rerequest` with `checks: write`, or the check-run re-request). If GitHub refuses that for an Actions suite, the App posts a comment on the setup pull request with the direct re-run link, and `setup`'s `next:` says the same. The finding is written into section 8. | 8.8, 8.2 |

Open, with their defaults:

| Id | Question | Needed by | Default |
| --- | --- | --- | --- |
| O4 | `setup` opens the pull request with `gh` when signed in, else prints the compare link. | 8.2 | As the design says. |
| O6 | The App setup page needs a GitHub sign-in in rung 2. | 8.9 | Yes, to show state. The check and the comment need none. |

## 3. Release and order

### 3.1 Release order

1. Engine tag `v0.10.0` through its release workflow: binaries, image, Homebrew tap.
2. Record the image digests and the engine commit in section 8.
3. Action: pin the engine image digests of that tag, push to `main`, tag `v0.10.0` through its release
   workflow. Record the commit.
4. Docs that name the Action commit (H3): merged after step 3, never before.
5. App: manifest entry for engine `v0.10.0` and the Action commit, next to the 0.9.0 entry. Merged with
   `RELEASE_ON_PUSH=false`, so the merge does not deploy. The App tag follows the shared number.
6. Deploy: started by the owner with `workflow_dispatch` (`mode=release`) after the exact commit, CI and
   rollback revisions are recorded (`gcloud run services describe`). The manager sends `NEED:` with the
   command.
7. Pins of the onboarded repositories (section 3.3).
8. Cold run (8.13).

### 3.2 The route of 0.10.0 (the cold run)

1. `brew install coupling-dev/tap/archfit` (on Linux: the release binary, named in the docs).
2. `archfit setup`: runs `init` and `replay --last 30` (at most 60 s), writes the workflow, opens the setup
   pull request (or prints the compare link), prints the App install link. The setup pull request shows a
   neutral check: "Install the App" (C1).
3. Install the App from that link. The App brings the check on the setup pull request up to date (P1): neutral,
   "merge to start" (C1, C2).
4. Merge the setup pull request. Advisory from the first merge; no sign-in, no passkey. `archfit setup`
   prints "rung 2: advisory" and the setup page.

### 3.3 Pins of the onboarded repositories

After the deploy: `coupling-dev/archfit` (the engine's own workflow), `archfit-4-mechanics`, `ccgram`, `pumba`.
Each pull request changes only the `uses:` line to the Action `v0.10.0` commit and its `# v0.10.0` comment.
A pin bump to a commit the App lists needs no passkey. The 0.9.0 manifest entry stays until all four have
moved; the cleanup that removes it is a later App change. The manager opens these pull requests through
workers, reads each diff, and asks archfit-76 before merging any of them.

## 4. Cold-run stumbles mapped to tasks

Numbers follow the cold-run report of 2026-11-21.

| # | Stumble (cold run) | Fixed by |
| --- | --- | --- |
| 0 | `init` writes `owners: [coupling-dev]`: an organization cannot approve the first policy. Refusal text names no fix. | H1 (merged, #36), H2 (merged, App #240), 8.9 (page shows the owner's next action) |
| 1 | Homebrew bootstrap failed on Linux arm64 (46 min, no `brew`). | 8.11 (Linux route is the release binary, one step) |
| 2 | Docs name no Linux asset. | 8.11 |
| 3 | Install route order puts the working route last. | 8.11 |
| 4 | `doctor` says hooks are inactive; docs say `init` installs them. | 8.2 (`setup` installs hooks); docs in 8.11 |
| 5 | `init` does not write the workflow; the Action README promises it. | H3 (docs stop the claim), 8.2 (`setup` writes it) |
| 6 | `check` with no `origin/HEAD` exits 2 `unknown`. | 8.2 (`setup` finds the default branch and passes it); 8.11 quick start |
| 7 | No App or Action setup link in any document. | 8.11, 8.9, `setup` prints the link |
| 8 | Version vocabulary and `checkout@v4` versus `@v7` disagree. | 8.11 (docs), 8.2 (the written workflow has no `checkout` step), 8.14 (one vocabulary) |
| 9 | Repository creation denied by the agent harness. | Not a product step. The owner creates the scratch repository (8.13). |
| 10 | A `pull_request`-only workflow ran as a `push` run with zero jobs and no log. | Cause found (8.2 proposal): the hand-written file had a double-quoted string inside `${{ }}`, so the file was invalid. `setup` writes a fixed workflow with no expression, checked by `actionlint`. |
| 11 | The Action commit is not in the docs. | H3 (exact commit and `git ls-remote`), 8.2 (`setup` writes the pin) |
| 12 | Policy and workflow read as two approvals. | 8.9 (one approval covers both, on the page and in the gate text), 8.14 (wording) |
| 13 | Merge through the crew-net gate (harness). | Not a product step. 8.11 says to merge with GitHub's button. |
| 14 | A fail verdict is `neutral` and a forbidden import can still merge. | Advisory stays non-blocking by design (D2). The block is rung 3: 8.6 (ruleset file), 8.9 ("Make it required"). |
| 15 | Process notes (`safe.directory`, a safety hook on a scratch repo). | Not product. |
| new | The setup pull request's first run fails before the App is installed. | 8.15 (neutral, "Install the App"), 8.8 (re-request on installation, or a comment with the re-run link) |

Deferred with reason: the report text prints a bare balance (report-model change), and the App owner-review
page diffs a stale branch. Both stay in the deferred list of the beta plan, section 8.

## 5. Contract changes and what must not break

| Item | 0.10.0 change | Compatibility rule |
| --- | --- | --- |
| Policy file | None. `setup` writes version 1 only. | `setup` never overwrites a version 1 file. O5: a policy written by `init` before `setup` is kept. |
| `archfit.result.v1` | None (design section 6). | A 0.9.0 result decodes in the 0.10.0 App with the same verdict. Tested in 8.7. |
| App manifest | Entries of 0.9.0 and 0.10.0 side by side. | The manifest reader takes several entries; checked in the App manifest step of 8.13. |
| Action outcome without an App installation | Neutral with an install link, not a failed job (8.15). | Only the "no installation" refusal changes. Every other refusal still fails the job. |
| App permission | None new (P1). | The re-request uses the existing `checks: write`; if GitHub refuses it, the comment with the re-run link needs only the existing pull-request comment permission. |
| Workflow line | `uses: coupling-dev/archfit-action@<40-hex> # vX`. | `doctor` reads `# vX` with no network. A missing comment is "cannot tell", never a mismatch. |
| `setup`, `replay`, `why` output | Text only. | A public JSON form would need a schema and a golden contract (repository `CLAUDE.md`). None is planned. |
| `doctor` output | Owner-without-person check (H1, merged). Text only. | `doctor` exits non-zero for an owners list with no person: a fix, since such a policy can never be approved. |
| Ruleset file (8.6) | New, a GitHub import file. | Not an archfit contract. Requires `archfit/gate`. |
| App gate before the first approval | Advisory verdicts, and a neutral "merge to start" check on the setup pull request (8.7). | Same results, App treatment only. `gate: required` keeps the first-approval rule. |

## 6. Tasks

Agent: `pi` (Pi worker, `router/openai-personal`), `claude` (Claude worker). Budget (archfit-76, 2026-11-22):
at most two Claude workers at a time, and only for design or contract work.

### 6.1 Carried from the hotfix

- [x] **H1 Engine: `init` names a person.** Merged as #36 (`11671833`).
- [x] **H2 App: refusal names the fix.** Merged as App #240 (`e693636f`), not deployed; ships with the 0.10.0
      deploy.
- [ ] **H3 Docs: the Action commit.** The engine and Action docs give the exact Action commit of `v0.10.0`
      and how to find it: `git ls-remote https://github.com/coupling-dev/archfit-action 'refs/tags/v0.10.0^{}'`.
      The Action README stops claiming that `init` writes the workflow (that part may merge now). The commit
      lines merge after the Action tag. Agent: `pi`.

H5 to H8 (0.9.1 tags, manifest, deploy, pins) are dropped (C8).

### 6.2 Epic 8

Gate before the build of 8.2 to 8.15: this revision merged. Proposals may start before that.

Engine:

- [x] **8.2 `archfit setup`.** The state machine of design section 4.2, built from the approved proposal
      (team report `b10-setup-design-plan.md`, its section 10). Plus C4 (`workflow` scope check before the
      push) and C5 (replay limited to 60 s). The engine re-run of a failed job is dropped: the App re-runs
      (C1). Its texts follow 8.14. Agent: `pi`. Engine #55 (`d0b3b38c`) and #60 (`b3e409a1`).
- [x] **8.4 `archfit replay --last N`.** First-parent history of the default branch; `gh` only labels changes.
      Text output. Accepts a deadline and reports a partial count when it runs out (C5). Agent: `pi`. Engine #59 (`ea0de577`).
- [x] **8.5 Open decisions at the end of `init`.** Printed by `init` and written in the report; never on a
      pull request. After 8.2 (same file). Agent: `pi`. Engine #58 (`4986fefa`).
- [x] **8.6 Ruleset file.** A ruleset JSON for GitHub's import and the `gh api` line, requiring
      `archfit/gate`. The setup page shows it at "Make it required". Agent: `pi`. Engine #45 (`fff5a4a7`); App #268 (`84da8d13`).
- [ ] **8.3 `archfit setup --agent`.** Owner-approved slip past 0.10.0 (C9); not complete. Agent: `pi`.
- [ ] **8.12 `archfit why <a> <b>`.** Owner-approved slip past 0.10.0 (C9); not complete. Agent: `pi`.

Action:

- [x] **8.15 Neutral before installation (C1).** When the upload is refused because the App is not installed
      on the repository, the job ends neutral and its summary says "Install the App: <link>", the link
      pre-filled as far as 8.8 finds possible. Every other refusal still fails. Agent: `pi`. Action #5 (`ffcb5088`).

App:

- [x] **8.7 Advisory first (D2, C2).** Advisory verdicts before the first approval; the setup pull request
      gets a neutral "merge to start" check, never `action_required`, never a passkey wait. The existing
      advisory mapping (pass → success, fail → neutral). A display-only record of rung-2 policy changes for
      8.9. Proposal approved. Agent: `pi`, merged by the manager. App #241 (`5cc3cf40`) and #247 (`41e485d1`).
- [x] **8.8 Install and admission (C1, C3).** A partner's account is admitted before the invite is sent. The
      App brings the check on the open setup pull request up to date when it is installed: first a test on a
      scratch repository of the check-suite re-request with the existing permissions; if GitHub refuses it, a
      comment with the direct re-run link (P1; no `actions: write`). One clear message each
      for: account not admitted; installer is not an organization administrator (who to ask, what to send).
      Verify whether GitHub's `installations/new` can pre-select the account and the repository, and use it
      if so. Owner-side admin steps are `NEED:` items. Agent: `pi`. App #243 (`41549a11`) and #244 (`d0fb365f`); live re-request and install preselection remain 8.13 checks.
- [x] **8.14 Mockups and one vocabulary (C7).** Every state of the App setup page, the `archfit setup` output
      step by step, and every pull-request text (check titles and summaries, comments), in one vocabulary
      and the App's existing style. A private page; `NEED:` to archfit-76 for the owner's review. Gates 8.9
      and 8.10; 8.2, 8.7 and 8.15 texts are aligned to it before their merge. Agent: `claude` (design). Engine #39 (`ffc5406d`), owner accepted.
- [x] **8.9 One setup page.** After 8.14 is reviewed. D6 is decided: the App passkey, and the first-passkey
      enrolment gap (G1) is closed here; the proposal picks how. One repository's state and one next
      action; never asks to copy a workflow. The last action "Make it required" carries D6, the first
      approval and the ruleset (8.6), and next to it "this week: N checked, N quiet, N asked" (C9). Before
      the first approval it lists every policy change since the setup pull request, with who merged it and
      its request (O1 amended). One approval covers the policy and the workflow. Agent: `pi`. App #267 (`3ccf8dc7`) and #268 (`84da8d13`), with prior slices; see team status for history limitations.
- [x] **8.10 Welcome comment.** A comment on the setup pull request: how to ask, what happens next. No digest
      comment (C9). After 8.14. Agent: `pi`. App #246 (`7cc55746`).
- [x] **8.16 Request duplicates.** A surface request is matched by its module edge and target package, so a
      question about another package on the same edge is not swallowed (engine request #46). App #242 (`ecf7c59a`).
- [ ] **8.17 Policy editor fixes.** Absorbed by 8.20 (the editor stays an advanced tool off the normal path):
      refuse an edit equal to the open policy pull request, Preview against it, a refused merge shown as a
      warning, a Reload that would drop lines says so first.

UX (C10; owner decision 2026-11-22):

- [ ] **8.18 Story and message spec.** The life of a change across terminal, pull request and App; the
      four-part message rule; the vocabulary list; a test in the engine, the Action and the App that fails on
      a banned word in user-facing text. Mockups (extends 8.14) for the owner's review. First of the UX tasks.
      Done when the owner accepts the mockups and the vocabulary test runs in all three repositories.
      Agent: `claude` (design), then `pi` for the three vocabulary tests. Spec: engine #52 (`1189f457`), owner accepted. Completion of all vocabulary coverage is not established here.
- [ ] **8.19 Answer pages.** One App page per request: the question, the evidence (code line, dependency), three
      or four answers as large buttons (allow, allow until a date, make it public, reject with `instead`), the
      effect of each before the click, one passkey; batch answers ("3 requests waiting"); undo ("Revert this
      answer"). Replaces the editor on the normal path. Done when an end-to-end test answers a request and the
      pull request passes with no editor and no Reload. After 8.18. Agent: `claude` (design), `pi` slices.
- [ ] **8.20 The App owns git.** The App keeps its policy pull request up to date with the default branch
      (rebuilds it on the tip with the same bytes, App commits only) and merges it itself when the required
      checks are green; an approval binds the policy bytes, so a default-branch move does not void it; if it
      must be voided, the page says why and offers one re-approve. Absorbs 8.17. Done when an end-to-end test
      moves main between the owner's approval and the merge and the owner does nothing extra. Agent: `claude`. App #245 (`8b9c36ca`); the stated end-to-end completion criterion remains unverified here.
- [x] **8.21 `archfit status`.** The story row and the next action (who, link, what happens after) in the
      terminal, for people and agents; the hooks and `archfit request` use the same lines. Tests per state;
      docs. After 8.18. Agent: `pi` (step-by-step brief). Engine #53 (`244d9e4a`).
- [x] **8.22 Notifications.** The App @-mentions the owner who must act on GitHub; the "Needs you" list shows
      the story row. Tests; mockup state. After 8.18. Agent: `pi`. App #253 (`c828ace3`) and #256 (`05c6e6d2`): primary Home list. Repos story rows are deferred: the owner-actions payload lacks a request id. The moving-on-its-own group is deferred by `briefs/b11-home-story.md`; live GitHub notification behavior remains unverified.
- [ ] **8.23 Human usability test.** Before the release, the owner (and ideally one person without AI) runs a
      scripted task list: set up, ask, answer, see the pull request pass, with no help; minutes and every
      confusion recorded in section 8. Release gate: no step where the tester had to ask what to do. HUMAN.

Docs:

- [x] **8.11 One route page.** "Try it on your pull request": the Pumba story, the route of section 3.2, two
      expected results, the fork limit, and "how to leave" (C6). The engine guide, the Action README and the
      App guide link to it and do not repeat it. Fixes stumbles 1, 2, 3, 7, 8 in the docs. Agent: `pi`. Engine #67 (`81cde1e5`), App #266 (`6fb209df`), Action #11 (`3cfd9696`).

Release:

- [ ] **8.13 Release 0.10.0 and cold run.** Section 3.1 in order: release notes (with H1, H2, #33, the Go
      1.27 note), engine tag, Action pin and tag, H3 commit lines, App manifest, deploy (`NEED:`), the four
      pins, then the cold run on a new scratch repository in `coupling-dev` (the owner creates it: `NEED:`
      with the exact `gh repo create` command) in a clean container, with the method of task 7.3. Minutes
      and steps in section 8. CONFIRM.

Gate for epic 8 done: 8.2, 8.4 to 8.11, 8.14 to 8.22 merged (8.3 and 8.12 if they fit); 8.23 passed; the
cold run under 10 minutes and at most 4 steps; every stumble of section 4 fixed or deferred with its reason; the
four pins on 0.10.0.

## 7. Order of work and lanes

- Now: 8.18 (Claude) first among the UX tasks; its mockups go to the owner; 8.20 (Claude) in parallel.
- Engine lane, one worker at a time where files overlap: 8.2, then 8.5; 8.4 in parallel in its own packages;
  8.6; then 8.3 and 8.12.
- Action lane: 8.15, then the pin at release.
- App lane: 8.7, 8.8, 8.16 (done or in review); 8.20 now; 8.19, 8.9, 8.10 and 8.22 after 8.18's mockups are accepted.
- Engine UX: 8.21 after 8.18.
- Docs lane: 8.11 after 8.2 and 8.9 have their texts.
- Then 8.23 (the owner), then 8.13.

No two workers edit the same files. At most 6 workers; at most 2 Claude workers.

## 8. Record

Filled as tasks finish. A later task reads its inputs here.

| Item | Value | Task |
| --- | --- | --- |
| H1, H2 | Engine #36 (`11671833`); App #240 (`e693636f`), not deployed. A 0.9.1 release-notes pull request (#37) was closed when 0.9.1 was dropped. | H1, H2 |
| D6 | Decided 2026-11-22: the App passkey; G1 closed in 8.9; G2 the O1 listing is enough; 2b on the roadmap as an alternative. Recorded in the threat model, section 7. | 8.1 |
| P1 | Decided 2026-11-22: no new permission. Re-request test result: (pending) | 8.8 |
| 8.14 review | Accepted 2026-11-22 with three answers (beta-access issue form; advisory `unknown` neutral only for verified reports; "passed", "needed an owner"). Engine #39. | 8.14 |
| Policy lines for setup and replay | Engine #44 (App policy pull request) admin-merged by the owner as `01a5eba7` (setup public, plus the #43 allowance); consumed in flow. The App could not bring its branch up to date (8.20). The replay line follows through the owner path of 8.19/8.20, or one verified editor step. | 8.2, 8.4 |
| Setup and replay port | Engine #55 (`d0b3b38c`), #60 (`b3e409a1`), merged. | 8.2 |
| Replay surface and command | Engine #57 (`d13feb6c`), #59 (`ea0de577`), merged. | 8.4 |
| Init open decisions | Engine #58 (`4986fefa`), merged. | 8.5 |
| Ruleset | Engine #45 (`fff5a4a7`), merged; import awaits cold run. | 8.6 |
| Check story | App #249 (`6e5faf5d`), merged; partial implementation, task gates remain. | 8.22 |
| Asker line and records-only batch | App #250 (`cce36846`), #254 (`f9d0bbb6`), merged; partial implementation, end-to-end gate remains. | 8.19 |
| Usability test | (pending) | 8.23 |
| Engine `v0.10.0`: commit, tag object, release run, tap commit, image digests | (pending) | 8.13 |
| Action `v0.10.0` commit and tag object | (pending) | 8.13 |
| App deploy. Live now: `archfit-public-r44923382504-1`, `archfit-worker-r44923382504-1`. Rollback target: the live revisions at the time of the deploy, recorded then. | (pending) | 8.13 |
| Pins on 0.10.0 of `archfit`, `archfit-4-mechanics`, `ccgram`, `pumba` | (pending) | 8.13 |
| Cold run 0.10.0: minutes, steps, scratch repository, container | (pending) | 8.13 |
| Deferred past 0.10.0, with reason | (pending) | 8.13 |
