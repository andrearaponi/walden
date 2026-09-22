# Install the Walden Skill

The Walden guide is distributed through the [Skills CLI](https://skills.sh) — the same channel as the companion skills `walden-history` and `walden-soundings`. The Walden binary does not install, inspect or update the guide; the Skills CLI is the only manager of your copy.

## Prerequisite: the CLI

The guide drives the `walden` binary and requires **v0.10.4 or a newer compatible release**. The skill never installs it. Install the binary yourself with the one-line installer from the repository README (macOS/Linux), with `go install github.com/andrearaponi/walden/cmd/walden@v0.10.4`, or from the [GitHub releases](https://github.com/andrearaponi/walden/releases) page, and verify with:

```bash
walden version
```

## Install

Project-level, into the current repository (default):

```bash
npx skills add andrearaponi/walden --skill walden
```

User-level, available across all projects:

```bash
npx skills add andrearaponi/walden --skill walden --global
```

The Skills CLI places the guide for every agent it detects; select agents explicitly with `--agent claude,codex,copilot,opencode` (or `'*'` for all). It supports Claude Code, Codex, Copilot and OpenCode, and resolves each agent's skills directory itself.

### Committing a project copy

By default the Skills CLI links the guide into agent directories. To commit a plain file that teammates get on clone, install with `--copy`:

```bash
npx skills add andrearaponi/walden --skill walden --copy
git add .claude/skills/walden .agents/skills/walden
```

## Update

```bash
npx skills update walden
```

Add `--global` or `--project` to update one scope only. Update the binary separately with `walden update` (macOS/Linux) or by reinstalling.

## Remove

```bash
npx skills remove walden
```

## Without Node

If `npx` is not available, copy the `skill/walden/` directory of this repository into your agent's skills directory (for example `~/.claude/skills/walden/`). This is a manual fallback, not a second channel: keep it current by copying again after the guide changes.

## Usage

Once installed, the agent activates the skill when your request matches its description — there is no slash command to remember. Describe the spec work:

```
Let's define the requirements for a user authentication feature with Walden.
Create the design for user-authentication.
Generate the implementation plan for user-authentication.
Execute task 1.1 for user-authentication.
```

The agent loads the guide, follows the Walden workflow and calls the `walden` CLI for every deterministic operation.

## If the CLI Is Missing

The skill never installs the CLI. If PATH (and `~/.local/bin/walden`, or `%USERPROFILE%\go\bin\walden.exe` on Windows) has no compatible `walden`, it stops, detects your platform and points you to the matching official path: the README installer on macOS/Linux, `go install …@v0.10.4` with Go, or the `walden-v0.10.4-windows-<arch>.exe` release asset otherwise. It never substitutes manual workflow metadata or silently changes persistent shell configuration.
