## Walden v0.5.0

The binary is now the whole distribution. The AI skill ships inside it, one command installs everything, and skill/CLI version drift is impossible by construction.

### Install in one line

```bash
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
```

No Go toolchain, no clone: the installer detects your platform (darwin/linux × amd64/arm64), resolves the latest release, verifies the binary against the published SHA-256 checksums (fail-closed), installs it to `~/.local/bin/walden`, and offers to set up the AI skill for your coding agent. Flags pass through the pipe with `sh -s --`:

| Flag | Effect |
| --- | --- |
| `--skill <agent\|all>` | Install the skill non-interactively (`claude`, `codex`, `copilot`, `opencode`, `all`) |
| `--version <tag>` | Pin a release instead of the latest |
| `--no-verify` | Skip checksum verification (releases ≤ v0.4.0 predate `checksums.txt`) |
| `--uninstall` | Remove the skill (all agents) and the binary |

This is the first release that publishes `checksums.txt` alongside the binaries.

### The skill lives in the binary

`skill/walden/SKILL.md` is embedded at build time. Whatever channel delivers the binary — the one-liner, `go install`, a manual download — delivers the skill at the exact matching version. The new `walden skill` command group manages placement:

```bash
walden skill install claude            # user scope: ~/.claude/skills/walden/
walden skill install claude --project  # project scope: commit it, your team gets the skill on clone
walden skill install --all             # every supported agent
walden skill status                    # in-sync / drifted, per agent and scope
walden skill show > anywhere.md        # escape hatch for agents Walden does not model yet
walden skill uninstall <agent>|--all
```

- **Drift is visible instead of silent:** installed skills carry a version stamp; `walden skill status` compares content against the embedded copy (stamp-normalized) and reports which binary version installed what — including diverging user/project installations.
- **Codex stays clean:** the `AGENTS.md` marker block is byte-compatible with previous `setup.sh` installs, reinstalls replace it in place, and uninstall extracts only the block, preserving everything else.
- **Atomic placement:** every write stages a temp file and renames — no failure path leaves a partial skill or binary behind.

### Behavior notes (not breaking)

- The JSON contract is unchanged: new `skills` and `content` result fields are additive within `schema_version: v0beta1`.
- Reinstalling the Codex skill now **replaces** the existing block (previously `setup.sh` skipped it) — refreshing the skill on binary upgrade is the point of the embed.
- `setup.sh` still works for building from a clone, but delegates all skill placement to the binary.

### Upgrading from v0.4.0

```bash
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
# or: go install github.com/andrearaponi/walden/cmd/walden@v0.5.0
```

Then refresh the skill for the agents you use — one command now:

```bash
walden skill install <agent>   # or --all
walden skill status            # confirm everything reads in-sync
```

### Built the Walden way

Both features were specified and executed through Walden's own gated ceremony: two full Requirements → Design → Tasks cycles (65 EARS acceptance criteria total, 100% task and proof reference coverage), every task completed through `walden task complete` with passing proofs — including live smoke tests of the installer's fail-closed path against the historical v0.4.0 release.

Still zero external dependencies: `go:embed` is standard library.
