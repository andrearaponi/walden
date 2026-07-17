# Walden Documentation

Walden is a spec-driven delivery kernel: a deterministic CLI that takes a feature from intention to certified release through reviewed documents, executable proofs, and durable evidence. These pages are the reference for the mechanism, organized by what you are trying to do.

## Learn

- **[Quickstart](quickstart.md)** — one real feature from `repo init` to a releasable verdict, in ten minutes.

## Understand

- **[The Spec Lifecycle](lifecycle.md)** — the whole mechanism, end to end: how a feature is born, sealed, chained, executed, proven, and certified. Start here to understand *why* the commands do what they do.
- **[Product Boundaries](boundaries.md)** — what the open-source kernel includes, what belongs to the orchestration layer above it, and where the line is.

## Operate

- **[The Daily Workflow](workflow.md)** — the command loop for authoring, approving, executing, reconciling, and certifying, phase by phase.
- **[Brownfield Adoption](adoption.md)** — bringing a repository with existing specs into the current contract: `walden adopt`, triaging the partition, and retiring superseded eras.
- **[CI Integration](ci.md)** — the generated validation workflow, evidence in pipelines, and gating releases on the certification verdict.

## Reference

- **[CLI Commands](reference/cli.md)** — every command: synopsis, flags, behavior, exit codes.
- **[JSON Contract](reference/json.md)** — the versioned envelope, per-command result fields, the three evidence surfaces, and the additivity policy.
- **[Spec File Format](reference/spec-format.md)** — the `.walden/` layout, document frontmatter, EARS criteria, the task and proof grammar, and every convention file.

## Project

- **[Roadmap](roadmap.md)** — current release, near-term work, and the enterprise layer.

## Reading paths

- *Evaluating Walden?* [Quickstart](quickstart.md), then [The Spec Lifecycle](lifecycle.md).
- *Adopting it on an existing repository?* [The Spec Lifecycle](lifecycle.md), then [Brownfield Adoption](adoption.md).
- *Using it every day?* [The Daily Workflow](workflow.md) with [CLI Commands](reference/cli.md) at hand.
- *Building tooling on top?* [JSON Contract](reference/json.md), then [Spec File Format](reference/spec-format.md).

Documentation describes the released version whose tag matches this checkout. Version history lives in the [CHANGELOG](../CHANGELOG.md); agent-facing operational instructions live in the embedded skill (`walden skill show`), which is distributed with the binary and intentionally self-contained.
