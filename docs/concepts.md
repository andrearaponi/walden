# Concepts

## Spec-Driven Delivery

Walden structures work as a sequence of reviewed documents. Every feature progresses through three phases before code is written:

1. **Requirements** — What the feature must do, expressed as EARS acceptance criteria.
2. **Design** — How the feature will be built, including architecture, alternatives, and tradeoffs.
3. **Tasks** — What code to write, in what order, with what verification proof.

This sequence is enforced by the CLI. You cannot skip phases, and you cannot execute tasks from an unapproved or stale plan.

## The Two Halves

Walden splits work into two complementary halves:

**Deterministic (CLI)** — repeatable, rule-based operations:
- Validating phase order and freshness
- Opening and approving review gates
- Checking execution readiness
- Running verification proofs
- Reconciling stale approval chains
- Logging lessons

**Non-deterministic (Skill or human)** — interpretation and judgment:
- Drafting requirements and acceptance criteria
- Designing architecture and evaluating alternatives
- Generating implementation plans
- Deciding when to re-plan from an earlier phase
- Reviewing and approving documents

The CLI never authors content. The skill never mutates workflow state.

## Documents

Every feature lives in `.walden/specs/{feature-name}/` with three files:

- `requirements.md` — problem statement, user stories, EARS acceptance criteria, non-functional requirements, constraints, and out-of-scope items.
- `design.md` — architecture, component interfaces, data models, options considered, failure modes, testing strategy, and requirement coverage mapping.
- `tasks.md` — two-level task hierarchy where every leaf task references acceptance criteria IDs (e.g., `R1.AC1`), design sections, and a verification proof.

## Frontmatter

Each document begins with YAML frontmatter that tracks its lifecycle:

```yaml
---
status: draft|in-review|approved
approved_at: 2026-03-22T10:00:00Z
last_modified: 2026-03-22T10:00:00Z
source_requirements_approved_at:  # design.md only
source_design_approved_at:        # tasks.md only
---
```

The `source_*` fields create an approval chain. If an upstream document is re-approved with a new timestamp, downstream documents become stale and must be reconciled.

## Freshness

A document is **fresh** when its `source_*` timestamp matches the current upstream `approved_at`. A document is **stale** when the upstream approval changed after the downstream was last approved.

Stale documents block execution. Use `walden reconcile` to repair the chain.

## Verification Proofs

Every leaf task in `tasks.md` includes a `Verification:` line. This is the command or script that proves the task is complete. The CLI runs this proof during `task complete` and only marks the task done if the proof passes.

Proofs use structured `command` format to avoid shell-quoting issues:

```markdown
- Verification:
  - command: ["go", "test", "./internal/workflow/..."]
```

## Constitution

`.walden/constitution.md` is an optional repo-wide file that captures stable project context: tech stack, conventions, sanity checks, key files, and hard rules. Unlike the three per-feature documents (`requirements.md`, `design.md`, `tasks.md`), the constitution is not part of the approval workflow. It does not block any phase transition, does not have freshness rules, and is not validated by the CLI. The skill reads it when present to reduce context rediscovery across features.

## Lessons

`.walden/lessons.md` is an append-only file of reusable patterns. Each lesson records a trigger (what happened), a pattern (what went wrong), and a guardrail (what to check next time). The skill reviews lessons before non-trivial work to avoid repeating mistakes.

## EARS

The Easy Approach to Requirements Syntax. Walden uses EARS for all acceptance criteria:

| Form | Template |
| --- | --- |
| Ubiquitous | The system SHALL [response] |
| Event-driven | WHEN [trigger], the system SHALL [response] |
| State-driven | WHILE [precondition], the system SHALL [response] |
| Optional | WHERE [feature], the system SHALL [response] |
| Unwanted | IF [trigger], THEN the system SHALL [response] |
| Complex | WHILE [precondition], WHEN [trigger], the system SHALL [response] |
