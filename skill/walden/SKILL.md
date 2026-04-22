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
- Use `walden validate <feature-name> [--all] [--json]` before phase transitions and before execution.
- Use `walden review open <feature-name> --phase requirements|design|tasks` and `walden review approve <feature-name> --phase requirements|design|tasks` for deterministic review-state transitions.
- Use `walden task status <feature-name> [--json]`, `walden task start <feature-name> [task-id] [--json]`, and `walden task complete <feature-name> <task-id> [--json]` for deterministic execution flow.
- Use `walden task complete-all <feature-name> [--json]` to complete all runnable leaf tasks in order, stopping on first failure.
- Use `walden reconcile <feature-name> [--json]` when approved upstream documents changed or the approval chain is stale.
- Use `walden lesson log --feature <feature-name> --phase requirements|design|tasks|execute|release --trigger "<event>" --lesson "<pattern>" --guardrail "<rule>" [--json]` after meaningful corrections, failed validation, or execution surprises.
- Use `walden version [--json]` to check the installed CLI version and schema version.

All `--json` commands return a versioned envelope:

```json
{
  "schema_version": "v0alpha1",
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
---
```

### `design.md`

```yaml
---
status: draft
approved_at:
last_modified: 2026-03-19T10:00:00Z
source_requirements_approved_at:
---
```

### `tasks.md`

```yaml
---
status: draft
approved_at:
last_modified: 2026-03-19T10:00:00Z
source_design_approved_at:
---
```

Apply these rules consistently:

- Set `status: draft` when first creating a document.
- Set `status: in-review` immediately before presenting a revision to the user.
- Set `status: approved` and populate `approved_at` only after explicit approval.
- Update `last_modified` on every edit.
- If an approved document is edited later, set it back to `in-review` until it is re-approved.
- When approving `design.md`, copy the current `requirements.md.approved_at` into `source_requirements_approved_at`.
- When approving `tasks.md`, copy the current `design.md.approved_at` into `source_design_approved_at`.
- If `requirements.md.approved_at` no longer matches `design.md.source_requirements_approved_at`, `design.md` and `tasks.md` are stale.
- If `design.md.approved_at` no longer matches `tasks.md.source_design_approved_at`, `tasks.md` is stale.

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

**On TRUE — checkpoint detected:** emit `[decision: <question>]` in the document, explain the fork in plain language, and stop generating further content. Wait for the user's response. On receiving a response, state how the answer will be applied to the document before resuming content generation in the same conversation turn. If the user's response surfaces a previously unidentified bifurcation-significant decision, emit a new `[decision: <question>]` marker for the newly identified fork before generating content that depends on it.

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
- Record the current `requirements.md.approved_at` in `source_requirements_approved_at` when the design is approved.
- In the Requirement Coverage table, wrap every ID in backticks (e.g., `| `R1` |`, `| `NFR1` |`). The deterministic validator matches this exact format and will reject rows without backticks.

### `design.md` Template

```markdown
---
status: draft
approved_at:
last_modified: 2026-03-19T10:00:00Z
source_requirements_approved_at:
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

## Data Models

[Entities, schemas, state, or storage decisions]

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
- Optionally add a `covers:` field on proof steps to declare which acceptance criteria the proof demonstrates. The CLI tracks proof reference coverage separately from task reference coverage and reports both in `walden validate --json`.
- Record the current `design.md.approved_at` in `source_design_approved_at` when the task list is approved.

### Verification Format

Use the structured `command:` format (follows the Kubernetes `command` pattern):

```markdown
    - Verification:
      - command: ["go", "test", "-run", "TestExample", "./pkg/example"]
```

For negative assertions (command must fail), use `expect_exit`:

```markdown
    - Verification:
      - command: ["grep", "-rq", "old_pattern", "."]
        expect_exit: 1
```

For shell operators (pipes, &&, globbing), use the Kubernetes shell pattern:

```markdown
    - Verification:
      - command: ["sh", "-c", "test -d .walden && go test ./..."]
```

Multi-step verification runs steps in order, stopping on first failure:

```markdown
    - Verification:
      - command: ["go", "build", "./..."]
      - command: ["go", "test", "./..."]
```

For proof reference coverage, add `covers:` to declare which acceptance criteria a proof step demonstrates:

```markdown
    - Verification:
      - command: ["go", "test", "-run", "TestAuth", "./internal/auth"]
        covers: ["R1.AC1", "R1.AC2"]
```

The CLI validates that `covers:` IDs reference known acceptance criteria and reports proof reference coverage separately from task reference coverage in the JSON output.

Legacy single-line format (`Verification: go test ./...`) still works but does not support quotes, pipes, or shell operators.

### `tasks.md` Template

```markdown
---
status: draft
approved_at:
last_modified: 2026-03-19T10:00:00Z
source_design_approved_at:
---

# Implementation Plan

- [ ] 1. [Top-level implementation objective]
  - [ ] 1.1 [Concrete coding step]
    - Requirements: `R1.AC1`, `R1.AC2`, `NFR1`
    - Design: [Relevant section]
    - Verification:
      - command: ["go", "test", "-run", "TestExample", "./pkg/example"]
        covers: ["R1.AC1", "R1.AC2"]

- [ ] 2. [Next incremental objective]
  - [ ] 2.1 [Concrete coding step]
    - Requirements: `R2.AC1`
    - Design: [Relevant section]
    - Verification:
      - command: ["grep", "-rq", "old_pattern", "."]
        expect_exit: 1
        covers: ["R2.AC1"]
```

### Review Loop

- Draft or update `tasks.md`.
- Re-plan from Design if the implementation sequence exposes missing architecture, missing interfaces, or untestable steps.
- Run `walden validate <feature-name>` before showing the task plan for approval when the CLI is available.
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
- If a test fails or the proof is weaker than expected, stop and re-plan instead of hand-waving the result.
- Before closing the execution step, report `Lesson Decision: none|logged`.
- Stop after the requested task or batch and wait for review.

### Spec Drift

- If implementation reveals a gap in the approved spec, pause execution.
- Update the earliest affected document.
- Re-run the approval gate from that phase forward.
- Prefer `walden reconcile <feature-name>` when upstream approval metadata or freshness is no longer valid.
- Log a lesson if the gap came from a missed pattern, missing guardrail, or design blind spot.
- Do not silently rewrite approved requirements or design during implementation.

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
