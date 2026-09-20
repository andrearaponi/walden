# JSON Contract

Every command accepts `--json` and emits one envelope on success and error paths. Parse fields by name, preserve diagnostics, and gate on the process exit code rather than human summary text.

## The envelope

```json
{
  "schema_version": "v0beta1",
  "command": "release-check",
  "ok": false,
  "result": {"summary": "...", "exit_code": 1}
}
```

- `schema_version`: the machine-output contract, independent of document and evidence formats.
- `command`: the invoked command identifier, such as `verify`, `adopt`, or `release-check`.
- `ok`: mirrors the process exit code; the exit code is the primary gate contract.
- `result`: the command-specific payload. Unknown added fields must not cause a permissive verdict.

## Common result fields

| Field | Type | Meaning |
| --- | --- | --- |
| `summary` | string | Human outcome; display, do not parse. |
| `next_action` | string | Suggested next action retaining the selected feature scope. |
| `blockers` | string[] | Blocking reasons with remedies. |
| `warnings` | string[] | Additional observations; never discard them. Known verify policy violations also produce failed outcomes and a nonzero exit. |
| `exit_code` | int | Process exit code. |
| `created_files`, `updated_files`, `changed_files`, `skipped_files` | string[] | Filesystem effects where applicable. |
| `current_phase` | string | Workflow position. |
| `completed_tasks`, `auto_completed` | string[] | Completion effects. |
| `validated_phases`, `skipped_phases` | string[] | Validation scope. |
| `document_schema_version` | string | Supported document schema (`v1alpha1`, reported by `version`). |

## Skill inspection (`skill status --json`)

`command` is `skill-status`. `result.skills` contains the six native agent/scope slots, preserving the fields `agent`, `scope`, `path`, `installed`, `state` and optional `version`. The same physical path can appear under both user and project scopes when the working directory is the user's home.

- `state`: `not-installed`, `in-sync`, `drifted`, or **`unreadable`** (added in v0.10.5).
- `unreadable` means a non-absence filesystem read failed. No content comparison or marker extraction was performed; `version` is omitted. `result.warnings` identifies the agent, scope, requested path and original read failure. Do not display this as different content.
- `installed` retains its legacy value, including `true` on read errors. It is neither a readability guarantee nor a manager/ownership assertion. `not-installed` refers to the queried native location, not every external store an agent might use.
- `in-sync` compares readable content, excluding the trailing version marker, with CRLF/LF and trailing newlines treated equivalently. Other whitespace, case, BOM and lone-CR changes are not ignored. Shared-file comparison is confined to the Walden block.
- A readable malformed block remains `drifted` with its corruption warning. An absent file or Walden block remains `not-installed`. User/project divergence is reported only when both bodies were successfully obtained.

A completed inspection keeps `ok: true` and `exit_code: 0`, even for drift/read errors: **successful reporting is not a successful content check**. Invocation/rendering failures still use the ordinary error envelope. Preserve warnings and handle unknown states conservatively. Inspection does not modify files, repair junctions, infer ownership or change which slots the updater selects.

## Evidence: one shape, three surfaces

Task evidence views use a shared ordered entry shape. It appears under `result.evidence` for verify/status, `result.adoption.features[].evidence` for adoption planning, and `result.release.features[].evidence` for certification. On-disk storage remains a map keyed by task ID; consumers should prefer the CLI views.

| Entry field | Type | Meaning |
| --- | --- | --- |
| `task_id` | string | Leaf task identifier. |
| `state` | string | `verified`, `stale-spec`, `stale-code`, `failed`, `unattested`, `unrecorded`, or `pending`. |
| `passed` | bool | Accepted outcome in this verify run, or stored record result in a read view. Omitted when no record exists. **Not a substitute for current state/assurance.** |
| `failure` | string | Current verify failure detail, including assertion/policy distinctions. |
| `recorded_identity`, `current_identity` | string | Available code identities; absent observations are not proof of equality. |
| `binding` | object | `state`, optional `source`, `effective_fingerprint`, `plan_fingerprint`, `path`, `commit`, `detail`. |
| `code_freshness` | string | `current`, `stale`, or `unavailable`. |
| `execution` | object | `state`, optional `facts`, and `detail`. |
| `gaps` | array | `{kind, message}` for every unmet condition, not just the one winning state precedence. |
| `profile` | object | Recorded diagnostic execution profile. |
| `profile_drift` | array | `{key, recorded, current}` diagnostic differences when probes were run. |
| `profile_legacy` | bool | Recorded profile is absent; this alone does not establish the record's age or execution policy. |

Binding states are `current`, `reconstructed-equivalent`, `changed`, `unknown`, or `contradictory`. A reconstructed-equivalent binding establishes the same complete proof contract, not historical purity.

Execution assessment states are `supported`, `legacy-unattested`, `unknown`, or `rejected`. When present, `execution.facts` contains:

- `origin`: `complete` or `verify`;
- `policy`: `completion-post-state/v1` or `verify-purity/v1`;
- `assertion_result`: the observed assertion outcome, distinct from policy acceptance;
- `integrity`: `post-state`, `pure`, `mutated`, `contaminated`, or `identity-unavailable`;
- optional `before_code_identity`, `after_code_identity`, `cause_task`, `changed_paths`.

A contaminated command can have `assertion_result: passed` and still produce `passed: false`, `state: failed`. Restoring files does not revive such a record. A pure earlier proof can retain `passed: true` while its current state becomes `stale-code` after a later mutation.

For recorded completed work, failures/known policy violations take precedence over known spec mismatches, then known code changes, then missing assurance (`unattested`), then `verified`. Pending and absent-record states stand apart. Always retain the independent dimensions: `stale-code` can coexist with legacy-unattested provenance.

### Ledger format and compatibility

The ledger `.walden/evidence/<feature>.json` now writes schema **`v1alpha2`**, independently of JSON envelope `v0beta1` and document schema `v1alpha1`. It stores step results, approved-chain fingerprints, `task_fingerprint_scheme: walden/task-definition/v2`, code identity, profile, execution facts, `result` and `verified_at`. It stores no derived task state.

Readers accept older `v1alpha1` or missing-version ledgers. Mixed ledgers preserve untouched legacy records; a new container version does not upgrade their assurance. Read-time recovery can use fingerprint-bound local plan snapshots but cannot infer execution policy from a timestamp, profile or CLI version label. Unsupported/corrupt ledgers fail read operations rather than becoming empty evidence.

Older binaries reject `v1alpha2`. Retain the ledger and use a compatible reader; do not delete, downgrade or backfill execution facts to satisfy an older binary. The addition of `unattested` extends the documented string vocabulary: exhaustive clients must handle it, and unknown future values must not be treated as verified.

## Scope

Scope is reported at `result.scope` for verify, `result.adoption.scope` for adoption, and `result.release.scope` for certification:

```json
{
  "kind": "feature",
  "features": ["user-auth"],
  "guarantee": "walden/evidence-integrity/v2"
}
```

`kind` is `feature` for a named selection or `portfolio` for an unqualified portfolio request. Feature names are sorted. A selected-feature pass is not a repository-wide certificate; the kernel does not silently remove features based on age, guessed obsolescence or missing fingerprints.

## Certification (`release check --json`)

- `result.completion`: `complete`, `with-pending`, or `with-waivers`, on successful and blocked verdicts alike.
- `result.certified_commit`: the captured commit, where available. In non-strict mode it is **not** a claim that local spec/evidence metadata is committed.
- `result.release`: `releasable`, `strict`, `scope`, `features`, and `worktree`.
- `release.features[].criteria[]`: the existing `chain`, `validation`, `decisions`, and `evidence` names, with `passed` and optional `blockers`.
- `release.features[].pending`: pending task IDs; `evidence` carries the shared assessment entries.
- `release.worktree`: `blockers`, `walden_dirty`, `git_skipped`, plus `input_binding` and optional `inputs`.

`input_binding` is `not-requested` in non-strict mode, or `matched`/`blocked` in strict mode. Each `inputs[]` entry names `path`, `state`, and optional `detail`. States include `matched`, `matched-absent`, `missing`, `different`, `unreadable`, `unavailable`, and `unsupported-kind`.

Strict binding compares the exact consumed spec/evidence snapshot with one commit, independently of clean-status or ignore rules. An unqualified request also compares the feature inventory. Equal ledger absence is legitimate for a wholly pending waived plan, not completed work. Other worktree blockers can reject a verdict even when selected inputs match.

`result.waiver`, when applicable, contains `{reason, tasks}` with feature-qualified task IDs. It waives pending scope only, not legacy assurance, known failures, or strict input binding. The archived envelope preserves that decision.

The criterion names and producer/judge split remain stable. Hardened identity, purity and committed-input enforcement intentionally reject the former false positives; compatibility must never turn a missing guarantee into a pass.

## Adoption (`adopt --json`)

`result.adoption` contains `apply`, `scope`, and `features`. Feature classes remain `backfill`, `re-prove`, `complete`, and `blocked`:

- plan: `sealable_docs`, `reprove_count`, and per-task `evidence` assessments;
- apply: `sealed_docs`, `verified`, `failed`, `skipped`;
- `reason`: a blocker or apply error.

Planning executes no proof or environment probe. `complete` means nothing to adopt, not that every planned feature task is complete. Apply executes only its explicit scope and exits `1` for failures, blocked selections or errors, retaining the partition. Technical classes do not decide current business applicability.

## Error paths and stability

Errors use the same envelope with `ok: false`, a diagnostic summary and `exit_code: 1`; there is no separate error format. A successful status/plan read is not a certification verdict merely because its process exits zero.

Within `v0beta1`, existing field names/types are preserved and fields are added. Treat unknown fields as additions and unknown assurance/state values fail-closed. Human strings are not a parsing contract. At CLI v1.0.0 the output schema will move to `v1` with a documented migration guide.
