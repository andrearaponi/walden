# The Daily Workflow

The complete command loop, from repository bootstrap to certification. This page is operational — for the *why* behind each step, read [The Spec Lifecycle](lifecycle.md); for every flag, the [CLI reference](reference/cli.md).

## 1. Bootstrap

```bash
walden repo init
```

Creates `.walden/` (`constitution.md` for stable project context, `lessons.md`, a scoped `.gitignore`) and the generated CI workflow `.github/workflows/validate-walden.yml`, pinned to the generating version. If git is not initialized, the CLI initializes it first.

```bash
walden feature init "User Auth"
```

Names normalize to kebab-case; the spec scaffolds in `.walden/specs/user-auth/` as three draft documents.

Optionally declare [environment probes](reference/spec-format.md#environmentmd) in `.walden/environment.md` for the toolchains your proofs depend on — evidence records will carry their outputs, and drift becomes a printed diagnosis instead of forensics.

## 2. Requirements

Edit `requirements.md`: introduction, user stories, [EARS acceptance criteria](reference/spec-format.md#ears-acceptance-criteria) with stable IDs (`R1.AC1`), non-functional requirements (`NFR1`), constraints (`C1`), out-of-scope. Then:

```bash
walden validate user-auth
walden review open user-auth --phase requirements
walden review approve user-auth --phase requirements
```

Approve runs full phase validation before sealing — nothing structurally invalid gets a fingerprint.

## 3. Design

Edit `design.md`: architecture, at least one alternative considered, simplicity review, failure modes and tradeoffs, testing strategy, requirement coverage table. Same gate:

```bash
walden validate user-auth
walden review open user-auth --phase design
walden review approve user-auth --phase design
```

Unresolved forks can be parked as `[decision: which store backs this?]` markers — but they must be resolved before certification: the release gate blocks on decision markers in approved documents.

## 4. Tasks

Edit `tasks.md`: a two-level hierarchy where every leaf task names its acceptance criteria, its design section, and an executable proof:

```markdown
- [ ] 1. Implement authentication service
  - [ ] 1.1 Add password hashing utility
    - Requirements: `R1.AC1`, `R1.AC2`, `NFR2`
    - Design: Authentication Service
    - Verification:
      - command: ["go", "test", "-run", "TestHash", "./internal/auth"]
        expect_output: "--- PASS: TestHash"
        covers: ["R1.AC1", "R1.AC2"]
```

Declare `expect_output` on test runners so a pattern matching zero tests cannot pass vacuously; declare `timeout:` on steps that legitimately run long (the default budget is 10 minutes per step). Prefer read-only proof commands — `["go", "mod", "tidy", "-diff"]`, not `["go", "mod", "tidy"]` — because [re-verification is pure](lifecycle.md#execution-two-lanes-one-proof-grammar).

```bash
walden validate user-auth
walden review open user-auth --phase tasks
walden review approve user-auth --phase tasks
```

## 5. Execute

```bash
walden task status user-auth        # readiness, blockers, next runnable task
walden task start user-auth         # normalized execution context for the next task
walden task complete user-auth 1.1  # runs the proof; only a pass checks the box
walden task complete-all user-auth  # all runnable tasks in order, stops at first failure
```

A passing completion writes the task's evidence record — proof steps, chain fingerprints, code identity, execution profile — to `.walden/evidence/user-auth.json`. A failing proof leaves the box unchecked; fix and retry.

## 6. Reconcile

Editing an approved document breaks its seal: the document and everything downstream become stale, and execution blocks. Repair the chain:

```bash
walden reconcile user-auth
```

This resets the modified document and its downstream to draft and clears their fingerprints; re-walk the review gates to record fresh seals. Content-identical re-approval does not stale anything.

## 7. Verify

When code or specs move, evidence tells you what still holds:

```bash
walden evidence status user-auth   # derived state per task; read-only, always exit 0
walden verify user-auth            # re-prove what is no longer verified
walden verify user-auth --all      # re-prove everything
walden verify user-auth --check    # report only, write nothing (CI mode)
```

A code-only bugfix needs no spec ceremony: evidence goes `stale-code`, and one `verify` proves the acceptance criteria still hold — or names the task whose proof broke. Failures on a drifted machine carry the diagnosis: `environment drift: go: recorded "go1.25.0" → current "go1.24.0"`.

## 8. Certify

```bash
walden release check               # every feature
walden release check user-auth     # one feature
walden release check --strict      # additionally require committed .walden/ state
```

One deterministic verdict per feature — fresh approved chain, full validation, no decision markers, evidence verified — plus the repository-level criterion: a clean worktree outside `.walden/`. Exit `0` iff no blocker exists; every blocker names its remedy. The gate executes no proofs and writes nothing.

**Pending tasks block by default.** Shipping less than the approved plan is a recorded decision, not a default:

```bash
walden release check --allow-pending --reason "auth hardening deferred to 1.3"
# RELEASABLE — 3 feature(s) certified, 2 task(s) waived (reason: auth hardening deferred to 1.3), commit ba53cfe55b40
```

The reason and the waived task identifiers ride the verdict (completion class `with-waivers`). `--allow-pending` without a non-empty `--reason` is refused. Uncommitted code outside `.walden/` always blocks with no bypass; a repository without usable git fails closed.

## 9. Lessons

After a correction, failed validation, or execution surprise, make the failure reusable:

```bash
walden lesson log \
  --feature user-auth \
  --phase execute \
  --trigger "test failed because mock diverged from real database" \
  --lesson "integration tests must hit a real database" \
  --guardrail "before approving design, confirm test strategy uses real dependencies"
```

Lessons append to `.walden/lessons.md` and are reviewed before similar future work.

## At any point

```bash
walden status user-auth       # phase, blockers, next action
walden validate user-auth     # structural validation of the current phase
walden validate --all         # every feature, full spec
```

Every command takes `--json` for a [versioned machine-readable envelope](reference/json.md) — same information, same exit codes.
