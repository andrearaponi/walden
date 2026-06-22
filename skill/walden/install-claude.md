# Install Walden Skill for Claude Code

## Prerequisites

1. Install the `walden` CLI and ensure it is available in `PATH`:

   ```bash
   go install github.com/andrearaponi/walden/cmd/walden@latest
   ```

   Verify with:

   ```bash
   walden version
   ```

2. Claude Code or a Claude-compatible agent environment with Agent Skills support.

## Install the Skill

Claude Code loads Agent Skills from a `skills/<name>/SKILL.md` layout and activates them automatically based on their `description`. Copy `SKILL.md` into the Walden skill directory.

**User-level** (available across all projects):

```bash
mkdir -p ~/.claude/skills/walden
cp skill/walden/SKILL.md ~/.claude/skills/walden/SKILL.md
```

**Project-level** (scoped to a single repository):

```bash
mkdir -p .claude/skills/walden
cp skill/walden/SKILL.md .claude/skills/walden/SKILL.md
```

`SKILL.md` is used as-is: its `name` and `description` frontmatter is what Claude Code reads to decide when to apply the skill.

## Usage

Once installed, Claude Code activates the skill automatically when your request matches its description — there is no slash command to remember. Just describe the spec work:

```
Let's define the requirements for a user authentication feature with Walden.
Create the design for user-authentication.
Generate the implementation plan for user-authentication.
Execute task 1.1 for user-authentication.
```

Claude detects the intent, loads the skill, and follows the Walden workflow, calling the `walden` CLI for all deterministic operations.

## If the CLI Is Missing

If `walden` is not in `PATH`, the skill will inform you and stop. Install the CLI before continuing. The skill does not fall back to manual frontmatter editing or legacy scripts.
