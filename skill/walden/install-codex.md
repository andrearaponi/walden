# Install Walden Skill for OpenAI Codex

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

**User-level** (applies to every Codex session):

```bash
walden skill install codex
```

Codex reads instructions from `AGENTS.md`. The command maintains a marker-delimited block inside `${CODEX_HOME:-~/.codex}/AGENTS.md`:

```
# --- BEGIN WALDEN SKILL ---
...skill content...
# --- END WALDEN SKILL ---
```

Everything outside the markers is preserved. Reinstalling replaces the block in place — run it after upgrading the binary to refresh the skill.

**Project-level** (an `AGENTS.md` in the repository root, shared with your team):

```bash
walden skill install codex --project
git add AGENTS.md
```

## Verify And Remove

```bash
walden skill status
walden skill uninstall codex
```

Uninstalling removes only the Walden block; the rest of your `AGENTS.md` is untouched.

## Usage

Once installed, invoke the skill by asking the agent to define requirements, create a design, generate tasks, or execute approved work for a feature. The agent will use the `walden` CLI for all deterministic operations.

## If the CLI Is Missing

If `walden` is not in `PATH`, the skill will inform you and stop. Install the CLI before continuing. The skill does not fall back to manual frontmatter editing or legacy scripts.
