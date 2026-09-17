# Install Walden Skill for OpenCode

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

The skill is embedded in the binary, so the CLI is the only prerequisite. An [OpenCode](https://opencode.ai) installation with native Agent Skills support is required.

## Install the Skill

```bash
walden skill install opencode
```

This writes the embedded skill to the global OpenCode skills directory. Resolution order: `$OPENCODE_HOME/skills/walden/SKILL.md` when `OPENCODE_HOME` is set, otherwise `${XDG_CONFIG_HOME:-~/.config}/opencode/skills/walden/SKILL.md`.

OpenCode discovers skills from several locations:

| Scope | Path |
| --- | --- |
| Global (OpenCode) | `~/.config/opencode/skills/walden/SKILL.md` |
| Global (shared with Claude) | `~/.claude/skills/walden/SKILL.md` |
| Project | `.opencode/skills/walden/SKILL.md` |

OpenCode also reads `~/.claude/skills/`, so if you already ran `walden skill install claude` the skill is picked up automatically — installing for both agents would surface it twice. For the OpenCode project scope, export the skill yourself:

```bash
mkdir -p .opencode/skills/walden
walden skill show > .opencode/skills/walden/SKILL.md
```

`SKILL.md` is used as-is: its `name`, `description`, and `metadata` frontmatter is already valid for OpenCode, and unknown fields are ignored.

## Verify And Update

```bash
walden skill status
```

After upgrading the binary, rerun `walden skill install opencode` to refresh the skill.

## Usage

Once installed, describe what you want — define requirements, create a design, generate tasks, or execute approved work for a feature. OpenCode loads the skill on demand through its built-in `skill` tool and follows the Walden workflow, using the `walden` CLI for all deterministic operations.

## If the CLI Is Missing

If PATH has no compatible `walden`, the skill also checks `~/.local/bin/walden` before offering an authorized, pinned binary-only installation. It verifies path/version afterward and stops if consent, release availability or verification is missing. It never substitutes manual workflow metadata or silently changes persistent shell configuration.
