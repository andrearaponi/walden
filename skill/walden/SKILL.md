---
name: walden
description: "Walden drafts and maintains feature specs in `.walden/specs/` with EARS requirements, reviewed design, tasks, and executable proofs. Use when the user asks for Walden, a feature spec, requirements, design, an implementation plan, or work governed by an existing Walden spec. First distinguish contract-preserving maintenance from new or changed behavior."
metadata:
  short-description: Walden spec workflow
---

# Walden

Author and review deliberately; delegate workflow mechanics to the CLI. Reply in the user's preferred language. The skill supplies judgment, not approval on the user's behalf.

## Entry Decision

Before creating a feature or choosing a phase, inspect the request, applicable project rules, and existing contracts. Read `.walden/constitution.md` and relevant `.walden/lessons.md` when present. Do not create a spec merely because the user mentioned Walden.

| Contract impact | Route |
| --- | --- |
| Preserve or restore intended behavior | Keep the work outside a new spec-authoring cycle, unless explicit user scope or project rules require a spec. Run existing tests and refresh applicable Walden evidence. |
| Introduce or change intended behavior | Enter authoring: requirements for a new feature, or the earliest affected phase of an existing spec. |
| Unclear | Ask a focused question about intended behavior before selecting a lane or creating speculative documents. |

A bugfix can restore an approved contract without changing it. A refactor can preserve it. Neither route bypasses tests, evidence checks, or execution authorization. An explicit request to implement a maintenance fix authorizes that work; approval of a planning document alone does not.

Before closing maintenance, identify the applicable current contract, run existing tests, and refresh the selected affected evidence with `walden verify <feature>` at the declared checkpoint. Use `--all` only for a justified forced check. Inspect the CLI outcome; raw test output alone does not refresh evidence. If no spec covers the change, state that limitation without creating a spec merely to obtain a checkbox.

For an existing portfolio, establish applicability **within the user's requested scope** before proposing adoption or replay:

| Proposed applicability | Review action |
| --- | --- |
| Current | Retain the required behavior in the selected verification scope. |
| Superseded | Cite the replacing/removing decision and successor; propose retirement, never perform it automatically. |
| Mixed | Identify surviving requirements and their agreed destination before discarding obsolete design/tasks. |
| Unresolved | Ask the specific business/scope question; do not invent conformity. |

Ground proposals in decision sources. Age, missing fingerprints and completed checkboxes do not establish obsolescence. Requirements can survive an outdated implementation plan. A current requirement does not make its historical bootstrap/deployment task a current regression check: inspect the actual proof before proposing replay. If it records one-time delivery work, propose the needed current baseline/regression contract for review, not automatic backfill plus execution merely because the CLI reports a re-prove count. Code describes observed behavior, not authority to rewrite intent; a newer draft does not automatically supersede an approved contract. If sources conflict, ask before retiring or changing the contract. A case-study request yields general findings for the product being improved, not unsolicited operations in the example repository.

For existing specs, use CLI status and validation rather than guessing freshness. For a new spec, use CLI-generated scaffolds as the canonical document templates. Do not reconstruct full templates from memory.

## CLI Prerequisite And Installation

Requires Walden CLI **v0.10.2 or a newer compatible release**; the installer supports macOS/Linux on amd64/arm64. Before the first CLI operation, check `command -v walden` and that executable's `version --json`. If PATH has no usable compatible CLI, check `$HOME/.local/bin/walden` too. Reuse a compatible binary by its verified path or a session-only PATH correction; do not downgrade it or edit persistent shell configuration. An unclear version is not assumed compatible, and a local candidate does not prove a public release exists.

If installation or replacement is needed, explain the version, official source and `~/.local/bin/walden` destination; obtain **explicit approval before any download or installation**. Only after approval, use this pinned binary-only bootstrap:

```sh
(
  set -e
  installer_dir=$(mktemp -d)
  trap 'rm -rf "$installer_dir"' EXIT
  curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/v0.10.2/install.sh -o "$installer_dir/install.sh"
  sh "$installer_dir/install.sh" --version v0.10.2 --no-skill
  "$HOME/.local/bin/walden" version --json
)
```

Confirm the actual executable's version is compatible before continuing, and use that path if an older CLI still shadows it. Installer exit zero is not enough. Refused consent, unsupported platforms, unavailable release assets or failed postchecks stop the dependent operation; do not substitute `latest`, `sudo`, `--no-verify` or manual workflow metadata. The pinned URL requires the matching public release; it is not a claim that an unpublished candidate is downloadable.

For a **Skills CLI**-managed guide, use `npx skills update walden` for the skill and the binary-only installer with an explicitly selected compatible release for the CLI. **Do not use `walden update` for that channel:** it also re-syncs skills. Native Walden installations keep their existing update flow.

Ownership is a recorded fact, not an inference. When the user reports native and Skills CLI installations, or copies are overlapping, run `walden skill status --json` before recommending any update path. Classify every `"installed": true` entry mechanically: an entry with a `version` field was written by `walden skill install` (Walden re-syncs it); an entry without `version` was not written by this CLI (Skills CLI, manual copy or pre-marker install) and `walden update` would overwrite it. `in-sync`, identical content, user/project scope and a missing `npx` are not ownership evidence; the user's stated channel wins over any inference. Report the classification per path, ask which manager owns each copy, and do not run `walden update` or `skill install` until the user confirms; never automatically remove copies, replace symlinks or reinstall the skill through another manager.

## Ownership And Authorization

- Resolve the CLI prerequisite above before any Walden operation. Bootstrap consent does not approve specifications, authorize task execution or permit publication; never invent a replacement state machine.
- Never hand-edit workflow frontmatter, timestamps, fingerprints, checkboxes, or evidence contents. Edit document bodies; use the CLI for state mutations. Preserve schema and extension fields already present in scaffolds.
- Never treat silence as approval. Present the relevant document and wait for explicit approval before `review approve`.
- Planning stops at approved tasks. Implementation of the plan requires an explicit execution request; it is not an automatic consequence of approving tasks.
- Waivers require the user's explicit approval of the pending scope and reason in the current conversation. Never supply `--allow-pending` for convenience.
- Retirement requires explicit user confirmation. Do not delete superseded specs or evidence as housekeeping.
- For a non-trivial request, state a short plan and its verification steps. If constraints conflict, a proof fails, or the spec is insufficient, stop and re-plan from the earliest affected phase.
- Before authoring, ask for stable project context if an existing constitution contains only placeholders. Do not manufacture the project's stack or rules.
- A fingerprint binds approved content, not semantic correctness. Good requirements, meaningful proofs, and human review remain necessary.

## Command Lookup

Use commands for mechanics; use `walden --help` and command help for syntax details. Read JSON results and warnings, not just a green-looking summary.

| Command | When to use it |
| --- | --- |
| `walden repo init` | Bootstrap an uninitialized repository after choosing the authoring lane. |
| `walden feature init <feature>` | Create the three canonical documents; names normalize to kebab-case. |
| `walden status <feature>` | Inspect phase, freshness, blockers, and next action. |
| `walden validate [<feature>] [--all]` | Validate the current phase; `--all` checks the full spec. Omit the name for the portfolio. |
| `walden review open <feature> --phase <phase>` | Present a ready document for review, using `requirements`, `design`, or `tasks`. |
| `walden review approve <feature> --phase <phase>` | Seal that phase only after explicit user approval. |
| `walden reconcile <feature>` | Repair a changed approved chain before re-reviewing it. |
| `walden task status <feature>` | Inspect executable work and evidence warnings. |
| `walden task start <feature> [task-id]` | Obtain execution context before implementing an authorized task. |
| `walden task complete <feature> <task-id>` | Run the declared proof and record completion only on success. |
| `walden task complete-all <feature>` | Complete an explicitly authorized runnable batch; stop on the first failure. |
| `walden evidence status <feature>` | Inspect derived evidence and environment differences; executes diagnostic profile probes. |
| `walden verify <feature>` | Re-prove completed tasks whose evidence is no longer verified. |
| `walden verify <feature> --all` | Force every completed task's proof to run, including currently verified tasks. |
| `walden verify <feature> --check` | Report without persisting the ledger; add `--all` to force all completed proofs. |
| `walden release check [<feature>] [--strict]` | Judge existing evidence and release blockers; never executes proofs or publishes. |
| `walden adopt [<feature>] [--apply]` | Inspect legacy bindings/freshness/provenance without proofs or probes; apply executes selected proofs and needs authorization. |
| `walden lesson log --feature <name> --phase <phase> --trigger "..." --lesson "..." --guardrail "..."` | Record a reusable correction and its prevention rule. Phases include `execute` and `release`. |
| `walden version` | Inspect binary/schema versions. |
| `walden skill show` / `walden skill status` | Inspect the embedded guide and installed-content drift. |

Commands support `--json`, including failures. The envelope contains `schema_version`, `command`, `ok`, and `result`; inspect warnings and command-specific fields. Do not re-implement it or infer success from stdout alone.

## Authoring Phases

After the entry decision, locate `.walden/specs/<feature>/`. Use the earliest missing, unapproved, or stale phase. New features always start at Requirements. Existing work may enter Design only with fresh approved requirements, and Tasks only with a fresh approved design.

Use the same review loop in each phase:

1. Read upstream content and relevant lessons; draft or revise the current body.
2. Validate the current phase and resolve structural defects and meaningful warnings.
3. Open its review through the CLI, present the document, and ask for approval.
4. After explicit approval, seal it through the CLI and proceed only within the user's requested scope.

Do not mechanically approve multiple phases from one vague acknowledgment. If implementation reveals a contract gap, pause, revise the earliest affected document, reconcile, and re-walk the affected review gates.

### Decision Checkpoint Protocol

Apply during requirements, design, and task drafting, not execution.

- **Bifurcation Test:** checkpoint only when a different choice would discard or substantially rewrite downstream content. When inconclusive, default to autonomous resolution.
- **Explore before asking:** check the codebase, constitution, and approved upstream documents. If they resolve the choice, record `<!-- assumed: <choice> (source: <file or document>) -->` and continue.
- **Checkpoint:** insert `[decision: <question>]`, explain the fork, recommend an option with a one-line rationale, and stop generating dependent content. If no option is defensibly better, do not fabricate a preference.
- **Resume:** explain how the user's answer will be applied before continuing. A newly exposed significant fork gets its own checkpoint before dependent content is written.
- **Autonomy:** if the user delegates a checkpoint decision, record `<!-- assumed: <choice> -->` and proceed without further checkpoints for decisions within that delegated scope.
- **Budget:** no more than five checkpoints per phase drafting session. Keep unresolved checkpoints in draft; do not open phase review while one remains unresolved.
- **Ordinary assumptions:** record `<!-- assumed: <choice> -->` inline without interrupting the user.

### Requirements

Once intent is sufficiently clear to select authoring, draft first, then iterate. User stories explain context; the acceptance criteria are the contract.

- Give requirements stable IDs (`R1`), criteria stable IDs (`R1.AC1`), quality attributes `NFR1`, and constraints `C1`. Do not renumber IDs already referenced elsewhere.
- Write each AC in EARS with one observable response. The CLI validates keyword structure, not the quality or completeness of the behavior.
- Include explicit out-of-scope items when scope risk is high. Do not split a feature automatically because its AC count crosses a threshold; reconsider cohesion and scope instead.
- Attach one short `Acceptance check:` continuation below each AC. Describe the observation separating success from failure, not a chosen framework or implementation command. If that observation cannot be stated, clarify or rewrite the criterion before requesting approval.

After every revision, including restoring original AC wording, inspect all presented criteria and preserve or reattach their acceptance checks. Keep the requested EARS sentence unchanged where appropriate; its separate check continuation is explanatory content, not a new behavior requirement.

| EARS form | Shape |
| --- | --- |
| Ubiquitous | `The system SHALL <response>` |
| Event-driven | `WHEN <event>, the system SHALL <response>` |
| State-driven | `WHILE <state>, the system SHALL <response>` |
| Optional feature | `WHERE <feature>, the system SHALL <response>` |
| Unwanted behavior | `IF <undesired condition>, THEN the system SHALL <response>` |
| Complex | `WHILE <state>, WHEN <event>, the system SHALL <response>` |

Quality checks during drafting:

- **Form:** invariants need no invented user trigger. Prefer “The system SHALL assign distinct identifiers” to “WHEN the user starts, ...” for an always-on uniqueness rule.
- **Atomicity:** split “generate the record and deliver the notification” into independently observable criteria. Check each response before drafting the next; the word “and” alone is not proof that two behaviors exist.
- **Specific events:** prefer “WHEN an OrderCreated message arrives” or “WHEN a lease expires” to “WHEN processing happens.” API and background events are as valid as clicks or submissions.
- **Failure coverage:** for each realistic constraint failure, identify the required response. Storage requirements need read, write, and relevant failure behavior, not only a happy-path save.
- **NFR promotion:** “IF the connection fails, THEN ... reconnect” is behavior and belongs in an AC; retain the resilience attribute as the NFR.
- **NFR bridge:** connect each NFR to testable ACs. “Accessible” needs concrete keyboard/screen-reader outcomes; a resource limit can need an internal measurement rather than an invented UI interaction. Ask about vague quality claims.

Minimal criterion example, not a document template:

```markdown
1. `R1.AC1` WHEN a lease expires, the system SHALL release its lock.
   - Acceptance check: another holder can acquire a lock after its lease expires.
```

Read `ears_validation`, `ears_distribution`, and warnings from `walden validate --json`. Review form appropriateness, response atomicity, specific events, realistic failures, NFR bridges, persistence balance, and domain-specific gaps. Counts are diagnostic, not a quota; zero unwanted forms may indicate missing failure handling. Use the normal review loop before Design.

### Design

Read approved requirements. Research external facts only where a decision depends on them. Use the CLI-generated design scaffold rather than inserting a standard long architecture document.

Exactly six headings are required by the current kernel:

- Architecture
- Options Considered
- Simplicity And Elegance Review
- Failure Modes And Tradeoffs
- Verification Plan
- Requirement Coverage

Other sections — components/interfaces, data models, error handling, security, testing strategy, diagrams — are optional expansions for relevant decisions, contracts, or verification needs. Do not add empty sections to demonstrate thoroughness.

Compare with a real alternative. When no meaningful alternative or discussion applies, give a one-line reason rather than inventing complexity. Requirement Coverage and Verification Plan still need substantive mappings and checks; non-applicability is not a way to omit them.

Challenge the first draft once: could a simpler shape or less coupling satisfy the same contract? Record real failure modes and accepted tradeoffs. Wrap every requirement/NFR ID in the coverage table in backticks: the validator expects rows such as ``| `R1` | ... |``.

If design exposes new scope or missing acceptance behavior, return to Requirements. Otherwise validate and use the review loop before Tasks.

### Tasks

Read approved requirements and design. Keep work incremental and testable, with at most two hierarchy levels. Each executable leaf names its AC IDs, design references, and verification steps. Use the canonical scaffold and minimal syntax below, not a copied full plan.

```markdown
- [ ] 1. Implement and test the behavior
  - Requirements: `R1.AC1`
  - Design: Architecture
  - Verification:
    - command: ["go", "test", "-v", "-count=1", "-run", "^TestExample$", "./pkg/example"]
      expect_output: "--- PASS: TestExample"
      covers: ["R1.AC1"]
```

Top-level leaf metadata uses two spaces; child tasks use two-space task indentation and four-space metadata. Proof steps sit two spaces deeper than `Verification:`, and attributes two spaces deeper than their command.

Assign proofs to sensible groups of criteria rather than running the same broad suite for every checkbox. An already-green suite is not proof of newly introduced behavior: each implementation leaf must include its corresponding new assertions before it can complete. Do not postpone those tests to a later leaf while claiming new ACs through an unchanged old PASS marker. Use a direct assertion or a new test-specific selector/marker that cannot pass before the intended check exists. A proof step asserting ACs must name them in `covers:`. If sequencing exposes a missing interface or untestable step, return to Design. Validate and request task-plan approval; then stop unless explicit execution was also requested.

## Proof Authoring

Before presenting a task plan:

1. **Actual assertion:** every step has an explicit pass condition; exit zero alone is useful only if the command actually asserts the intended property.
2. **Non-vacuity:** prevent a passing run with zero intended tests. Use native fail-on-no-tests behavior, structured test results, or an appropriate output assertion. Go's `-run` may match nothing successfully: use an anchored selector, `-v`, `-count=1`, and the named PASS output as shown above.
3. **Read-only re-verification:** assertions must not alter the worktree. Prefer `go mod tidy -diff` over `go mod tidy`; route build/generator outputs outside the repository. A missing file or failing assertion must not be “repaired” by its proof.
4. **Bounded execution:** each step has a 10-minute default timeout. Declare a positive `timeout:` for a justified different budget, such as `timeout: 30m`. Expiry fails the step.
5. **Declared coverage:** `covers:` names known asserted AC IDs, not a promise that the command is semantically sufficient. Inspect task-reference and proof-reference coverage separately.
6. **Reproducible conditions:** use known inputs and relevant environment probes. Do not disguise environment failures by weakening the approved proof.

Commands are JSON argv arrays, executed without implicit shell interpretation. Pipes, redirects, and operators require an explicit shell, for example `command: ["sh", "-c", "test -d src && test -f src/main.go"]`. Use `expect_exit: 1` for a genuinely negative assertion. `expect_output` is a substring of combined output, not a regular expression. Multi-step proofs stop on the first failure.

The old single-line `Verification: go test ./...` format cannot express quoting, shell operators, or attributes; use structured steps for new work. Do not retrofit approved proofs outside an authorized revision and re-review cycle.

## Execute And Recover

For an explicitly authorized task or batch:

1. Read the three approved documents. Use `task status` and `task start` for readiness and context; never implement an unapproved or stale plan.
2. Declare the authorized batch scope and its verification checkpoint, then implement with focused tests. Do not run the full suite unless requested or included in the approved verification plan. If no task/batch was identified, resolve the next task and ask before implementing.
3. Complete through `task complete`, not a manual checkbox. The CLI executes proofs and records evidence before moving completion state. Complete children before containers.
4. Commit evidence with the work only when committing is authorized and project policy permits it. This skill does not grant permission to commit, open PRs, tag, or publish.
5. Inspect evidence warnings, but ordinary repo-wide stale-code after an edit is not a reason to replay every completed prefix. Bring targeted checks forward for a changed dependency, failed proof or contract gap; otherwise aggregate re-verification at the declared batch, feature or delivery checkpoint. Every leaf still earns its own completion proof. At an intermediate pause keep the batch and residual staleness explicit; at closure finish code, README and other deliverables before the final scoped verification, then inspect the release verdict and report any deferred gaps.

TDD describes development order, not a passing proof. When TDD is required, observe a runnable behavioral assertion fail before implementing the behavior; minimal compilable scaffolding is fine. Compile/import failures and named PASS output do not establish that sequence. If code already exists, report tests added afterward or a test-driven repair accurately. Do not break and restore existing code to manufacture retrospective TDD; label mutation testing separately.

A document-body edit breaks its approval seal. `reconcile` resets affected approval state; review the changed chain again. Re-approving identical content does not stale downstream content. Missing legacy fingerprints need adoption or reconciliation, not hand-written seals.

Completion can legitimately run a generator and records the resulting tree. Re-verification is different: its proofs are read-only. `verify --check` prevents ledger writes, not command side effects. A detected mutation or missing required identity contaminates that run: later executions cannot earn verified evidence even if the bytes are restored. Investigate the named cause; after any authorized proof repair/re-review, use a new clean verification of the affected scope. Never automatically roll back user source.

Evidence states are derived: `verified`, `stale-spec`, `stale-code`, `failed`, `unattested`, `unrecorded`, and `pending`. Inspect **binding, code freshness and execution-integrity provenance separately**, including gaps hidden by state precedence. `unattested` means required assurance is missing, not a newly observed code failure. A recorded `passed` is not enough for current verification. Intended contract changes need the affected review cycle and new proofs; code-only maintenance need not create a new spec. Committing unchanged bytes does not by itself stale code evidence; use the CLI's reported differences rather than guessing the cause.

### Environment context

Optional `.walden/environment.md` probes use argv, for example `- go: ["go", "version"]` and `- node: ["node", "--version"]`. Names are lowercase kebab-case; `platform` and `walden` are reserved profile keys. Probes share a 30-second budget. Malformed declarations fail evidence-producing commands; failed or timed-out probes yield diagnostic marker values.

Profiles are diagnostic, not a state gate or proof of producer policy. Compare environment drift before blaming code; never weaken an assertion to hide a version mismatch. When the request forbids environment probes, use the read-only `adopt <feature>` assessment rather than `evidence status`.

### Adoption and retirement

`adopt` assesses existing Walden specs, not an arbitrary codebase into a specification. After establishing the current contract, present its non-executing plan: `backfill`, `re-prove`, `complete`, or `blocked` are technical classes, not business applicability. A recovered binding does not attest old execution purity. `complete` means nothing to adopt, not that all product work is implemented. No upgrade or inspection automatically replays the portfolio.

Backfill trusts recorded approval and seals the current body; disclose that intervening pre-fingerprint edits cannot be detected. Present contradictions require human reconciliation. Apply runs real proofs: request it only for the reviewed scope, retaining the feature selector. New `v1alpha2` ledgers can contain untouched legacy records; use a compatible CLI and never delete or downgrade evidence to satisfy an old reader.

Retirement requires explicit confirmation. **Before deletion**, verify that Git preserves each spec/evidence file at a recoverable last-live commit, identify the reason and successor, and agree where any surviving requirements belong. Missing history or an unresolved mixed contract means stop, not fabricate recovery or create an unauthorized commit. Do not offer history-free deletion as an equivalent Walden retirement or a waiver of its recovery prerequisite. Then remove only the authorized files and record name, date, reason, last-live commit and successor in `.walden/RETIRED.md`. The optional `walden-history` companion can assist; it is not required, and retirement does not independently authorize committing or publishing.

### Release judgment

When asked about releasability or a delivery hand-off, use `release check`; do not invent a verdict from separate status summaries. The command judges existing state and never publishes. Produce evidence with `verify` separately.

Pending work blocks by default. A user-approved partial delivery may use `--allow-pending --reason "..."`; it never waives missing legacy assurance or strict input binding. `--strict` compares the actual judged spec/evidence bytes and presence with one captured commit, even for ignored files. Non-strict output does not claim that local metadata is committed. Uncommitted code has no bypass, and usable git is required.

Report scope, guarantee, blockers, completion class, commit and waiver accurately. A selected-feature pass is **not a repository-wide certificate**. Unclassified/unassessed features cannot disappear from a whole-portfolio claim; exclusion requires explicit scope selection or authorized retirement. Do not equate reference coverage or a stored pass with formal correctness.

## Lessons And Summaries

Review relevant lessons before similar work. User corrections, rejected drafts, failed validation, re-planning, or unexpected execution failures prompt an internal decision about a reusable lesson. Log meaningful patterns with `walden lesson log`: trigger, lesson, and a guardrail that would prevent repetition. Do not turn ordinary expected TDD red tests into a list of invented mistakes.

Describe the observed trigger accurately. If the user first requested A+B and later keeps only A, record “scope narrowed from A+B to A,” not “the agent invented B.” Do not claim two behaviors shared one AC if they were already separate. Lessons preserve facts and useful guardrails, not invented mistakes. Quote or closely paraphrase the actual non-sensitive scope clarification for the trigger: “requested” is not “approved.” A user may request a preventive lesson without any mistake having occurred; label it as preventive rather than reconstructing a failure story.

A newly logged lesson gets a short user-facing entry with its practical guardrail. With no new lesson, omit lesson-status output entirely; the internal decision still happens.

Summaries should state what changed, what was actually checked, unresolved blockers, and the next required user decision. Distinguish planned, executed, failed, and not-run checks. Be concise and do not confuse approval of the current phase with permission for later phases or publication.
