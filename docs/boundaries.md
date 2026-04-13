# Product Boundaries

This document explains what Walden includes today and what is planned but not yet implemented.

## Open Source Core (Current)

The open source release is a spec-driven delivery kernel. It includes:

- **CLI** — deterministic workflow engine for phase enforcement, validation, review gates, task execution with verification proofs, reconciliation, and lesson logging.
- **Skill** — optional AI-powered authoring layer that drafts requirements, designs, and task plans while delegating all deterministic operations to the CLI.
- **File model** — `.walden/specs/` documents with YAML frontmatter for approval tracking and freshness.
- **Templates** — repository bootstrap and feature scaffolding templates.
- **Examples** — complete example projects that demonstrate the workflow end to end.

This core is designed to work locally, in a single repository, with one developer or a small team.

## Enterprise Roadmap (Not Yet Implemented)

The following capabilities are planned but not part of the current release:

- **GitHub App** — a control plane that listens to repository events and automates workflow transitions.
- **Issue and Project sync** — projection of capabilities and tasks into GitHub Issues and Projects.
- **Approval event writeback** — automatic frontmatter updates triggered by GitHub review approvals.
- **Governance pack** — repository templates, reusable workflows, ruleset defaults, and CODEOWNERS configuration for organization-wide adoption.
- **Org dashboards and metrics** — centralized visibility into workflow health, stale specs, and delivery progress across repositories.
- **Multi-repo sync** — coordinated workflow state across multiple repositories in an organization.

These are described honestly as roadmap items. They are not hidden behind ambiguous marketing.

## The Rule

If you can do it with the CLI today, it is part of the open source core.

If it requires a GitHub App, org-level coordination, or centralized state beyond a single repository, it is enterprise roadmap.

## Why This Matters

Publishing the core as open source before the enterprise layer is complete is a deliberate choice. The deterministic CLI, the file model, and the spec workflow are already strong enough to deliver value. Waiting for the full platform would delay a useful release without improving the core.

The enterprise layer will build on top of the open source core without changing the file model or the CLI contract. The boundary is designed to be additive, not divergent.
