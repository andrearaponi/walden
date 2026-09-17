# Install Walden Skill for Claude Code

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

**User-level** (available across all projects):

```bash
walden skill install claude
```

This writes the embedded skill to `~/.claude/skills/walden/SKILL.md` and removes the legacy `/walden` command file if one is present.

**Project-level** (scoped to a single repository, shared with your team):

```bash
walden skill install claude --project
git add .claude/skills/walden/SKILL.md
```

Committing the file gives every teammate the skill on clone — no installation step on their side.

## Verify And Update

```bash
walden skill status
```

Reports whether the installation is `in-sync` or `drifted` relative to the binary's embedded copy, and which binary version installed it. After upgrading the binary, rerun `walden skill install claude` to refresh the skill.

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

If PATH has no compatible `walden`, the skill also checks `~/.local/bin/walden` before offering an authorized, pinned binary-only installation. It verifies path/version afterward and stops if consent, release availability or verification is missing. It never substitutes manual workflow metadata or silently changes persistent shell configuration.
