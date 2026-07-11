## Walden v0.9.0

Nobody releases a task. You release a repository.

v0.8.0 made every execution claim provable — but the truth stayed scattered: freshness in `status`, structure in `validate`, execution in `evidence status`. This release folds it into the one question those commands exist to answer:

```bash
walden release check
```

**Is this releasable, right now?** One deterministic verdict, one exit code.

### The criteria

| Criterion | Blocks when |
| --- | --- |
| `chain` | any document is unapproved or stale |
| `validation` | full-spec validation fails |
| `decisions` | an approved document carries an unresolved `[decision:]` marker |
| `evidence` | a completed task is not `verified` on the current tree |
| `worktree` | uncommitted changes exist outside `.walden/` |

All criteria always evaluate — no short-circuiting. A failed certification is a complete work list, and **every blocker names its remedy**: `run walden verify <feature>`, `commit it before certifying`, `resolve it and re-approve`.

### Judge, never execute

`release check` runs no proofs and writes nothing. `verify` produces evidence; `release check` judges it. That split keeps certification at milliseconds — a 26-feature production portfolio certifies in about a quarter of a second — and makes it safe to run on any checkout. The CI recipe composes the two:

```bash
walden verify <feature>        # re-prove execution on the current tree
walden release check --json    # certify; gate the pipeline on the exit code
```

### The policy

- **Planned work never blocks.** The plan is a promise, not a debt. Unexecuted tasks are listed as informational; `--strict` promotes them for plans-complete shops.
- **Uncommitted code fails closed. No bypass flag exists.** What you certify is exactly what you will tag.
- **Dirty `.walden/` only warns** — a freshly refreshed ledger legitimately precedes its own commit.
- **Certification is not release.** No tags, no changelogs, no publishing — that stays with you and your orchestrator.

### For pipelines and agents

The JSON envelope gains an additive `release` block: `releasable`, per-feature per-criterion results with blockers, pending task ids, the worktree partition. The embedded skill gains a Release Certification section, so agents run the gate on releasability questions, read blockers as a work list, and never look for a bypass — reinstall with `walden skill install <agent>` or let `walden update` re-sync.

### Compatibility

Everything additive within `schema_version: v0beta1`; evidence schema `v1alpha1` and spec frontmatter untouched — no existing chain or ledger is affected. And a commitment: **the gate's criteria are a public contract from this release.** Once your pipeline gates on them, semantic changes to a criterion are breaking changes and will be treated as such.

---

Install or update:

```bash
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
# or, from an existing install
walden update
```
