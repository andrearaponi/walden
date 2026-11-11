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

## Evidence: one shape, three surfaces

Task evidence views use a shared ordered entry shape. It appears under `result.evidence` for verify/status, `result.adoption.features[].evidence` for adoption planning and executed apply outcomes, and `result.release.features[].evidence` for certification. On-disk storage remains a map keyed by task ID; consumers should prefer the CLI views.

| Entry field | Type | Meaning |
| --- | --- | --- |
| `task_id` | string | Leaf task identifier. |
| `state` | string | `verified`, `delivered`, `stale-spec`, `stale-code`, `failed`, `unattested`, `unrecorded`, or `pending`. |
| `passed` | bool | Accepted outcome in this verify run, or stored record result in a read view. Omitted when no record exists. **Not a substitute for current state/assurance.** |
| `failure` | string | Current verify failure detail, including assertion/policy distinctions. Proof output in it is limited to the last 4096 bytes of the step's combined output, preceded by the number of bytes omitted. |
| `recorded_identity`, `current_identity` | string | Available code identities; absent observations are not proof of equality. |
| `binding` | object | `state`, optional `source`, `effective_fingerprint`, `plan_fingerprint`, `path`, `commit`, `detail`. |
| `code_freshness` | string | `current`, `stale`, or `unavailable`. |
| `execution` | object | `state`, optional `facts`, and `detail`. |
| `gaps` | array | `{kind, message}` for every unmet condition, not just the one winning state precedence. |
| `profile` | object | Recorded diagnostic execution profile. |
| `profile_drift` | array | `{key, recorded, current}` diagnostic differences when probes were run. |
| `profile_legacy` | bool | Recorded profile is absent; this alone does not establish the record's age or execution policy. |
| `elapsed_ms` | integer | Measured proof-attempt duration in adoption apply; absent from planning, historical read views and ordinary verify output. |
| `explanation` | object | `evidence status` only: what changed since the record was written. See [Explanations](#explanations-evidence-status---json). |
| `recorded_duration_ms` | integer | `evidence status` only: how long the recorded execution took, the sum of its steps. Omitted when the record has no step or a step without a measurement: absence is never reported as zero. Not `elapsed_ms`, which times a current attempt. |
| `delivery` | bool | `true` when the plan declares the task's proof as a delivery proof. Omitted for a regression proof. |
| `attestation` | object | Present when `state` is `delivered`: `commit`, the commit whose tree the proof passed on, and `sealed`, `true` when adoption wrote the attestation and `false` when an execution did. |
| `awaits_adoption` | bool | `true` when the task holds a pass that awaits adoption: a pass of the approved proof with supported execution provenance, recorded before the plan declared the delivery. `walden verify` leaves it alone unless `--task` names its task, and `walden adopt` says whether it can be sealed. Omitted otherwise. |

Binding states are `current`, `reconstructed-equivalent`, `changed`, `unknown`, or `contradictory`. A reconstructed-equivalent binding establishes the same complete proof contract, not historical purity.

Execution assessment states are `supported`, `legacy-unattested`, `unknown`, or `rejected`. When present, `execution.facts` contains:

- `origin`: `complete` or `verify`;
- `policy`: `completion-post-state/v1` or `verify-purity/v1`;
- `assertion_result`: the observed assertion outcome, distinct from policy acceptance;
- `integrity`: `post-state`, `pure`, `mutated`, `contaminated`, or `identity-unavailable`;
- optional `before_code_identity`, `after_code_identity`, `cause_task`, `changed_paths`.

A contaminated command can have `assertion_result: passed` and still produce `passed: false`, `state: failed`. Restoring files does not revive such a record. A pure earlier proof can retain `passed: true` while its current state becomes `stale-code` after a later mutation.

For recorded completed work, failures/known policy violations take precedence over known spec mismatches, then known code changes, then missing assurance (`unattested`), then `verified`. Pending and absent-record states stand apart. Always retain the independent dimensions: `stale-code` can coexist with legacy-unattested provenance.

### Delivery proofs

A leaf task whose plan declares `- Proof: delivery` holds a delivery proof, and its entry carries `delivery: true` on every surface. See [Delivery proofs](../lifecycle.md#delivery-proofs) for what the declaration means.

A record that holds a readable attestation is judged through it, after `pending` and `failed`. The state is `delivered` while the task's definition is the attested one and the statements the attestation names read as they did, and `stale-spec` otherwise. `binding.source` is then `attestation`, and the gaps are `binding-changed` or `binding-contradictory` for the definition and `attested-statements-changed` for the statements. The code identity and the document fingerprints take no part in that reading: `code_freshness` is still reported, may read `stale` on a delivered entry, and no gap goes with it. A record without a readable attestation is judged as any other record, whatever the plan declares.

`delivered` extends the state vocabulary: exhaustive clients must handle it, and it is not `verified`. It says that the proof passed on the tree of `attestation.commit`, not that it passes now.

Other fields for these tasks:

- `verify --json` adds two fields to `result`: `kept`, `{task_id, commit, sealed}` for each delivered task the run did not prove again, and `left_for_adoption`, the ids of the tasks whose recorded pass awaits adoption. Each is omitted when empty. Neither kind of task has an entry in `result.evidence`, which lists the tasks the run proved.
- `task status --json`, `task start --json` and `task complete --json`: `task.proof` is `delivery` for a task that declares it, and omitted for a task that holds a regression proof.
- `release check --json` adds `delivery_only` to each feature, and `adopt --json` adds `sealable_passes`, `sealed_passes` and `unsealed_passes`: see [Certification](#certification-release-check---json) and [Adoption](#adoption-adopt---json).

### Explanations (`evidence status --json`)

`walden evidence status` also says what changed since a record was written. An explanation sits beside the assessment and never feeds it: states, exit statuses and release verdicts are the same with and without it. No other command carries these fields.

`current_tree`, a field of the result beside `evidence`, describes what the current code identity is made of:

| Field | Type | Meaning |
| --- | --- | --- |
| `commit` | string | The commit `HEAD` names. Omitted in a repository without a commit, where every path is `added`. |
| `paths` | array | `{path, kind, untracked}` for each path outside `.walden/` that differs from that commit. `kind` is `changed`, `added` or `removed`; `untracked` is `true` on a path git does not track and omitted otherwise. Omitted when no path differs. Ignored files are not inputs of the identity and are not listed. |

An entry may carry `explanation`, with any of:

| Field | Present when | Content |
| --- | --- | --- |
| `explanation.code` | `code_freshness` is `stale` | `recorded_commit`, and `paths` as `{path, kind}` for every path whose content identity differs between the recorded tree and the current one. Otherwise `unavailable` with a reason code and `detail` with its sentence. |
| `explanation.spec` | an approval changed since the record | `approvals`, any of `requirements`, `design` and `task`. When the requirements changed, `statements` as `{id, kind}` with `kind` `changed` or `removed` for the statements the task named, or `statements_unchanged` set to `true` when none of them changed. Otherwise `unavailable` and `detail`. |
| `explanation.attestation` | the task is a completed delivery task whose recorded pass is no attestation | `unavailable` with a reason code and `detail` with its sentence, which ends with what attests the task: a pass of `walden verify` on a tree without uncommitted changes or, for a pass that awaits adoption, `walden adopt`. |

An explanation is exact or withheld. The reason codes are:

| Reason | Meaning |
| --- | --- |
| `no-commit` | The record keeps no commit: it was written before these facts were kept, in a repository without a commit, or rewritten by a CLI that drops them. |
| `paths-not-kept` | More paths differed from the commit than the recording limit, so the record kept their number alone. |
| `commit-missing` | The recorded commit is not in this repository: rewritten history, a shallow clone, or a commit that was never pushed. |
| `identity-mismatch` | The recorded commit and paths do not rebuild a tree with the recorded code identity. No path is named. |
| `no-statement-fingerprints` | The record keeps no statement fingerprints, or keeps them in a scheme this CLI does not read. |

`explanation.attestation` has reason codes of its own. They are checked in this order, and the first that holds is reported:

| Reason | Meaning |
| --- | --- |
| `no-tree` | The record keeps no commit and no paths, so it does not show whether its tree was committed. |
| `no-commit` | No commit could be read when the proof ran. |
| `uncommitted-paths` | The proof ran on a tree with uncommitted paths; `detail` gives their number. |
| `undeclared-run` | The pass was recorded on a commit by a run that did not read the task's proof as a delivery proof. |

A delivered task carries no `explanation`: that the code and the documents moved on since the delivery is what the state allows. For a task judged through its attestation that reads `stale-spec`, `explanation.spec.statements` compares the statements the attestation holds with the current ones.

Re-proving a task writes a record with the facts available at that moment. Nothing adds them to an existing record: work that is still verified keeps its earlier record until it is re-proven, and `walden verify <feature> --all` re-proves every completed task that is not delivered.

JSON lists are complete. Text mode applies a display limit of 10 items to each list: the first in order, then the number omitted.

### Ledger format and compatibility

The ledger `.walden/evidence/<feature>.json` now writes schema **`v1alpha2`**, independently of JSON envelope `v0beta1` and document schema `v1alpha1`. It stores step results, approved-chain fingerprints, `task_fingerprint_scheme: walden/task-definition/v2`, code identity, profile, execution facts, `result` and `verified_at`. It stores no derived task state.

A record may also keep three optional groups of facts. They explain a stale state later and decide nothing: state derivation never reads them. Adoption reads `statements` in one case, to establish that the statements a task names read as they did at a pass it is about to seal.

- `tree`: the `commit` that `HEAD` named when the code identity was captured, with `paths`, a map from each path that differed from that commit to its `mode:blob` entry, or to an empty string when the commit held the path and the working tree did not. Over the recording limit of 100 paths the record keeps `paths_omitted`, their number, instead. No `tree` is written without a commit.
- `statements`: the `scheme`, `walden/statement-block/v1`, and the `fingerprints` of the requirement statements the task names in its `Requirements:` line or in a `covers:` attribute. A fingerprint covers the statement and the lines indented under it, its acceptance check included.
- `duration_ms` on each step: how long that step took.

These fields are additive within `v1alpha2` and absent on earlier records. An earlier CLI that rewrites a ledger drops them, and the next verification records them again.

A record may hold an `attestation`, and state derivation does read it. It says that the recorded pass of a delivery proof ran on the tree of a commit:

- `commit`: the commit whose tree has the code identity of the record;
- `task_fingerprint`: the task definition it attests;
- `statements`: the `scheme` and the `fingerprints` of the statements the task named, as they read then;
- `sealed`: `true` when adoption wrote the attestation, omitted when an execution did.

A ledger in which at least one record holds an attestation is written as **`v1alpha3`**. Any other ledger is written as `v1alpha2`, as before, and a ledger whose last attestation is gone is written as `v1alpha2` again.

Readers accept older `v1alpha1` or missing-version ledgers. Mixed ledgers preserve untouched legacy records; a new container version does not upgrade their assurance. Read-time recovery can use fingerprint-bound local plan snapshots but cannot infer execution policy from a timestamp, profile or CLI version label. Unsupported/corrupt ledgers fail read operations rather than becoming empty evidence.

Older binaries reject `v1alpha2`, and releases before delivery proofs reject `v1alpha3`, naming the version: a CLI that does not know delivery proofs cannot prove a delivery again and replace its attestation. Retain the ledger and use a compatible reader; do not delete, downgrade or backfill execution facts to satisfy an older binary. The addition of `unattested` extends the documented string vocabulary: exhaustive clients must handle it, and unknown future values must not be treated as verified.

## Scope

Scope is reported at `result.scope` for verify, `result.adoption.scope` for adoption, and `result.release.scope` for certification:

```json
{
  "kind": "feature",
  "features": ["user-auth"],
  "guarantee": "walden/evidence-integrity/v2"
}
```

`kind` is `feature` for a named selection, `portfolio` for an unqualified portfolio request, or `task` for `verify --task`, which adds the selected id in `tasks`, a field every other scope omits. Feature names are sorted. A selected-task pass says nothing about the other tasks, and `warnings` names those not verified. A selected-feature pass is not a repository-wide certificate; the kernel does not silently remove features based on age, guessed obsolescence or missing fingerprints.

## Certification (`release check --json`)

- `result.completion`: `complete`, `with-pending`, or `with-waivers`, on successful and blocked verdicts alike.
- `result.certified_commit`: the captured commit, where available. In non-strict mode it is **not** a claim that local spec/evidence metadata is committed.
- `result.release`: `releasable`, `strict`, `scope`, `features`, and `worktree`.
- `release.features[].criteria[]`: the existing `chain`, `validation`, `decisions`, and `evidence` names, with `passed` and optional `blockers`.
- `release.features[].pending`: pending task IDs; `evidence` carries the shared assessment entries.
- `release.features[].delivery_only`: the statements that only delivery proofs cover, in requirements order. Once those tasks are delivered, nothing proves these statements again. Omitted when there is none.
- `release.worktree`: `blockers`, `walden_dirty`, `git_skipped`, plus `input_binding` and optional `inputs`.
- `release.repository_warnings`: optional, additive repository-level notes such as a due contract consolidation. They never change `releasable`, blockers, completion or the exit code.

`input_binding` is `not-requested` in non-strict mode, or `matched`/`blocked` in strict mode. Each `inputs[]` entry names `path`, `state`, and optional `detail`. States include `matched`, `matched-absent`, `missing`, `different`, `unreadable`, `unavailable`, and `unsupported-kind`.

Strict binding compares the exact consumed spec/evidence snapshot with one commit, independently of clean-status or ignore rules. An unqualified request also compares the feature inventory. Equal ledger absence is legitimate for a wholly pending waived plan, not completed work. Other worktree blockers can reject a verdict even when selected inputs match.

`result.waiver`, when applicable, contains `{reason, tasks}` with feature-qualified task IDs. It waives pending scope only, not legacy assurance, known failures, or strict input binding. The archived envelope preserves that decision.

A `delivered` entry satisfies the `evidence` criterion: it adds no blocker and is reported with `state: delivered` and its `attestation`, never as `verified`. A releasable verdict states the number of delivered tasks in `summary`. A pending delivery task blocks and is waived as any pending task.

The criterion names and producer/judge split remain stable. Hardened identity, purity and committed-input enforcement intentionally reject the former false positives; compatibility must never turn a missing guarantee into a pass.

## Adoption (`adopt --json`)

`result.adoption` contains `apply`, `scope`, and `features`. Feature classes remain `backfill`, `re-prove`, `complete`, and `blocked`:

- plan: `sealable_docs`, `reprove_count`, `sealable_passes`, and per-task `evidence` assessments;
- apply: `sealed_docs`, `verified`, `failed`, `skipped`, `sealed_passes`, plus observed task details;
- both: `unsealed_passes`;
- `reason`: a blocker or feature-level apply error.

Planning executes no proof or environment probe. `complete` means nothing to adopt, not that every planned feature task is complete. Apply executes only its explicit scope and exits `1` for failures, blocked selections or errors, retaining the partition. Technical classes do not decide current business applicability.

`sealable_passes` and `sealed_passes` are `{task_id, commit}`: the recorded pass of a completed delivery task, written before the plan declared the delivery, with the commit that holds the tree the pass ran on. The plan lists what apply can seal as an attestation; apply lists what it sealed, without executing the proof. `unsealed_passes` are `{task_id, reason, detail}` for the passes left as they were: `reason` is `no-provenance`, `proof-changed`, `statements-unknown` or `no-commit`, and `detail` ends with what proves the task again. The three lists are omitted when empty.

A feature with a pass to seal has class `backfill`. A feature whose only open work is a pass left unsealed for `statements-unknown` or `no-commit` has class `re-prove` with no task to re-prove: apply executes nothing for it, and `walden verify <feature> --task <id>` proves the task again. A task whose pass awaits adoption is counted neither in `reprove_count` nor in the workload, and a pass left unsealed does not change the exit code.

### Workload

`result.adoption.workload` sums available feature workloads. `assessed_tasks` counts completed tasks selected for re-proving; `assessed_steps` counts their declared command steps, not tests or expanded shell processes. A supported legacy proof counts as one step. `unassessed_features` lists blocked features in the selected scope; their unknown work does not silently become zero. Example:

```json
{
  "assessed_tasks": 2,
  "assessed_steps": 5,
  "unassessed_features": ["blocked-feature"]
}
```

Each `features[].workload` has `available`. Known zero retains explicit `tasks` and `steps`:

```json
{
  "available": true,
  "tasks": 0,
  "steps": 0
}
```

Unavailable work omits both numbers and includes its reason:

```json
{
  "available": false,
  "reason": "tasks assessment unavailable: malformed proof"
}
```

An absent or unapproved task plan contributes zero to the existing adoption selection, not a claim of product completeness. Workloads in apply describe its initial plan. The verifier still selects against the current state when each feature begins; counts are neither a frozen schedule nor execution authorization.

### Executed work and time

The paths below are inside `result.adoption`, except the explicitly named warnings field:

| Field | Meaning |
| --- | --- |
| `elapsed_ms` | Apply-operation elapsed time, including planning; absent on read-only planning. |
| `features[].elapsed_ms` | Time processing that feature, including sealing, verification and saving. |
| `features[].evidence` | Only actual attempted-task outcomes in apply, using the shared evidence shape; historical assessments in plan. |
| `features[].evidence[].elapsed_ms` | Time in the attempted proof steps, including runner wait/timeout cleanup, excluding progress rendering, probes and identity capture. |
| `features[].evidence_persisted` | Present only when the verifier attempted a ledger write: true after success, false after failure. A pruning-only write does not imply a proof execution. |
| `features[].reason` | Feature-level blocker/error, separate from individual `evidence[].failure` diagnoses. |
| `result.warnings` | Feature-qualified verifier warnings, including run-contamination diagnostics. |

Durations are measured monotonic intervals in integer milliseconds, not an ETA or historical estimate. Enclosing intervals are not sums of nested measurements. Measured zero remains present; skipped tasks have no new outcome or duration. No duration, raw-output tail or new diagnostic sidecar is stored in the ledger.

Each outcome retains the verifier's diagnosis and final feature-level assessment. `passed` is an accepted execution, not a substitute for `state`, `execution.facts.assertion_result` or `execution.facts.integrity`. A pure prefix can retain `passed: true` with `state: stale-code`. A later feature can still invalidate earlier code freshness; this is not a new repository-wide release verdict.

The legacy `verified`/`failed` identifier partitions keep their accounting for features whose verification returned normally. If a late save fails, the feature still reports its observed outcomes but they are not added to those partitions. An in-memory accepted result is not durable evidence. Example excerpt (binding/profile fields omitted):

```json
{
  "feature": "sample",
  "class": "re-prove",
  "reason": "persist refreshed evidence: write failed",
  "evidence_persisted": false,
  "evidence": [
    {
      "task_id": "1",
      "state": "verified",
      "passed": true,
      "elapsed_ms": 125
    }
  ]
}
```

An entry failure creates no task result. A fatal planning error after apply begins can carry invocation time without a scope or workload; argument errors before apply begins carry no invented timing. JSON mode emits one final envelope without text progress on stdout. Existing stderr warnings retain their behavior.

All additions remain optional for consumers of older compatible CLIs: missing fields are not zero measurements or proof of success. Diagnostics may contain existing verifier command output; there is no automatic redaction or log capture. Document/evidence formats and assurance semantics are unchanged.

## Consolidation (`consolidate --json`)

`walden consolidate` (and `consolidate start`, `open`, `approve`, with command names `consolidate`, `consolidate-start`, `consolidate-open`, `consolidate-approve`) reports `result.consolidation`. `status --json` carries the same object without the report-only fields; a consolidation problem never changes the status exit code or its other fields.

| Field | Type | Meaning |
| --- | --- | --- |
| `tracking` | string | `not-started`, `tracking`, or `unknown`. |
| `pending` | array | `{feature, kind}` with kind `new`, `revised`, or `removed`. |
| `backlog` | string[] | Features recorded at start and not yet consolidated. |
| `unconsolidated` | int | Approved features while tracking has not started. |
| `threshold` | string | `none`, `suggested` (two pending), or `due` (three or more). |
| `view` | string | `absent`, `draft`, `in-review`, `approved`, or `stale`. |
| `problem`, `remedy` | string | Present when `tracking` is `unknown`. |
| `scope` | array | Report only: `{feature, pending, hub_only, links}`; `links[]` are `{kind, feature, path}` with kind `cited-by`, `cites`, or `shared-file`. |
| `hubs` | array | Report only: `{path, features}` for files cited by at least five features. |
| `findings` | array | Report only: `{kind, feature, id, subject, message}` with kind `missing-file`, `dangling-reference`, `reserved-reuse`, `view-mismatch`, or `coherence-review`. Advisory, except that `consolidate open` and `consolidate approve` refuse while `view-mismatch` or `coherence-review` findings remain. |
| `identifiers` | object | Report only: defined criterion, NFR and constraint identifiers per scoped feature. |
| `comparisons` | array | Report only: per pending feature `{feature, kind, statements, linked}`. `statements[]` are `{id, text, mark, previous}` with mark `added`, `changed`, `unchanged`, `unverified` (recorded without text) or `removed` (with the recorded text); `previous` carries the recorded text of a `changed` statement and is omitted otherwise. `linked[]` are `{feature, hub_only, links, statements}`; hub-only features carry no statements. |

Warnings about a suggested or due consolidation, a stale view or an unknown state appear in `result.warnings` of `status` and the report; `feature init` adds only the due warning, and `release check` uses `release.repository_warnings`.

## Error paths and stability

Errors use the same envelope with `ok: false`, a diagnostic summary and `exit_code: 1`; there is no separate error format. A successful status/plan read is not a certification verdict merely because its process exits zero.

Within `v0beta1`, existing field names/types are preserved and fields are added. Treat unknown fields as additions and unknown assurance/state values fail-closed. Human strings are not a parsing contract. At CLI v1.0.0 the output schema will move to `v1` with a documented migration guide.
