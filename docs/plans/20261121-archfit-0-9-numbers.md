# Archfit 0.9.0: the numbers against the v5.0.0 baseline

Status: evidence for task 3.7 of the [beta plan](20261120-archfit-0-9-new-home-and-beta.md),
2026-11-21. It gates the `v0.9.0` tag (task 3.8).

Engine `8f04ffe4` (`main`, all fixes of epic 3) is compared with the v5.0.0 baseline of task 0.2
on the four onboarded repositories. Each difference in the JSON of `check` and `report` has a
named cause. Nothing is left unexplained. The seam table for archfit is in the
[appendix](20261121-archfit-0-9-numbers-seams.md).

## 1. Result

| Repository | Leg | v5.0.0 (exit) | 0.9.0 (exit) | Fields | A | B | B+C | B-derived | V1-fixture | V1-base | R | M |
| --- | --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| archfit | head | pass (0) | pass (0) | 7 | | | | | 4 | | 1 | 2 |
| archfit | base | pass (0) | unknown (2) | 15 | | | | | 3 | 9 | 1 | 2 |
| archfit | base-v1 | pass (0) | pass (0) | 6 | | | | | 3 | | 1 | 2 |
| archfit | report | drift (0) | drift (0) | 452 | 237 | 28 | 7 | 118 | 60 | | 1 | 1 |
| archfit-4-mechanics | head | pass (0) | pass (0) | 7 | | | | | 4 | | 1 | 2 |
| archfit-4-mechanics | base | pass (0) | unknown (2) | 20 | | | | | 8 | 9 | 1 | 2 |
| archfit-4-mechanics | base-v1 | pass (0) | pass (0) | 7 | | | | | 4 | | 1 | 2 |
| archfit-4-mechanics | report | clean (0) | clean (0) | 12 | | | | | 10 | | 1 | 1 |
| ccgram | head | pass (0) | pass (0) | 7 | | | | | 4 | | 1 | 2 |
| ccgram | base | pass (0) | unknown (2) | 16 | | | | | 3 | 10 | 1 | 2 |
| ccgram | base-policy | pass (0) | unknown (2) | text | | | | | | status | | |
| ccgram | base-v1 | pass (0) | unknown (2) | not comparable | | | | | | | | |
| ccgram | report | clean (0) | clean (0) | 83 | | | | | 81 | | 1 | 1 |
| pumba | head | pass (0) | pass (0) | 7 | | | | | 4 | | 1 | 2 |
| pumba | base | pass (0) | unknown (2) | 16 | | | | | 3 | 10 | 1 | 2 |
| pumba | base-policy | pass (0) | unknown (2) | text | | | | | | status | | |
| pumba | base-v1 | pass (0) | unknown (2) | not comparable | | | | | | | | |
| pumba | report | clean (0) | clean (0) | 50 | 21 | | | | 27 | | 1 | 1 |
| **Total** | | | | **705** | **258** | **28** | **7** | **118** | **218** | **38** | **14** | **24** |

"Fields" counts the normalized JSON fields that differ (section 3). The cause columns add up to
it in every row. The text and the JSON of each leg exit with the same code.

In short:

- Every head check still passes. Every report keeps its state: archfit `drift` with the same 2
  errors, the other three `clean`.
- Every base check with the recorded command changes from `pass` to `unknown` (exit 2). Cause:
  0.9.0 reads only policy version 1 (section 5). This is the one verdict that changes.
- The report scores of archfit and pumba change only by task 3.1: 46 seams lose the `model` raise
  from a Go type hint (A), 7 seams become `intrusive` (B, B+C). Findings and metrics follow from
  those 7 seams.

## 2. What was compared

| Item | Value |
| --- | --- |
| Baseline | `~/Workspace/archfit.reset-evidence/baseline/`: v5.0.0 (`d5a6f8d`) outputs at archfit-4-mechanics `1f95386e`, archfit `301c6b23`, ccgram `c672b31f`, pumba `23968548`, taken 2026-11-20. |
| New engine | `8f04ffe4`, built from `git archive` with `GOWORK=off go build`. A local build reports engine `dev`. |
| Input of the new engine | `rc1-work/<repo>` branch `rc1`: the baseline commit plus one commit that changes only `version: 3` to `version: 1` in `.archfit.yaml`. The harness checks both facts. |
| Control | The same v5.0.0 binary, run again on the baseline commits. It reproduces every recorded JSON output except `date`, so the host and its tools did not change. |
| Raw outputs | `~/Workspace/archfit.reset-evidence/v0.9.0/` (README, `run.env`, `new/`, `control/`, `analysis/`). |
| Harness | `run.sh`, `norm.py`, `analyze.py` in `~/Workspace/archfit-beta-team/reports/b9-numbers/`, with a copy in `v0.9.0/harness/`. |

Legs, the same commands as the baseline:

| Leg | Command | Baseline file |
| --- | --- | --- |
| head | `archfit check` | `check-head.json` |
| base | `archfit check --base $(cat check-base)` | `check-20.json` |
| base-policy | `archfit check --base $(cat check-base-policy)` (ccgram, pumba) | `check-policy.txt` (text only) |
| base-v1 | `archfit check --base base-v1`, the migrated-base fixture of `rc1-work` | `check-20.json` |
| report | `archfit report` | `report.json` |

Each leg ran with `--format json` and `--format text`.

## 3. Method

1. **Normalize.** Every array of objects becomes a map keyed by a stable id: seams by
   `from -> to`, findings by rule and module pair (never by finding id, which hashes the score),
   metrics by name, observations by edge, references by path and selector. Nothing is dropped.
   Two arrays keep their index (`evidence.snapshot.unknowns`, `repository.unread`); neither
   differs.
2. **Attribute.** Each differing field gets one cause from section 4. A seam gets its cause from
   its edges (below). Findings and metrics get "B-derived" only after the recomputation in step 3
   holds. A field with no cause fails the run. There are none.
3. **Reconcile.** `analysis/checks.md` lists 87 checks; all pass:
   - **The new report follows the 3.1 rule on every seam.** For each edge, the target package
     either matches the target module's `public` globs (contract) or does not (intrusive, with
     the target's volatility). The scores are contract 9, intrusive into `high` 2, intrusive into
     `medium` 5. The predicted strength and the n, min, max and mean of every seam equal the
     report: archfit 67 seams, pumba 8.
   - **Metrics.** `critical_band_edges`, `high_or_worse_edges` and `tight_seams` equal the sums
     over the seams, before and after. `qualifying_edges` and `unbalanced_edge` equal the
     intrusive edges into a `high` target. `distributed_monolith_seams` equals the tight
     `introduce_contract` seams into a `high` target. `encapsulation` equals
     (scored − intrusive) / scored.
   - **Findings.** One `bc/imbalanced_coupling` finding exists for each seam with hypothesis
     `introduce_contract`, before and after. Each new finding groups exactly the intrusive edges
     of its seam.
   - **Agreement with the check.** For every A seam that is an allowed dependency, the check's
     public-use balance (`repository.balances`) is 9 in both versions. The check already called
     these uses `contract`; only the v5.0.0 report did not. Every seam with a use past the surface
     is under a `rule: coupling` exception.

## 4. Causes

| Cause | What changed and why |
| --- | --- |
| A | Task 3.1 A. A use of the target's public part is `contract`. The raise to `model` from a Go type hint is gone. Seam: `model` 7 → `contract` 9, severity `low` → none. |
| B | Task 3.1 B. A use past the target's declared surface is `intrusive`. In v5.0.0 the report guessed it from Go types as `functional` or `model`. |
| B+C | Task 3.1 B and C. As B, and the intrusive use takes the target's volatility. The source is `high` and the target `medium`, so it scores 5, where the source's volatility would give 2. |
| B-derived | Findings and coupling and modularity metrics, recomputed from the B and B+C edges (section 3). |
| V1-fixture | Policy version 1, as input. The one-line policy edit changes the policy digest, the policy references, the snapshot and identity digests, the head commit and tree, the default base (the first parent of the fixture commit) and `differs`. On the base leg of archfit-4-mechanics, the recorded base was HEAD (a push, which shows the repository state); now HEAD is the fixture commit. |
| V1-base | Policy version 1, as behaviour. 0.9.0 reads only `version: 1`. A base with a version 2 or 3 policy gives `unknown` with `version N is not supported` (section 5). |
| R | Task 1.5 rename. `protected` is true when a workflow uses `coupling-dev/archfit-action`. The four workflows still pin `alexei-led/archfit-action`, so `protected` goes from `true` to `false` on every leg. Epic 6 changes the `uses:` line and restores it. |
| M | Metadata: the run date, and the engine version of the local build (`dev`). |

Not exercised by these runs: the fields and wording of task 3.3 (`surface`, `package`, `what`,
the next step past a surface) and the balance inputs of task 3.2. Both appear only on a check
error, and every head check passes. Task 3.4 (dates on exceptions) is not exercised: `init` does
not run, and the fixture policies predate #15. The evidence for #15 is separate
(`reports/b9-engine-init-self-exception-dates.md` in the team folder). Task 3.5 (hooks) is not
exercised either. Task 3.6 changes `init`, `doctor` and the text of `report`. The first two do
not run here, and the text is not compared (section 8).

Outside the JSON, one line of stderr differs. The ccgram report's unresolved-import hint said
`archfit config update`; it now says `archfit doctor`. Task 1.4 deleted the `config update`
hints.

## 5. Base checks

**With the recorded command, the base check goes from `pass` (exit 0) to `unknown` (exit 2) on
all four repositories.** Cause: V1-base. Every recorded base carries an old policy: archfit
`40b7562c` and archfit-4-mechanics `1f95386e` have version 3; ccgram `26c63f0e` and pumba
`f8caf006` have version 2. v5.0.0 read version 3. For a version 2 base it printed "judged with
the local policy until its conversion pull request merges". 0.9.0 reads only version 1, so it
reports `version N is not supported` for the base policy and gives no verdict. The base-policy
leg of ccgram and pumba (bases `3f651159` and `60c1fe18`, version 3) gives the same result.

Consequence for epic 6: **the `--base` check of every migration pull request is `unknown` until
its base branch carries a version 1 policy.** This holds in all four repositories, not only in
pumba (task 6.4). The target branch of a migration pull request still has the old policy, so the
check cannot give that pull request a verdict.

The migrated-base fixture (`base-v1`) is the secondary comparison:

- archfit and archfit-4-mechanics: `base-v1` is the recorded base plus `version: 3` →
  `version: 1`. The check passes as before. The 6 and 7 fields that differ are V1-fixture, R and
  M only.
- ccgram and pumba: not comparable. Their base policy has the module field `owner`, removed
  before policy version 1. A change of the version line alone leaves it invalid, so `base-v1` is
  the unchanged version 2 base and gives the same `unknown` as the base leg.

## 6. Per repository

### archfit

Head 7, base 15 and base-v1 6 fields: section 1. The report differs in 452 fields:

| Cause | Fields | Entities |
| --- | ---: | --- |
| A | 237 | 42 seams: `model` → `contract` |
| B | 28 | 6 seams become `intrusive` |
| B+C | 7 | 1 seam: `policy-config-adapter -> evidence-adapters` |
| B-derived | 118 | 7 findings (88 fields), their 4 entries in the coupling dimension (20), 6 coupling metrics (6), `encapsulation` (4) |
| V1-fixture | 60 | 51 policy references, 6 snapshot digests, the scope policy reference, 2 identity digests |
| R | 1 | `repository.protected` |
| M | 1 | `identity.engine_version` |

49 of 67 seams changed; the [appendix](20261121-archfit-0-9-numbers-seams.md) lists each one.

Findings (`bc/imbalanced_coupling`), severity, strength, score, grouped edges:

| Module pair | v5.0.0 | 0.9.0 |
| --- | --- | --- |
| `analysis-application -> assessment-repair` | critical, functional, 2, 7 edges | critical, intrusive, 2, 15 edges |
| `analysis-application -> relationship-analysis` | critical, functional, 2, 1 edge | critical, intrusive, 2, 1 edge |
| `config-lifecycle -> assessment-repair` | critical, functional, 2, 1 edge | critical, intrusive, 2, 1 edge |
| `evidence-acquisition -> assessment-repair` | critical, functional, 2, 1 edge | critical, intrusive, 2, 1 edge |
| `evidence-acquisition -> evidence-adapters` | medium, functional, 5, 1 edge | medium, intrusive, 5, 3 edges |
| `policy-config-adapter -> evidence-adapters` | none | medium, intrusive, 5, 1 edge (new) |
| `policy-config-adapter -> relationship-analysis` | none | critical, intrusive, 2, 1 edge (new) |

Metrics that changed:

| Metric | v5.0.0 | 0.9.0 | Reconciled as |
| --- | ---: | ---: | --- |
| `critical_band_edges` | 10 | 19 | sum of seam `critical_edges` |
| `high_or_worse_edges` | 10 | 19 | sum of seam `high_or_worse_edges` |
| `qualifying_edges` | 10 | 19 | intrusive edges into a `high` target |
| `unbalanced_edge` | 0 | 19 | intrusive edges into a `high` target; v5.0.0 never labelled an edge `intrusive` |
| `tight_seams` | 5 | 7 | seams with quadrant `tight` |
| `distributed_monolith_seams` | 4 | 5 | tight `introduce_contract` seams into a `high` target |
| `encapsulation` | absent | 0.948 | (440 − 23 intrusive) / 440; the metric is absent without an intrusive edge |

`scored_edges` (440), `seams` (67), the drift (2 errors) and the 78 allowed-dependency balances do
not change.

A cross-check with #15 (`1afbf8d7`): the fixture policy has 14 `rule: coupling` exceptions. The 7
seams with a use past the surface are under 7 of them. The other 7 cover no use past the surface.
They are exactly the 7 `coupling` exceptions that #15 removed as obsolete.

### archfit-4-mechanics

No seams and no findings, before or after. Head 7, base 20, base-v1 7 and report 12 fields: all
V1-fixture, V1-base (base only), R and M.

### ccgram

The report scores none of its 2481 Python edges, in v5.0.0 and in 0.9.0. So it has no seams,
findings or coupling metrics to change. Head 7, base 16 and report 83 fields: V1-fixture (81 in
the report: 72 policy references across 24 modules, 6 snapshot digests, 3 identity and scope
digests), V1-base (base only), R and M.

### pumba

Head 7 and base 16 fields: section 1. The report differs in 50 fields: A 21, V1-fixture 27, R 1,
M 1. 4 of 8 seams changed, all by A:

| Seam | v5.0.0: strength, min / mean / max | 0.9.0 | Edges |
| --- | --- | --- | ---: |
| `cmd -> pkg_chaos` | model, 7 / 8.05 / 9 | contract, 9 / 9 / 9 | 19 |
| `pkg_chaos -> pkg_container` | model, 7 / 7.9 / 9 | contract, 9 / 9 / 9 | 49 |
| `pkg_runtime -> pkg_container` | model, 7 / 7.26 / 9 | contract, 9 / 9 / 9 | 31 |
| `pkg_runtime -> pkg_tc` | model, 7 / 8.2 / 9 | contract, 9 / 9 / 9 | 5 |

No edge goes past a surface, so there are no findings, before or after.

## 7. Findings

| Finding | Answer |
| --- | --- |
| The recorded base check is `unknown` on all four repositories; the handover harness had used a different base (`base-v1`) and reported only ccgram and pumba as not comparable. | Manager, 2026-11-21: lead with the recorded command, cause V1-base. Keep `base-v1` as the secondary comparison. State the epic 6 consequence (section 5). |

No other difference is without a cause.

## 8. Limits

- The head legs judge different changes. v5.0.0 judged the last real commit of each repository.
  0.9.0 judges the fixture commit, because a target branch with a version 3 policy cannot be read.
  Both pass with no errors; the comparison is of status, counts and the report.
- The report is the whole tree at the same code, so its comparison is exact.
- Text output is compared by exit code only. The task covers the JSON.
- cargo, cargo-modules, dotnet-msbuild and roslyn are absent on this host (owner decision). The
  tool statuses of every report are the same in v5.0.0 and 0.9.0. A missing tool explains no
  difference here.

## 9. Reproduce

From an engine checkout, with the inputs of section 2:

```sh
WORK=$(mktemp -d)/work ~/Workspace/archfit-beta-team/reports/b9-numbers/run.sh 8f04ffe4 <new-dir>
python3 ~/Workspace/archfit-beta-team/reports/b9-numbers/analyze.py \
  ~/Workspace/archfit.reset-evidence/baseline <new-dir> <new-dir>/analysis
```

`analyze.py` exits 1 when a field has no cause or a check fails. Two runs on 2026-11-21 gave the
same summary.
