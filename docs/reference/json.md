# JSON Contract

Every command takes `--json` and emits one versioned envelope on stdout — on success and error paths alike. This page is the contract for tooling: field names, the three evidence surfaces, and what "additive" promises.

## The envelope

```json
{
  "schema_version": "v0beta1",
  "command": "release-check",
  "ok": true,
  "result": { }
}
```

- `schema_version` — the contract version. `v0beta1` until the CLI stabilizes at v1.0.0; within a schema version, changes are **additive only**: existing fields keep their names and meanings, new fields appear. Parse by name, ignore what you don't know.
- `command` — the invoked command's identifier (e.g. `verify`, `evidence-status`, `release-check`).
- `ok` — `false` when the result carries an error outcome. Gate on the **process exit code**, which is the primary contract; `ok` mirrors it.
- `result` — the command-specific payload, drawn from one shared field vocabulary.

## Common result fields

Every command populates the subset that applies:

| Field | Type | Meaning |
| --- | --- | --- |
| `summary` | string | One-line human outcome — display it, don't parse it |
| `next_action` | string | The single next step, when one exists |
| `blockers` | string[] | Named blockers, each carrying its remedy |
| `warnings` | string[] | Non-blocking observations. **Do not drop these** — verify's purity violations surface here |
| `exit_code` | int | Mirrors the process exit code |
| `created_files`, `updated_files` | string[] | Filesystem effects (init, scaffold) |
| `current_phase` | string | Workflow position (status, review) |
| `completed_tasks`, `auto_completed` | string[] | Execution effects (task complete) |
| `validated_phases`, `skipped_phases` | string[] | Validation scope |
| `document_schema_version` | string | Supported document schema (`version` command) |

## Evidence: one shape, three surfaces

Task evidence appears on three surfaces. Two are **views** — ordered lists under `result.evidence`, sharing one entry shape and one set of field names. One is **storage** — the on-disk ledger. Consumers should parse the commands, not the file.

The shared entry shape (`result.evidence[]`, emitted by both `verify --json` and `evidence status --json`):

| Field | Type | Notes |
| --- | --- | --- |
| `task_id` | string | Leaf task identifier |
| `state` | string | `verified` \| `stale-spec` \| `stale-code` \| `failed` \| `unrecorded` \| `pending` |
| `passed` | bool | The proof outcome — this run's (verify) or the stored result (status). Omitted when no record exists |
| `failure` | string | Failure detail, when the entry failed in this run (verify) |
| `recorded_identity` | string | Code identity the record binds |
| `current_identity` | string | Code identity of the present tree |
| `profile` | object | Recorded execution profile, `{"key": "value"}` |
| `profile_drift` | array | `{key, recorded, current}` per differing profile entry (status) |
| `profile_legacy` | bool | Record predates profiles |

The **on-disk ledger** (`.walden/evidence/<feature>.json`, schema `v1alpha1`) is a state map keyed by task identifier, holding facts only — proof steps, chain fingerprints, `code_identity`, `profile`, `result: passed|failed`, `verified_at`. It never stores derived states: those are computed at read time by the views above. The map-versus-list difference is deliberate — the ledger is a claim about the plan's tasks; the outputs are reports.

## Certification (`release check --json`)

```json
{
  "result": {
    "summary": "RELEASABLE — 3 feature(s) certified, completion complete, commit ba53cfe55b40",
    "completion": "complete",
    "certified_commit": "ba53cfe55b40…",
    "release": {
      "releasable": true,
      "strict": false,
      "features": [
        {
          "feature": "user-auth",
          "criteria": [
            {"name": "chain", "passed": true},
            {"name": "validation", "passed": true},
            {"name": "decisions", "passed": true},
            {"name": "evidence", "passed": true}
          ],
          "pending": []
        }
      ],
      "worktree": {"git_skipped": false}
    }
  }
}
```

- `completion` — `complete` | `with-pending` | `with-waivers`. Populated on releasable and blocked verdicts alike.
- `certified_commit` — the HEAD revision the verdict ran against; empty when git is unusable (which itself blocks).
- `release.features[].criteria[]` — per-criterion verdicts; failing criteria carry `blockers` with remedies.
- `release.worktree` — repository-level criterion: `blockers` (uncommitted paths outside `.walden/`), `walden_dirty` (informational by default, blocking under strict), `git_skipped` (no usable git — blocking).
- `waiver` — present only when a waiver was active: `{"reason": "<text>", "tasks": ["feature: 1.2", …]}`. The archived envelope is the waiver's durable record.

The gate criteria (`chain`, `validation`, `decisions`, `evidence`, plus the worktree policy) are a **frozen public contract**: semantic changes to what they judge are breaking changes, announced as such.

## Adoption (`adopt --json`)

`result.adoption`: `{"apply": bool, "features": []}` where each feature carries `class` (`backfill` | `re-prove` | `complete` | `blocked`) and, per mode: plan — `sealable_docs`, `reprove_count`; apply — `sealed_docs`, `verified`, `failed`, `skipped`; plus `reason` when blocked or errored. Exit `1` when apply produced failures; the JSON envelope always carries the full report.

## Error paths

Errors use the same envelope: `ok: false`, `result.summary` naming the error, `exit_code: 1`. There is no separate error format — a parser that handles the envelope handles everything.

## Stability policy

- Within `v0beta1`: additive only. New fields appear (recent examples: `completion`, `certified_commit`, `profile`, `waiver`, `adoption`); nothing is renamed or removed, and existing semantics hold.
- Exit codes and the produce/judge split are contract.
- Human-facing strings (`summary`, blocker texts, warnings) are **not** contract — display them, never parse them.
- At CLI v1.0.0 the schema bumps to `v1` with a documented migration guide.
