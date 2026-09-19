# Install Walden Skill for Claude Code

## Choose One Installation Channel

This page describes **native Walden** skill installation. If Skills CLI manages your copy, keep using `npx skills update walden` for the guide and the official installer with `--version v0.10.4 --no-skill` for the executable. Do not also run the native install/update commands below on that copy. Ask before changing ownership or removing overlapping instructions; see the guide's [CLI Prerequisite](SKILL.md#cli-prerequisite) section (the skill never installs the CLI itself).

## Prerequisites

Use Walden CLI v0.10.4 or a newer compatible release and ensure the selected executable is usable. On macOS/Linux use the installer from the repository README; on Windows (or anywhere with Go) install from source:

```bash
go install github.com/andrearaponi/walden/cmd/walden@v0.10.4
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

The skill never installs the CLI. If PATH (and `~/.local/bin/walden`, or `%USERPROFILE%\go\bin\walden.exe` on Windows) has no compatible `walden`, it stops, detects your platform and points you to the matching official path: the README installer with `--no-skill` on macOS/Linux, `go install …@v0.10.4` on Windows with Go, or the `walden-v0.10.4-windows-<arch>.exe` release asset otherwise. It never substitutes manual workflow metadata or silently changes persistent shell configuration.
