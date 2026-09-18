## Walden v0.10.3

v0.10.3 changes one thing: **the skill no longer installs the Walden CLI.**

### Why

The v0.10.2 guide included a consent-gated, release-pinned bootstrap (`curl … install.sh` with `--version v0.10.2 --no-skill`). It was explicit about approval, pinned to a published release and verified checksums. A third-party security audit run by skills.sh (Gen Agent Trust Hub) nevertheless classifies any skill that downloads and executes remote code as HIGH risk — the pattern, not the safeguards, is what it scores. Two other auditors (Socket, Snyk) passed the skill. We agree with the stricter reading: a guide that is installed with one command should not itself contain a code-execution channel, however guarded.

### What changed

- The guide declares **Walden CLI v0.10.3 or newer**, checks `command -v walden` and `$HOME/.local/bin/walden`, and reuses a compatible binary by verified path or session-only PATH fix.
- If the CLI is missing or incompatible, the skill **stops and points to the repository README / GitHub releases**. It refuses to run an installer, fetch a script or binary, or write install commands for the agent to execute — even when asked. Installation is the user's action in their own terminal.
- `install.sh --no-skill` remains the documented way for Skills CLI users to install the binary themselves without touching the guide.
- The Skills CLI update path and the mechanical ownership rule (`walden skill status --json` `version` field) are unchanged.

### What did not change

No Go source, template, workflow or installer line changed. The binary is rebuilt only because the guide is embedded in it; `walden skill show` and native installs now distribute the pointer guide with a `v0.10.3` stamp.

### Evidence

Four fixed, isolated agent sessions on this exact guide and binary: compatible CLI on PATH, compatible CLI in `$HOME/.local/bin`, missing CLI followed by an explicit "install it for me" request, and native/Skills CLI ownership overlap. The v0.10.2 pilots (evidence integrity, authoring, bootstrap) were observed and certified at commit `102c17b` on the previous guide; this text-only change is orthogonal to what they measured and they are not re-run.

---

## Walden v0.10.2

v0.10.2 hardens the relationship between approved assertions, the code a proof actually checked, and the inputs a release verdict certifies. It also makes the embedded guide smaller and clearer about contract scope, verification checkpoints and the limits of execution claims.

### Proof identity includes the whole assertion

Task fingerprints bind the parsed command arguments, expected exit and output, timeout declaration and asserted coverage — not just a display string. Changing an approved assertion invalidates evidence for that task without resetting unrelated task progress or silently reusing the old result.

### Contaminated verification cannot become a valid pass by restoring files

`walden verify` records before/after code observations for each proof. Once a proof changes the checked tree, or a required identity cannot be observed, later executions in that invocation cannot earn verified evidence merely because their commands exit successfully. Restoring the original files does not revive results obtained on a contaminated tree.

Assertion results and execution-policy failures remain distinct. Valid pure prefixes retain their actual evidence. Task completion keeps its separate generator-capable lane and binds the resulting post-proof tree; re-verification remains the read-only lane. Walden does not automatically roll back source changes.

### Strict release checks bind the actual inputs to the commit

`walden release check --strict` compares the spec/evidence inputs actually judged with a captured commit, including files hidden by ignore rules. A clean `git status` is no longer enough when those inputs are missing from, or differ from, the named commit.

Missing, different or unreadable required inputs block the verdict. Unrelated ignored scratch files are not certification inputs. The check remains read-only: it executes no proofs and publishes nothing.

### Legacy evidence: recover what is known, preserve what is not

Read-only `walden adopt [<feature>]` separates:

- recoverable proof-contract binding;
- freshness of the code;
- available execution-integrity provenance.

Recovering a fingerprint from the current plan or local Git history does not prove historical execution purity. Records without the required assurance remain explicitly `unattested`, rather than being silently promoted to verified or mislabeled as newly failed.

An upgrade does not automatically replay historical tasks. Establish the current contract and requested scope before authorizing applicable verification; old bootstrap or deployment work does not automatically become a recurring regression check.

### Clearer authoring and more accurate handoffs

The embedded guide routes contract-preserving maintenance separately from new behavior, uses CLI-generated scaffolds, attaches observable acceptance checks, and keeps the design focused on the required sections. Human approvals and execution authorization remain separate.

Aggregate re-verification belongs at the declared batch or delivery checkpoint, rather than after every ordinary edit. Final code, README and other delivery edits precede the final scoped verification and release judgment.

A passing proof does not establish TDD chronology. The guide distinguishes original behavioral test-first development, tests added afterward, test-driven repairs and mutation testing. Breaking and restoring existing code cannot manufacture an earlier test-first history. Reports should use the CLI's actual state and input differences, not infer correctness from checkboxes or attribute staleness to a commit alone.

### Skill-first onboarding without a second skill installer

The canonical skill can be installed through Skills CLI with `npx skills add andrearaponi/walden --skill walden`. It checks for a compatible CLI (v0.10.2 or newer), asks before downloading/installing, and validates the executable actually used afterward. Its pinned bootstrap requires the corresponding published release; an unpublished local candidate is not a download source.

`install.sh --no-skill` installs only the binary, preserving the existing checksum/atomic-install path without prompting for or changing skills. It cannot be combined with `--skill` or `--uninstall`. Skills CLI users keep that tool as the owner of the guide and use binary-only installation for the executable; native users retain `walden update`, which also re-syncs native skills. Overlapping copies are not automatically migrated or deleted.

### Known behavior in the observed bootstrap sessions

In the fixed `install-failed` scenario, after the pinned download failed the agent ran a diagnostic probe against the unpinned `main` installer URL. The fixture blocked it, nothing was installed and no fallback was declared, but the probe departs from the "pinned only" spirit of the guide. It is recorded in the retained transcript rather than corrected in this release; the guide text itself already forbids installing from `latest`/unpinned sources.

### Compatibility and upgrade considerations

- **Evidence ledgers now write `v1alpha2`.** Legacy ledgers remain readable, with explicit assurance limits. Older binaries refuse the new format: keep producers and readers on a compatible Walden version; do not delete or downgrade evidence as an upgrade strategy.
- **Derived evidence states add `unattested`.** Consumers must handle it as missing assurance, not verified evidence. Stronger integrity checks can block outcomes that v0.10.1 accepted.
- **The JSON envelope remains `v0beta1`.** Existing field names/types are retained and diagnostic facts are added; clients must account for the new state and failure semantics.
- **The document schema remains `v1alpha1`.** Intact document approval fingerprints are not reset merely by upgrading.
- **Release scope remains explicit.** A feature-scoped verdict is not a whole-repository certificate, and non-strict output does not claim that local metadata is committed.

For an existing repository, start with the non-executing assessment:

```sh
walden adopt <feature> --json
```

Review the scope and assurance gaps before authorizing any proof execution. See [Brownfield Adoption](docs/adoption.md), [the CLI reference](docs/reference/cli.md) and [the JSON contract](docs/reference/json.md).

These changes improve the integrity and clarity of the workflow. They do not prove semantic completeness of arbitrary tests or guarantee that every generated application will be better.
