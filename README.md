[![CI](https://github.com/andrearaponi/walden/actions/workflows/go-test.yml/badge.svg)](https://github.com/andrearaponi/walden/actions/workflows/go-test.yml)
[![Release](https://img.shields.io/github/v/release/andrearaponi/walden)](https://github.com/andrearaponi/walden/releases)
[![Go Version](https://img.shields.io/badge/go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-zero-brightgreen)](go.mod)

Walden is an open-source, spec-driven delivery kernel: a deterministic CLI that takes a feature from intention to certified release through reviewed documents, executable proofs, and durable evidence.

<p align="center">
  <img src="site/walden-og.jpg" alt="Walden — Intention before code. Proof before completion." />
</p>

Most teams run Walden through a coding agent. The embedded AI skill handles the non-deterministic half — asking clarifying questions, drafting requirements, designing architecture, planning tasks — and drives the CLI at every step. The CLI enforces the deterministic half: phase order, freshness fingerprints, verification proofs, execution evidence, and the release gate. You keep the judgment calls — nothing gets approved on your behalf.

## How It Works

Every feature progresses through four phases. Each phase has an approval gate that must pass before the next begins.

```
Requirements ──▶ Design ──▶ Tasks ──▶ Execute
     │              │          │          │
  validate       validate   validate   verify
  review         review     review     proofs
  approve        approve    approve    complete
```

Requirements are written as [EARS](docs/reference/spec-format.md) acceptance criteria with stable IDs; the design must cover every criterion; every leaf task carries an executable verification proof. Completing a task records durable evidence bound to the approved spec chain and to the code it proved. If anything changes after approval — a document, the code — staleness surfaces instead of hiding behind a checked box, and `walden release check` certifies the whole repository with one deterministic verdict.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
```

Downloads the latest release binary for your platform (darwin/linux, amd64/arm64), verifies its SHA-256 checksum, installs it to `~/.local/bin/walden`, and offers to install the AI skill for your coding agent — Claude Code, Codex, Copilot, or OpenCode. Flags pass through the pipe with `sh -s --`:

| Flag | Effect |
| --- | --- |
| `--skill <agent\|all>` | Install the skill non-interactively (`claude`, `codex`, `copilot`, `opencode`, `all`) |
| `--version <tag>` | Install a specific release instead of the latest |
| `--uninstall` | Remove the skill (all agents) and the binary |

From source: `go install github.com/andrearaponi/walden/cmd/walden@latest`, then `walden skill install claude` — the skill ships inside the binary, always at the matching version. Later, `walden update` upgrades the binary (checksum-verified, atomic) and re-syncs every installed skill.

## Quickstart

Install, then open your coding agent and say:

```
We need to build a user authentication system. Let's design it with Walden.
```

That's it. The skill asks clarifying questions, drafts requirements in EARS format, designs the architecture, breaks the work into tasks with verification proofs, and walks you through execution — invoking the CLI on your behalf at every step:

| The skill authors | The CLI enforces |
| --- | --- |
| Asks the right questions | Phase ordering: Requirements → Design → Tasks → Execute |
| Drafts requirements in EARS format | Document freshness and approval chains |
| Designs architecture, evaluates alternatives | Acceptance-criteria traceability (100% coverage) |
| Generates implementation tasks with proofs | Verification proofs on every task |
| Reviews lessons before similar work | Execution evidence bound to spec and code identity |

Human review and approval remain yours: the skill drafts and proposes — it never approves.

Prefer to drive the CLI directly? The [Quickstart](docs/quickstart.md) walks one real feature from `walden repo init` to a releasable verdict, command by command.

## Documentation

Full documentation lives in [docs/](docs/README.md) and on the [website](https://andrearaponi.github.io/walden/docs/).

- **Learn** — [Quickstart](docs/quickstart.md) · [The Agentic Flow](docs/agentic.md)
- **Understand** — [The Spec Lifecycle](docs/lifecycle.md) · [Product Boundaries](docs/boundaries.md)
- **Operate** — [The Daily Workflow](docs/workflow.md) · [Brownfield Adoption](docs/adoption.md) · [CI Integration](docs/ci.md)
- **Reference** — [CLI Commands](docs/reference/cli.md) · [JSON Contract](docs/reference/json.md) · [Spec File Format](docs/reference/spec-format.md)
- **Project** — [Roadmap](docs/roadmap.md) · [Changelog](CHANGELOG.md)

A complete working example lives in [examples/todo-app-demo](examples/todo-app-demo).

## Development

Pure Go standard library — zero external dependencies. Run the tests with `go test ./...`.

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines. For non-trivial changes, create a feature spec with `walden feature init` and follow the gated workflow.

## On the Name

Walden is named after Thoreau's *Walden, or Life in the Woods*, where he writes:

> *"I went to the woods because I wished to live deliberately, to front only the essential facts of life."*

My grandfather taught me that principle before I had words for it: do fewer things, but do them with full attention. Software rarely does. This tool is an attempt to apply that discipline — to require intention before code, and proof before completion.

## License

Apache-2.0. See [LICENSE](LICENSE).
