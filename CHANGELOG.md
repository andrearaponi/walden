# Changelog

All notable changes to Walden will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). This project uses semantic versioning. The JSON contract uses `v0beta1` until the CLI stabilizes to v1.0.0.

## [0.9.1] - 2026-07-14

### Added

- **Per-command help everywhere.** Every command and subcommand now answers `--help`/`-h` with its syntax, summary, and flags on stdout (exit 0), resolved before positional validation — `walden validate --help` explains itself instead of failing with `feature spec not found: help`. Group commands (`task --help`, `review --help`, …) list their subcommands. Help output derives from a single command registry, so the top-level usage table and per-command help can never drift apart.
- **Uniform unknown-flag rejection.** Every command rejects undeclared flags with a non-zero exit, an error naming the flag, and a pointer to `--help`; with `--json` the rejection honors the JSON envelope contract. A registry-driven uniformity test makes every current and future command inherit both behaviors by registration alone.
- **Portfolio validation: `walden validate` with no feature name validates every feature** under `.walden/specs/` in sorted order, reporting one verdict per feature and exit 1 if any fails — mirroring `release check [<feature>]`. `--all` applies full-spec scope to every feature. JSON output gains an additive `features` array (`feature`, `valid`, `summary`). An empty repository fails with a pointer to `walden feature init`.

### Changed

- **Breaking (approval contract): review approve now runs full validation before recording approval.** The approve gate previously checked workflow state and freshness only, so a document the validator (or the execution parser, for tasks) rejects could still become approved — the defect then surfaced at execution time, far from its cause. `walden review approve` now refuses invalid documents with `approval refused: <defect>` and exit code 1, leaving the document untouched. Documents that used to slip through must be fixed before re-approval; `walden validate <feature>` reproduces the refusal reason exactly.
- **Breaking (gate criteria contract): `walden release check` fails closed without a code identity.** A repository without usable git previously skipped the worktree criterion and could certify releasable; it now emits a repository-level blocker — certification requires a git-backed code identity, because a verdict must name the exact tree it certified. Non-release commands keep their documented no-git tolerance. (`git_skipped` keeps its meaning in JSON; the verdict changes.)
- **Breaking (gate criteria contract): `--strict` requires committed `.walden/` state.** Dirty `.walden/` paths still warn in the default mode — a freshly refreshed ledger legitimately precedes its own commit — but under `--strict` each one is promoted to a worktree blocker: a plans-complete certification must be reproducible from the commit it judges. The `walden_dirty` JSON partition is unchanged.
- **Breaking (gate criteria contract): unterminated HTML comments block the decisions criterion.** A dangling `<!--` in an approved document used to swallow the rest of the document from the decision-marker scan — hiding any open checkpoint after it. The malformed comment is now itself a blocker naming the document and remedy.

### Fixed

- **Top-level leaf tasks now execute.** The execution parser hardcoded the classic child offsets (metadata at 4 spaces, proof lines at 6/8), so a top-level leaf task written with the natural two-space metadata validated as a draft but failed `task start`/`complete` with `invalid metadata indentation`. Offsets are now relative to task nesting: top-level leaves accept metadata at 2 or 4 spaces, children keep 4; proof lines bind relative to their `Verification:` line. Every document that parsed before still parses.
- Draft validation and the execution parser now share one layout check (`spec.CheckTaskLayout`), so a draft that validates can no longer be structurally rejected at execution time; indentation errors from both name the offsets they expected.
- Draft task validation now derives the same leaf set as the full parser: a top-level task without dotted subtasks is a leaf (subject to the draft metadata checks), instead of being invisible to draft validation and then rejected at review time.

## [0.9.0] - 2026-07-11

Nobody releases a task — you release a repository. v0.8.0 made every execution claim provable; this release folds those truths into the question they exist to answer: *is this releasable, right now?*

### Added

- **`walden release check [<feature>] [--strict] [--json]`** — the aggregate certification gate: one deterministic verdict with one exit code, composing the guarantees the kernel already enforces. Per feature: the approval **chain** approved and fresh, **full-spec validation**, no unresolved **decision markers** in approved documents (HTML comments are stripped first — assumed notes that mention the marker never count), and every completed task's **evidence** derived `verified`. Once per repository: a clean **worktree** outside `.walden/`. All criteria always evaluate — a failed certification is a complete work list, and every blocker names its remedy.
- **The judging half of a produce/judge split.** `release check` executes no proofs and persists nothing — `verify` produces evidence, `release check` judges it. Certification runs in milliseconds and is safe on untrusted checkouts; the CI recipe composes the two: `walden verify <feature>` then `walden release check --json`.
- **The in-flight policy.** Planned-but-unexecuted tasks are informational and never block — the plan is a promise, not a debt; `--strict` promotes them for plans-complete shops. Uncommitted code outside `.walden/` fails closed with **no bypass flag** (what you certify is exactly what you tag), while dirty `.walden/` only warns — a freshly refreshed ledger legitimately precedes its own commit. Repositories without usable git skip the worktree criterion, reported.
- **An additive `release` block** in the JSON envelope: `releasable`, `strict`, per-feature per-criterion results with blockers, pending task ids, and the worktree partition.
- The embedded skill gains a **Release Certification** section: run the gate on releasability questions and before any tag or hand-off, read blockers as a work list, never look for a bypass.

### Compatibility

- All JSON output changes are additive within `schema_version: v0beta1`; evidence schema `v1alpha1` and spec frontmatter are untouched — no existing chain or ledger is affected.
- **The gate's criteria are a public contract from this release.** Once pipelines gate on them, semantic changes to a criterion are breaking changes and will be treated as such in this changelog.
- Exercised end-to-end on three real repositories before shipping (one of them a 26-feature portfolio certified in 260 ms); the battery's one false-positive class — decision markers mentioned in prose comments — is fixed in this release by comment stripping.

## [0.8.0] - 2026-07-11

The release that extends the fingerprint chain past the approved plan and into execution: "done" becomes a provable, re-checkable claim instead of a checkbox that can outlive its own truth.

### Added

- **Execution evidence ledger.** Every `task complete` records durable evidence in `.walden/evidence/<feature>.json` (schema `v1alpha1`): each proof step's argv, exit codes, and output digest, bound to the approved chain fingerprints and to a deterministic **code identity** of the working tree. Evidence is written before the checkbox moves — a completion the ledger cannot remember fails closed. The document is shared repository state, committed and reviewed like the specs it proves; `repo init` now writes a `.walden/.gitignore` for transient staging artifacts.
- **Derived execution states — never stored.** `verified`, `stale-spec`, `stale-code`, `failed`, `unrecorded`, `pending`, recomputed on every read from the record's bindings: requirements/design fingerprints plus a per-task definition fingerprint (editing one task never stales its siblings; checking boxes never stales anything). `reconcile` does not touch evidence — a re-approved chain shows honest `stale-spec` until proofs rerun.
- **`walden verify <feature> [--all] [--check]`.** Re-executes the proofs of completed tasks against the current code and refreshes the ledger (recording the post-proof identity — build and generate proofs change the tree they prove); prunes entries for task ids that left the plan, naming each removal; `--check` reports without writing for CI gates; exits non-zero naming every failing task.
- **`walden evidence status <feature>`** — the derived state per task with recorded and current identities; a report, not a gate (always exit 0). `walden task status` warns when completed tasks are no longer verified.
- **The code identity** is a uniform path→(mode, blob) map: `git ls-tree` seeds committed entries, dirty and untracked paths overlay via batched `git hash-object`, symlinks hash as their target string, file modes participate canonicalized to git's trio, and `.walden/` is excluded at every layer — committing the evidence never invalidates it, untracked→committed transitions are invisible, an executable-bit flip is a code change. Repositories without usable git degrade to a warned, absent identity (two absent identities compare equal).
- **`expect_output` on proof steps** — the step passes only when its combined output contains the declared content, closing the vacuous pass where a test command matching zero tests exits 0.

### Changed

- The embedded skill delegates the evidence half of execution to the CLI: verify and evidence status join the deterministic helpers, evidence warnings are never dismissed, spec drift ends with a verify pass, and proof authoring prefers `expect_output` on test-running steps.
- A code-only bugfix still needs no spec ceremony: evidence goes `stale-code` everywhere (the honest global doubt), and one `verify` either restores `verified` or names the task whose proof broke (the local blame).

### Compatibility

- All JSON output changes are additive within `schema_version: v0beta1`; spec-document fingerprints and frontmatter are untouched — no existing approved chain goes stale.
- Tasks completed by earlier releases report `unrecorded`; one `walden verify <feature> --all` migrates a feature (exercised on a 26-feature portfolio).
- Evidence documents carrying an unknown schema version are refused, corrupt ledgers warn with the recovery path (`rm` + `verify --all`), and concurrent walden processes cannot corrupt the ledger: whole-document atomic writes mean at worst a lost record that surfaces as `unrecorded`. One walden version per repository is recommended while the evidence schema is alpha.

## [0.7.2] - 2026-07-11

### Fixed

- **`repo init` generated an uninstallable CI pin from source builds.** The v0.7.1 pin feature stamped the generating binary's version verbatim into `validate-walden.yml`; clone builds carry git-describe versions (`v0.7.1-2-gabc1234`) that `go install` cannot resolve, so the generated workflow failed its first run with `invalid version: unknown revision`. The pin is now an allowlist of the only installable shape — strict `vMAJOR.MINOR.PATCH` tags; every other version (dev builds, describe suffixes, pseudo-versions) falls back to `@latest`. Release binaries always stamped real tags and were unaffected. If a clone build generated a broken pin, re-run `walden repo init` to refresh it. Found by the first dogfooded project's CI, one day after v0.7.1 shipped.

## [0.7.1] - 2026-07-10

Hardening patch driven by an external technical review of the kernel: the core guarantees stood, the surfaces around them leaked. No change to fingerprint semantics, the document schema, or the workflow model — existing approved chains stay fresh.

### Fixed

- **The JSON contract now survives failure.** Every `--json` command returns the versioned envelope on workflow and input errors — `ok:false`, canonical `command` name, blocking cause in the summary — instead of plain text on stderr with an empty stdout. `review open`/`review approve` and the whole `skill` group were affected; `task` commands already complied. All error rendering now flows through one shared renderer, and a per-command contract-test matrix locks the envelope shape. Unknown commands and subcommands keep plain usage text.
- **Spec document writes are atomic.** Documents are staged in a temp file inside their directory and renamed into place: interruption, full disk, or permission failures can no longer truncate an approved spec. Failed writes leave the previous content untouched.
- **The shipped todo-app demo reported stale** under every release since v0.4.0 (its approvals predate content fingerprints), contradicting its own README on first contact. It is migrated to a fresh fingerprinted chain and now doubles as a compatibility fixture: CI builds the binary and smoke-tests the demo on every change, so future strict migrations break there first.
- **`repo init` pins the generated CI workflow** to the walden version that generated it (`validate-walden.yml` installed `@latest`, so a new Walden release could break user CI without any repository change). Release builds stamp an exact pin, source builds fall back to `@latest`; re-running `repo init` after `walden update` refreshes the pin.

### Changed

- Argument-validation errors on recognized commands (for example `validate` with no feature name) return a precise message — and an envelope under `--json` — instead of the generic usage dump.
- Repository CI gained `go vet`, race-detector, and demo-smoke gates; the real process runner and the embedded template filesystems gained direct test suites; the public roadmap now reflects the v0.7.x line.

## [0.7.0] - 2026-07-10

### Added

- **`walden update` — self-update from GitHub releases.** One explicit command brings the installed binary to a target release and re-syncs installed skills:
  - Resolves the latest release through the `releases/latest` redirect (no GitHub API, no rate limits); `--version <tag>` pins a release for downgrades and rollbacks.
  - Downloads the platform asset and verifies its SHA-256 against the release's `checksums.txt` — fail-closed with no bypass flag: a missing checksums file, a missing entry, or a digest mismatch aborts with staging cleaned up. Releases ≤ v0.4.0 predate checksums and cannot be installed by `walden update`.
  - Swaps the executable atomically (staged in the install directory, which doubles as the writability probe and fails before any download), then smoke-tests the new binary and restores the previous one if it fails to run — no path ends without a working executable. Symlinked installs replace the resolved target.
  - Re-installs every detected skill installation through the new binary, so agents pick up the embedded skill matching the installed release. Re-sync failures degrade to warnings (`walden skill install <agent>` repairs), and targets below v0.5.0 skip re-sync with a warning.
  - `walden update --check` reports the current version, the target version, and availability without writing anything — a report, not a gate: exit 0 either way.
  - New additive `update` block in JSON output (`current_version`, `target_version`, `update_available`, `applied`) within `schema_version: v0beta1` — the envelope is unchanged and existing integrations keep working.
- Networking stays confined to the explicit update command: `net/http` lives only under `internal/selfupdate`, and no other command checks for updates or touches the network.

### Fixed

- `walden version` on `go install` binaries now reports the real module version from Go build info instead of `dev` (release ldflags still win when present). True source builds keep reporting `dev` and refuse `walden update`, with guidance to rebuild from the clone.

## [0.6.0] - 2026-07-09

### Changed

- **Decision Checkpoint Protocol refined in the embedded skill:**
  - Every `[decision:]` checkpoint now arrives with the skill's recommended option and a one-line rationale; when no option is defensibly better, the fork is presented without a fabricated preference.
  - New **Explore before asking** rule: when a decision passes the Bifurcation Test, the skill checks whether the codebase, the constitution, or approved upstream documents already answer the question before interrupting; repository-sourced resolutions are recorded inline as `<!-- assumed: <choice> (source: <file or document>) -->` (additive marker suffix — existing `assumed` markers stay valid).
  - Protocol invariants are untouched: five-checkpoint budget per phase, draft status on unresolved checkpoints, autonomous resolution as the default when in doubt.
- Prompt-only release: no CLI behavior or JSON contract changes (`schema_version: v0beta1` unchanged). Refresh installed skills with `walden skill install <agent>`; `walden skill status` reports stale installs as drifted.

## [0.5.0] - 2026-07-02

### Added

- **The AI skill ships inside the binary.** `skill/walden/SKILL.md` is embedded at build time (`go:embed`), so every distribution channel delivers the skill at the version matching the CLI. New `walden skill` command group:
  - `walden skill install <agent>|--all [--project]` — places the skill for `claude`, `codex`, `copilot`, or `opencode`. User scope by default; explicit `--project` for claude (`.claude/skills/`, committable to share with the team) and codex (`AGENTS.md` in the working directory). Writes are atomic (temp + rename); the Codex marker block stays byte-compatible with historical `setup.sh` installs.
  - `walden skill uninstall <agent>|--all [--project]` — symmetric, idempotent removal; Codex block extraction preserves all unrelated `AGENTS.md` content.
  - `walden skill status` — classifies every agent×scope slot as `in-sync`/`drifted` against the embedded copy, reports which binary version installed it, and flags diverging user/project installations.
  - `walden skill show` — prints the embedded SKILL.md verbatim: the escape hatch for agents Walden does not model yet.
- Installed skills carry a version stamp (`<!-- walden-skill-version: ... -->`) recording the installing binary; drift comparison normalizes it, so the stamp itself never reads as drift. Legacy stamp-less installs are classified without error.
- **One-liner installer consuming release binaries.** `curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh` — detects the platform (darwin/linux × amd64/arm64), resolves the latest release through the `releases/latest` redirect (no GitHub API, no rate limits), downloads the prebuilt binary into a temporary workspace, verifies its SHA-256 against the release's `checksums.txt` (fail-closed; `--no-verify` escape hatch for releases ≤ v0.4.0 that predate checksums), installs atomically to `~/.local/bin`, and hands skill placement to `walden skill install`. Flags: `--skill <agent|all>`, `--version <tag>`, `--no-verify`, `--uninstall`.
- The release pipeline publishes `checksums.txt` with SHA-256 digests of every binary asset.
- `skills` and `content` fields on command results (additive within `schema_version: v0beta1` — the envelope is unchanged and existing integrations keep working).

### Changed

- `setup.sh` delegates every skill install/uninstall/verify operation to the binary (`walden skill ...`); the interactive agent prompt remains in the script. Shell placement logic is gone.
- Reinstalling the Codex skill now replaces the existing marker block in place (previously `setup.sh` skipped when a block was present) — refreshing the skill on binary upgrade is the point of the embed.
- README leads with the one-liner install; per-agent guides document `walden skill install` in place of manual copy steps.
- Installing the claude skill at user scope removes the legacy `~/.claude/commands/walden.md` command file when present (parity with `setup.sh`, now enforced by the binary).

## [0.4.0] - 2026-07-02

### Added

- **Freshness hardening: content fingerprints.** `walden review approve` now records a SHA-256 fingerprint of the approved document body (`approved_fingerprint`) and binds downstream approvals to the upstream's fingerprint (`source_requirements_fingerprint`, `source_design_fingerprint`). Staleness verdicts are decided by fingerprint comparison alone — editing an approved document without re-approval, a missing fingerprint, or a malformed fingerprint all fail closed and block execution with a named cause. Timestamps remain in frontmatter as human-readable context.
- `stale_causes` and `approved_fingerprint` fields on document entries in `status`/`validate` JSON output (additive within `schema_version: v0beta1` — the envelope is unchanged and existing integrations keep working).
- Fingerprint normalization treats task checkbox state as execution progress, not content: completing tasks does not stale the approved plan.

### Changed

- **BREAKING (strict migration):** documents approved by pre-fingerprint versions report stale with cause `approval fingerprint missing`, and execution on them is blocked until migrated. Run `walden reconcile <feature>` once (documents reset to `draft`; task checkboxes are preserved) and re-approve each phase. This is deliberate: a grace mode that trusted fingerprint-less approvals would silently keep the old falsifiable behavior.
- **BREAKING:** `walden task complete-all` on a gate-blocked feature now exits non-zero with the blocker (previously reported "no runnable tasks" with exit 0). CI scripts relying on the old exit code must be updated.
- `walden reconcile` now resets a modified-after-approval document to `draft` (previously `in-review`): changed content is unreviewed content.
- Content-identical re-approval of an upstream document no longer marks downstream documents stale (previously any re-approval staled the chain via timestamps).

### Fixed

- `setup.sh` dev builds now report the full `git describe` version (e.g. `v0.3.0-2-g153f439`) instead of the bare latest tag, so binaries built from a branch are identifiable.

## [0.3.0] - 2026-06-26

### Changed

- JSON output contract advanced from `schema_version: v0alpha1` to `v0beta1` — a maturing-but-not-yet-frozen contract. The envelope shape is unchanged; this signals that integrations can build on it with care, while it may still change before the stable `v1` (which lands at v1.0.0). Match the `v0` prefix rather than the exact string.

## [0.2.0] - 2026-06-25

### Added

- Global `--help`/`-h` flag with an honest, useful usage block: a `Usage:` heading, every command with a one-line description, and the key global flags (`--json`, `--all`, `--phase`).
- OpenCode skill target in `setup.sh` — installs the Walden skill where OpenCode loads it (`~/.config/opencode/skills/`), with an `install-opencode.md` guide.
- GitHub Copilot skill target in `setup.sh`, with an `install-copilot.md` guide.
- Decision Checkpoint Protocol in the spec-drafting skill: it stops at genuine bifurcation points with a `[decision: ...]` marker and records autonomous choices inline as `<!-- assumed: ... -->`, so specs do not silently bake in unreviewed assumptions.

### Changed

- **BREAKING:** Claude Code now installs Walden as a model-invoked Agent Skill at `~/.claude/skills/walden/SKILL.md` instead of a `/walden` slash command. Claude activates the skill by intent; the `/walden` command is removed. `setup.sh` migrates existing installs automatically by deleting the legacy `~/.claude/commands/walden.md` on install and uninstall.
- The CLI usage block is headed `Usage:` instead of the misleading `Planned commands:`, and lists each command with a description plus the key global flags.
- README now leads with a hero image.

### Fixed

- The CI workflow generated by `walden repo init` ran a non-existent `python3 scripts/validate_walden_spec.py` and broke every pull request touching `.walden/**`; it now sets up Go, installs the `walden` CLI, and runs `walden validate <feature> --all --json`.
- The bundled `todo-app-demo` example used the undocumented `argv:` proof keyword; aligned to the canonical `command:` (the parser still accepts both).
- The bootstrapped `lessons.md` header referenced a non-existent `scripts/log_walden_lesson.py`; it now matches the `walden lesson log ...` template, removing the last Python reference from the project.

## [0.1.0] - 2026-04-13

First public release of the Walden OSS core.

### Added

- AC-level coverage enforcement: the validator now verifies that every acceptance criterion (`R*.AC*`) from `requirements.md` is referenced by at least one leaf task.
- Optional `constitution.md` template for project-wide context (tech stack, conventions, key files), bootstrapped by `walden repo init`.
- Skill Phase Router reads `.walden/constitution.md` when present for project-wide context.
- Versioned JSON output envelope with `schema_version: v0alpha1` for all `--json` commands.
- Kubernetes-style structured verification format with `expect_exit` support for negative assertions.
- `walden version [--json]` subcommand reporting build version and schema version.
- Public AI skill bundle at `skill/walden/` with install guides for Claude Code and Codex.
- Documentation pack: `docs/concepts.md`, `docs/workflow.md`, `docs/boundaries.md`, `docs/roadmap.md`.
- Example project at `examples/todo-app-demo/` with shell-safe structured verification.
- `LICENSE` (Apache-2.0), `CONTRIBUTING.md`, `SECURITY.md`.
- GitHub issue templates (bug report, feature request).
- Release workflow for cross-platform binary builds (linux/darwin × amd64/arm64).
- `setup.sh` installer and uninstaller for binary + AI skill.

### Changed

- Task template, skill, and docs now use AC-level IDs (`R1.AC1`) in leaf task `Requirements:` lines instead of parent-only references (`R1`).
- Legacy single-line verification format deprecated — structured `command:` format required for shell-safe execution.
- Project renamed from AndyArch to Walden.

## [v0-alpha.1] - 2026-03-22

### Added

- Core CLI commands: `repo init`, `feature init`, `status`, `validate`, `review open`, `review approve`, `task status`, `task start`, `task complete`, `task complete-all`, `reconcile`, `lesson log`.
- Phase-aware validation with `--all` flag for full-spec strictness.
- Structured argv verification format for shell-safe task proofs.
- `--json` output for machine-readable consumption.
- Embedded repository and spec templates.
- GitHub Actions CI workflow for Go tests.
