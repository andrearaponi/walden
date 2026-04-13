# Roadmap

This is the public roadmap for Walden. It distinguishes committed open source work from exploratory and enterprise-only work.

## Current Release: v0.1.0

The first public release includes:

- Deterministic CLI with all core commands
- Versioned JSON output contract (`v0alpha1`)
- Public skill bundle for Claude and Codex
- Documentation pack (concepts, workflow, boundaries)
- Example project with shell-safe verification
- Repository bootstrap and feature scaffolding templates
- Apache-2.0 license

## Near-Term (Open Source)

These improvements are planned for the open source core:

- **CLI polish** — improved error messages, help text, and edge case handling.
- **JSON contract stabilization** — move from `v0alpha1` toward a stable `v1` schema as the contract matures.
- **Skill refinement** — tighter CLI delegation, fewer redundant instructions, better example conversations.
- **Additional examples** — backend service demo, multi-feature workflow example.
- **CI integration patterns** — documented patterns for using Walden validation in CI pipelines.

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

The open source CLI follows semantic versioning. The JSON output contract uses `v0alpha1` until the CLI stabilizes at v1.0.0, at which point it will bump to `v1` with a documented migration guide. Breaking CLI changes are documented in the changelog.
