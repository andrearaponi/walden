## Walden v0.8.0

The external review of the kernel closed with one sentence: *"not yet a complete proof that the current implementation satisfies the current specification."*

This release retires it.

### Done is now a provable claim

Every task completion records durable evidence in `.walden/evidence/<feature>.json`: the proof's steps — argv, exit codes, output digests — bound to the approved chain fingerprints and to a deterministic **code identity** of the working tree. The checkbox in `tasks.md` stays the human-readable projection; the ledger is the authoritative execution state, committed and reviewed like the specs it proves.

States are **derived at read time, never stored**:

| State | Meaning |
| --- | --- |
| `verified` | The recorded proof matches the current specs and the current code |
| `stale-spec` | Requirements, design, or this task's definition moved since the proof ran |
| `stale-code` | The working tree moved since the proof ran |
| `failed` | The last re-verification failed |
| `unrecorded` | Completed before the ledger existed — `walden verify` upgrades it |
| `pending` | Not completed yet |

```bash
walden verify <feature>            # re-prove what is no longer verified
walden verify <feature> --all      # re-prove everything
walden verify <feature> --check    # CI mode: report only, write nothing
walden evidence status <feature>   # derived states, always exit 0
```

Delete the implementation after completing a task, and `task status` stops claiming the plan is complete. Change the requirements, reconcile, re-approve — the checkboxes stay checked, and the evidence honestly reads `stale-spec` until proofs rerun. Those were the review's two reproductions; both now fail as they should.

### The identity that survives its own commit

The code identity is a uniform path→(mode, blob) map — `git ls-tree` for committed entries, batched `git hash-object` for dirty ones, symlinks as their target string, modes canonicalized, `.walden/` excluded at every layer. Committing the evidence never invalidates the evidence; moving a file from untracked to committed never reads as a change; a `chmod +x` does. No usable git? The identity degrades to a warned absence instead of noise.

### Proofs that cannot pass vacuously

`expect_output` requires a proof step's output to contain declared content — a test pattern matching zero tests can no longer complete a task:

```markdown
- Verification:
  - command: ["go", "test", "-run", "TestAuth", "./internal/auth/"]
    expect_output: "--- PASS: TestAuth"
```

### Bugfixes still need no ceremony

A code-only fix outside the workflow marks evidence `stale-code` everywhere — the honest global doubt — and one `verify` either restores `verified` or names the exact task whose proof broke. The no-ceremony lane stays; it is just no longer blind.

### Battle-tested before shipping

Three eval rounds across four real repositories — a todo app and a 26-feature corporate portfolio with its Go sibling — caught a main package that was never committed, a lockfile that breaks every fresh clone, a placeholder frontend committed over the proven build, and one flaky proof. Zero false positives. Every kernel defect the evals found was fixed on the branch with a regression test, and the whole battery now runs as compiled end-to-end tests in CI on every change.

### Upgrading

```bash
walden update
```

Existing completions report `unrecorded`; one `walden verify <feature> --all` migrates them. One walden version per repository while the evidence schema (`v1alpha1`) is alpha.

### Built the Walden way

7 requirements, 30 EARS acceptance criteria, 13 tasks completed through `walden task complete`, three mid-execution design amendments handled with reconcile and re-approval, five lessons logged — and the feature's own evidence ledger was dogfooded on the very spec that built it.
