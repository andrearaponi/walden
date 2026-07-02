## Walden v0.4.0

Freshness gets teeth. Approval now binds to the exact content that was reviewed: staleness is proven by SHA-256 content fingerprints, not asserted by timestamps.

### ⚠️ Breaking — read before upgrading

**Existing specs report stale after upgrading.** Documents approved by v0.3.0 and earlier carry no approval fingerprint, so the new binary treats them as stale (`approval fingerprint missing`) and blocks execution on them. Migration is one deterministic cycle per feature:

```bash
walden reconcile <feature>   # documents reset to draft; task checkboxes are preserved
# then re-approve each phase:
walden review open <feature> --phase requirements && walden review approve <feature> --phase requirements
walden review open <feature> --phase design       && walden review approve <feature> --phase design
walden review open <feature> --phase tasks        && walden review approve <feature> --phase tasks
```

This strictness is deliberate: a grace mode that trusted fingerprint-less approvals would silently keep the old, falsifiable behavior.

**`task complete-all` exit code.** On a gate-blocked feature it now exits non-zero with the blocker (previously "no runnable tasks" with exit 0). Update CI scripts that relied on the old code.

**Not breaking:** the JSON contract. The envelope is unchanged and the new fields (`stale_causes`, `approved_fingerprint`) are additive within `schema_version: v0beta1` — existing integrations keep working.

### What fingerprints change

- `walden review approve` records a SHA-256 fingerprint of the approved body (`approved_fingerprint`) and, on downstream documents, the upstream's approval fingerprint (`source_requirements_fingerprint`, `source_design_fingerprint`).
- **Tamper detection:** editing an approved document without re-approval makes it stale — no matter what its metadata claims. Missing or malformed fingerprints fail closed. Every verdict carries a named cause, in human output and in `--json` (`stale_causes`).
- **Chain freshness by content identity:** a downstream document is fresh when its recorded source fingerprint equals the upstream's current approval fingerprint. Corollary: re-approving an upstream document *without changing its content* no longer stales the chain — timestamp noise is gone.
- **Execution progress is not content:** fingerprint normalization treats task checkbox state (and line endings) as non-content, so completing tasks never invalidates the approved plan.
- `walden reconcile` repairs everything deterministically: tampered, malformed, and legacy documents reset to `draft`, chain resets cascade, re-approval records fresh fingerprints.
- Timestamps stay in frontmatter as human-readable context; fingerprints alone carry the verdict.

No new commands, no new dependencies: the hardening rides `review approve`, `validate`, `status`, `task *`, and `reconcile`, pure Go standard library.

### Built the Walden way

This release was specified and executed through Walden's own gated ceremony. The gates caught two real defects during execution: the checkbox mutation gap (fixed by amending the approved requirements mid-execution, with a full gate re-run) and the `complete-all` silent success on blocked features. Verified by the full test suite, an end-to-end tamper → block → reconcile → restore lifecycle test, and a cross-agent field test: a different coding agent ran the complete ceremony and 22 proof-gated task completions against this build.

### Install

```bash
go install github.com/andrearaponi/walden/cmd/walden@v0.4.0

# or: build the binary and optionally install the AI skill
git clone https://github.com/andrearaponi/walden.git
cd walden && ./setup.sh
```

Pre-built binaries for linux/darwin × amd64/arm64 are attached to this release.

**Skill update:** `SKILL.md` documents the fingerprint model — re-run `./setup.sh` (or re-copy the skill) for the agents you use.

Still zero external dependencies.
