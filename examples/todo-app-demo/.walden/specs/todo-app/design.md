---
status: approved
approved_at: 2026-03-22T12:10:00Z
last_modified: 2026-03-22T12:10:00Z
source_requirements_approved_at: 2026-03-22T12:00:00Z
---

# Feature Design

## Overview

A single shell script (`src/todo.sh`) implements add, list, and complete subcommands. Data is stored in a plain text file (`todos.txt`) with one item per line using the format `STATUS|DESCRIPTION`.

## Architecture

The script uses a case statement to route subcommands. Each operation reads and writes `todos.txt` directly.

## Options Considered

### Option A

- Summary: Single shell script with flat file storage.
- Why chosen: Simplest possible implementation that satisfies all requirements with POSIX utilities only.

### Option B

- Summary: Separate scripts per subcommand with a shared data module.
- Why rejected: Adds file coordination overhead for no benefit at this scale.

## Simplicity And Elegance Review

- Simplest viable shape: One script, one data file, three operations.
- Coupling check: No external dependencies. Data format is trivial to inspect and debug.
- Future-proofing: Intentionally deferred. This is a demo, not a production application.

## Components And Interfaces

### todo.sh

- Purpose: CLI entrypoint for all todo operations.
- Inputs/Outputs: Subcommand and arguments on stdin, human-readable output on stdout, exit code 0 on success, 1 on error.
- Dependencies: POSIX shell, `sed`, `cat`, `wc`.
- Requirements: `R1`, `R2`, `R3`, `NFR1`

## Data Models

```text
TODO|Buy groceries
DONE|Write tests
TODO|Read documentation
```

Each line is `STATUS|DESCRIPTION` where STATUS is `TODO` or `DONE`.

## Error Handling

- Missing data file: created on first `add`.
- Invalid index on `complete`: error message and exit code 1.
- No arguments: usage message and exit code 1.

## Failure Modes And Tradeoffs

- Failure mode: Data file corruption from concurrent writes.
- Mitigation: Not addressed. Single-user demo scope only.
- Tradeoff: Accepted for simplicity; a real application would use file locking.

## Testing Strategy

- Shell-based tests in `tests/test_todo.sh` that exercise each subcommand.
- Verification wrapper in `scripts/verify.sh` for task proof execution.

## Verification Plan

- Requirement proof: each subcommand is tested by at least one test case.
- Test evidence: `scripts/verify.sh` runs targeted tests per task.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| `R1` | add subcommand, test_add |
| `R2` | list subcommand, test_list |
| `R3` | complete subcommand, test_complete |
| `NFR1` | POSIX-only implementation |
