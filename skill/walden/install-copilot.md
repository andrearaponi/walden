# Install Walden Skill for GitHub Copilot

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

The skill never installs the CLI. If PATH (and `~/.local/bin/walden`, or `%USERPROFILE%\go\bin\walden.exe` on Windows) has no compatible `walden`, it stops, detects your platform and points you to the matching official path: the README installer with `--no-skill` on macOS/Linux, `go install …@v0.10.4` on Windows with Go, or the `walden-v0.10.4-windows-<arch>.exe` release asset otherwise. It never substitutes manual workflow metadata or silently changes persistent shell configuration.
