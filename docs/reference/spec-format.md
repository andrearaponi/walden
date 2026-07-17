# Spec File Format

The complete file-level contract: the `.walden/` layout, document frontmatter, EARS criteria, the task and proof grammar, and every convention file. Everything here is plain Markdown and JSON in your repository — reviewable, diffable, recoverable from git.

## Layout

```text
.walden/
  constitution.md          # optional stable project context (no approval workflow)
  environment.md           # optional declared toolchain probes
  lessons.md               # append-only structured lessons
  RETIRED.md               # retirement index (convention; created at first retirement)
  .gitignore               # excludes transient staging artifacts only
  specs/<feature>/
    requirements.md
    design.md
    tasks.md
  evidence/<feature>.json  # execution evidence ledger (committed, reviewed)
```

Feature names are kebab-case (`feature init` normalizes). A feature is a directory by contract; plain files under `specs/` are ignored.

## Frontmatter

Every spec document opens with YAML frontmatter:

```yaml
---
walden_schema_version: v1alpha1
status: draft            # draft | in-review | approved
approved_at:             # stamped at approval (UTC, RFC 3339)
last_modified: 2026-07-17T10:00:00Z
approved_fingerprint:    # sha256:… — recorded at approval
source_requirements_approved_at:      # design.md only
source_requirements_fingerprint:      # design.md only
source_design_approved_at:            # tasks.md only
source_design_fingerprint:            # tasks.md only
---
```

Rules the loader enforces:

- **Schema version.** `walden_schema_version: v1alpha1` is scaffolded on new documents and stamped on every CLI save (existing repositories migrate through normal use). A document declaring an unsupported version is refused, naming the declared version, the supported one, and the remedy. Documents without the field load as legacy.
- **Unknown fields are refused** — the writer's allowlist is the loader's rejection list, so a typo'd field cannot silently vanish on the next save.
- **`x-` extensions survive.** Fields prefixed `x-` (e.g. `x-tracking-url`) are preserved verbatim through every CLI mutation, serialized in lexicographic order after the core keys. They never participate in fingerprints: attaching metadata cannot invalidate an approval.

### Fingerprints

`approved_fingerprint` is SHA-256 over the document **body** — frontmatter never participates, so timestamps, status flips, and extensions don't break seals. One path-aware rule: for `tasks.md` only, checkbox states (`[ ]`/`[x]`) are normalized out — executing the plan is not editing the plan. Everywhere else, every body byte counts.

The `source_*` pairs chain approvals: design binds to the requirements fingerprint, tasks to the design's. Freshness derives from these fields alone; see [the lifecycle](../lifecycle.md#the-chain).

## `requirements.md`

Required structure (validated):

```markdown
# Requirements Document

## Introduction
## Requirements
### R1 <Short title>
**User Story:** As a <role>, I want <capability>, so that <benefit>
#### Acceptance Criteria
1. `R1.AC1` WHEN <trigger>, the system SHALL <response>
## Non-Functional Requirements
- `NFR1` <requirement> (bridged by `R1.AC1`)
## Constraints And Dependencies
- `C1` <constraint>
## Out Of Scope
```

### EARS acceptance criteria

Every criterion has a stable ID (`R<n>.AC<m>`) and one EARS form, classified by the validator:

| Form | Shape |
| --- | --- |
| ubiquitous | `The system SHALL <response>` |
| event-driven | `WHEN <trigger>, the system SHALL <response>` |
| state-driven | `WHILE <state>, the system SHALL <response>` |
| optional | `WHERE <feature is present>, the system SHALL <response>` |
| unwanted | `IF <undesired condition>, THEN the system SHALL <response>` |
| complex | combined keywords |

One `SHALL` per criterion (two SHALLs = two criteria); `IF` requires a matching `THEN` before the `SHALL`. The validator reports the form distribution and warns when no unwanted-behavior (IF/THEN) criteria exist — failure modes deserve criteria too. IDs are the traceability currency: tasks and proofs reference them, and they should never be renumbered once referenced.

## `design.md`

Required structure: overview, architecture, **options considered** (at least one alternative with a why-rejected), simplicity review, components and interfaces (each naming the requirements it covers), data models, error handling, security considerations, failure modes and tradeoffs, testing strategy, verification plan, and a requirement coverage table mapping every `R*`/`NFR*` to what covers it.

Open forks can be parked as `[decision: which store backs this?]` markers — visible, greppable, and blocking at the release gate while unresolved in an approved document. HTML comments (`<!-- assumed: … -->`) are the place for recorded assumptions; an *unterminated* HTML comment in an approved document blocks certification, because it would blind the decision scan.

## `tasks.md`

A two-level plan; only leaf tasks execute:

```markdown
# Implementation Plan

- [ ] 1. <Top-level objective>
  - [ ] 1.1 <Concrete step>
    - Requirements: `R1.AC1`, `NFR1`
    - Design: <Design section name>
    - Verification:
      - command: ["go", "test", "-run", "TestX", "./internal/x"]
        expect_exit: 0
        expect_output: "--- PASS: TestX"
        timeout: 30m
        covers: ["R1.AC1"]
```

Per leaf task:

- **`Requirements:`** — acceptance-criterion IDs this task implements. Validated against known IDs.
- **`Design:`** — the design section it follows.
- **`Verification:`** — one or more proof steps. Each step:
  - `command:` — argv as a JSON array; no shell interpretation, no quoting pitfalls.
  - `expect_exit:` — required exit code (default `0`).
  - `expect_output:` — substring the combined output must contain. Declare it on test runners so a `-run` pattern matching zero tests cannot pass vacuously.
  - `timeout:` — positive Go duration (`90s`, `30m`) bounding the step; default 10 minutes. Expiry kills the step's process group and fails the proof naming the budget. A *declared* timeout participates in the task-definition fingerprint; the default does not.
  - `covers:` — acceptance-criterion IDs this proof demonstrates; validated, and reported as proof coverage distinct from task references.

A legacy single-line form (`Verification: go test ./...`) still parses but supports no quotes, pipes, or attributes.

Checkbox flips are workflow state, not content: they are excluded from the tasks fingerprint, and `task complete` is the intended way to flip them (proof first, checkbox second).

## `constitution.md`

Optional, repo-wide, and deliberately outside the approval workflow: project summary, tech stack, conventions, sanity-check commands, key files, hard rules. No freshness rules, no gates — the CLI never validates it. Agent skills read it to avoid rediscovering stable context per feature.

## `environment.md`

Declared toolchain probes, in the same argv format as proof steps:

```markdown
# Environment Probes

- go: ["go", "version"]
- node: ["node", "--version"]
```

Probe outputs (trimmed) join every evidence record's execution profile alongside the reserved keys `platform` and `walden`. Probes run once per command under a 30-second shared budget; a failing or hung probe degrades to a marker value, never a failed command. Prefer commands with stable output — timestamps or absolute paths in probe output read as permanent drift.

## `lessons.md`

Append-only, written by `walden lesson log`: feature, phase, trigger, lesson, guardrail. The structured form is the point — a lesson without a guardrail is a story; with one, it is a check.

## `RETIRED.md`

The retirement index (see [Adoption](../adoption.md#retirement)): one line per retired feature — name, date, reason, last commit where the spec was alive. The spec directories themselves are deleted; git keeps the full history and `git show <commit>:.walden/specs/<name>/requirements.md` recovers any of it.

## `evidence/<feature>.json`

The execution ledger, schema `v1alpha1`: a map keyed by task ID, each record holding proof steps and outcomes, the chain fingerprints and code identity at proof time, the execution profile, `result`, and `verified_at`. Facts only — states are derived at read time. Committed and reviewed like the specs it proves; fully regenerable by re-proving (history lives in git). The field-level shape is documented in the [JSON reference](json.md#evidence-one-shape-three-surfaces).
