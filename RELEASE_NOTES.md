## Walden v0.6.0

The skill interrupts you less, and earns it more. Two refinements land in the Decision Checkpoint Protocol.

### Every checkpoint comes with a recommendation

When drafting hits a genuine fork, the skill no longer presents it neutrally: it states its recommended option with a one-line rationale. You can answer "do that" instead of re-deriving context the skill already has. And when no option is defensibly better, it says so — no fabricated preferences.

### Explore before asking

Before emitting a checkpoint, the skill now checks whether the codebase, the constitution, or approved upstream documents already answer the question. If they do, it resolves autonomously and records the source inline:

```markdown
<!-- assumed: v0.6.0 minor bump (source: CHANGELOG.md) -->
```

Your attention is spent only on decisions the repository cannot answer. The existing `<!-- assumed: ... -->` markers stay valid — the source suffix is additive.

### What did not change

The protocol's invariants are byte-level untouched: at most five checkpoints per phase, unresolved checkpoints keep the document in `draft`, and autonomous resolution remains the default when in doubt. Prompt-only release — no CLI behavior or JSON contract changes (`schema_version: v0beta1`).

### Upgrading

```bash
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
# or: go install github.com/andrearaponi/walden/cmd/walden@v0.6.0
```

Then refresh the skill for the agents you use:

```bash
walden skill install <agent>   # or --all
walden skill status            # confirm everything reads in-sync
```

### Built the Walden way

Specified and shipped through Walden's own gated ceremony: Requirements → Design → Tasks with 14 EARS acceptance criteria, 100% task and proof reference coverage, every task completed through `walden task complete` with passing proofs — and yes, the spec that refined the checkpoint protocol used the refined protocol on itself.
