# Install Walden Skill for OpenCode

## Prerequisites

1. Install the `walden` CLI and ensure it is available in `PATH`:

   ```bash
   go install github.com/andrearaponi/walden/cmd/walden@latest
   ```

   Verify with:

   ```bash
   walden version
   ```

2. An [OpenCode](https://opencode.ai) installation with native Agent Skills support.

## Install the Skill

Copy `SKILL.md` into your global OpenCode skills directory:

```bash
mkdir -p ~/.config/opencode/skills/walden
cp skill/walden/SKILL.md ~/.config/opencode/skills/walden/SKILL.md
```

OpenCode discovers skills from several locations. Pick the one that matches your scope:

| Scope | Path |
| --- | --- |
| Global (OpenCode) | `~/.config/opencode/skills/walden/SKILL.md` |
| Global (shared with Claude) | `~/.claude/skills/walden/SKILL.md` |
| Project | `.opencode/skills/walden/SKILL.md` |

`SKILL.md` is used as-is: its `name`, `description`, and `metadata` frontmatter is already valid for OpenCode, and unknown fields are ignored. No edits are required.

If `XDG_CONFIG_HOME` is set, OpenCode resolves the global directory under `$XDG_CONFIG_HOME/opencode/skills/` instead of `~/.config/opencode/skills/`. The `setup.sh` installer honors this automatically.

## Usage

Once installed, describe what you want — define requirements, create a design, generate tasks, or execute approved work for a feature. OpenCode loads the skill on demand through its built-in `skill` tool and follows the Walden workflow, using the `walden` CLI for all deterministic operations.

## If the CLI Is Missing

If `walden` is not in `PATH`, the skill will inform you and stop. Install the CLI before continuing. The skill does not fall back to manual frontmatter editing or legacy scripts.
