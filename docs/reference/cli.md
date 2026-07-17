# CLI Reference

Every command, with behavior, flags, and exit codes. Global conventions first:

- **`--json`** on any command emits the [versioned envelope](json.md) instead of text — same facts, same exit code.
- **Exit codes** are the contract: `0` means the command's claim holds (valid, releasable, all proofs passed); `1` means blockers, failures, or a refused invocation — always with the remedy named. Reports that never gate (like `evidence status`) always exit `0`.
- **Refusals over surprises**: unknown flags, missing required values, and invalid combinations are rejected with the fix named, on every command uniformly. Each command supports `--help`.
- Commands run against the current working directory's repository.

## Setup

### `walden version [--json]`

Prints CLI version, JSON contract version (`v0beta1`), and supported document schema (`v1alpha1`).

### `walden update [--check] [--version <tag>] [--json]`

Self-updates the binary from GitHub releases with checksum verification and rollback on failure, then re-syncs every installed skill. `--check` reports availability without applying; `--version` pins a specific tag (strict `vX.Y.Z` tags only).

### `walden repo init [--json]`

Initializes Walden in the current repository: `.walden/` (`constitution.md`, `lessons.md`, scoped `.gitignore`), a PR template, and the generated CI workflow pinned to this CLI version. Initializes git first if absent. Idempotent.

### `walden feature init <name> [--json]`

Scaffolds `.walden/specs/<kebab-name>/` with the three draft documents. Refuses names that normalize to empty.

## Authoring and gates

### `walden status <feature> [--json]`

Current phase, blockers, and the next action. Read-only.

### `walden validate [<feature>] [--all] [--json]`

Structural validation: frontmatter contract, EARS criteria shape, traceability (task references to criteria and design sections), proof coverage (`covers:`), freshness of the approval chain. Without `--all`, validates up to the current phase; with it, the full spec. With no feature name, validates the whole portfolio. Exit `1` on any finding.

### `walden review open <feature> --phase <requirements|design|tasks> [--json]`

Opens the review gate: moves the phase document to `in-review`. Refuses out-of-order phases.

### `walden review approve <feature> --phase <phase> [--json]`

Runs full phase validation, then seals the approval: `status: approved`, `approved_at`, and the body's `approved_fingerprint`; downstream documents record the upstream fingerprint as their `source_*_fingerprint` when they are approved in turn. Nothing invalid can be sealed.

### `walden reconcile <feature> [--json]`

Repairs a stale chain after upstream edits: resets the modified document and everything downstream to draft, clearing their fingerprints for re-approval. Read-only on fresh chains.

## Execution

### `walden task status <feature> [--json]`

Execution readiness: gate state, blockers, the next runnable task, and evidence warnings when completed tasks are no longer verified.

### `walden task start <feature> [task-id] [--json]`

Resolves normalized execution context for a task (the next runnable one when no id is given): requirements referenced, design sections, verification steps.

### `walden task complete <feature> <task-id> [--json]`

Runs the task's proof steps; on pass, records the evidence (steps, chain fingerprints, post-proof code identity, execution profile) and checks the box. On failure the box stays unchecked and the failing step is named. Every step runs under its declared `timeout:` or the 10-minute default; expiry kills the step's process group and fails the proof.

### `walden task complete-all <feature> [--json]`

Completes all runnable tasks in order, stopping at the first failing proof while preserving earlier completions.

## Evidence

### `walden evidence status <feature> [--json]`

Derives each task's evidence state — `verified`, `stale-spec`, `stale-code`, `failed`, `unrecorded`, `pending` — by comparing recorded fingerprints, code identity, and profile against the present. Shows recorded-versus-current profile drift per task. Pure read, milliseconds, always exit `0`: a report, never a gate.

### `walden verify <feature> [--all] [--check] [--json]`

Re-executes the proofs of completed tasks against the current tree: those no longer `verified` by default, every one with `--all`. Fresh evidence replaces each task's record; `--check` reports without persisting anything. A failing proof never aborts the run — every selected task is re-proven and failures are collected. Exit `1` if any proof failed.

Re-verification is **pure**: a proof that modifies the working tree (`.walden/` excluded) fails its task naming the modified paths. Records bind the code identity captured at run start, so one mutating proof cannot poison the identity of tasks proven after it; the run warning names both the mutated paths and the affected tasks. Entries for tasks that left the plan are pruned.

### `walden adopt [<feature>] [--apply] [--json]`

Brownfield lane. Default is a read-only plan classifying every feature (`backfill` / `re-prove` / `complete` / `blocked`); `--apply` seals recorded approvals (absent fingerprints only — present-but-wrong is `blocked`, never written), repairs empty chain links, then re-proves unrecorded completed work through the verify machinery. Resumes by classification; idempotent once finished. Exit `1` when any proof failed, with the full per-feature partition rendered. See [Brownfield Adoption](../adoption.md).

## Certification

### `walden release check [<feature>] [--strict] [--allow-pending --reason <text>] [--json]`

One deterministic verdict per feature — `chain`, `validation`, `decisions`, `evidence` criteria — plus the repository-level clean-worktree check. Executes no proofs, writes nothing. Exit `0` iff no blocker.

- Pending leaf tasks **block by default**; `--allow-pending --reason "<text>"` waives them for this verdict, recording reason and task identifiers in the result (completion class `with-waivers`). `--allow-pending` without a non-empty reason, or `--reason` alone, is refused.
- `--strict` additionally requires committed `.walden/` state.
- Uncommitted changes outside `.walden/` always block, with no bypass. No usable git fails closed. Dirty `.walden/` warns by default (a refreshed ledger legitimately precedes its own commit) and blocks under `--strict`.
- The verdict carries `completion` (`complete` | `with-pending` | `with-waivers`) and `certified_commit`.

## Lessons

### `walden lesson log --feature <name> --phase <phase> --trigger <text> --lesson <text> --guardrail <text> [--json]`

Appends a structured lesson to `.walden/lessons.md`: what happened, what was learned, and the guardrail that prevents the repeat.

## Skills

### `walden skill install <agent>|--all [--project] [--json]`

Installs the embedded AI skill for `claude`, `codex`, `copilot`, or `opencode` (user scope by default, `--project` for repository scope). The skill is versioned with the binary; `walden update` re-syncs installed copies.

### `walden skill uninstall <agent>|--all [--project] [--json]`

Removes installed skills.

### `walden skill status [--json]`

Reports installed skills and drift against the embedded copy.

### `walden skill show [--json]`

Prints the embedded `SKILL.md`.
