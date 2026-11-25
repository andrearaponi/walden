# Todo App Demo

A minimal todo application with a complete Walden specification. It shows what `walden trace` prints: each statement of the spec beside the proof that covers it, and whether that proof still stands.

You need git, a POSIX shell and the `walden` CLI, v0.16.1 or newer ([install](../../README.md#install)).

```bash
git clone https://github.com/andrearaponi/walden.git
cd walden/examples/todo-app-demo
```

## What it holds

```text
.walden/specs/todo-app/
  requirements.md   # six acceptance criteria in EARS form
  design.md         # one shell script, one data file
  tasks.md          # three tasks, one proof step for each criterion
src/todo.sh         # the todo CLI: add, list, complete
tests/test_todo.sh  # one test for each criterion, run alone by name
```

The three documents are approved, and no task is completed: the proofs have not run on your machine yet.

```bash
walden status todo-app
```

## Run the proofs

```bash
walden task complete-all todo-app
```

Each task runs its proof steps, and its box in the plan is checked only when they all pass. What each proof did goes to `.walden/evidence/todo-app.json`, the evidence ledger. Completing the tasks writes it on your machine: it is not part of this repository, which ignores it.

## Read the trace

```bash
walden trace todo-app --markdown
```

<!-- trace:delivered -->

| Statement | Named by | Proof step | Evidence | Command |
| --- | --- | --- | --- | --- |
| `R1.AC1` | 1 | 1 step 1 | verified | `sh tests/test_todo.sh add` |
| `R1.AC2` | 1 | 1 step 2 | verified | `sh tests/test_todo.sh add_creates_file` |
| `R2.AC1` | 2 | 2 step 1 | verified | `sh tests/test_todo.sh list` |
| `R2.AC2` | 2 | 2 step 2 | verified | `sh tests/test_todo.sh list_empty` |
| `R3.AC1` | 3 | 3 step 1 | verified | `sh tests/test_todo.sh complete` |
| `R3.AC2` | 3 | 3 step 2 | verified | `sh tests/test_todo.sh complete_out_of_range` |
| `C1` | 1 | 1 step 2 | verified | `sh tests/test_todo.sh add_creates_file` |

Holes: none

<!-- /trace:delivered -->

One row for each statement of the requirements that the plan names: the task that names it, the proof step that covers it, the state of that task's evidence and the command of the step.

## Break something

Make a typo in `src/todo.sh`: in `cmd_complete`, replace the first line with the second.

```diff
-    sed -i.bak "${index}s/^TODO/DONE/" "$DATA_FILE" && rm -f "${DATA_FILE}.bak"
+    sed -i.bak "${index}s/^TODO/DOEN/" "$DATA_FILE" && rm -f "${DATA_FILE}.bak"
```

The code changed, so no proof stands on it until it runs again:

```bash
walden verify todo-app
```

The run fails and names task 3, the one whose proofs exercise `complete`. The trace shows which statements are no longer proven:

```bash
walden trace todo-app --markdown
```

<!-- trace:after-change -->

| Statement | Named by | Proof step | Evidence | Command |
| --- | --- | --- | --- | --- |
| `R1.AC1` | 1 | 1 step 1 | verified | `sh tests/test_todo.sh add` |
| `R1.AC2` | 1 | 1 step 2 | verified | `sh tests/test_todo.sh add_creates_file` |
| `R2.AC1` | 2 | 2 step 1 | verified | `sh tests/test_todo.sh list` |
| `R2.AC2` | 2 | 2 step 2 | verified | `sh tests/test_todo.sh list_empty` |
| `R3.AC1` | 3 | 3 step 1 | failed | `sh tests/test_todo.sh complete` |
| `R3.AC2` | 3 | 3 step 2 | failed | `sh tests/test_todo.sh complete_out_of_range` |
| `C1` | 1 | 1 step 2 | verified | `sh tests/test_todo.sh add_creates_file` |

Holes: 2 (0 unnamed, 0 uncovered, 2 not-standing)

- `R3.AC1`: 3 step 1 is failed
- `R3.AC2`: 3 step 2 is failed

<!-- /trace:after-change -->

Both criteria of task 3 read `failed`: the state is the one of the task, and a task is proven when all its steps pass.

## Put it back

```bash
git checkout src/todo.sh
walden verify todo-app
walden trace todo-app
```

The three tasks are proven again and the trace has no hole.

To return the example to the state it shipped in, discard what the commands wrote:

```bash
git checkout .
rm -rf .walden/evidence
```
