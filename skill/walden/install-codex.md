# Install Walden Skill for OpenAI Codex

## Prerequisites

1. Install the `walden` CLI and ensure it is available in `PATH`:

   ```bash
   go install github.com/andrearaponi/walden/cmd/walden@latest
   ```

   Verify with:

   ```bash
   walden version
   ```

2. An OpenAI Codex-compatible agent environment.

## Install the Skill

Copy the `skill/walden/` directory from this repository into your agent's instruction set or tool configuration.

The key file is `SKILL.md` — it contains the full workflow instructions the agent follows.

## Usage

Once installed, invoke the skill by asking the agent to define requirements, create a design, generate tasks, or execute approved work for a feature. The agent will use the `walden` CLI for all deterministic operations.

## If the CLI Is Missing

If `walden` is not in `PATH`, the skill will inform you and stop. Install the CLI before continuing. The skill does not fall back to manual frontmatter editing or legacy scripts.
