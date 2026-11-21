# Rules

A rule is anything `archfit check` enforces on a change. The built-in rules have fixed ids and need no entry in `.archfit.yaml`. The custom rules are in its `rules` list. An exception permits an error of one rule in one place. Every key of the policy is in [Configuration](configuration.md).

This page describes a version 1 policy. A version 2 policy (Archfit 3) has other rules: see [Version 2 (Archfit 3)](#version-2-archfit-3).

```yaml
version: 1
owners: [alexei-led]
modules:
  pkg_chaos:
    paths: [pkg/chaos/**]
    depends_on: [pkg_runtime]
  pkg_runtime:
    paths: [pkg/runtime/**]
    volatility: low
builtin: { graph: deny, min_balance: 8 }
rules:
  - id: no-chaos-to-runtime
    deny: { from: pkg_chaos, to: pkg_runtime }
    why: A direct import ties a chaos action to one runtime.
    instead: Add a method to container.Client.
exceptions:
  - rule: no-chaos-to-runtime
    from: pkg_chaos
    to: pkg_runtime
    expires: 2026-11-19
    why: "Request #431 is open."
```

Run `archfit rules <rule-id>` to see one rule in full, and `archfit rules <path>` for the rules that apply to a path.

## Built-in rules

They apply to what a change adds.

| Id | What it says | Default mode |
| --- | --- | --- |
| `graph` | A dependency between two modules must be an allowed dependency (`depends_on`). | `ask` |
| `coupling` | New knowledge across a module boundary needs a balance of at least `min_balance` (default 7). | `deny` |
| `duplication` | Knowledge that another module already owns is not copied. | `ask` |
| `cycle` | No new cycle between modules. | `deny` |
| `layers` | Dependencies follow the layer order: an inner layer does not depend on an outer one. | `deny` |
| `unowned` | New production code belongs to a module. | `deny` |

A rule has one mode:

| Mode | An error says |
| --- | --- |
| `deny` | `next: fix the code`, or the `instead` of the rule. A `coupling` error of a use past the surface asks for the surface instead, in every mode: see [Coupling](coupling.md). |
| `ask` | An owner may approve: `next: archfit request -m "<why ...>"`. |
| `off` | The rule is not checked. |

Change a mode, or the lowest balance `coupling` accepts, with the `builtin` block:

```yaml
builtin: { graph: off, duplication: deny, min_balance: 8 }
```

`min_balance` is from 1 to 10.

### `coupling`

The balance of a dependency is shown as `n/10 (min m): strength, module is volatility`. It follows where the use lands in the surface of the target and the volatility that applies:

| Volatility that applies | Use of the public part (`contract`) | Use of the model part (`model`) | Use past the surface (`intrusive`) | Copy |
| --- | ---: | ---: | ---: | ---: |
| `low` | 9 | 8 | 8 | 8 |
| `medium` | 9 | 7 | 5 | 5 |
| `high` | 9 | 7 | 2 | 1 |

- For every use, the volatility of the target applies. For a copy, the higher volatility of the two modules applies.
- The check uses the declared volatility (`volatility`, else from `subdomain`, else `high`).
- The `public` and `model` globs of the target module are its surface. A module that declares neither is public as a whole.
- A use past the surface names the package and the surface, and asks one question: should this package be part of the surface? See [Coupling](coupling.md).

In words: a module may use the surface of a module it is allowed to depend on: its public part, and its model part when the balance allows. It may reach past the surface only into a module declared `low`. The `coupling` rule applies inside allowed dependencies too. `archfit rules <path>` lists the surface of each allowed dependency.

### `duplication`

The check finds one kind of copy: an identical literal. A string, a regular expression, or a table of values that the change adds in one module, and that production code of another module already has. It counts a literal only when:

- it is at least 12 characters long;
- it is not a single word or a struct tag;
- it is not in a declared `tests` or `generated` path;
- fewer than 3 modules hold it.

The precision is measured, not promised: about 80% on the repositories used to tune it. A copy in other words is not found. The error names where the literal already lives:

```text
pkg/tc/interface.go:12: duplication: this literal already lives in pkg/util/util.go:15
  why: two copies must change together and nothing checks that they do
  next: use pkg_util, or run: archfit request -m "<why a copy is needed>" --expires 2026-12-10
archfit: fail, 1 error
```

### `unowned`

A file with added lines that no module path matches is an error. A file the policy declares in `tests` or `generated` is not. A file that is only renamed adds no line, so it passes.

## Custom rules

Each custom rule has an `id`, exactly one form, and a `why`:

| Form | Says | Needs |
| --- | --- | --- |
| `deny: { from, to }` | No dependency from these modules or paths to those. | `why`, `instead` |
| `only: { use, in }` | A library, or a package set, is used only in the listed modules. | `why` |
| `note: <text>` | A rule nothing can check today, written down with its reason. | `why` |

- `id` is a short name, as in a linter, such as `no-chaos-to-runtime`. It appears in every error. It must be unique and must not be a built-in id.
- `from`, `to`, and `in` take a module name or a path glob, or a list of them. A name with a `/` or a wildcard that is not a module is a path glob. Any other name must be a module of the policy; an unknown name is an error with a suggestion.
- `use` is a library import path, such as `github.com/docker/docker`, or a package set: `network`, `process`, or `database`. A set lists the standard and common packages of each language, so `only: { use: process, in: [pkg_runtime] }` keeps process launches in one adapter.
- A `deny` rule is direct. It does not stop A from reaching C through an allowed B. Use `layers` for a direction that must hold all the way.
- A `note` shows in `archfit rules` as "not checked".

## Exceptions

An exception permits the errors of one rule in one place, with a reason and usually a date. It is the only way to accept an error.

```yaml
exceptions:
  - rule: duplication
    from: pkg_tc
    to: pkg_util
    expires: 2026-11-19
    why: "Request #431 is open."
```

| Field | Meaning |
| --- | --- |
| `rule` | A built-in or custom rule id. |
| `from` | The module the errors come from. For `unowned`, the glob of the code no module owns. |
| `to` | The module the errors reach. Not used for `unowned` and for a custom `only` rule: the `only` rule names its library itself. |
| `expires` | The last day the exception holds, as `YYYY-MM-DD`. Absent: no date. |
| `why` | The reason. Required. |

An exception matches by rule, `from`, and `to`. A `duplication` exception matches the two modules in either order. An `only` exception matches by rule and `from`.

**A date has teeth.** After `expires`, while the code still breaks the rule, every change fails, also a change that touches none of that code:

```text
.archfit.yaml: exception: duplication pkg_tc -> pkg_util expired on 2026-11-19
  why: an exception with a date fails every change after that date
  next: wait for an owner: #431
archfit: fail, 1 error (not from this change)
```

`next` is `wait for an owner: #n` when the `why` names a request `#n`. Otherwise it is `fix the code, or ask an owner to extend the exception`.

## Added or existing

A change is judged by what it adds. The check reads the merge base and the working tree in one run and counts the errors of each by a key: the rule, the from module, the to module, and the symbol (the imported package or library; for `unowned`, the file; for `duplication`, the literal). For `cycle`, each dependency inside a cycle is one key.

- A key the base does not have, or has fewer times, is added by the change: an error of the change.
- One more use of an already broken dependency is an error of the change.
- An error that exists on the base, and that the change does not add to, is not shown. It is the owners' work.

When the base cannot be read, the status is `unknown`, never `pass`. See [results](results.md).

## Tests and generated code

The rules skip the paths that `tests` and `generated` list. Only listed paths are out: a file name that looks generated is not enough.

```yaml
tests: ["**/*_test.go", "**/testdata/**"]
generated: ["internal/gen/**"]
```

## Version 2 (Archfit 3)

A version 2 policy has rule types (`forbidden_dependency`, `module_dependencies`, `forbidden_layer_direction`, `module_cycle`, `public_api_only`, `forbidden_pattern`, and others), a `gate` of `fail`, `warn`, or `off` on each rule, and waivers. `check` and `report` require version 1. The full reference is the [Rules page of v3.4.0](https://github.com/coupling-dev/archfit/blob/2692cd6b8aab23e559d0aa76712a192d69b4229f/docs/rules.md).

### Waivers

Archfit 3 accepted findings with `waivers`: a rule ID, selectors, a reason, approver metadata, and an expiry date. In version 1, exceptions replace them. See the [waivers of v3.4.0](https://github.com/coupling-dev/archfit/blob/2692cd6b8aab23e559d0aa76712a192d69b4229f/docs/rules.md#waivers).
