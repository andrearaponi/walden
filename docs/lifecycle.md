# The Spec Lifecycle

This is the whole mechanism, told once, end to end — the biography of a feature from intention to certified release. Every command in the CLI exists to serve one station of this lifecycle; if you understand this page, the reference pages become obvious.

## The problem Walden solves

A checked checkbox proves that *one day* a test passed. Then the code changes, the requirements change — and the checkbox keeps saying "done" forever, even when it is no longer true. A spec document approved in March says nothing about whether the code in July still implements it.

Walden's answer is to make every claim **re-provable**. Approvals are sealed with content fingerprints, completions are recorded with executable proofs bound to a snapshot of the world, and every state you can ask about is *derived by comparison at read time* — never stored, never trusted from memory. When something no longer holds, Walden is the one that tells you.

This matters most in the [agentic flow](agentic.md), where a coding agent authors the documents and writes the code: the guarantees below are enforced by the CLI at every station, so they hold identically whoever — or whatever — did the typing. The skill drafts and implements; the kernel seals, proves, and judges; the human approves. Trust comes from the gates, not from the model.

Three independent facts run through the lifecycle: the **approved contract**, the **code identity**, and the **execution policy/provenance**. Matching contract and code cannot reconstruct an integrity observation that was never recorded. The skill also establishes which business contract is current; the kernel does not infer that from document age or implementation presence.

## Birth

`walden feature init <name>` scaffolds `.walden/specs/<name>/` with three documents, each in `status: draft`:

| Document | Question it answers |
| --- | --- |
| `requirements.md` | *What* must be true, as [EARS acceptance criteria](reference/spec-format.md#ears-acceptance-criteria) with stable IDs |
| `design.md` | *How* it will be built — architecture, alternatives considered, failure modes, coverage |
| `tasks.md` | *In what order*, with an executable proof per leaf task |

New authoring adds a one-line **Acceptance check** beneath each criterion: the observation that would distinguish success from failure, without choosing a framework or command. It remains ordinary body content, not a new kernel field. Design scaffolding expands only the six required headings; reviewers add optional detail when it serves a relevant decision or verification need.

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
  - command: ["go", "test", "-v", "-count=1", "-run", "^TestAuth$", "./internal/auth"]
    expect_output: "--- PASS: TestAuth"
    timeout: 30m
    covers: ["R1.AC1", "R1.AC2"]
```

A step passes when its exit code matches (`expect_exit`, default `0`) and its output contains the declared pattern (`expect_output`). The skill requires a runner-appropriate anti-vacuity safeguard: native failure on no tests, structured results, or an output assertion. For Go, `-v` exposes the named PASS line and `-count=1` avoids cached execution, so a selector matching zero tests cannot satisfy this example. Every step runs under a timeout — declared, or a 10-minute default — and expiry kills the step's whole process group and fails the proof naming the exceeded budget. `covers` declares which acceptance criteria the proof demonstrates; the validator tracks proof coverage separately from task references.

The same grammar serves two lanes with deliberately different contracts:

**Completion** (`walden task complete`) is where implementation happens. The proof runs, and only a pass checks the box. The evidence record binds the tree the proof *left behind* — builds and generators legitimately mutate here, and the post-proof tree is what you will commit.

**Re-verification** (`walden verify`) is where trust is refreshed. It re-executes the proofs of completed tasks against the current tree — and it is **pure by contract**: a proof that modifies the working tree fails its task, naming the modified paths (`.walden/` excluded, as the ledger's own legitimate write path). A failing proof never aborts the run; every selected task is re-proven and failures are collected into one honest partition.

Verify records bind each task's actually observed **pre-proof identity**, retaining its post-proof identity and policy outcome. A detected mutation or unavailable required identity creates sticky **contamination**: that execution and all later executed tasks in the invocation cannot earn verified evidence, even if a later command restores the initial bytes. Remaining selected proofs continue for diagnosis, with assertion failures distinguished from policy failures. Earlier pure results remain real records, subject to normal freshness checks; skipped records are not invented executions.

An unavailable initial identity rejects verify before proof execution. Normal verify persists truthful failures; `--check` suppresses ledger writes but does not sandbox commands. The CLI never rolls back source automatically. Detection samples a task's proof boundaries using the existing manifest scope; transient changes restored between captures and excluded paths are outside this guarantee.

Author verify-able proofs as read-only assertions: prefer `["go", "mod", "tidy", "-diff"]` over `["go", "mod", "tidy"]`, route build outputs outside the repository, and let tests assert instead of regenerate.

## Evidence

Completions and verifications write to `.walden/evidence/<feature>.json` — the **evidence ledger**. Each task's record holds facts only:

- the complete task-contract fingerprint scheme, including ordered argv, expected exit/output, timeout declarations and asserted coverage,
- the proof's steps with their outcomes and completion/verify execution provenance,
- the fingerprints of the approved chain at proof time (the spec photograph),
- the code identity at proof time (the code photograph — a deterministic digest over every tracked file's content, `.walden/` excluded so committing evidence never invalidates it),
- the [execution profile](#the-environment) of the machine that ran it,
- `result: passed | failed` and a timestamp.

What the ledger deliberately does **not** hold is any state label. When you ask — `walden evidence status`, or any command that reads evidence — Walden takes today's photographs and compares:

| State | Meaning |
| --- | --- |
| `verified` | Required binding, available code identity and supported producer-policy facts match the current assessment. |
| `stale-spec` | A known contract or approval-chain mismatch exists. |
| `stale-code` | Available recorded/current code identities differ. |
| `failed` | A recorded assertion failure or known verification-policy violation blocks acceptance. |
| `unattested` | A record exists but a required binding, identity or execution-assurance fact is unknown. |
| `unrecorded` | The task is checked but has no evidence record. |
| `pending` | Not completed yet. |

*Stale* does not mean broken. It means **"no longer known"** — the world moved since the claim was proven, and the honest state is a declared doubt. A code-only bugfix needs no spec ceremony: evidence goes `stale-code`, and one `walden verify` either restores `verified` on today's tree or names exactly which task's proof broke.

The ledger is a current-state map; historical recovery depends on actually preserved Git history. Normal feature-local verify prunes orphaned task entries, while inspection never prunes. A missing record is `unrecorded`; a corrupted or unsupported ledger is a read error, not an empty trusted map. Preserve it for diagnosis rather than deleting history as a universal remedy.

New ledgers use `v1alpha2`; old records remain readable with explicit uncertainty. [Legacy binding recovery](adoption.md) can translate a justified full-plan witness without replay, but cannot manufacture old producer/purity facts. Binding, code freshness and execution provenance are reported separately even when one state takes precedence. Document schema and intact approvals remain unchanged.

## The environment

Every record also carries the **execution profile** of the machine that produced it: `platform` (OS/arch) and `walden` (recording CLI version) always, plus the outputs of probes the repository declares in `.walden/environment.md`:

```markdown
# Environment Probes

- go: ["go", "version"]
- node: ["node", "--version"]
```

Profiles are diagnostic by design — they never change a derived state or establish an execution-integrity guarantee. A missing profile does not by itself prove a record's age or invalidate otherwise supported provenance. Their payoff is the failure you can finally read: when a proof that passed under go 1.25 fails on a machine running 1.24, the failure says so — `environment drift: go: recorded "go1.25.0" → current "go1.24.0"` — and `evidence status` shows recorded-versus-current differences per task. Fix the environment, or knowingly re-record on the current one; never edit proofs to paper over drift.

## The verdict

`walden release check` folds the whole lifecycle into one deterministic answer per feature, plus one repository-level check:

| Criterion | What must hold |
| --- | --- |
| `chain` | Every document approved, every fingerprint intact, the chain fresh |
| `validation` | Full-spec structural validation passes |
| `decisions` | No unresolved `[decision: …]` markers in approved documents |
| `evidence` | Every completed task's evidence derives `verified`; every planned task executed — or explicitly waived |
| worktree | No uncommitted changes outside `.walden/` — what you certify is what you tag |

Exit `0` if and only if no blocker exists; every blocker names its remedy. The gate is **judgment only**: it executes no proofs or environment probes and writes nothing. `verify` produces evidence; `release check` judges it. The verdict names its selected scope and guarantee. A named-feature pass is not a repository-wide certificate; unqualified requests retain every feature in the present portfolio.

**Pending work blocks by default.** An approved plan's unexecuted task means unimplemented acceptance criteria in the thing being tagged. Partial releases remain expressible — as a recorded decision: `--allow-pending --reason "<text>"` waives pending work for that verdict, the completion class becomes `with-waivers` (against `complete` and `with-pending`), and the reason plus the waived task identifiers ride the verdict itself. No reason, no waiver. Waivers do not exempt failed or unattested completed evidence. `--strict` compares the presence and exact bytes of the captured spec/evidence inputs with one existing commit, independently of ignore rules, and checks the committed portfolio inventory for an unqualified request. Missing/different/unreadable inputs and observed HEAD movement block it. An equally absent ledger is legitimate for a wholly pending waived plan, never for completed work. Ignored scratch outside the consumed set is irrelevant. Non-strict mode still tolerates local Walden metadata and explicitly reports that committed-input binding was not requested.

A repository without usable git fails closed: certification requires a git-backed code identity. There is no bypass flag for a dirty worktree.

## Beyond one feature

Two lanes extend the lifecycle across a whole portfolio:

- **[Brownfield adoption](adoption.md)** — establish the current business contract first; `walden adopt` then assesses legacy binding, freshness and provenance without execution. Apply requires an explicit reviewed scope and records real proof outcomes.
- **[Retirement](adoption.md#retirement)** — confirmed superseded specs can be removed after verifying recoverable Git history and the destination of surviving requirements. An index entry in `.walden/RETIRED.md` records reason, last-live commit and successor; nothing silently disappears from a portfolio claim.

## The invariants

Five rules hold everywhere and explain most design decisions you will encounter:

1. **Derive, don't store.** States (`verified`, `stale-*`, phase freshness) are computed from facts at read time. There is nothing to forget to update.
2. **Produce and judge are separate.** Commands that execute proofs (`task complete`, `verify`, `adopt --apply`) write evidence; the gate (`release check`) only reads. No verdict has side effects.
3. **Fail closed, name the remedy.** Missing fingerprints, unusable git, unsupported schema versions — each is a named blocker with its fix, never a silent pass.
4. **The relaxation is always recorded.** There is no quiet bypass: waivers require a reason and ride the verdict; adoption's trust assumption is stated in the plan.
5. **The JSON contract only grows.** Machine output is versioned (`schema_version: v0beta1`) and changes additively; see the [JSON reference](reference/json.md).
