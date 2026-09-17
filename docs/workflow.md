# The Daily Workflow

The complete command loop, from repository bootstrap to certification. This page is operational — for the *why* behind each step, read [The Spec Lifecycle](lifecycle.md); for every flag, the [CLI reference](reference/cli.md).

In [the agentic flow](agentic.md), the authoring steps below (drafting requirements, design, tasks; implementing code) are what the skill does for you, driving these exact commands and stopping at each gate for your approval. The loop is the same either way — this page describes what runs underneath.

## Choose the entry point

Before bootstrapping a feature, decide whether intended behavior changes. Contract-preserving bugfixes and refactoring can use existing tests and evidence re-verification without new spec authoring, unless user or project policy requires it. New or changed contracts enter the earliest affected phase. If the impact is unclear, clarify it before creating documents. Approval of a plan alone does not authorize execution.

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

Edit the body of `requirements.md`, retaining CLI-owned frontmatter: introduction, user stories, [EARS acceptance criteria](reference/spec-format.md#ears-acceptance-criteria) with stable IDs (`R1.AC1`), non-functional requirements (`NFR1`), constraints (`C1`), out-of-scope. Put a short `Acceptance check:` continuation under each AC describing the observable result, not an implementation command. Then:

```bash
walden validate user-auth
walden review open user-auth --phase requirements
walden review approve user-auth --phase requirements
```

Approve runs full phase validation before sealing — nothing structurally invalid gets a fingerprint.

## 3. Design

Start from the six scaffolded design headings: Architecture, Options Considered, Simplicity And Elegance Review, Failure Modes And Tradeoffs, Verification Plan, and Requirement Coverage. Components, data models, security, and other sections are optional expansions when relevant. A discussion may explain non-applicability in one line rather than invent an alternative; coverage and verification still need real mappings and checks. Same gate:

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
      - command: ["go", "test", "-v", "-count=1", "-run", "^TestHash$", "./internal/auth"]
        expect_output: "--- PASS: TestHash"
        covers: ["R1.AC1", "R1.AC2"]
```

Declare a runner-appropriate anti-vacuity check: native failure on zero selected tests, structured results, or an output assertion as above. Named Go PASS lines require `-v`; `-count=1` prevents test-cache reuse. Map each step's asserted ACs through `covers:`. Declare `timeout:` on steps that legitimately run long (the default budget is 10 minutes per step). Prefer read-only proof commands — `["go", "mod", "tidy", "-diff"]`, not `["go", "mod", "tidy"]` — because [re-verification is pure](lifecycle.md#execution-two-lanes-one-proof-grammar).

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

A passing completion writes the complete contract fingerprint, observed proof results, post-state code identity and completion provenance to `.walden/evidence/user-auth.json`. A failing proof leaves the box unchecked. Declare the authorized batch and its verification checkpoint: every leaf earns its own proof, but ordinary repo-wide staleness does not require replaying the completed prefix after each edit. A changed dependency or failing assertion can justify an earlier targeted check.

## 6. Reconcile

Editing an approved document breaks its seal: the document and everything downstream become stale, and execution blocks. Repair the chain:

```bash
walden reconcile user-auth
```

This resets the modified document and its downstream to draft and clears their fingerprints; re-walk the review gates to record fresh seals. Content-identical re-approval does not stale anything.

## 7. Verify

When code or specs move, evidence tells you what still holds:

```bash
walden adopt user-auth --json      # legacy assessment; no proof or profile probe
walden evidence status user-auth   # evidence view; includes diagnostic profile probes
walden verify user-auth            # needed completed proofs in this feature only
walden verify user-auth --all      # explicitly force every completed proof in this feature
walden verify user-auth --check    # execute selected proofs without persisting evidence
```

Establish the current contract and applicable feature scope before replaying old work; requirements can survive obsolete delivery tasks, and uncertain business intent needs a decision. At batch/feature/delivery closure, refresh the selected applicable evidence and report residual gaps. Code-only maintenance need not create a new spec, but raw tests alone do not refresh Walden evidence.

A reconstructed legacy binding is not execution attestation: `unattested` and independent provenance gaps remain explicit. Verify rejects mutations and contaminates later executions in that run; restoring bytes does not revive failed records. `--check` does not sandbox proof side effects. Environment drift remains diagnostic, not a substitute for proof binding or integrity.

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

The reason and waived task identifiers ride the verdict. Waivers cover pending work only, not legacy assurance or strict input binding. `--strict` compares the exact judged input snapshot with its captured commit regardless of ignore rules; non-strict mode does not claim local metadata is committed. A named-feature verdict is not certification of the whole repository. Uncommitted code still blocks, and usable Git is required.

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
