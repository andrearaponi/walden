# Brownfield Adoption

A long-lived Walden portfolio contains different kinds of history: current requirements, superseded product rules, obsolete implementation plans, approvals without fingerprints, and records without execution provenance. An upgrade must preserve those distinctions rather than automatically replaying every historical task.

## Establish the current contract

Before requesting execution, review the relevant decision sources **within the requested scope**:

| Proposed applicability | Meaning and next decision |
| --- | --- |
| Current | Behavior still required by the intended product contract. |
| Superseded | Intentionally replaced or removed; identify the decision and successor before proposing retirement. |
| Mixed | Some requirements survive an obsolete design or delivery plan; agree their destination before retiring the old material. |
| Unresolved | Sources conflict or intent is unknown; ask the specific business/scope question. |

These are review labels used by the skill, not kernel lifecycle states. Age, absent fingerprints, checked tasks, removed test files, or agreement with today's code do not decide which business rules remain desirable. A newer draft does not automatically replace an approved contract. A case study for improving another product is not authorization to migrate the example repository.

## Inspect without execution

```bash
walden adopt user-auth --json  # one feature, no proofs or environment probes
walden adopt --json            # explicit whole-portfolio assessment
```

The read-only plan reports technical classes and per-task evidence assessments:

| Class | Meaning | What an authorized apply would do |
| --- | --- | --- |
| `backfill` | Recorded approvals lack fingerprints | Seal eligible current bodies, then re-prove applicable completed work in this scope. |
| `re-prove` | Completed work lacks the requested current assurance | Run its needed proofs through verify. |
| `complete` | Nothing to adopt | Nothing; this is not a claim that every planned task is implemented. |
| `blocked` | A present contradiction, incompatible plan, or unreadable ledger prevents adoption | Preserve it and report the specific blocker. |

The evidence view separates:

1. **Proof binding:** a current complete contract, a reconstructed-equivalent historical contract, a change, contradiction, or missing witness.
2. **Code freshness:** whether the recorded and current available code identities agree.
3. **Execution-integrity provenance:** known producer/policy facts versus missing or rejected assurance.

A matching full-plan fingerprint can recover assertions omitted from an older task hash, using the current body or locally available Git snapshots. The adapter executes no proof, checks out no source, fetches no history, and rewrites no ledger. Matching argv alone is insufficient. Missing snapshots or unsupported historical syntax remain explicit gaps.

**A recovered binding is not a purity attestation.** Legacy provenance can remain unknown even when the contract and code match. Such a record is `unattested` unless another condition takes state precedence; its independent gaps remain visible. A stored `passed` is historical evidence, not an automatic current guarantee.

`evidence status` also displays these dimensions but retains diagnostic environment probes. Use `adopt` when the request requires a probe-free assessment.

## Apply only the reviewed scope

```bash
walden adopt user-auth --apply
# Or, when the chain is already intact:
walden verify user-auth
```

Apply is explicitly proof-executing. With no feature selector it applies to the whole present portfolio; do not drop a selector from a reviewed plan or suggested retry. The plan's counts describe work needed for the requested assurance, not proof that every historical task is still a current business obligation.

Approval backfill is a separate trust assumption: an absent seal can be added to a recorded approval's current body, but intervening pre-fingerprint edits cannot be detected. A present contradictory seal needs human reconciliation, never automatic resealing. Backfill cannot establish what a historical proof executed.

Apply records actual outcomes through the hardened verify lane. A mutation or unavailable required identity rejects the affected verification and contaminates later executions in that invocation. Restoring source bytes does not revive those failed records; a new valid execution is needed. Assertion outcomes and policy failures remain distinct.

Failures, blocked selections and apply errors produce exit `1`, with the per-feature partition retained. Resume by classification after addressing the cause. Do not fix historical failures by weakening a current contract, silently declaring a spec obsolete, or rerunning live deployment/bootstrap work without authorization.

A full strong certificate over a legacy portfolio may still require new executions. Avoiding automatic replay does not manufacture missing facts or exempt unknown features from an unqualified portfolio claim. History lookup has a current-body fast path and per-invocation reuse; its cost depends on available history, while apply costs what the selected proofs cost.

## Format compatibility

New records use ledger schema `v1alpha2`, with an explicit task-fingerprint scheme and completion/verify provenance. The reader still accepts `v1alpha1` and older missing-version ledgers. A new container may hold untouched legacy records; merely loading it or changing its container version does not promote them to trusted evidence.

Older binaries reject `v1alpha2`. Use a compatible reader; **do not delete or downgrade the ledger to satisfy an older binary**. Preserve an unreadable file for diagnosis or restore a justified known-good artifact. Unchanged document approvals and document schema `v1alpha1` do not need to change solely for this evidence upgrade.

## Retirement

Retirement remains an explicit human decision using Git and `.walden/RETIRED.md`, not a new kernel state machine:

1. Confirm the exact superseded scope, reason and successor. A mixed contract needs an agreed home for surviving requirements first.
2. Verify that every spec/evidence file to be removed is recoverable at a **last-live commit**. Untracked or ignored local-only history is not preserved merely because the repository uses Git.
3. Only after authorization and recovery checks, remove the selected feature's spec directory and evidence file, recording name, date, reason, last-live commit and successor in `.walden/RETIRED.md`.
4. Commit the ceremony together only when committing is separately authorized and project policy permits it. Never delete history or create an unauthorized commit just to make a gate green.

Missing history or unresolved business intent means stop and ask, not fabricate recovery. The optional [`walden-history`](../skill/walden-history/SKILL.md) companion can assist; it is not required.

After authorized retirement the default gate judges the remaining on-disk portfolio. An explicitly selected-feature verdict is not a repository-wide certificate. See the [daily workflow](workflow.md) for checkpoint-based verification and the [JSON reference](reference/json.md) for precise scope and assurance fields.
