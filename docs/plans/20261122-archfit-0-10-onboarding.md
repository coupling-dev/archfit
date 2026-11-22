# Archfit 0.9.1 and 0.10.0: onboarding

Status: proposed, 2026-11-22. Owner: Alexei Ledenev. Plan owner: archfit-76. Team: b10 (manager
`b10-manager`).

Sources:

- The beta plan, `docs/plans/20261120-archfit-0-9-new-home-and-beta.md`: epic 8, section 8 (record).
- The onboarding design, `docs/plans/20261120-archfit-onboarding-design.md`, accepted 2026-11-20
  with open point O1 amended. Sections 2, 4, 5 and 6 are used here.
- The cold run of 0.9.0 (task 0.4 terminal part and task 7.3), 2026-11-21: 1 h 44 min to the first
  verdict, 9 stranger steps to the first pass. Its stumbles are listed in section 4 of this plan.

This plan covers two releases. **0.9.1** is a hotfix for the three stumbles that stop a partner
before 0.10.0. **0.10.0** is epic 8: a stranger reaches a verdict in under ten minutes and in at most
four steps.

## 1. Outcome

1. A repository set up with 0.9.1 never has an organization as its only owner. The docs give the
   exact Action commit of each release, and say how to find it.
2. A stranger, on the route of section 3.2 and using only the published documents, reaches a verdict
   on a pull request in under ten minutes and in at most four steps. The cold run of task 8.13
   measures it on a new scratch repository.
3. 0.9.0 stays accepted by the App until the four onboarded repositories have moved their pin. No
   0.10.0 change breaks a 0.9.x result (section 5).

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

Open, for the owner (archfit-76 asks):

| Id | Question | Needed by | Default |
| --- | --- | --- | --- |
| D6 | Approval option for "Make it required" (8.1 threat model). | 8.9 | Recommendation of 8.1; the owner decides. |
| O2 | `expires` date `init` writes. | 3.4 (done in 0.9.0) | 90 days. |
| O4 | `setup` opens the pull request with `gh` when signed in, else prints the compare link. | 8.2 | As the design says. |
| O6 | The App setup page needs a GitHub sign-in in rung 2. | 8.9 | Yes, to show state. The check and the comment need none. |
| Infra | Any new GCP resource for the 7-day digest (8.10): a scheduler or a job. | 8.10 | Owner approves the resource before it is created. |

## 3. Releases and their order

### 3.1 Release order, both releases

The same order as 0.9.0:

1. Engine tag (`vX.Y.Z`) through its release workflow: binaries, image, Homebrew tap.
2. Record the image digests and the engine commit in section 8.
3. Action: pin the engine image digests of that tag, push to `main`, tag `vX.Y.Z` through its release
   workflow. Record the commit.
4. Docs that name the Action commit: merged after step 3, never before (the commit does not exist
   before).
5. App: manifest entry for the engine tag and the Action commit, next to the older entries. The App
   tag follows the shared release number. Merged with `RELEASE_ON_PUSH=false`, so the merge does not
   deploy.
6. Deploy: started by the owner with `workflow_dispatch` (`mode=release`) after the exact commit, CI
   and rollback revisions are recorded (`gcloud run services describe`). The manager sends `NEED:`
   with the command.
7. Pins of the onboarded repositories (section 3.3). One pull request per repository.

### 3.2 The route of 0.10.0 (the cold run)

1. `brew install coupling-dev/tap/archfit` (on Linux: the release binary, named in the docs).
2. `archfit setup`: runs `init` and `replay --last 30`, writes the workflow, opens the setup pull
   request (or prints the compare link), prints the App install link.
3. Install the App from that link. Advisory from the first merge; no sign-in.
4. Merge the setup pull request. `setup` prints "rung 2: advisory" and the setup page.

### 3.3 Pins of the onboarded repositories

For 0.9.1 and again for 0.10.0: `coupling-dev/archfit` (engine's own workflow), `archfit-4-mechanics`,
`ccgram`, `pumba`. Each pull request changes only the `uses:` line to the new Action commit (and its
`# vX` comment). A pin bump to a commit the App lists needs no passkey. The 0.9.0 manifest entry stays
until all four have moved; removing it is part of the 0.10.0 cleanup (task H7 and 8.13).

Rule: the manager opens these pull requests through workers, reads each diff, and asks archfit-76
before merging any of them.

## 4. Cold-run stumbles mapped to tasks

Numbers follow the cold-run report of 2026-11-21.

| # | Stumble (cold run) | Fixed by |
| --- | --- | --- |
| 0 | `init` writes `owners: [coupling-dev]`: an organization cannot approve the first policy. Refusal text names no fix. | H1 (engine), H2 (App refusal text), 8.9 (page shows the owner's next action) |
| 1 | Homebrew bootstrap failed on Linux arm64 (46 min, no `brew`). | 8.11 (Linux route is the release binary, one step; no bootstrap in the route) |
| 2 | Docs name no Linux asset. | 8.11 |
| 3 | Install route order puts the working route last. | 8.11 |
| 4 | `doctor` says hooks are inactive; docs say `init` installs them. | 8.2 (`setup` installs hooks), 8.5 (`init` output); docs in 8.11 |
| 5 | `init` does not write the workflow; the Action README promises it. | H3 (docs stop the claim), 8.2 (`setup` writes it) |
| 6 | `check` with no `origin/HEAD` exits 2 `unknown`. | 8.2 (`setup` runs the check with the target branch); 8.11 quick start |
| 7 | No App or Action setup link in any document. | 8.11 (one route page), 8.9 (setup page), `setup` prints the link |
| 8 | Version vocabulary and `checkout@v4` versus `@v7` disagree. | 8.11 (docs), 8.2 (one `checkout` version in the written workflow) |
| 9 | Repository creation denied by the agent harness. | Not a product step. Scratch repository created by the owner (section 6). |
| 10 | A `pull_request`-only workflow ran as a `push` run with zero jobs and no log. | 8.2: investigate on the scratch repository; if the cause is the workflow file, fix the written workflow; the 8.13 cold run must show no zero-job run. |
| 11 | The Action commit is not in the docs. | H3 (0.9.1: exact commit and `git ls-remote`), 8.2 (`setup` writes the pin) |
| 12 | Policy and workflow need two approvals as the record reads. | 8.9 (one approval covers both, said on the page and in the gate text) |
| 13 | Merge through the crew-net gate (harness). | Not a product step. 8.11 says to merge with GitHub's button. |
| 14 | A fail verdict is `neutral` and a forbidden import can still merge. | Advisory stays neutral by design (D2: it never blocks). The block is rung 3: 8.6 (ruleset file), 8.9 ("Make it required" as the last action). |
| 15 | Process notes (`safe.directory`, a safety hook on a scratch repo). | Not product. |

Deferred with reason (none yet): the report text prints a bare balance (report-model change), and the
App owner-review page diffs a stale branch. Both stay in the deferred list of the beta plan, section 8.

## 5. Contract changes and what must not break

| Item | 0.10.0 change | Compatibility rule |
| --- | --- | --- |
| Policy file | None. `setup` writes version 1 only. | `setup` never overwrites a version 1 file (design 4.2). O5: a policy written by `init` before `setup` is kept. |
| `archfit.result.v1` | None (design section 6). | A 0.9.x result decodes in the 0.10.0 App. Test with stored 0.9.0 and 0.9.1 results before 8.7 merges. |
| App manifest | Entries of 0.9.x and 0.10.x side by side. | Verify in task H7 that the manifest reader takes several entries; if not, the fix is part of H7, not 8.13. |
| Workflow line | `uses: coupling-dev/archfit-action@<40-hex> # vX`. | `doctor` reads `# vX` with no network. A missing comment is "cannot tell", never a mismatch. |
| `archfit setup`, `replay`, `why` JSON output, if any | New. | A public JSON form needs a schema and a golden contract (repository `CLAUDE.md`). The 8.2 and 8.4 proposals say whether they add one. |
| `doctor` output | Owner-without-person check (H1). | Text output only unless a JSON form exists; then schema and golden contract as above. |
| Ruleset file (8.6) | New, a GitHub import file. | Not an archfit contract. Generated from the policy's required check name; checked against `archfit/gate`. |
| App gate on first approval | Advisory state for an unapproved policy (8.7). | Same result, App treatment only. A 0.9.x App keeps `first_approval` until its own release. |

Contract work is design-heavy: 8.2 and 8.4 proposals and any new result field go to the manager's
`READY:` review, and a `contract` change sets `Merge: manager`.

## 6. Tasks

Lanes: **H** = 0.9.1 hotfix, **8.x** = epic 8. Agent: `pi` (Pi worker, `router/openai-personal`),
`claude` (Claude worker). Budget (archfit-76, 2026-11-22): at most two Claude workers at a time, and
only for design or contract work.

### 6.1 Hotfix 0.9.1

- [ ] **H1 Engine: `init` names a person.** `init` never writes an organization, or any non-person,
      as the only owner. Order: the authenticated `gh` user; else the git identity mapped to a GitHub
      login if one exists; else a placeholder, and `next:` tells the user to put their GitHub login in
      `owners`. `doctor` reports an owners list with no person. Done when: table-driven tests cover
      org-only, person, placeholder and no-`gh` cases; `doctor` test covers the org-only list.
      Repository docs for `init` updated. Agent: `pi`, merged by the worker. Repository: `archfit`.
- [ ] **H2 App: refusal names the fix.** The first-approval refusal text says what to do: name a
      person in `owners` and push. Done when: App test covers the text; `make all` green. Agent: `pi`.
      Repository: `archfit-app`. Deploy: with H7.
- [ ] **H3 Docs: the Action commit.** The engine and Action docs give the exact Action commit for
      the release and how to find it: `git ls-remote https://github.com/coupling-dev/archfit-action
      'refs/tags/v0.9.1^{}'`. The Action README stops claiming that `init` writes the workflow. Merged
      after H6's tag. Agent: `pi`. Repositories: `archfit` (getting started, CI, commands), `archfit-action`.
- [ ] **H4 Ship what is on `main`.** The archfit 1.x policy error (#33) and the Go 1.27 note are
      already on `main` since 0.9.0; they ride in 0.9.1. Done when the release notes say so. Agent: manager (release notes in the engine release PR).
- [ ] **H5 Engine release 0.9.1.** Tag `v0.9.1` through the release workflow after H1 and H4 merge.
      Record the commit, tag object, image digests and tap commit in section 8. Agent: manager (tag is
      a one-way step; reported to archfit-76 with rollback = a new patch, never a moved tag).
- [ ] **H6 Action 0.9.1.** Pin the engine v0.9.1 image digests; `ci.yaml` green; tag `v0.9.1`. Record
      the commit. Agent: `pi`, merge by worker; the tag by the manager.
- [ ] **H7 App manifest 0.9.1.** Add the engine v0.9.1 and Action v0.9.1 entries next to 0.9.0. Verify
      the manifest reader takes several entries. Merged with `RELEASE_ON_PUSH=false`. Deploy: NEED to
      archfit-76 with the exact command. Agent: `pi`, merge by manager.
- [ ] **H8 Pins of the four repositories to 0.9.1.** One pull request each, only the `uses:` line
      (section 3.3). Start after H7 is deployed. Agent: `pi`, one worker per repository, merge after
      archfit-76 agrees.

Gate for 0.9.1 done: H1 to H8 merged; the four pins on 0.9.1; 0.9.0 still accepted until the last
pin merges; section 8 filled.

### 6.2 Epic 8: onboarding (0.10.0)

Gate before the build of 8.2 to 8.12 starts: this plan accepted by the owner; 0.9.1 released (H5, H6 at least).
Proposals (8.1, and the 8.2 and 8.4 proposals) may start before that.

- [ ] **8.1 Threat model and approval options (D6).** One page in `docs/plans/`, no code: what an
      approval must prove and against whom: a coding agent holding the developer's GitHub token, a
      developer who is not an owner, a stolen laptop. Options: the passkey in the App (today); a signed
      policy commit checked against the owners' public keys (with and without a hardware key with
      touch); a GitHub code-owner review; a GitHub environment approval. Recommend one; the owner
      decides. Agent: `claude` (design). HUMAN decision.
- [ ] **8.2 `archfit setup`.** One resumable command, the state machine of design section 4.2. Writes
      the workflow with the Action commit of its own version (`git ls-remote`), installs hooks, opens
      the setup pull request (`gh`, else compare link), prints the App links, prints one `next:`. A
      second run never repeats a done step. Its proposal goes first (`claude`, design); the build is
      `pi` after approval (budget). Fixes stumbles 4, 5, 6, 8, 10, 11 (section 4). Tests use a stubbed
      `git ls-remote`; the real pin is checked in 8.13.
- [ ] **8.3 `archfit setup --agent`.** Prints the runbook for a coding agent, matched to the installed
      version (design 4.2 "The agent runbook"). The prompt a user copies is one line. Depends on 8.2.
      Agent: `pi`.
- [ ] **8.4 `archfit replay --last N`.** The policy run over merged pull requests: how many stay quiet,
      how many would have asked an owner. Output feeds the setup pull request body. Proposal says
      whether it adds a JSON form (section 5). Agent: `pi` after a short proposal reviewed by the manager.
- [ ] **8.5 Open decisions at the end of `init`.** What the policy leaves undecided, printed by `init`
      and written in the report; never on a pull request (design 5.2). Depends on H1 (same file).
      Agent: `pi`.
- [ ] **8.6 Ruleset file.** A ruleset JSON for GitHub's import and the `gh api` line, requiring
      `archfit/gate`. `setup` prints it at "Make it required". Agent: `pi`.
- [ ] **8.7 Advisory first (App).** With `gate: advisory` (or no key) the App comments and shows a neutral
      check from the first merge of the setup pull request. No sign-in, no passkey. Done when: 0.9.0 and
      0.9.1 stored results still decode and give the same verdicts. Agent: `pi`, manager merges (App
      treatment on a shared result).
- [ ] **8.8 Admission by invite (App).** A partner's account is admitted before the invite is sent, so
      the install never waits for the operator. Owner-side admin steps are NEED items. Agent: `pi`.
- [ ] **8.9 One setup page (App).** Shows one repository's state and one next action. Never asks the user to
      copy a workflow by hand. The last action "Make it required" carries D6, the first approval and the
      ruleset. Before the first approval it lists every policy change since the setup pull request, with
      who merged it and its request (O1, amended). One approval covers the policy and the workflow, said
      on the page and in the gate text (cold-run item 12). Depends on 8.1 (D6) and 8.6. Agent: `pi`.
- [ ] **8.10 Welcome and digest (App).** A comment on the setup pull request: how to ask, what happens next.
      After seven days: checked, quiet, asked. Any scheduler or job is a new GCP resource: NEED first.
      Agent: `pi`.
- [ ] **8.11 One route page.** "Try it on your pull request": the Pumba story, the route of section 3.2,
      two expected results, the fork limit. The engine guide, the Action README and the App guide link to
      it and do not repeat it. Also fixes stumbles 1, 2, 3, 7, 8 in the docs. Agent: `pi`, last among docs.
- [ ] **8.12 `archfit why <a> <b>`.** Who approved a dependency, when, and the reason, read from the history of
      the policy file (`# request #N` lines). Agent: `pi`.
- [ ] **8.13 Release 0.10.0 and cold run.** Engine tag, Action tag (both by section 3.1), App manifest and
      deploy (NEED), pins of the four repositories (section 3.3). Then the cold run on a new scratch
      repository in `coupling-dev` with the method of task 7.3 in a clean container: minutes and steps
      recorded in section 8. CONFIRM. Owner creates the scratch repository (NEED).

Gate for epic 8 done: 8.1 to 8.12 merged; the cold run under 10 minutes and at most 4 steps; every
stumble of section 4 fixed or deferred with its reason; four pins on 0.10.0 or 0.9.1 with the 0.9.0
entry removed in the release PR.

## 7. Order of work and lanes

- Wave 1, now: H1 (`pi`), H2 (`pi`), 8.1 (`claude`), and the plan acceptance. H3 text is prepared in the
  same worker as H6 and merged after the tag.
- Wave 2, after H5 and H6: H3 merge, H7, then H8 per repository.
- Wave 3, after 8.2's proposal is approved: 8.2 build, 8.5, 8.6 (engine lane: one engine worker at a time
  where files overlap; the manager checks file overlap in each brief).
- Wave 4: 8.3, 8.4, 8.12 (engine); 8.7, 8.8, 8.9, 8.10 (App, after 8.1 and D6).
- Wave 5: 8.11 (docs), then 8.13.

No two workers edit the same files. At most 6 workers; at most 2 Claude workers.

## 8. Record

Filled as tasks finish. A later task reads its inputs here.

| Item | Value | Task |
| --- | --- | --- |
| Engine `v0.9.1` commit, tag object, release run, tap commit, image digests | (pending) | H5 |
| Action `v0.9.1` commit and tag object | (pending) | H6 |
| App manifest entry 0.9.1 and deploy revisions. Live now: `archfit-public-r44923382504-1`, `archfit-worker-r44923382504-1`. Rollback target (0.9.0): `archfit-public-r44906507642-1`, `archfit-worker-r44906507642-1`. | (pending) | H7 |
| Pins on 0.9.1 of `archfit`, `archfit-4-mechanics`, `ccgram`, `pumba` | (pending) | H8 |
| Epic 8 decisions: D6 | (pending, owner) | 8.1 |
| Engine `v0.10.0`, Action `v0.10.0`, App deploy, rollback revisions | (pending) | 8.13 |
| Cold run 0.10.0: minutes, steps, scratch repository, container | (pending) | 8.13 |
| Deferred past 0.10.0, with reason | (pending) | 8.13 |
