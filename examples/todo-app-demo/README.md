# Todo App Demo

This example demonstrates the complete Walden workflow on a minimal todo application. It is designed to run from a clean checkout without any external dependencies beyond the `walden` CLI.

## Prerequisites

- `walden` installed and in `PATH`
- A shell (bash or zsh)

## What This Example Contains

```text
.walden/
  specs/todo-app/
    requirements.md   # Approved requirements with EARS criteria
    design.md         # Approved design with component boundaries
    tasks.md          # Approved tasks with verification proofs
src/
  todo.sh             # Minimal todo CLI implementation
tests/
  test_todo.sh        # Shell-based tests
scripts/
  verify.sh           # Verification wrapper for task proofs
```

## Walk Through The Workflow

### 1. Check Status

```bash
walden status todo-app
```

All three documents are approved and fresh, so execution is ready.

### 2. Inspect Execution Readiness

```bash
walden task status todo-app
```

### 3. Start and Complete a Task

```bash
walden task start todo-app
# The CLI shows the next task, its requirements, and verification proof
walden task complete todo-app 1.1
```

### 4. Complete All Remaining Tasks

```bash
walden task complete-all todo-app
```

### 5. Run Tests Directly

```bash
scripts/verify.sh 1.1
```

## Key Patterns

- **Verification proofs use wrapper scripts** — `scripts/verify.sh` takes a task ID and runs the appropriate check. This avoids shell-quoting issues in task definitions.
- **Tasks reference requirements and design** — every leaf task traces back to its source requirement and design section.
- **Documents are approved** — this example ships with all phases approved so you can focus on the execution flow.

## Evidence

Every completion above records execution evidence in `.walden/evidence/todo-app.json`, bound to the spec fingerprints and to a code identity of this directory. Try it:

```bash
walden evidence status todo-app    # verified / stale-code / unrecorded per task
walden verify todo-app --check     # re-run every proof against the current code, write nothing
```

Break something the proofs cover, and `walden task status` stops claiming the plan is complete.
