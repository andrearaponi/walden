# The Spec Lifecycle

This is the whole mechanism, told once, end to end — the biography of a feature from intention to certified release. Every command in the CLI exists to serve one station of this lifecycle; if you understand this page, the reference pages become obvious.

## The problem Walden solves

A checked checkbox proves that *one day* a test passed. Then the code changes, the requirements change — and the checkbox keeps saying "done" forever, even when it is no longer true. A spec document approved in March says nothing about whether the code in July still implements it.

Walden's answer is to make every claim **re-provable**. Approvals are sealed with content fingerprints, completions are recorded with executable proofs bound to a snapshot of the world, and every state you can ask about is *derived by comparison at read time* — never stored, never trusted from memory. When something no longer holds, Walden is the one that tells you.

Two photographs run through everything below: the fingerprint of the **specs** as approved, and the identity of the **code** as proven. Every mechanism in the lifecycle is a way of taking, chaining, or comparing those photographs.

## Birth

`walden feature init <name>` scaffolds `.walden/specs/<name>/` with three documents, each in `status: draft`:

| Document | Question it answers |
| --- | --- |
| `requirements.md` | *What* must be true, as [EARS acceptance criteria](reference/spec-format.md#ears-acceptance-criteria) with stable IDs |
| `design.md` | *How* it will be built — architecture, alternatives considered, failure modes, coverage |
| `tasks.md` | *In what order*, with an executable proof per leaf task |

Every document declares `walden_schema_version: v1alpha1` in its frontmatter. The CLI stamps the version on every save (so existing repositories migrate through normal use), refuses documents declaring an unsupported version, and rejects unknown frontmatter fields — while preserving `x-` prefixed extension fields verbatim, so integrators can attach durable metadata without forking the format.

Phases are ordered and enforced: you cannot approve a design before its requirements, and you cannot execute tasks from an unapproved or stale plan. `walden validate` checks each document's structure and traceability at any point; `walden status` tells you where you are and what comes next.

## The seal

Approval is a two-step gate — `walden review open --phase <phase>` moves a document to `in-review`, `walden review approve` moves it to `approved` — and the approve step runs full phase validation first: nothing structurally invalid can be sealed.

At approval, the CLI records `approved_fingerprint`: a SHA-256 over the document **body**. Three properties make this seal precise rather than brittle:

- **Body-only.** Frontmatter never participates. Timestamps, status flips, and `x-` extension fields can change freely without invalidating an approval — only the approved *content* is sealed.
- **Path-aware normalization.** For `tasks.md` only, checkbox states (`[ ]`/`[x]`) are normalized out of the fingerprint: executing the plan is not editing the plan. For every other document, every body byte counts.
- **Fail-closed.** A missing or malformed fingerprint on an approved document is a named blocker, not a shrug. Documents approved by pre-fingerprint versions of the CLI repair through one `reconcile` + re-approval cycle — or in bulk through the [adoption lane](adoption.md).

From this moment, "approved" is not a status label anyone typed. It is a claim the CLI can check byte-for-byte, forever.

## The chain

Approvals do not stand alone — they bind to what they were built on. When `design.md` is approved, its frontmatter records the requirements' fingerprint (`source_requirements_fingerprint`); when `tasks.md` is approved, it records the design's. The three seals form a chain:

```text
requirements.md ──fingerprint──▶ design.md ──fingerprint──▶ tasks.md
     (what)          binds           (how)        binds       (in what order)
```

Freshness is decided by fingerprints alone:

- A document is **intact** when its body still matches its own `approved_fingerprint`.
- A downstream document is **fresh** when it is intact *and* its `source_*` fingerprint equals the upstream's current one.

Edit an approved requirement, and the design and tasks built on it are stale *by construction* — no timestamps, no heuristics. Re-approving identical content does not stale anything: content-identical means fingerprint-identical.

**Drift and repair.** Software changes; requirements change with it. When an upstream document must move, `walden reconcile <feature>` resets the modified document and everything downstream of it to draft and clears their fingerprints; the chain is then re-walked through the normal review gates, and re-approval records fresh seals. Stale documents block execution until reconciled — the plan you execute is always the plan that was approved against the requirements that exist.

## Execution: two lanes, one proof grammar

Every leaf task carries a proof — one or more steps in argv form, immune to shell quoting:

```markdown
- Verification:
  - command: ["go", "test", "-run", "TestAuth", "./internal/auth"]
    expect_output: "--- PASS: TestAuth"
    timeout: 30m
    covers: ["R1.AC1", "R1.AC2"]
```

A step passes when its exit code matches (`expect_exit`, default `0`) and its output contains the declared pattern (`expect_output` — declare it on test runners, so a `-run` pattern matching zero tests cannot pass vacuously). Every step runs under a timeout — declared, or a 10-minute default — and expiry kills the step's whole process group and fails the proof naming the exceeded budget. `covers` declares which acceptance criteria the proof demonstrates; the validator tracks proof coverage separately from task references.

The same grammar serves two lanes with deliberately different contracts:

**Completion** (`walden task complete`) is where implementation happens. The proof runs, and only a pass checks the box. The evidence record binds the tree the proof *left behind* — builds and generators legitimately mutate here, and the post-proof tree is what you will commit.

**Re-verification** (`walden verify`) is where trust is refreshed. It re-executes the proofs of completed tasks against the current tree — and it is **pure by contract**: a proof that modifies the working tree fails its task, naming the modified paths (`.walden/` excluded, as the ledger's own legitimate write path). A failing proof never aborts the run; every selected task is re-proven and failures are collected into one honest partition.

Verify records bind the code identity captured **at the start of the run**. Under purity the distinction is invisible — a compliant proof leaves the tree where it found it — but when a proof does mutate, the anchoring confines the damage: the mutant fails on its own side effects, while the tasks proven after it keep the identity the run promised, and the run-level warning names both the paths and the affected tasks (`proof side effects modified the repository: go.mod; tasks re-proven on the modified tree: 5.2, 5.3`). One misbehaving proof fails alone.

Author verify-able proofs as read-only assertions: prefer `["go", "mod", "tidy", "-diff"]` over `["go", "mod", "tidy"]`, route build outputs outside the repository, and let tests assert instead of regenerate.

## Evidence

Completions and verifications write to `.walden/evidence/<feature>.json` — the **evidence ledger**. Each task's record holds facts only:

- the proof's steps with their outcomes,
- the fingerprints of the approved chain at proof time (the spec photograph),
- the code identity at proof time (the code photograph — a deterministic digest over every tracked file's content, `.walden/` excluded so committing evidence never invalidates it),
- the [execution profile](#the-environment) of the machine that ran it,
- `result: passed | failed` and a timestamp.

What the ledger deliberately does **not** hold is any state label. When you ask — `walden evidence status`, or any command that reads evidence — Walden takes today's photographs and compares:

| State | Meaning |
| --- | --- |
| `verified` | Both photographs match: the proof's claim holds on the specs and code you have right now. |
| `stale-spec` | The approved chain moved since the proof ran — the evidence certifies a different plan. |
| `stale-code` | The tree moved since the proof ran — the evidence certifies a different implementation. |
| `failed` | The recorded proof outcome is a failure — re-proven and found broken, with the task named. |
| `unrecorded` | The task is checked but has no record — completed outside the ledger; re-prove it. |
| `pending` | Not completed yet. |

*Stale* does not mean broken. It means **"no longer known"** — the world moved since the claim was proven, and the honest state is a declared doubt. A code-only bugfix needs no spec ceremony: evidence goes `stale-code`, and one `walden verify` either restores `verified` on today's tree or names exactly which task's proof broke.

The ledger is regenerable, not archival: it is the current truth, and history lives in git — the file is committed and reviewed like the specs it proves. Records for tasks that leave the plan are pruned on the next verify; a corrupted or lost record simply reads as `unrecorded` and is recreated by re-proving. Never a silent lie.

## The environment

Every record also carries the **execution profile** of the machine that produced it: `platform` (OS/arch) and `walden` (recording CLI version) always, plus the outputs of probes the repository declares in `.walden/environment.md`:

```markdown
# Environment Probes

- go: ["go", "version"]
- node: ["node", "--version"]
```

Profiles are diagnostic by design — they never change a derived state, so evidence recorded in CI keeps certifying on any machine. Their payoff is the failure you can finally read: when a proof that passed under go 1.25 fails on a machine running 1.24, the failure says so — `environment drift: go: recorded "go1.25.0" → current "go1.24.0"` — and `evidence status` shows recorded-versus-current differences per task. Fix the environment, or knowingly re-record on the current one; never edit proofs to paper over drift.

## The verdict

`walden release check` folds the whole lifecycle into one deterministic answer per feature, plus one repository-level check:

| Criterion | What must hold |
| --- | --- |
| `chain` | Every document approved, every fingerprint intact, the chain fresh |
| `validation` | Full-spec structural validation passes |
| `decisions` | No unresolved `[decision: …]` markers in approved documents |
| `evidence` | Every completed task's evidence derives `verified`; every planned task executed — or explicitly waived |
| worktree | No uncommitted changes outside `.walden/` — what you certify is what you tag |

Exit `0` if and only if no blocker exists; every blocker names its remedy. The gate is **judgment only**: it executes no proofs and writes nothing. `verify` produces evidence; `release check` judges it — so certification is cheap enough to run on every pipeline, and the verdict is reproducible from the commit it names (`certified_commit` in the result).

**Pending work blocks by default.** An approved plan's unexecuted task means unimplemented acceptance criteria in the thing being tagged. Partial releases remain expressible — as a recorded decision: `--allow-pending --reason "<text>"` waives pending work for that verdict, the completion class becomes `with-waivers` (against `complete` and `with-pending`), and the reason plus the waived task identifiers ride the verdict itself. No reason, no waiver. `--strict` adds one requirement — committed `.walden/` state, so the verdict is reproducible including its evidence — and composes with a waiver.

A repository without usable git fails closed: certification requires a git-backed code identity. There is no bypass flag for a dirty worktree.

## Beyond one feature

Two lanes extend the lifecycle across a whole portfolio:

- **[Brownfield adoption](adoption.md)** — `walden adopt` brings repositories whose specs predate the current contract into it: sealing recorded approvals with fingerprint backfill, re-proving unrecorded work, and reporting an honest verified/failed partition.
- **[Retirement](adoption.md#retirement)** — superseded specs are deleted, not labeled: history lives in git, an index entry in `.walden/RETIRED.md` records the ceremony, and the release gate judges only the living portfolio.

## The invariants

Five rules hold everywhere and explain most design decisions you will encounter:

1. **Derive, don't store.** States (`verified`, `stale-*`, phase freshness) are computed from facts at read time. There is nothing to forget to update.
2. **Produce and judge are separate.** Commands that execute proofs (`task complete`, `verify`, `adopt --apply`) write evidence; the gate (`release check`) only reads. No verdict has side effects.
3. **Fail closed, name the remedy.** Missing fingerprints, unusable git, unsupported schema versions — each is a named blocker with its fix, never a silent pass.
4. **The relaxation is always recorded.** There is no quiet bypass: waivers require a reason and ride the verdict; adoption's trust assumption is stated in the plan.
5. **The JSON contract only grows.** Machine output is versioned (`schema_version: v0beta1`) and changes additively; see the [JSON reference](reference/json.md).
