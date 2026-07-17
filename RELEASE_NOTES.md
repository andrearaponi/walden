## Walden v0.10.1

One misbehaving proof now fails alone. This patch ships the remediation of the first field feedback on v0.10.0's `verify` — reported by a team running it in production within a week of release, and matching what the largest brownfield adoption run had already measured at scale.

### The stale-code cascade is gone

Until now, every `verify` record bound the tree its proof left behind. Honest — and unforgiving: one non-read-only proof (`go mod tidy` rewriting `go.mod`, a build regenerating an artifact) mid-run meant every task proven after it bound the mutated tree. Revert the mutation and all of them turned `stale-code`, guilty of nothing.

From this release, verify records bind the identity captured at the start of the run. The v0.9.2 purity contract is what makes this correct: a compliant proof leaves the tree where it found it, so for pure runs nothing changes — byte-identical records. When a proof does mutate, it still fails its own task naming the modified paths, and the run warning now names the victims too:

```text
proof side effects modified the repository: go.mod; tasks re-proven on the modified tree: 5.2, 5.3, 6.1
```

Task completion is untouched: that lane legitimately mutates (generators, builds), and its records keep binding the post-proof tree. Existing ledgers need no migration — the next `verify` rewrites its records through normal use.

### One record shape, every surface

`verify --json` and `evidence status --json` always shared one record shape internally; each populated half of it. If your tooling guessed field names by fallback chain, that ends here:

- `evidence status --json` gains `passed` — the stored proof result, omitted for tasks without a record.
- `verify --json` gains `recorded_identity`, `current_identity`, and `profile` — and on a contaminated run the recorded-versus-current divergence is visible right in the output.

The three evidence surfaces — the two command views and the on-disk ledger's state map — are now documented in `docs/concepts.md`. The short version: the ledger is storage, keyed by task id, facts only; the commands are the contract. Parse the commands.

### `adopt --apply` shows its work

A failed apply in text mode now renders the full per-feature partition before the summary and next action — `release check`'s failed-verdict convention — instead of hiding the report behind a two-line error.

### Proof authoring: prefer the read-only variant

The embedded skill (re-sync with `walden skill install --all` or `walden update`) teaches the run-start anchoring contract and the pattern the field converged on independently:

```markdown
    - Verification:
      - command: ["go", "mod", "tidy", "-diff"]   # asserts tidiness, writes nothing
```

Same assertion, no mutation, re-verification stays pure.

### Compatibility

JSON output changes are additive within `schema_version: v0beta1`; the evidence ledger schema stays `v1alpha1`. No breaking changes.

---

Install or update:

```bash
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
# or, from an existing install
walden update
```
