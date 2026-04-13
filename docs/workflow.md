# Workflow

This document walks through the complete Walden workflow from repository bootstrap to lesson logging.

## 1. Bootstrap

Initialize Walden in an existing or new repository:

```bash
walden repo init
```

This creates `.walden/` (including an optional `constitution.md` for project-wide context), `.github/workflows/`, and helper files. If Git is not initialized, the CLI initializes it first.

Then scaffold a feature:

```bash
walden feature init user-auth
```

This creates `.walden/specs/user-auth/` with `requirements.md`, `design.md`, and `tasks.md` in draft status.

## 2. Requirements

Edit `requirements.md` with:
- A problem statement
- User stories for context
- EARS acceptance criteria with stable IDs (`R1.AC1`, `R1.AC2`)
- Non-functional requirements (`NFR1`, `NFR2`)
- Constraints (`C1`, `C2`)
- Out-of-scope items

When ready, validate and open review:

```bash
walden validate user-auth
walden review open user-auth --phase requirements
```

After the reviewer approves:

```bash
walden review approve user-auth --phase requirements
```

## 3. Design

With approved requirements, edit `design.md`:
- Architecture and component boundaries
- At least one alternative option considered
- Simplicity review
- Failure modes and tradeoffs
- Testing strategy
- Requirement coverage matrix

Validate, review, and approve:

```bash
walden validate user-auth
walden review open user-auth --phase design
walden review approve user-auth --phase design
```

## 4. Tasks

With approved design, edit `tasks.md`:
- Two-level task hierarchy
- Every leaf task references acceptance criteria IDs (e.g., `R1.AC1`) and design sections
- Every leaf task has a `Verification:` proof

```markdown
- [ ] 1. Implement authentication service
  - [ ] 1.1 Add password hashing utility
    - Requirements: `R1.AC1`, `R1.AC2`, `NFR2`
    - Design: Authentication Service
    - Verification:
      - command: ["go", "test", "./internal/auth/..."]
```

Validate, review, and approve:

```bash
walden validate user-auth
walden review open user-auth --phase tasks
walden review approve user-auth --phase tasks
```

## 5. Execute

Check readiness and start working:

```bash
walden task status user-auth
walden task start user-auth
```

Implement the task, then complete it with proof:

```bash
walden task complete user-auth 1.1
```

The CLI runs the verification proof. If it passes, the task is marked complete. If it fails, the task stays unchecked and you fix the issue first.

To complete all remaining tasks in order:

```bash
walden task complete-all user-auth
```

This stops on the first failing proof while preserving earlier completions.

## 6. Reconcile

If you edit an approved document, downstream documents become stale. Reconcile before continuing:

```bash
walden reconcile user-auth
```

This resets stale downstream documents to draft and updates approval metadata.

## 7. Lessons

After a correction, failed validation, or execution surprise:

```bash
walden lesson log \
  --feature user-auth \
  --phase execute \
  --trigger "test failed because mock diverged from real database" \
  --lesson "integration tests must hit a real database" \
  --guardrail "before approving design, confirm test strategy uses real dependencies"
```

Lessons are appended to `.walden/lessons.md` and reviewed before similar future work.

## Status and Validation

At any point, check where you are:

```bash
walden status user-auth
walden validate user-auth
```

For machine-readable output:

```bash
walden status user-auth --json
walden validate user-auth --json
```

## JSON Contract

All `--json` commands return a versioned envelope:

```json
{
  "schema_version": "v0alpha1",
  "command": "status",
  "ok": true,
  "result": {
    "summary": "workflow status for user-auth",
    "current_phase": "tasks",
    "next_action": "Start execution from the next unchecked task"
  }
}
```
