# Changelog

All notable changes to Walden will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). This project uses semantic versioning. The JSON contract uses `v0beta1` until the CLI stabilizes to v1.0.0.

## [Unreleased]

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
