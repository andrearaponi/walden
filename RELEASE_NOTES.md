## Walden v0.10.0

The plan becomes the contract. One deliberate breaking change — the gate's last unsafe default, flipped and announced — plus the two capabilities that make the whole evidence model adoptable and diagnosable at portfolio scale.

### Pending work blocks certification

An approved plan's unexecuted task means unimplemented acceptance criteria in the thing you are about to tag. Until now the default verdict shrugged at that; from this release it blocks:

```text
task 1.2 is pending — execute it, or waive with --allow-pending --reason
```

Partial releases are still yours to make — as a recorded decision:

```bash
walden release check --allow-pending --reason "auth hardening deferred to 1.3"
# RELEASABLE — 3 feature(s) certified, 2 task(s) waived (reason: auth hardening deferred to 1.3), commit ba53cfe55b40
```

The reason and the waived task identifiers ride the verdict itself — an additive `waiver` object in JSON, the clause in the summary, completion class `with-waivers`. No reason, no waiver: `--allow-pending` alone is refused. `--strict` keeps exactly its remaining job (committed `.walden/` state) and composes with a waiver. The gate still executes no proofs and writes nothing: the verdict your pipeline already archives *is* the waiver's record.

Repositories with no pending tasks see byte-identical verdicts. This is the release's only breaking change, shipped alone by design.

### Evidence knows where it was born

Every evidence record now carries an execution profile: platform and recording CLI version always, plus the outputs of probes you declare in `.walden/environment.md`:

```markdown
# Environment Probes

- go: ["go", "version"]
- node: ["node", "--version"]
```

The payoff is the failure you can finally read. When a proof that passed under go 1.25 fails on a machine running 1.24, the re-verification failure says so:

```text
verification failed for task "2.1": … exited with code 1; environment drift: go: recorded "go1.25.0" → current "go1.24.0"
```

`walden evidence status` prints recorded-versus-current differences per task. Probes run once per command, bounded and contained; a broken probe becomes a marker value, never a blocked run. Profiles are diagnostic by design — they never change a derived evidence state, so evidence produced in CI keeps certifying anywhere, and pre-profile records simply read as legacy.

### `walden adopt` — the brownfield lane

Every repository that specced before this contract existed can now join it with one command. The default is a read-only plan:

```text
ADOPTION PLAN — 133 backfill, 0 re-prove, 2 complete, 0 blocked (399 doc(s) to seal, 931 task(s) to re-prove)
```

Four classes, one honesty rule: an *absent* fingerprint is sealable — `--apply` stamps the current content's fingerprint under the approval already on record (a stated, reviewable trust assumption) and repairs empty chain links; a *present but wrong* fingerprint is `blocked` — human reconcile territory the lane never writes to. Then it re-proves unrecorded completed work through the verify machinery and hands you the honest partition: verified, or failed with a profile that names why. Interrupted runs resume; a second apply is a no-op; the adoption diff is ordinary git state you review and commit. A 135-feature portfolio plans in half a second and adopts in the time its proofs take to run.

### The skill knows all of this

The embedded skill (re-sync with `walden skill install --all` or `walden update`) teaches the three mechanisms — including the one rule that matters most: **a waiver is the user's decision**; the skill never passes `--allow-pending` without your explicit approval.

### Compatibility

JSON output changes are additive within `schema_version: v0beta1`; the evidence schema stays `v1alpha1` (the profile is an optional field; older ledgers keep working). The single breaking change is the release-policy flip above. Field-validated on production-scale portfolios before shipping.

---

Install or update:

```bash
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
# or, from an existing install
walden update
```
