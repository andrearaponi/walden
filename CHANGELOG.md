# Changelog

All notable changes to Walden will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). This project uses semantic versioning. The JSON contract uses `v0alpha1` until the CLI stabilizes to v1.0.0.

## [Unreleased]

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
