## Walden v0.9.2

What a proof may do is now part of the contract. This release bounds every proof in time, guarantees that re-verification never touches the repository it certifies, and teaches the document format to declare its own version — the groundwork every future extension will stand on.

### Every proof step is bounded

```markdown
    - Verification:
      - command: ["go", "test", "-run", "TestSlowIntegration", "./internal/integration"]
        timeout: 30m
```

Declare a `timeout:` on steps that legitimately run long; everything else runs under a 10-minute default. Expiry kills the step's **whole process group** — `sh -c` pipelines, test binaries, their children — and reports a proof failure naming the exceeded budget. A hung proof fails your pipeline in minutes instead of blocking it forever; an orphan process holding stdout, which previously hung the command even without a timeout, now surfaces as a prompt, explicit error.

A declared timeout is part of the task definition — changing it invalidates evidence. Plans that declare none render byte-identically: no existing fingerprint or ledger moves on upgrade.

### Re-verification is pure

`walden verify` re-executes proofs to prove the current tree still satisfies them — so a proof that *modifies* the tree contradicts the claim. It now fails its task:

```text
working tree changed while task 1.2 proof ran: modified paths: go.mod
```

The run continues across remaining tasks, the failure is recorded as evidence, and a run-level warning reports overall drift. `--check` stays byte-untouched. `task complete` is deliberately exempt: implementation proofs legitimately mutate (generators, builds), and the recorded identity binds the tree the proof leaves behind. The rule of thumb ships in the skill: author verify-able proofs as read-only assertions, route build outputs outside the repository.

This closes a real field defect: a completed task's proof running `go mod tidy` inside `verify --check` used to silently rewrite the host repository's `go.mod`.

### The document evolution contract

Every spec document now declares its format:

```yaml
walden_schema_version: v1alpha1
```

Scaffolded on `feature init`, stamped on every save — existing repositories converge through normal use, no migration command needed. Legacy documents keep loading; a document from a newer CLI is refused with the exact remedy instead of being silently misread.

Two more halves of the same contract:

- **`x-` frontmatter extensions survive every mutation**, verbatim, deterministically ordered — attach tracking links or provenance metadata without forking the format. Approval fingerprints stay body-only, so extensions never invalidate approvals.
- **Unknown non-namespaced fields are refused, not silently deleted.** The writer used to drop them on the next save; the loader now names the field and the remedy (`rename it to x-<field>, or remove it`). A typo'd core field is a loud error instead of a quiet data loss.

And one correction to fingerprint semantics: checkbox normalization is now scoped to `tasks.md`, the one document where a checkbox is execution progress. A checkbox edit in approved requirements or design counts as a content change and stales the approval — as it always should have. Marker-free documents keep byte-identical fingerprints; the rare affected document migrates with one reconcile and re-approval.

### The verdict names what it certified

```text
RELEASABLE — 3 feature(s) certified, commit dbbfb20dffa2
```

`walden release check` output gains two kernel-derived facts, additive in JSON: `certified_commit` — the HEAD revision an auditor checks out to reproduce the judgment — and `completion`, distinguishing `complete` (every planned leaf task executed) from `with-pending`. Pipeline policy can gate on completeness today; `with-waivers` is reserved for the planned strict-by-default flip.

### Compatibility

JSON output changes are additive within `schema_version: v0beta1`; the evidence schema stays `v1alpha1`. Three behavioral changes are breaking by design and documented in the changelog: mutating proofs fail re-verification, unknown frontmatter fields are refused at load, and checkbox edits in approved requirements/design stale the approval. Field-validated on two production-scale portfolios (135 and 26 features) before shipping: portfolio validation and full certification stay under a quarter second, with zero loader refusals across ~430 real documents.

---

Install or update:

```bash
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
# or, from an existing install
walden update
```
