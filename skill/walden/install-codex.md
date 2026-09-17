# Install Walden Skill for OpenAI Codex

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

If PATH has no compatible `walden`, the skill also checks `~/.local/bin/walden` before offering an authorized, pinned binary-only installation. It verifies path/version afterward and stops if consent, release availability or verification is missing. It never substitutes manual workflow metadata or silently changes persistent shell configuration.
