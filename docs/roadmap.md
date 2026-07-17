# Roadmap

This is the public roadmap for Walden. It distinguishes committed open source work from exploratory and enterprise-only work.

## Current Release: v0.10.x

The open source core today includes:

- Deterministic CLI with all core commands
- Content-fingerprint freshness chain: tamper detection and provable approval binding on every spec document, with document schema versioning (`walden_schema_version: v1alpha1`) and preserved `x-` frontmatter extensions
- Versioned JSON output contract (`v0beta1`), honored on success and error paths alike
- AI skill embedded in the binary (`walden skill install|status|show`) for Claude Code, Codex, Copilot, and OpenCode, with drift detection against the embedded copy
- One-liner installer consuming checksum-verified release binaries
- Self-update from GitHub releases (`walden update`) with fail-closed verification, rollback, and skill re-sync
- Execution evidence ledger: task completions bound to spec fingerprints and a code identity, derived states, `walden verify` re-verification with a purity contract and run-start identity anchoring, per-step proof timeouts, `expect_exit`/`expect_output` assertions, and execution profiles with declared environment probes
- Aggregate release gate (`walden release check`): one deterministic releasability verdict per repository — chain freshness, full-spec validation, decision-marker lint, evidence states, clean-worktree policy — judging only, executing nothing; pending work blocks by default, with recorded waivers (`--allow-pending --reason`), completion classes, and a certified commit in every verdict
- Brownfield adoption lane (`walden adopt`): read-only classification plan, fingerprint backfill for recorded approvals, bulk re-proving with an honest verified/failed partition
- Retirement convention (`.walden/RETIRED.md`) and the `walden-history` companion skill for sourced chronicles over committed spec history
- Documentation pack (quickstart, lifecycle, workflow, adoption, CI, and full CLI/JSON/format references)
- Example project with shell-safe verification, validated in CI as a compatibility fixture
- Repository bootstrap and feature scaffolding templates (generated CI pinned to the generating version)
- Apache-2.0 license

## Near-Term (Open Source)

These improvements are planned for the open source core:

- **CLI polish** — improved error messages, help text, and edge case handling.
- **JSON contract stabilization** — now at `v0beta1`; move toward a stable `v1` schema as the contract matures.
- **Skill refinement** — tighter CLI delegation, fewer redundant instructions, better example conversations.
- **Additional examples** — backend service demo, multi-feature workflow example.
- **Scoped code identity** — narrowing the identity a proof binds from the whole tree to the slice it depends on, so unrelated changes stop staling a feature's evidence at portfolio scale.
- **Multi-root identity** — code identity spanning multiple repositories, unblocking cross-repo proofs and multi-repo certification.
- **Rework provenance** — recording the origin of post-approval rework to close the loop between shipped specs and what they missed.

## Medium-Term (Enterprise, Exploratory)

These capabilities are under design but not committed:

- **GitHub App (read-only)** — listen to repository events and project spec state into GitHub Issues and Projects.
- **GitHub App (read-write)** — automate review gate transitions based on PR approvals and writeback frontmatter updates.
- **Governance pack** — organization-level repository templates, reusable workflows, and ruleset defaults.

## Long-Term (Enterprise, Target State)

These represent the full platform vision:

- **Multi-repo sync** — coordinated workflow state across an organization.
- **Org dashboards** — centralized metrics for delivery health, stale specs, and progress.
- **Assignment automation** — rule-based task assignment integrated with GitHub team membership.
- **Pilot and rollout model** — structured adoption path for large organizations.

## What Is Not Planned

- Replacing human review with automated approval
- Building a proprietary workflow engine separate from the open source CLI
- Vendor lock-in to any specific AI provider

## Versioning

The open source CLI follows semantic versioning. The JSON output contract is at `v0beta1` and remains pre-stable until the CLI stabilizes at v1.0.0, at which point it will bump to `v1` with a documented migration guide. Breaking CLI changes are documented in the changelog.
