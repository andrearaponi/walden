# Install Walden Skill for Claude

## Prerequisites

1. Install the `walden` CLI and ensure it is available in `PATH`:

   ```bash
   go install github.com/andrearaponi/walden/cmd/walden@latest
   ```

   Verify with:

   ```bash
   walden version
   ```

2. Claude Code or a Claude-compatible agent environment.

## Install as a Claude Code Custom Slash Command

1. In your project, create the directory `.claude/commands/` if it does not exist.

2. Copy `SKILL.md` into `.claude/commands/walden.md`:

   ```bash
   mkdir -p .claude/commands
   cp skill/walden/SKILL.md .claude/commands/walden.md
   ```

3. You can now invoke the skill with `/walden` inside Claude Code.

## Install as a User-Level Slash Command

To make the skill available across all projects:

```bash
mkdir -p ~/.claude/commands
cp skill/walden/SKILL.md ~/.claude/commands/walden.md
```

## Usage

Once installed, use the `/walden` command to define requirements, create designs, generate implementation plans, or execute approved tasks.

Examples:

```
/walden Define the requirements for a user authentication feature
/walden Create the design for user-authentication
/walden Generate the implementation plan for user-authentication
/walden Execute task 1.1 for user-authentication
```

## If the CLI Is Missing

If `walden` is not in `PATH`, the skill will inform you and stop. Install the CLI before continuing. The skill does not fall back to manual frontmatter editing or legacy scripts.
