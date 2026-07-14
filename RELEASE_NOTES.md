## Walden v0.9.1

The first five minutes matter. This release polishes the surface a newcomer actually touches — `--help`, flag errors, "validate everything" — and hardens the gates underneath so that what Walden certifies is even harder to fake.

### Help that helps

Every command and subcommand now answers `--help`/`-h` with its syntax, summary, and flags — resolved before positional validation, on stdout, exit 0:

```bash
walden validate --help     # explains itself
walden task --help         # lists the task subcommands
```

Previously `walden validate --help` answered `feature spec not found: help`. Help output derives from a single command registry — the same table that builds the top-level usage — so per-command help and the usage screen can never drift apart.

Unknown flags are rejected uniformly: non-zero exit, an error naming the flag, a pointer to `--help`. With `--json`, even the rejection honors the envelope contract. A registry-driven uniformity test makes every current and future command inherit both behaviors by registration alone.

### Validate the whole repository

```bash
walden validate            # every feature under .walden/specs/, sorted
walden validate --all      # full-spec scope, per feature
```

One verdict per feature, exit 1 if any fails — mirroring `release check [<feature>]`. The JSON envelope gains an additive `features` array (`feature`, `valid`, `summary`). An empty repository fails with a pointer to `walden feature init`.

### Gates that fail closed

- **`review approve` now runs full validation before recording approval.** A document the validator rejects can no longer become approved and detonate at execution time: the gate refuses with `approval refused: <defect>` and exit 1, leaving the document untouched.
- **`release check` requires a git-backed code identity.** A repository without usable git used to skip the worktree criterion and could certify releasable; it now emits a repository-level blocker — a verdict must name the exact tree it certified.
- **`--strict` requires committed `.walden/` state.** Dirty ledger paths still warn in the default mode, but a plans-complete certification must be reproducible from the commit it judges.
- **Unterminated HTML comments block the decisions criterion** instead of swallowing every marker after them.

### Top-level leaf tasks execute

A leaf task at the top level of `tasks.md`, written with natural two-space metadata, validated as a draft but failed `task start`/`complete` with `invalid metadata indentation`. Offsets are now relative to task nesting, and draft validation and the executor share one layout check — a draft that validates can no longer be structurally rejected at execution time. Every document that parsed before still parses.

### Compatibility

JSON output changes are additive within `schema_version: v0beta1`; evidence schema `v1alpha1` and spec frontmatter are untouched. Two behavioral changes are breaking by design and documented in the changelog: invalid documents can no longer be approved, and gate criteria tightened (`git_skipped`, `--strict`). Documents that used to slip through must be fixed before re-approval — `walden validate <feature>` reproduces the refusal reason exactly.

---

Install or update:

```bash
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
# or, from an existing install
walden update
```
