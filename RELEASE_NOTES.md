## Walden v0.1.0 — First Public Release

Walden is a spec-driven delivery kernel. It enforces a four-phase gated workflow — Requirements → Design → Tasks → Execute — where every feature must have approved specifications before a line of code is written, and every task must pass a verification proof before it can be marked complete.

### Why Walden

AI tools give you code at speed. Walden gives you the discipline to ship software you can defend. Every requirement is traceable to a task. Every task has a proof. Every proof runs automatically. Nothing ships without going through the gate.

### What's Included

**13 CLI commands, zero external dependencies**

| Command | Description |
| --- | --- |
| `walden repo init` | Bootstrap `.walden/` with constitution and lessons log |
| `walden feature init <name>` | Scaffold spec files for a new feature |
| `walden validate <feature>` | Structural validation, EARS grammar, AC traceability |
| `walden review open/approve` | Phase-gated approval workflow |
| `walden task status/start/complete/complete-all` | Execution with shell-safe verification proofs |
| `walden reconcile <feature>` | Repair stale approval chains after upstream edits |
| `walden lesson log` | Append reusable patterns to the lessons log |
| `walden version` | Build and schema version (`--json` supported) |

**Key properties**

- **Zero external dependencies** — pure Go standard library
- **Shell-safe verification** — structured `exec.Command` format, no implicit shell interpretation
- **EARS acceptance criteria** — six validated forms (Ubiquitous, Event-driven, State-driven, Optional, Unwanted, Complex)
- **100% AC traceability** — every acceptance criterion must be covered by at least one task, enforced at validate time
- **Freshness chain** — if a requirement or design document changes after approval, downstream documents go stale and block execution until reconciled
- **Versioned JSON output** — `schema_version: v0alpha1` envelope for CI pipelines, scripts, and agent toolchains
- **AI skill** — optional Claude Code / Codex skill for non-deterministic authoring (drafting requirements, designing architecture) paired with deterministic CLI enforcement

### Install

```bash
# From source (Go 1.25+)
go install github.com/andrearaponi/walden/cmd/walden@latest

# Via setup script (builds binary + optionally installs AI skill)
git clone https://github.com/andrearaponi/walden.git
cd walden && ./setup.sh
```

Pre-built binaries for linux/darwin × amd64/arm64 are attached to this release.

### JSON Contract

All `--json` commands return a versioned envelope:

```json
{
  "schema_version": "v0alpha1",
  "command": "status",
  "ok": true,
  "result": {}
}
```

The schema version is `v0alpha1` for this release. It will stabilize to `v1` at v1.0.0 with a documented migration guide.

### What's Next (v1.0.0)

- JSON schema stabilization (`v0alpha1` → `v1`)
- Legacy single-line verification format removal (deprecation active since v0.1.0)
- Error message improvements and more actionable CLI output
- AI skill instruction hardening and additional example conversations
- Additional example projects (backend service, multi-feature workflow)
