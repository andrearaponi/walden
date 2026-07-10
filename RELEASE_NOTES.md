## Walden v0.7.1

The first release you install with Walden itself:

```bash
walden update
```

A hardening patch driven by an external technical review of the kernel. The verdict on the core held — fingerprint freshness, phase gates, proof execution all stood up to adversarial reproduction — but the review caught the surfaces around them leaking. This release seals them. Nothing changes in fingerprint semantics, the document schema, or the workflow model: existing approved chains stay fresh.

### The JSON contract now survives failure

Every `--json` command returns the versioned envelope on workflow and input errors — `ok:false`, the canonical command name, the blocking cause in the summary — instead of plain text on stderr with an empty stdout. Agents and CI pipelines parse exactly the errors they most need to handle: stale chains, missing prerequisites, blocked gates.

```json
{
  "schema_version": "v0beta1",
  "command": "review-approve",
  "ok": false,
  "result": {
    "summary": "requirements.md must be in-review before approval",
    "exit_code": 1
  }
}
```

All error rendering flows through one shared renderer, and a per-command contract-test matrix keeps it that way.

### No failure can truncate a spec

Document writes are staged and renamed atomically — interruption, full disk, or permission errors leave the previous approved content untouched, never a half-written file.

### The demo is now a fixture

The shipped todo-app demo predated content fingerprints and reported stale under every release since v0.4.0 — the first thing an evaluator touched contradicted the README. It now carries a fresh approval chain, and CI smoke-tests it with a freshly built binary on every change: future strict migrations break in Walden's CI, not in your repository.

### Generated CI is pinned

`walden repo init` stamps its own version into the validation workflow it generates. A new Walden release can no longer change your pull-request checks without a change in your repository; re-run `repo init` after `walden update` to move the pin deliberately.

### Upgrading

```bash
walden update                  # from v0.7.0
# older installs, one last time:
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
```

### Built the Walden way

Specified and shipped through Walden's own gated ceremony: 7 EARS requirements with 20 acceptance criteria, 100% task and proof reference coverage, every task closed through `walden task complete` — including one proof that failed fail-closed mid-execution and forced the plan to get more honest before the checkbox moved.
