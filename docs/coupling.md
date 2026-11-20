# Coupling model

archfit scores module and declared-external dependencies with strength, distance, and volatility.
The score describes balance. The report keeps metrics and seams as measurements;
it does not compare them with a stored reference. See [metrics](metrics.md) for metric behavior,
[configuration](configuration.md) for policy keys, and [results](results.md) for
report fields.

## The three inputs

### Strength

Strength measures how much knowledge crosses a boundary. Lower ordinals mean
weaker coupling.

| Strength     | Ordinal | Meaning                                                           |
| ------------ | ------: | ----------------------------------------------------------------- |
| `contract`   |       1 | The dependency uses the target's public part (`public`).          |
| `model`      |       3 | The dependency uses the target's model part (`model`).            |
| `functional` |       8 | Shared business responsibility. An import does not show it.       |
| `symmetric`  |       9 | Both modules hold the same knowledge: a copy (`duplication`).     |
| `intrusive`  |      10 | The dependency uses anything else in the target: past its surface. |

One rule gives a dependency between two modules its strength: where the target
is declared in the surface of its module. The check, `can`, a request and the
report use the same rule, so one dependency has one strength and one balance.

- A path in `public` is `contract`, a path in `model` is `model`, and any other
  path of the module is `intrusive`.
- A module that declares neither `public` nor `model` is public as a whole. So
  is a module the report adds on its own (a Go workspace member, a Rust module
  that no policy module covers): it declares nothing.
- A path matches a part as the check matches it: a glob covers the path, or,
  for a file, the directory that holds it (`public: [pkg/api]` covers
  `pkg/api/client.ts`, not `pkg/api/deep/x.ts`).
- A dependency inside one module, or to a path in no module, has no strength:
  the report abstains.

Type facts that an extractor reads (Go `go/types`, .NET Roslyn symbols, a
Python private name) never set or change a strength, and connascence does not
either. The extractors still read them; the report does not show them. A C#
`internal` member used through `InternalsVisibleTo` in a public file stays
`contract`, because the check cannot see it and one dependency has one number.

### Distance

Distance measures the cost of changing modules together. The shipped v7 model is
level-relative:

| Boundary                 |          Distance |
| ------------------------ | ----------------: |
| Same module              |                 2 |
| Any module boundary      |                 9 |
| Declared external system |                10 |
| Unresolved boundary      | Unknown. Abstain. |

Owner and deploy-unit labels name module boundaries. They do not change the
distance. A cross-owner or cross-deploy-unit dependency still has D=9. Path
containment provides the boundary context used in reports.

### Volatility

Volatility measures the expected rate of change. The ordinals are `frozen` 1,
`low` 3, `medium` 6, and `high` 10. Undeclared volatility scores as 10, but
remains unrated. Reports identify this gap and can recommend `declare_volatility`.

Explicit `volatility:` takes precedence over `subdomain:`. `core` means high,
`supporting` and `generic` mean low. Every dependency uses the target module's
volatility: the upstream module decides. A copy (`symmetric`) uses the higher
volatility of both modules. The optional cascade propagates volatility through the module graph to
a fixed point.

Finding explanations use **effective volatility**, not a claim about target churn.
Declared external systems use their own target volatility. Cascade can also raise
a declared value. `State.evidence.components` distinguishes asserted, assumed,
assessed, and unknown component origins. These origins are not attestations or a
per-seam provenance scalar. A seam aggregates edges, so its
strongest classification need not describe every advisory subgroup. These static
signals indicate change-propagation risk, not proof of an observed runtime cascade.

## Balance formula

For scored edges, archfit uses the Chapter 10 formula:

```text
balance = max(|S − D|, 10 − V) + 1
```

The result is an integer from 1 to 10. Higher values mean better balance.
Unknown strength or distance abstains. Same-module edges have no strength, so
they abstain in `local_coupling` and do not enter `coupling_balance`.

The score maps to these severity bands:

| Balance | Band       |
| ------: | ---------- |
|     1–2 | `critical` |
|     3–4 | `high`     |
|     5–6 | `medium`   |
|     7–8 | `low`      |
|    9–10 | `none`     |

### Examples

A use of the public part of a module across its boundary has S=1, D=9, and
V=10. Its balance is 9, with band `none`.

A use past the surface of a module with V=10 has S=10, D=9. Its balance is 2,
with band `critical`.

The same use past the surface of a module with V=3 has balance 8, with band
`low`.

## The `coupling` rule

Archfit 4 adds a built-in `coupling` rule to the check of policy version 1. It
is not released yet. The rule judges a dependency a change adds: new knowledge
across a module boundary needs a balance of at least 7. The balance uses the
formula above at D=9 and is shown as `9/10 (min 7)`.

| Target volatility | Public part (`contract`) | Model part (`model`) | Past the surface (`intrusive`) | Copy |
| ----------------- | -----------------------: | -------------------: | -----------------------------: | ---: |
| `low`             |                        9 |                    8 |                              8 |    8 |
| `medium`          |                        9 |                    7 |                            5 ✗ |  5 ✗ |
| `high`            |                        9 |                    7 |                            2 ✗ |  1 ✗ |

- The strength comes from the declared surface of the target, as above.
- The volatility that applies is the target's for every use, and the higher of
  the two modules for a copy.
- The rule uses the declared volatility. The cascade is not used, because with
  it one new dependency changes the balance of code the change did not touch.
- The `public` and `model` globs alone tell the three uses apart, so the check
  needs no type analysis. With `min_balance: 8`, a use of the model part passes
  only into a `low` module.

In words: a module may use the public part and the model part of a module it
is allowed to depend on. It may reach past the surface only into a `low`
module.

## Distributed-monolith seam measurements

The report groups dependencies into ordered module pairs, called seams. One
seam can contain many dependency edges. A seam qualifies as a distributed-monolith
measurement when at least one import edge meets all of these conditions:

- Its strength is `intrusive`.
- It crosses a module boundary.
- Its effective volatility is `high` and comes from a declaration, inheritance,
  or the enabled cascade.
- Its source role does not matter: a `composition_root`, `generated`, or `test`
  role never excuses an intrusive use.

Medium volatility does not qualify. Qualification is separate from
the balance score. Owner and deploy labels do not change D=9 or make a seam
qualify by themselves.

The seam ledger, `coupling_balance`, and `unbalanced_edge` are report measurements.
They have no stored-reference gate or metric ratchet. The Archfit 4 check applies
the `coupling` rule to errors added by the change, independently of these report
measurements. See [metrics](metrics.md#deltas-and-gates) and [rules](rules.md).

## Copies and limits

The `duplication` rule checks identical literals added across module boundaries.
It is separate from import-edge scoring. See [rules](rules.md) for its limits.

Go reads `go/types` data and .NET reads Roslyn compiler symbols from the
`archfit-roslyn` helper. Both are evidence on the edge; neither sets a strength.

Published-language coupling between services has no type edge and is not
measured in any language: message records duplicated in each service, a
`.proto` that another project compiles directly, and HTTP or YARP runtime calls.
These stay outside the dependency graph.

Every language gets its strength from the declared surface; a declared
external system has none and abstains. Runtime async evidence is report-only. It does not
change distance or gate results. Unknown facts can lower confidence or leave a
dimension partial. See [evidence](evidence.md) for claim boundaries and
[configuration](configuration.md) for policy and extractor settings.
