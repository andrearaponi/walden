# CLI Reference

Every command, with behavior, flags, and exit codes. Global conventions first:

- **`--json`** on any command emits the [versioned envelope](json.md) instead of text — same facts, same exit code.
- **Exit codes** are the contract: `0` means the command's claim holds (valid, releasable, all proofs passed); `1` means blockers, failures, or a refused invocation — always with the remedy named. A successful diagnostic read (such as `evidence status`) exits `0` regardless of derived states; malformed inputs and refused reads still exit `1`.
- **Refusals over surprises**: unknown flags, missing required values, and invalid combinations are rejected with the fix named, on every command uniformly. Each command supports `--help`.
- Commands run against the current working directory's repository.

## Setup

### `walden version [--json]`

Prints CLI version, JSON contract version (`v0beta1`), and supported document schema (`v1alpha1`).

### `walden update [--check] [--version <tag>] [--json]`

Self-updates the binary from GitHub releases with checksum verification and rollback on failure, then re-syncs every installed skill. `--check` reports availability without applying; `--version` pins a specific tag (strict `vX.Y.Z` tags only).

This is the native distribution flow, not a binary-only update. For a Skills CLI-managed guide, use Skills CLI for the guide and the official installer with `--version <compatible-tag> --no-skill` for the executable. `--no-skill` is an **installer option**, not a flag on `walden update`. The current guide requires v0.10.2 or a newer compatible CLI and available matching release assets.

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

Runs the task's proof steps; on pass, records the complete proof fingerprint, chain bindings, post-proof code identity, profile and completion-policy facts, then checks the box. On failure the box stays unchecked and the failing step is named. Every step runs under its declared `timeout:` or the 10-minute default; expiry kills the step's process group and fails the proof.

### `walden task complete-all <feature> [--json]`

Completes all runnable tasks in order, stopping at the first failing proof while preserving earlier completions.

## Evidence

### `walden evidence status <feature> [--json]`

Derives `verified`, `stale-spec`, `stale-code`, `failed`, `unattested`, `unrecorded`, or `pending`, reporting proof binding, code freshness and execution provenance separately. Profiles are diagnostic and do not gate state. This command executes declared profile probes but does not write evidence; use `adopt <feature>` for a probe-free legacy assessment. Successful inspection exits `0` even for non-verified states; unreadable inputs are errors.

### `walden verify <feature> [--all] [--check] [--json]`

Re-executes the proofs of completed tasks against the current tree: those no longer `verified` by default, every one with `--all`. Fresh evidence replaces each task's record; `--check` reports without persisting anything. A failing proof never aborts the run — every selected task is re-proven and failures are collected. Exit `1` if any proof failed.

Re-verification is **pure by contract**: each task records its captured before/after identities. Mutation or a missing required identity creates sticky contamination; later executed tasks cannot earn verified evidence in that invocation, even after source restoration. Assertion and policy outcomes remain distinct. An unavailable initial identity rejects execution, and no source rollback occurs automatically. `--check` prevents ledger writes, not command side effects. Detection is manifest-based boundary sampling, not a sandbox. Normal verify prunes orphaned records only in the selected feature; default selection and `--all` never expand to other features.

### `walden adopt [<feature>] [--apply] [--json]`

Brownfield lane. Default is a read-only plan for the named feature or, when omitted, the portfolio: technical class (`backfill` / `re-prove` / `complete` / `blocked`) plus independent binding/freshness/provenance assessments. It executes no proof or environment probe. Recoverable legacy bindings do not supply missing execution attestations; `complete` means nothing to adopt, not a finished product plan.

`--apply` explicitly executes the selected scope: seal eligible recorded approvals, then re-prove needed completed work through verify. A present contradictory fingerprint is never resealed automatically. Apply resumes by classification and exits `1` for failures, blocked selections or errors, preserving the partition. No inspection or binary upgrade automatically replays historical work. Establish current business applicability before applying; see [Brownfield Adoption](../adoption.md).

## Certification

### `walden release check [<feature>] [--strict] [--allow-pending --reason <text>] [--json]`

One deterministic verdict per feature — `chain`, `validation`, `decisions`, `evidence` criteria — plus the repository-level clean-worktree check. Executes no proofs, writes nothing. Exit `0` iff no blocker.

- Pending leaf tasks **block by default**; `--allow-pending --reason "<text>"` waives them for this verdict, recording reason and task identifiers in the result (completion class `with-waivers`). `--allow-pending` without a non-empty reason, or `--reason` alone, is refused.
- `--strict` binds the exact consumed spec/evidence bytes and presence to one existing commit, even for ignored inputs. An unqualified request also checks the committed feature inventory. Missing, different, unreadable or symlinked inputs block; ignored scratch outside the input set does not.
- Uncommitted changes outside `.walden/` always block, with no bypass. No usable git fails closed. Dirty `.walden/` warns by default (a refreshed ledger legitimately precedes its own commit) and blocks under `--strict`.
- The verdict carries `completion`, `certified_commit`, explicit feature/portfolio scope and guarantee, and committed-input binding status. A named-feature pass is not a repository-wide certificate. Non-strict mode does not claim local metadata is committed; pending waivers never waive unattested evidence or strict input defects.

## Lessons

### `walden lesson log --feature <name> --phase <phase> --trigger <text> --lesson <text> --guardrail <text> [--json]`

Appends a structured lesson to `.walden/lessons.md`: what happened, what was learned, and the guardrail that prevents the repeat.

## Skills

### `walden skill install <agent>|--all [--project] [--json]`

Installs the embedded AI skill for `claude`, `codex`, `copilot`, or `opencode` (user scope by default, `--project` for repository scope). The skill is versioned with the binary; `walden update` re-syncs installed copies. Do not apply this native installer to a copy managed by Skills CLI unless the user explicitly chooses to change its ownership.

### `walden skill uninstall <agent>|--all [--project] [--json]`

Removes installed skills.

### `walden skill status [--json]`

Reports installed skills and drift against the embedded copy.

### `walden skill show [--json]`

Prints the embedded `SKILL.md`.
