# Install Walden Skill for GitHub Copilot

## Prerequisites

Install the `walden` CLI and ensure it is available in `PATH`:

```bash
go install github.com/andrearaponi/walden/cmd/walden@latest
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

If `walden` is not in `PATH`, the skill will inform you and stop. Install the CLI before continuing. The skill does not fall back to manual frontmatter editing or legacy scripts.
