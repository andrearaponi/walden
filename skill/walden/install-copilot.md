# Install Walden Skill for GitHub Copilot

## Choose One Installation Channel

This page describes **native Walden** skill installation. If Skills CLI manages your copy, keep using `npx skills update walden` for the guide and the official installer with `--version <compatible-tag> --no-skill` for the executable. Do not also run the native install/update commands below on that copy. Ask before changing ownership or removing overlapping instructions; see the [canonical bootstrap](SKILL.md#cli-prerequisite-and-installation).

## Prerequisites

Use Walden CLI v0.10.2 or a newer compatible release and ensure the selected executable is usable. Once the matching tag is published, a source-install alternative is:

```bash
go install github.com/andrearaponi/walden/cmd/walden@v0.10.2
```

Verify with:

```bash
walden version
```

The skill is embedded in the binary, so the CLI is the only prerequisite.

## Install the Skill

```bash
walden skill install copilot
```

This writes the embedded skill to `${COPILOT_HOME:-~/.copilot}/skills/walden/SKILL.md`. Copilot support is user-scoped; `--project` is not available for this agent. If your Copilot setup loads instructions from a repository path, export the skill there yourself:

```bash
walden skill show > path/your/copilot/setup/reads.md
```

## Verify And Update

```bash
walden skill status
```

After upgrading the binary, rerun `walden skill install copilot` to refresh the skill.

## Usage

Once installed, invoke the skill by asking the agent to define requirements, create a design, generate tasks, or execute approved work for a feature. The agent will use the `walden` CLI for all deterministic operations.

## If the CLI Is Missing

If PATH has no compatible `walden`, the skill also checks `~/.local/bin/walden` before offering an authorized, pinned binary-only installation. It verifies path/version afterward and stops if consent, release availability or verification is missing. It never substitutes manual workflow metadata or silently changes persistent shell configuration.
