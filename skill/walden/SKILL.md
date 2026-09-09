---
name: walden
description: "Walden drafts and maintains feature specs in `.walden/specs/` with a gated workflow: EARS requirements, design documents, implementation tasks, and execution of approved tasks. Use when the user asks for Walden, a feature spec, requirements, a design doc, an implementation plan, or work from an existing `.walden/specs/` folder."
metadata:
  short-description: Walden spec workflow
---

# Walden

Use this skill to turn an idea into a reviewed feature spec, or to execute reviewed work from an existing spec.
Reply in the user's preferred language when possible.

## When To Use

- A user wants a new feature specification
- A user wants requirements written in EARS
- A user wants a design document from approved requirements
- A user wants an implementation plan from approved design
- A user wants to execute approved tasks from `.walden/specs/{feature-name}/`

## Prerequisites

The `walden` CLI must be installed and available in `PATH`. The CLI is the single source of truth for all deterministic workflow mechanics. This skill handles authoring, reasoning, and review interaction; it does not re-implement workflow rules.

If `walden` is not available, inform the user and point them to the install instructions before continuing.

## Product Boundary

Walden is an open source spec-driven delivery kernel. It is not a complete enterprise platform. The CLI and this skill together cover the local workflow: requirements, design, tasks, execution, reconciliation, and lessons. Capabilities like GitHub App integration, multi-repo sync, org dashboards, and governance packs are future enterprise scope.

## Deterministic Helpers

- Prefer the `walden` CLI for deterministic workflow mechanics.
- Use `walden repo init` to bootstrap a repository when Walden has not been initialized yet.
- Use `walden feature init <feature-name>` to scaffold the canonical spec files.
- Use `walden status <feature-name> [--json]` to inspect phase, blockers, and next action.
- Use `walden validate [<feature-name>] [--all] [--json]` before phase transitions and before execution; omit the feature name to validate every feature in the repository.
- Use `walden review open <feature-name> --phase requirements|design|tasks` and `walden review approve <feature-name> --phase requirements|design|tasks` for deterministic review-state transitions.
- Use `walden task status <feature-name> [--json]`, `walden task start <feature-name> [task-id] [--json]`, and `walden task complete <feature-name> <task-id> [--json]` for deterministic execution flow.
- Use `walden task complete-all <feature-name> [--json]` to complete all runnable leaf tasks in order, stopping on first failure.
- Use `walden verify <feature-name> [--all] [--check] [--json]` to re-execute completed tasks' proofs against the current code and refresh execution evidence; `--check` reports without persisting anything. Re-verification is pure: a proof that modifies the working tree fails its task naming the changed paths — author proofs as read-only assertions and route build outputs outside the repository (task completion keeps accepting generator mutations; its recorded identity binds the resulting tree). Verify records bind the tree the run started from, so one mutating proof fails alone instead of staling the tasks proven after it; the run warning names both the modified paths and the tasks re-proven on the modified tree.
- Use `walden evidence status <feature-name> [--json]` to inspect each task's derived evidence state: verified, stale-spec, stale-code, failed, unrecorded, or pending.
- Use `walden release check [<feature-name>] [--strict] [--allow-pending --reason "<text>"] [--json]` to certify the repository (or one feature) as releasable in one deterministic verdict; it executes no proofs and writes nothing. Pending leaf tasks block the verdict by default; the waiver flags are the only relaxation and require the user's explicit approval (see Release Certification).
- Use `walden adopt [<feature-name>] [--apply] [--json]` to onboard a repository whose specs predate the current contract: the default is a read-only plan classifying every feature; `--apply` seals recorded approvals and re-proves unrecorded work (see Brownfield Adoption).
- Use `walden reconcile <feature-name> [--json]` when approved upstream documents changed or the approval chain is stale.
- Use `walden lesson log --feature <feature-name> --phase requirements|design|tasks|execute|release --trigger "<event>" --lesson "<pattern>" --guardrail "<rule>" [--json]` after meaningful corrections, failed validation, or execution surprises.
- Use `walden version [--json]` to check the installed CLI version and schema version.

All `--json` commands return a versioned envelope:

```json
{
  "schema_version": "v0beta1",
  "command": "<command-name>",
  "ok": true,
  "result": {}
}
```

## Core Rules

- New features must progress `Requirements -> Design -> Tasks`.
- Planning stops after approved `tasks.md`.
- Execution is a separate invocation path and starts only when the user explicitly asks to execute a task.
- Existing specs may enter at Design only if `requirements.md` is approved.
- Existing specs may enter at Tasks only if `design.md` is approved.
- Execution requires approved and non-stale `requirements.md`, `design.md`, and `tasks.md`.
- For non-trivial work, start with a short plan that includes the next phase steps and how you will verify them.
- If ambiguity, failed validation, or conflicting constraints appear, stop and re-plan from the earliest affected phase instead of pushing forward.
- Review `.walden/lessons.md` before non-trivial work when the current request resembles earlier mistakes or rejected drafts.
- For deterministic state inspection or mutation, prefer the `walden` CLI over manual frontmatter edits or helper scripts.
- Before closing any revision, correction, or recovery step, make an explicit `Lesson Decision: none|logged`.
- If the work included a user correction, failed validation, rejected draft, re-plan, or unexpected execution issue, default to `Lesson Decision: logged` unless there is a clear reason not to.
- Never treat silence as approval.
- If an upstream document changes, mark dependent downstream documents stale and reset their `status` to `draft` before continuing.

## Files And Naming

- Store documents in `.walden/specs/{feature-name}/`.
- Normalize `{feature-name}` to kebab-case.
- Use exactly these files:
  - `requirements.md`
  - `design.md`
  - `tasks.md`

## Approval And Staleness Model

Every document must begin with YAML frontmatter.

### `requirements.md`

```yaml
---
status: draft
approved_at:
last_modified: 2026-03-19T10:00:00Z
approved_fingerprint:
---
```

### `design.md`

```yaml
---
status: draft
approved_at:
last_modified: 2026-03-19T10:00:00Z
approved_fingerprint:
source_requirements_approved_at:
source_requirements_fingerprint:
---
```

### `tasks.md`

```yaml
---
status: draft
approved_at:
last_modified: 2026-03-19T10:00:00Z
approved_fingerprint:
source_design_approved_at:
source_design_fingerprint:
---
```

Apply these rules consistently:

- Set `status: draft` when first creating a document.
- Set `status: in-review` immediately before presenting a revision to the user.
- Set `status: approved` and populate `approved_at` only after explicit approval — prefer `walden review approve`, which also records the approval fingerprints.
- Update `last_modified` on every edit.
- Never hand-edit fingerprint fields (`approved_fingerprint`, `source_*_fingerprint`): they are computed and verified by the CLI. A fingerprint that does not match its document's content makes the document stale.
- Freshness is decided by fingerprint comparison: an approved document is stale when its body no longer matches its `approved_fingerprint`, and a downstream document is stale when its `source_*_fingerprint` differs from the upstream's current `approved_fingerprint`. Timestamps remain as human-readable context.
- If an approved document is edited later, it is stale until the chain is repaired: run `walden reconcile` (the document resets to draft) and take it through review again.
- Approved documents that lack fingerprints (created by pre-fingerprint CLI versions) are stale by definition; `walden reconcile` plus one re-approval cycle migrates them.

## Phase Router

Before doing any work:

1. Determine the feature name and inspect `.walden/specs/{feature-name}/`.
2. Read `.walden/constitution.md` when it exists for project-wide context (tech stack, conventions, key files). Skip without error when absent. If the file exists but contains only placeholder text (sections with bracket patterns like `[What this project does...]`), stop and ask the user to fill it in before proceeding — an empty constitution means every spec will be written without stable project context.
3. Review `.walden/lessons.md` when it exists and the feature type or failure mode is similar.
4. Read existing frontmatter and approval timestamps.
5. Run `walden status <feature-name>` and `walden validate <feature-name>` when the CLI is available and the feature folder already exists.
6. Choose the earliest phase that is missing, unapproved, or stale.
7. For non-trivial work, state a short plan for the current phase plus the verification gate.
8. Honor the user's requested entry point only if all prerequisites are approved and fresh.
9. For a new feature, always start at Requirements.

## Decision Checkpoint Protocol

Apply this protocol during Phase 1, 2, and 3 drafting. Do not apply during Phase 4.

**Bifurcation Test:** a decision merits a `[decision: <question>]` checkpoint if and only if choosing differently would require discarding or substantially rewriting document content produced after the choice. When in doubt, default to autonomous resolution.

**Explore before asking:** when a decision passes the Bifurcation Test, check whether the codebase, the constitution, or approved upstream documents already answer the question before emitting a checkpoint. If they do, resolve autonomously and record the assumption with its source: `<!-- assumed: <choice> (source: <file or document>) -->`.

**On TRUE — checkpoint detected:** emit `[decision: <question>]` in the document, explain the fork in plain language, and state your recommended option with a one-line rationale — if no option is defensibly better, present the fork without a recommendation. Stop generating further content. Wait for the user's response. On receiving a response, state how the answer will be applied to the document before resuming content generation in the same conversation turn. If the user's response surfaces a previously unidentified bifurcation-significant decision, emit a new `[decision: <question>]` marker for the newly identified fork before generating content that depends on it.

**On FALSE — autonomous resolution:** record the chosen assumption as `<!-- assumed: <choice> -->` inline in the document and continue drafting without interruption.

**Autonomous-at-checkpoint:** if the user asks the skill to decide autonomously at a checkpoint, record the resolution as `<!-- assumed: <choice> -->` in the document and continue drafting without emitting further checkpoints for decisions within the same scope.

**Constraints:** emit no more than five `[decision:]` checkpoints across a single phase drafting session. If a `[decision:]` checkpoint is left unresolved at the end of a conversation turn, the document remains in `draft` status and the skill shall not present it for phase-transition review.

## Phase 1: Requirements

Generate a first draft before asking clarifying questions. Then iterate with the user.

### Requirements Standard

- Apply the Decision Checkpoint Protocol during drafting.
- Give every requirement a stable ID: `R1`, `R2`, `R3`.
- Give every acceptance criterion a stable ID: `R1.AC1`, `R1.AC2`, `R2.AC1`.
- Use EARS syntax for every acceptance criterion. The CLI validates keyword-level structure: single SHALL, form classification (WHEN, WHILE/DURING, WHERE, IF/THEN before SHALL), IF/THEN pairing, non-empty template slots, and warns on likely inverted forms. It does not validate semantic quality of slot content. The skill guides content quality; the CLI enforces structural conformance.
- Use user stories as context, not as the acceptance contract.
- Give non-functional requirements stable IDs: `NFR1`, `NFR2`.
- Give constraints and dependencies stable IDs: `C1`, `C2`.
- Include explicit out-of-scope items when scope risk is high.

### EARS Forms

- Ubiquitous: `The system SHALL [response]`
- Event-driven: `WHEN [trigger], the system SHALL [response]`
- State-driven: `WHILE [precondition], the system SHALL [response]`
- Optional feature: `WHERE [feature], the system SHALL [response]`
- Unwanted behavior: `IF [trigger], THEN the system SHALL [response]`
- Complex: `WHILE [precondition], WHEN [trigger], the system SHALL [response]`

### EARS Quality Rules

Apply these rules during drafting, not only during review.

**Form selection.** Choose the form that matches the behavioral nature of the criterion:
- If the behavior is always true regardless of user action (invariants, automatic behaviors, system properties), use **ubiquitous**. Example: "The system SHALL ensure no two players receive identical cards." Do not force a WHEN trigger on something that has no external trigger.
- If the behavior responds to a specific user action or system event, use **event-driven**. The trigger must name what happens, not just that something happens.
- If the behavior is active only while a condition holds, use **state-driven** (WHILE or DURING).
- If the behavior handles a failure, invalid input, or error condition, use **unwanted** (IF/THEN).
- If the behavior requires both a precondition and a trigger, use **complex** (WHILE + WHEN).

**One behavior per criterion.** Each AC must describe exactly one observable system response. If the response slot contains "and" connecting two distinct behaviors, split into separate ACs. Example — split this: "the system SHALL generate cards server-side and send them to each player via WebSocket" into two ACs: one for generation, one for delivery. **Self-check after each AC**: before writing the next criterion, re-read the response slot just drafted. If it contains "and" connecting two independently observable behaviors, split immediately. Do not defer to review — splitting later requires ID renumbering that cascades through the entire spec.

**Concrete triggers.** Every event-driven trigger must name the specific interaction: clicks, taps, submits, opens, presses, types, drags, scrolls, navigates — not generic verbs like triggers, initiates, requests, performs, executes. If you cannot name the interaction, the requirement may be underspecified.

**Failure mode coverage.** For each constraint, ask: "what happens if this fails or is unavailable?" Draft at least one IF/THEN criterion per constraint that has a realistic failure mode. A spec with constraints but zero unwanted forms is almost certainly missing error handling.

**NFR promotion.** If an NFR contains IF/THEN language describing a specific system behavior, it is not a non-functional requirement — it is a functional requirement that belongs in the Requirements section with its own ACs. Move the behavioral specification to a new requirement, and reduce the NFR to the quality attribute it represents. Example: "IF a player loses connection, THEN the system SHALL reconnect" belongs in a Reconnection Handling requirement, not in an NFR. The NFR should say: "The system SHALL tolerate intermittent network connectivity without data loss."

**NFR-to-AC bridge.** After writing all requirements and before writing NFRs, draft the NFR list. Then, for each NFR: identify the concrete user-facing behavior it implies and draft at least one AC in the appropriate requirement. If the NFR mentions accessibility, draft ACs for keyboard navigation and screen reader announcements. If the NFR mentions offline support or reliability, draft ACs for what the user sees in degraded conditions. If you cannot identify a concrete behavior, the NFR may be too vague — ask the user what observable outcome they expect. An NFR without a corresponding testable AC is a wish, not a requirement.

### Mutation Ownership Check

Before approving `requirements.md`, verify every requirement that mutates a state field answers: **who is allowed to set this field, who is allowed to clear it, and who is forbidden from touching it?**

State fields include any persistent flag, status, counter, or lock that more than one actor can write. Examples: a `blocked`/`suspended` flag set by both admins and automated jobs; a `verified` flag set by a background worker and cleared by support staff; a `retry_count` incremented by a queue processor and reset by a fixer job.

For each such field, the spec MUST name:

- **The setter actor(s):** who is allowed to set the field to its active value.
- **The clearer actor(s):** who is allowed to clear it.
- **The forbidden combination:** if one actor sets it and another clears it, state explicitly whether the clearer may clear a value the setter placed — and if not, that constraint is a requirement.

A requirement that says "the system SHALL unblock the user" without specifying *which kinds of blocks* it may clear is under-specified. If two actors can both set a field to the same value but one is an admin and the other is automated, the spec MUST either:

1. Split the field into two distinct fields (one per actor), each with its own set/clear rules, or
2. Add an explicit constraint: "WHEN a [non-admin actor] clears [the field], the system SHALL NOT clear values set by [admin actor]."

**Self-check:** scan every AC that contains "set", "clear", "reset", "unblock", "block", "mark", "flag", "activate", "deactivate", "enable", or "disable." For each, identify the actor behind the mutation. If two ACs with different actors mutate the same field, verify the spec states whether one may undo the other. If it does not, the requirement is under-specified — fix it before review.

### `requirements.md` Template

```markdown
---
status: draft
approved_at:
last_modified: 2026-03-19T10:00:00Z
---

# Requirements Document

## Introduction

[Short problem statement and scope]

## Requirements

### R1 [Short title]

**User Story:** As a [role], I want [capability], so that [benefit]

#### Acceptance Criteria

1. `R1.AC1` WHEN [trigger], the system SHALL [response]
2. `R1.AC2` IF [failure trigger], THEN the system SHALL [response]

### R2 [Short title]

**User Story:** As a [role], I want [capability], so that [benefit]

#### Acceptance Criteria

1. `R2.AC1` WHILE [precondition], WHEN [trigger], the system SHALL [response]

## Non-Functional Requirements

- `NFR1` [Performance, security, accessibility, reliability, or scalability requirement]

## Constraints And Dependencies

- `C1` [Technical, team, infrastructure, or external dependency constraint]

## Out Of Scope

- [Explicitly excluded work for this iteration]
```

### Review Loop

- Draft or update `requirements.md`.
- Re-plan from Requirements if the problem statement, scope boundary, or EARS structure becomes ambiguous during review.
- Run `walden validate <feature-name> --json` before presenting for review. Read `warnings`, `ears_validation`, and `ears_distribution` from the JSON output.
- Verify the EARS Quality Rules were applied during drafting. Specifically check:
  - **Form selection**: Read `ears_distribution` for form counts. The CLI reports counts but does not validate whether forms are appropriate -- that is your responsibility. Ask: are invariants expressed as ubiquitous? Are event responses tied to specific triggers? A spec with zero ubiquitous forms may be forcing everything into event-driven.
  - **One behavior per AC**: Scan each AC response slot for "and" connecting two distinct behaviors. Split if found.
  - **Concrete triggers**: Scan event-driven ACs for generic verbs without a concrete interaction. Suggest replacements.
  - **Failure mode coverage**: If CLI warns "no unwanted-behavior criteria found", or if `ears_distribution.unwanted` is zero with multiple constraints, ask the user to consider failure modes.
  - **NFR-to-AC bridge**: For each NFR, confirm at least one AC specifies the concrete testable behavior. Flag NFRs that remain untestable.
  - **Persistence balance**: If constraints mention storage, check ACs cover both read and write sides.
  - **Mutation ownership**: Scan ACs that mutate state fields (set, clear, block, unblock, mark, reset). For each, confirm the spec names the actor and whether different actors may undo each other's mutations. Flag any field mutated by two actors without a constraint on cross-actor clearing.
  - **Domain-specific gaps**: Use the constitution and constraint list to surface missing coverage the rules above do not catch.
- Prefer `walden review open <feature-name> --phase requirements` for the deterministic state change to `in-review`.
- Ask for approval.
- After explicit approval, prefer `walden review approve <feature-name> --phase requirements` for the deterministic state change to `approved`.
- If the user corrects the scope or the validator exposes a recurring defect pattern, log a lesson before revising again with `walden lesson log ...` when available.
- Before closing the review step, report `Lesson Decision: none|logged`.
- Do not proceed to Design without explicit approval.

## Phase 2: Design

Design starts only from approved and non-stale requirements.

### Design Standard

- Apply the Decision Checkpoint Protocol during drafting.
- Read the approved requirements first.
- Research only when a design decision depends on current external facts, library behavior, or official documentation.
- Keep the design traceable to requirement IDs.
- Compare the preferred design against at least one viable alternative.
- Include `## Options Considered`, `## Simplicity And Elegance Review`, `## Failure Modes And Tradeoffs`, and `## Verification Plan`.
- Challenge the first draft once before showing it: ask whether a simpler shape, lower coupling, or fewer moving parts would satisfy the same requirements.
- Use diagrams only when they clarify decisions.
- Approve with `walden review approve`, which records the upstream approval timestamp and fingerprint (`source_requirements_approved_at`, `source_requirements_fingerprint`).
- In the Requirement Coverage table, wrap every ID in backticks (e.g., `| `R1` |`, `| `NFR1` |`). The deterministic validator matches this exact format and will reject rows without backticks.

### Single Entry Point Principle

When a design defines a shared algorithm — invoice numbering, price calculation, tax application, ID generation, slug creation, checksum computation — the design document MUST:

1. Name the single service function (or method, procedure, module function) that implements it.
2. Enumerate every call site that MUST invoke it.
3. State explicitly: "Inline computation of `<algorithm>` is forbidden. All call sites MUST call `<function>`."

This prevents the most common implementation drift: multiple call sites each re-implementing the same algorithm independently, diverging over time, and producing inconsistent results.

**Self-check:** scan the design for every named algorithm or computation. For each, confirm:
- There is a named function that owns it.
- Every place that needs the result is listed as a required call site.
- The "inline computation forbidden" constraint is stated.

If an algorithm has no named owner function, the design is incomplete. If a call site is missing from the list, the task plan will miss it.

### Batch Processing Transaction Boundaries

When a design specifies a flow that processes multiple items (records, jobs, requests, messages, users) and an error in one item MUST NOT abort the remaining items, the design MUST specify:

- **The transaction isolation mechanism:** the concrete primitive that isolates each item's processing. Framework-neutral examples:
  - SQL databases: a savepoint per item (`SAVEPOINT` / `RELEASE SAVEPOINT` / `ROLLBACK TO SAVEPOINT`).
  - Message queues: per-message acknowledgement with dead-letter routing.
  - Stream processors: per-record checkpoint offsets.
  - In-memory pipelines: per-item try/catch with the loop continuing on catch.
- **The error-handling boundary:** where the exception is caught, what state is logged, and how the loop advances to the next item.
- **The recovery semantics:** after an item fails and its transaction is rolled back, the next item starts from a clean transaction state — not from the poisoned state of the failed item.

"Continue processing remaining items" without a transaction boundary is a design gap, not an implementation concern. The implementation cannot retroactively add correct isolation if the design did not specify the boundary — it will either over-isolate (every item in its own outer transaction, killing performance) or under-isolate (catch the exception but leave the transaction poisoned).

**Self-check:** scan the design for every flow that iterates over a collection and says "continue," "skip," "proceed to next," or "do not abort." For each, confirm the design names the isolation primitive and the error boundary. If it does not, the design is incomplete.

### Field Default Annotation

The data model section MUST annotate every field that has no default and no auto-population mechanism (`auto_now_add`, `DEFAULT` in SQL, `Option<T>` with no fallback in Rust, zero-value initialization in Go). Annotate such fields with: **"must be provided explicitly on every create."**

A `create()` call that omits a no-default field fails at runtime with a constraint violation — a defect the design should have flagged as a hard requirement on the caller. This is especially dangerous in error-handling paths: a `log_job()` call inside an exception handler that omits a required field will itself fail, poisoning the transaction and masking the original error.

**Self-check:** scan every entity, struct, table, or model in the data model. For each field, check: does it have a default? Is it auto-populated? If neither, annotate it as "must be provided explicitly on every create." If a field is used in error-handling or logging paths, double-check that every creation call site in the design provides it.

### `design.md` Template

```markdown
---
status: draft
approved_at:
last_modified: 2026-03-19T10:00:00Z
approved_fingerprint:
source_requirements_approved_at:
source_requirements_fingerprint:
---

# Feature Design

## Overview

[High-level approach and key design choices]

## Architecture

[Components, boundaries, and data flow]

## Options Considered

### Option A

- Summary: [Preferred approach]
- Why chosen: [Why it is the best fit]

### Option B

- Summary: [Viable alternative]
- Why rejected: [Why it is less suitable]

## Simplicity And Elegance Review

- Simplest viable shape: [How the design minimizes moving parts]
- Coupling check: [How boundaries stay clean]
- Future-proofing: [What is intentionally deferred]

## Components And Interfaces

### [Component name]

- Purpose: [What it does]
- Inputs/Outputs: [Interface contract]
- Dependencies: [What it relies on]
- Requirements: `R1`, `R2`

## Shared Algorithms

### [Algorithm name]

- Owner function: `<module.function_name>`
- Required call sites: [exhaustive list]
- Constraint: Inline computation of `<algorithm>` is forbidden. All call sites MUST call `<function>`.

## Data Models

[Entities, schemas, state, or storage decisions. Annotate every no-default field with: "must be provided explicitly on every create."]

## Batch Processing Boundaries

[For each flow that processes multiple items with continue-on-error: name the isolation primitive, the error boundary, and the recovery semantics]

## Error Handling

[Validation, retries, failure modes, logging]

## Security Considerations

[Only when relevant]

## Failure Modes And Tradeoffs

- Failure mode: [What can go wrong]
- Mitigation: [How the system contains it]
- Tradeoff: [What was accepted and why]

## Testing Strategy

[Unit, integration, and end-to-end scope]

## Verification Plan

- Requirement proof: [How each critical requirement will be demonstrated]
- Test evidence: [Which tests or checks prove the design]
- Operational evidence: [Logs, metrics, alerts, or dashboards if relevant]

## Requirement Coverage

<!-- Every ID MUST be wrapped in backticks — the validator rejects rows without them -->
| Requirement | Covered By |
| --- | --- |
| `R1` | [Component/flow] |
| `R2` | [Component/flow] |
| `NFR1` | [Control/test/monitoring] |
```

### Review Loop

- Draft or update `design.md`.
- Re-plan from Requirements if the design exposes new scope, contradictory requirements, or missing acceptance contracts.
- Run `walden validate <feature-name>` before showing the design for approval when the CLI is available.
- Verify the design authoring rules were applied during drafting. Specifically check:
  - **Single entry point**: scan for named algorithms or shared computations. For each, confirm there is a named owner function, an exhaustive call-site list, and the "inline computation forbidden" constraint. Flag any algorithm with no named owner.
  - **Batch transaction boundaries**: scan for flows that iterate with continue-on-error. For each, confirm the design names the isolation primitive, the error boundary, and the recovery semantics. Flag any "continue processing" statement with no transaction boundary.
  - **Field default annotation**: scan every data model field. For each without a default or auto-population, confirm it is annotated "must be provided explicitly on every create." Flag unannotated no-default fields, especially those used in error-handling or logging paths.
- Prefer `walden review open <feature-name> --phase design` for the deterministic state change to `in-review`.
- Ask for approval.
- After explicit approval, prefer `walden review approve <feature-name> --phase design` for the deterministic state change to `approved`.
- If the user rejects the design or asks for a simpler approach, log the lesson before the next revision with `walden lesson log ...` when available.
- Before closing the review step, report `Lesson Decision: none|logged`.
- Do not proceed to Tasks without explicit approval.
- If requirements change, prefer `walden reconcile <feature-name>` rather than resetting downstream approval state by hand.

## Phase 3: Tasks

Task generation starts only from approved and non-stale design.

### Task Standard

- Apply the Decision Checkpoint Protocol during drafting.
- Produce only implementation tasks that write, modify, or test code.
- Use a maximum two-level hierarchy.
- Keep tasks incremental and testable.
- Reference acceptance criteria IDs (e.g., `R1.AC1`, `R1.AC2`) on every leaf task, not just parent requirement IDs.
- Reference design sections on every leaf task.
- Add a `Verification:` block on every leaf task using the structured `command` format (Kubernetes pattern). The CLI executes commands via `exec.Command` without a shell, so use JSON arrays for exact argument control.
- Optionally add a `covers:` field on proof steps to declare which acceptance criteria the proof demonstrates.
- Prefer an `expect_output` assertion on test-running proof steps so a pattern that matches zero tests cannot pass vacuously.
- Declare `timeout:` on proof steps that legitimately run long; every step is otherwise bounded by the executor's 10-minute default, and exceeding the budget is a proof failure. The CLI tracks proof reference coverage separately from task reference coverage and reports both in `walden validate --json`.
- Approve with `walden review approve`, which records the upstream approval timestamp and fingerprint (`source_design_approved_at`, `source_design_fingerprint`).

### Task Granularity: One Distinct Flow Per Task

A task that covers more than one distinct user-facing flow or behavioral unit MUST be split into separate tasks, each with its own verification line.

**Rule:** if a reviewer cannot tell from the task title alone whether a specific sub-behavior is implemented, the task is too coarse.

Examples of flows that must be separate tasks:
- A CRUD admin interface vs. a migration/reassignment page triggered by deletion.
- A list view with filters vs. a custom assignment panel on the detail page.
- A create endpoint vs. a validation rule that rejects duplicates with a specific status code.
- A form with standard fields vs. a form with a conditional dynamic sub-form.

A sub-bullet is not a task. If a behavior has its own acceptance criterion, it deserves its own task — or at minimum its own verification line that exercises that specific behavior.

**Self-check:** for each task, read the acceptance criteria it covers. If a single task covers two or more ACs that describe different user-facing flows, split it. If a task says "with migration page" or "including validation" as a sub-bullet, the sub-bullet is likely a missing task.

### Verification Strength Tiers

Every task's `Verification:` line MUST specify the strongest applicable tier. State the tier explicitly in a comment on the verification block.

**Tier 1 — Config/Build:** Commands that confirm the project loads, compiles, or type-checks.
- Examples: `go build ./...`, `tsc --noEmit`, `cargo check`, `python -c "import django; django.setup()"`, `python manage.py check`.
- Confirms: the code is syntactically valid and the project configuration is correct.
- Sufficient alone for: infrastructure tasks (config files, migrations that only add columns, dependency updates).
- **Insufficient alone for:** any task with a behavioral acceptance criterion. A Tier 1 pass does not prove the feature works — it proves the project loads.

**Tier 2 — Unit:** A named test that exercises the logic of the task in isolation.
- Examples: `go test -run TestInvoiceNumber ./internal/billing`, `pytest tests/test_invoice.py::test_assign_invoice_number`, `cargo test invoice_number`.
- Confirms: the function or module under test produces correct output for given inputs.
- Required for: any task with a computational acceptance criterion (algorithm, calculation, transformation).

**Tier 3 — Functional:** A test that exercises the user-facing flow end-to-end through the public interface.
- Examples: a test that submits a form and asserts the response, a test that calls an API endpoint and checks the status code and body, a test that triggers an admin action and verifies the database state changed.
- Confirms: the behavior the acceptance criterion describes actually happens when the user interacts with the system.
- Required for: any task with a UI, API, or admin acceptance criterion. If the AC says "the admin sees a migration page," only a Tier 3 test satisfies it.

**Mandatory tier assignment:** every leaf task MUST state its verification tier. If a task's AC describes a user-facing behavior (Tier 3) but the verification line only lists a config/build command (Tier 1), the task is under-verified — `walden validate` SHOULD reject the tasks phase, and the reviewer MUST flag it before approval.

**Anti-pattern — the Tier 1 false positive:** `python manage.py check` (or `go build`, `tsc --noEmit`, `cargo check`) passing does not mean a feature is implemented. It means the project loads. A task marked complete on Tier 1 evidence alone, when its AC requires Tier 3, is a latent defect — the feature may not exist at all, only the scaffolding does.

### Verification Format

Use the structured `command:` format (follows the Kubernetes `command` pattern):

```markdown
    - Verification:  # Tier 2
      - command: ["go", "test", "-run", "TestExample", "./pkg/example"]
```

For negative assertions (command must fail), use `expect_exit`:

```markdown
    - Verification:  # Tier 2
      - command: ["grep", "-rq", "old_pattern", "."]
        expect_exit: 1
```

For output assertions — and to prevent vacuous passes where a test pattern matches zero tests — add `expect_output`:

```markdown
    - Verification:  # Tier 2
      - command: ["go", "test", "-run", "TestExample", "./pkg/example"]
        expect_output: "--- PASS: TestExample"
```

For shell operators (pipes, &&, globbing), use the Kubernetes shell pattern:

```markdown
    - Verification:  # Tier 1
      - command: ["sh", "-c", "test -d .walden && go test ./..."]
```

Multi-step verification runs steps in order, stopping on first failure:

```markdown
    - Verification:  # Tier 3 (functional)
      - command: ["go", "build", "./..."]   # Tier 1 gate
      - command: ["go", "test", "-run", "TestAdminMigrationPage", "./internal/admin"]
```

For proof reference coverage, add `covers:` to declare which acceptance criteria a proof step demonstrates:

```markdown
    - Verification:  # Tier 2
      - command: ["go", "test", "-run", "TestAuth", "./internal/auth"]
        covers: ["R1.AC1", "R1.AC2"]
```

The CLI validates that `covers:` IDs reference known acceptance criteria and reports proof reference coverage separately from task reference coverage in the JSON output.

Per-step `timeout:` (a positive Go duration string) bounds a slow proof; steps without one run under the executor's 10-minute default, and exceeding the budget is a proof failure:

```markdown
    - Verification:  # Tier 3
      - command: ["go", "test", "-run", "TestSlowIntegration", "./internal/integration"]
        timeout: 30m
```

Prefer the read-only variant of ecosystem commands when one exists — the assertion stays, the mutation goes:

```markdown
    - Verification:  # Tier 1
      - command: ["go", "mod", "tidy", "-diff"]   # asserts tidiness, writes nothing
```

`["go", "mod", "tidy"]` would rewrite `go.mod` mid-run and fail its task as a side effect; the `-diff` form proves the same fact and keeps re-verification pure.

Legacy single-line format (`Verification: go test ./...`) still works but does not support quotes, pipes, or shell operators.

### `tasks.md` Template

```markdown
---
status: draft
approved_at:
last_modified: 2026-03-19T10:00:00Z
approved_fingerprint:
source_design_approved_at:
source_design_fingerprint:
---

# Implementation Plan

- [ ] 1. [Top-level implementation objective]
  - [ ] 1.1 [Concrete coding step — one distinct flow]
    - Requirements: `R1.AC1`, `R1.AC2`, `NFR1`
    - Design: [Relevant section]
    - Verification:  # Tier 2
      - command: ["go", "test", "-run", "TestExample", "./pkg/example"]
        covers: ["R1.AC1", "R1.AC2"]

- [ ] 2. [Next incremental objective]
  - [ ] 2.1 [Concrete coding step — one distinct flow]
    - Requirements: `R2.AC1`
    - Design: [Relevant section]
    - Verification:  # Tier 3
      - command: ["go", "test", "-run", "TestMigrationPage", "./internal/admin"]
        expect_output: "--- PASS: TestMigrationPage"
        covers: ["R2.AC1"]
```

### Review Loop

- Draft or update `tasks.md`.
- Re-plan from Design if the implementation sequence exposes missing architecture, missing interfaces, or untestable steps.
- Run `walden validate <feature-name>` before showing the task plan for approval when the CLI is available.
- Verify the task authoring rules were applied during drafting. Specifically check:
  - **One flow per task**: scan each task for sub-bullets that describe distinct user-facing flows. If a task covers two flows (e.g., "CRUD admin with migration page"), split it. If a behavior has its own AC, it deserves its own task or verification line.
  - **Verification tier match**: for each task, read the AC it covers. If the AC describes a user-facing behavior (UI, API, admin action), confirm the verification is Tier 3 (functional), not just Tier 1 (config/build). Flag any task with a Tier 3 AC and only Tier 1 verification as under-verified.
  - **No Tier 1-only tasks for behavioral ACs**: if a task's AC says "the user sees X" or "the admin can do Y" and the only verification is a build/check command, the task is under-verified and must be rejected before approval.
- Prefer `walden review open <feature-name> --phase tasks` for the deterministic state change to `in-review`.
- Ask for approval.
- After explicit approval, prefer `walden review approve <feature-name> --phase tasks` for the deterministic state change to `approved`.
- If the user corrects sequencing or coverage, log a lesson before revising again with `walden lesson log ...` when available.
- Before closing the review step, report `Lesson Decision: none|logged`.
- Stop after approval. Do not start implementation unless the user explicitly asks.

## Phase 4: Execute

Execution is for approved specs only.

### Execution Standard

- Read `requirements.md`, `design.md`, and `tasks.md` before writing code.
- Use `walden task status <feature-name>` to verify that execution is allowed and to resolve the next runnable task when the CLI is available.
- For non-trivial implementation work or a requested batch, start with a short execution plan and the verification steps you will use.
- If the user names a task, execute only that task unless they explicitly request a batch.
- If the user does not name a task, use `walden task status <feature-name>` to identify the next unchecked task and wait for confirmation before implementing.
- Use `walden task start <feature-name> [task-id]` to obtain normalized execution context before writing code.
- Complete sub-tasks before their parent task.
- Write the minimum production code needed for the requested task.
- Write thorough tests for the task.
- Run targeted tests for the changed area. Do not run the full suite unless the user asks.
- Treat the task's `Verification:` line as mandatory proof. Prefer `walden task complete <feature-name> <task-id>` so proof execution and checkbox mutation remain deterministic.
- Task completion records execution evidence in `.walden/evidence/<feature-name>.json`; commit it with the work — it is shared repository state, reviewed like the specs it proves.
- If `walden task status` warns that completed tasks are no longer verified (stale-spec, stale-code, failed, unrecorded), run `walden verify <feature-name>` before building on top of them; never dismiss an evidence warning.
- If a test fails or the proof is weaker than expected, stop and re-plan instead of hand-waving the result.
- Before closing the execution step, report `Lesson Decision: none|logged`.
- Stop after the requested task or batch and wait for review.

### Spec Drift

- If implementation reveals a gap in the approved spec, pause execution.
- Update the earliest affected document.
- Re-run the approval gate from that phase forward.
- Prefer `walden reconcile <feature-name>` when upstream approval metadata or freshness is no longer valid.
- After reconciliation and re-approval, run `walden verify <feature-name>`: completed tasks whose evidence went stale-spec must be re-proven against the updated contract, not assumed.
- Log a lesson if the gap came from a missed pattern, missing guardrail, or design blind spot.
- Do not silently rewrite approved requirements or design during implementation.

## Release Certification

- When the user asks whether the work is releasable — or before any tag, release branch, or delivery hand-off — run `walden release check` and report its verdict; do not assemble the answer from separate status checks.
- The gate certifies and never releases: approved fresh chains, full-spec validation, decision markers in approved documents, execution evidence, and a clean worktree outside `.walden/` fold into one exit code. Tags, changelogs, and publishing stay with you and the user, after certification passes.
- Read a failed certification as a work list: every blocker names its remedy. Apply the remedies and rerun the gate; never edit state by hand to silence a blocker.
- Pending leaf tasks block certification by default: the plan is a promise the release must keep or visibly defer. The only relaxation is `--allow-pending --reason "<text>"`, which waives them for that verdict and records the reason and the waived task ids in the output. **Never pass `--allow-pending` unless the user explicitly approves the waiver and its reason in the current conversation** — a waiver is the user's recorded decision, not a convenience; report the waived tasks back after the run.
- The verdict names the certified commit and a completion class — `complete` when every planned leaf task is executed, `with-pending` when pending work blocks, `with-waivers` when it was explicitly waived; JSON carries `certified_commit`, `completion`, and the `waiver` record for pipeline policy.
- `--strict` requires committed `.walden/` state — commit specs and evidence before a final certification; it composes with a waiver (committed state stays required, pending stays waived).
- The dirty-worktree blocker has no bypass by design: the remedy is committing the work. Do not look for a flag. Certification also fails closed without usable git — a verdict must name the code identity it certified — and on unterminated HTML comments in approved documents.
- Compose production and judgment: `walden verify <feature-name>` re-proves execution, then `walden release check` judges the result. In CI, gate the pipeline on the exit code and use `--json` for structure.

## Brownfield Adoption

- When a repository carries specs that predate the current contract — approved documents without approval fingerprints (stale chains, `walden verify` gate-blocked) or completed tasks without evidence (`unrecorded` blockers) — use `walden adopt`, not manual reconciliation: reconcile-and-re-approve ceremony across a portfolio is exactly what the lane eliminates.
- Always run the read-only plan first and present it to the user before `--apply`. The plan classifies every feature: `backfill` (approved documents to seal), `re-prove` (fresh chain, evidence to record), `complete` (nothing to adopt), `blocked` (a present fingerprint contradicts the content — human reconcile territory; adopt never writes there).
- Sealing trusts recorded approvals: it stamps the fingerprint of the document's current body under the approval already recorded. State this assumption when presenting the plan — an edit made between the old approval and the seal is invisible to pre-fingerprint history, and the seal grandfathers it in.
- `--apply` seals, then re-proves through the verify machinery. The verified/failed partition is the honest work list: failures record real evidence with execution profiles, so environment drift is diagnosable per task. Rerunning `--apply` resumes — verified tasks are skipped, failed ones retry.
- The adoption diff (sealed documents, new evidence ledgers) is ordinary repository state: review and commit it like any other change. Exit code 1 means the partition contains failures to triage, not that adoption must be repeated from scratch.
- When the failed partition reflects a superseded product generation rather than broken code, those specs are candidates for retirement, not repair: delete their directories (history lives in git) and record each in `.walden/RETIRED.md` — one line naming the retirement commit and the successor. The `walden-history` companion skill officiates the ceremony and narrates the history it preserves.

## Environment Probes

- `.walden/environment.md` declares named probes — commands whose trimmed output joins every evidence record's execution profile, alongside the always-present `platform` and `walden` (CLI version) keys:

```markdown
# Environment Probes

- go: ["go", "version"]
- node: ["node", "--version"]
```

- Declare probes for the toolchains the project's proofs depend on when initializing or adopting a repository; prefer commands that print stable version strings — nondeterministic output (timestamps, paths) reads as permanent drift.
- Probe names are lowercase kebab; `platform` and `walden` are reserved. A malformed declaration fails evidence-producing commands loudly; a failing or hung probe degrades to a marker value (`probe failed: …`, `probe timed out`) and never blocks the run.
- Read drift before blaming code: `walden evidence status` prints recorded-versus-current profile differences, and a failed re-verification appends `environment drift: go: recorded "go1.25.0" → current "go1.24.0"` to the failure. Fix the environment (or knowingly re-record on the current one); never edit proofs to paper over drift.
- Profiles are diagnostic only: they never change a derived evidence state, and records written before profiles existed read as `legacy record: no profile`.

## Self-Improvement Loop

- Review `.walden/lessons.md` before non-trivial work when earlier patterns are relevant.
- After any user correction, failed validation, rejected design, or execution surprise, append a lesson with `walden lesson log ...` when available.
- Treat these as automatic lesson triggers: user correction, failed validation, rejected draft, explicit simplification request, re-plan, failed test caused by a wrong assumption, or spec gap discovered during execution.
- Record three things in every lesson: the trigger, the mistake pattern, and a guardrail that would have prevented it.
- Apply the new guardrail in the next revision before presenting it.
- If no trigger occurred, still make and report the explicit decision: `Lesson Decision: none`.

## Output Standards

- Be concise, decisive, and developer-to-developer.
- Explain the reasoning behind recommendations when it matters.
- Prefer small examples over long exposition.
- Keep production code minimal and tests thorough.
- Cite sources in the design phase when external research informed a decision.
- In every phase summary, include `Lesson Decision: none` or `Lesson Decision: logged`.
