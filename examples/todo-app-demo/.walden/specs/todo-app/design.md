---
walden_schema_version: v1alpha1
status: approved
approved_at: 2026-11-25T16:15:06Z
last_modified: 2026-11-25T16:15:06Z
approved_fingerprint: sha256:e90643745dda28c570552f3a7fcaf0a6c8eaffe2b991a371b62639628812f476
source_requirements_approved_at: 2026-07-10T17:57:14Z
source_requirements_fingerprint: sha256:a3da471ec0241fe0027c1b3be7ad775dc8c5f6caa1d51ebb9070d26b51ca0530
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

- Shell tests in `tests/test_todo.sh`, one for each acceptance criterion. Each runs in an empty temporary directory and can be run alone by name.

## Verification Plan

- Requirement proof: each acceptance criterion has a test of its own, and each proof step of the plan runs one test and names the criterion it covers.
- Test evidence: a proof step passes when its test prints `PASS:` followed by its name.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| `R1` | add subcommand, test_add and test_add_creates_file |
| `R2` | list subcommand, test_list and test_list_empty |
| `R3` | complete subcommand, test_complete and test_complete_out_of_range |
| `NFR1` | POSIX-only implementation |
