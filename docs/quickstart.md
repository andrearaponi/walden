# Quickstart

One real feature, from an empty repository to a releasable verdict. Ten minutes, no AI required — the CLI is the whole mechanism; an agent skill can author the documents for you later, but everything below is what actually happens underneath.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
walden version
```

## Initialize

```bash
git init my-project && cd my-project
walden repo init
```

This creates `.walden/` (a `constitution.md` for stable project context, a `lessons.md`, a scoped `.gitignore`) and a generated CI workflow (`.github/workflows/validate-walden.yml`) pinned to the version that generated it.

```bash
walden feature init "Greeting Service"
```

Feature names normalize to kebab-case: the spec lives in `.walden/specs/greeting-service/` as three draft documents — `requirements.md`, `design.md`, `tasks.md`.

## Phase 1 — Requirements

Edit `.walden/specs/greeting-service/requirements.md`. Keep the frontmatter; replace the template body:

```markdown
# Requirements Document

## Introduction

A command that greets a named user, as the smallest end-to-end slice of the CLI.

## Requirements

### R1 Greeting

**User Story:** As a user, I want a greeting by name, so that I know the CLI sees my input.

#### Acceptance Criteria

1. `R1.AC1` WHEN the user runs `greet <name>`, the system SHALL print `Hello, <name>!`.
2. `R1.AC2` IF `<name>` is empty, THEN the system SHALL exit non-zero naming the missing argument.

## Non-Functional Requirements

- `NFR1` The command SHALL complete in under one second. (bridged by `R1.AC1`)

## Constraints And Dependencies

- `C1` Standard library only.

## Out Of Scope

- Localization.
```

Validate, then walk the review gate:

```bash
walden validate greeting-service
walden review open greeting-service --phase requirements
walden review approve greeting-service --phase requirements
```

Approval does more than flip a status: it records a SHA-256 **fingerprint of the approved body** in the frontmatter. From now on, any edit to this content is detectable — and stales everything built on top of it. This is the first link of the [approval chain](lifecycle.md#the-seal).

## Phase 2 — Design

Edit `design.md` (architecture, at least one alternative considered, failure modes, testing strategy, a requirement coverage table — the validator checks the structure), then:

```bash
walden validate greeting-service
walden review open greeting-service --phase design
walden review approve greeting-service --phase design
```

The design's approval binds to the requirements' fingerprint. If requirements change later, the design is stale by construction, not by convention.

## Phase 3 — Tasks

Edit `tasks.md`: a two-level plan where every leaf task names the acceptance criteria it implements, the design section it follows, and — the part that matters — an executable **proof**:

```markdown
# Implementation Plan

- [ ] 1. Greeting command
  - [ ] 1.1 Implement greet with argument validation
    - Requirements: `R1.AC1`, `R1.AC2`, `NFR1`
    - Design: Command Surface
    - Verification:
      - command: ["go", "test", "-run", "TestGreet", "./..."]
        expect_output: "--- PASS: TestGreet"
        covers: ["R1.AC1", "R1.AC2"]
```

```bash
walden validate greeting-service
walden review open greeting-service --phase tasks
walden review approve greeting-service --phase tasks
```

## Execute

Write the code and the test, then complete the task — the CLI runs the proof and refuses the completion if it fails:

```bash
walden task status greeting-service     # readiness and next runnable task
walden task complete greeting-service 1.1
```

A passing completion checks the box **and** writes a record to `.walden/evidence/greeting-service.json`: the proof's steps and outcome, bound to the approved spec fingerprints and to a digest of the working tree. "Done" is now a claim you can re-prove.

## Trust, later

Weeks pass, code changes. Ask what still holds:

```bash
walden evidence status greeting-service   # derived state per task, ~milliseconds, never writes
walden verify greeting-service            # re-runs proofs for anything no longer verified
```

States are **derived at read time** by comparing recorded fingerprints against the present — `verified`, `stale-spec`, `stale-code`, `failed`, `unrecorded`, `pending`. Nothing ever stores "stale" on disk; the comparison is recomputed every time you ask.

## Certify

```bash
walden release check
```

One deterministic verdict: approved fresh chains, full-spec validation, no unresolved `[decision:]` markers, execution evidence verified, pending work executed or explicitly waived, clean worktree. Exit `0` if and only if no blocker exists — and every blocker names its remedy. The gate executes no proofs and writes nothing: `verify` produces evidence, `release check` judges it.

```text
Summary: RELEASABLE — 1 feature(s) certified, completion complete, commit 3f2a91c40d77
```

## Where to next

- The mechanism behind each step: [The Spec Lifecycle](lifecycle.md)
- The full command loop, including reconciliation and lessons: [The Daily Workflow](workflow.md)
- An existing repository with specs that predate Walden's current contract: [Brownfield Adoption](adoption.md)
