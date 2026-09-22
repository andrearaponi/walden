## Walden v0.11.0

The binary no longer distributes the AI skill. The Skills CLI is the only channel for the guide; GitHub releases are the only channel for the binary; neither knows about the other. This closes the dual-channel period that started when `walden skill install` was added in v0.5.0 and that the last two patches spent reconciling.

### Removed: `walden skill …`, skill re-sync, installer skill handoff

`walden skill install|uninstall|status|show` no longer exist — `walden skill …` is an unknown command, indistinguishable from a typo, and the binary embeds no copy of `SKILL.md`, knows no agent path and names no channel. `walden update` replaces the executable and nothing else; `changed_files` has one entry. `install.sh` installs the binary, rejects `--skill <agent>` with a pointer to the Skills CLI, and `--uninstall` removes only `~/.local/bin/walden`. `--no-skill` is accepted as a no-op so older instructions keep working; it is not advertised and will be removed later.

### Changed: one channel, one sentence

Install the guide with `npx skills add andrearaponi/walden` (project-level by default, `--global` for user scope, `--copy` for a committable file) and update it with `npx skills update walden`. This is the same model the companion skills `walden-history` and `walden-soundings` already used. The guide's CLI Prerequisite section replaces two paragraphs of ownership protocol with one sentence naming the two update paths, and its command table loses the `skill show` / `status` row. The floor stays **v0.10.4**: nothing in the guide depends on new CLI behavior, and raising it would have forced the very users being migrated to update first. The four per-agent install pages collapse into `skill/walden/install.md`.

### Compatibility: what a ≤ v0.10.5 user sees once

That binary's `walden update` swaps in v0.11.0 and then, as it always did, calls the new binary's `skill install <agent>` for every copy it detected before the swap. The new binary answers "unknown command" and does no I/O, so the update completes and any Skills CLI-managed copy is untouched. The old binary wraps that answer in its usual non-fatal warning, including its own "run `walden skill install <agent>` to repair" hint. It is verbose and names a command that no longer exists; it happens once. Ignore it and use the Skills CLI for the guide. Prior releases are not modified or re-published.

The JSON envelope remains `v0beta1`; the removal is a reduction, not a reshape. Windows behavior is unchanged: `walden update` still refuses there.

### Why

Two managers for one file need an ownership protocol, and the protocol had to live in the guide, the README, the installer and the kernel at once. It also produced real defects: false drift on CRLF copies and a junction into the Skills CLI store that Windows refused to traverse — the last two patches. Removing the second channel removes the protocol and the class of defects with it. The one relation that remains is the one that was always legitimate: the guide declares which CLI it needs and checks it through `walden version --json`.

### Evidence

Seven executable leaves, each with its own new assertion: unknown-command parity for every `skill` form against a control typo; the updater's fake runner recording exactly one post-swap call (`version`); `internal/skilldist` absent and the module stdlib-only; no `go:embed` in `skill/`; the installer harness rejecting `--skill`, accepting `--no-skill`, leaving a seeded agent file byte-identical on `--uninstall`; the revised guide contract tests; and a distribution contract test whose every negative check (no embed, no agent path, no channel literal in Go sources or templates, no removed command in current docs, guide description absent from the built binary) is paired with a positive control that must match a pre-change fixture. `walden verify` re-proves all seven against the final tree.

---

## Walden v0.10.5

This patch makes `walden skill status` distinguish a failed read from an actual content difference and compare LF/CRLF copies consistently. It does not change the embedded guide or installation management.

### Unreadable paths are reported as unreadable

A Windows junction can exist while the OS refuses to traverse it. Previously that read error became `drifted`, even though no content comparison had happened. The new `unreadable` state keeps the requested path and reports the underlying cause and comparison-not-performed diagnostic. Other slots remain visible, and a missing body does not manufacture a user/project divergence warning.

### Same guide, different newline convention

LF, CRLF and mixed endings are normalized only in the inspection view, before shared-block and trailing-marker parsing. Substantive edits still report drift. The installed files, unrelated shared-file content and raw install/uninstall writer behavior are not changed.

### Compatibility and limits

The JSON envelope remains `v0beta1`; existing field names/types, scope rows and report-only exit code remain. The new state is additive. `installed=true` preserves the legacy slot flag, including read failures: it is not proof of readability or ownership, and `version` absence does not identify a manager. The updater's selection policy remains unchanged.

The patch does not repair a blocked junction, disable Windows protections, synchronize a Skills CLI installation, or make an agent's external store a native installation. Claude's real installation may still need a separately authorized repair. The embedded skill remains byte-identical; prior binary-bound behavioral reports are historical evidence, not certification of this new executable.

---

## Walden v0.10.4

A patch driven by one real Windows onboarding session: the user succeeded only because Go happened to be installed, saw "embedded version dev" on a v0.10.3 binary, and was refused three times before getting a command that worked on his platform.

### Fixed: `go install` builds now report their version everywhere

`walden version` resolved the module version from build info via `effectiveVersion()`; `skill status` and `skill install` read the raw ldflags variable and therefore reported and stamped `dev`. Both call sites now use `effectiveVersion()`. Release builds are unaffected (ldflags win); source builds still say `dev`.

### Added: Windows binaries, and an honest `walden update` on Windows

The release now ships `walden-<tag>-windows-amd64.exe` and `walden-<tag>-windows-arm64.exe`, listed in `checksums.txt`. `install.sh` remains POSIX-only (darwin/linux) and, on any other system, names the Windows paths instead of only "Unsupported OS".

On Windows `walden update` and `walden update --check` refuse **before any network use**: a running `.exe` cannot rename itself, and a half-swapped binary is worse than a clear message. Update with `go install github.com/andrearaponi/walden/cmd/walden@<tag>` or by replacing the `.exe`.

### Changed: the prerequisite pointer knows the platform

The guide still never installs the CLI. It now determines the platform, resolves `go` on PATH, and gives one pointer: `go install …@v0.10.4` wherever `go` resolves, otherwise the repository page (`https://github.com/andrearaponi/walden`: README installer on macOS/Linux, `.exe` on the releases page for Windows). If asked to install anyway it restates the refusal once and repeats the same single pointer or block — never the other branch. No `curl`, `wget`, `install.sh` URL or pinned bootstrap appears in the guide. The four companion install guides are aligned to v0.10.4 and covered by the documentation contract test that previously missed them.

### Evidence

Kernel change: two call sites plus one guard, each red→green on existing seams. Five fixed, isolated agent sessions on this guide and binary, including a simulated Windows host (fixture `uname`/`go` stubs, labelled as such) with an explicit "install it for me" follow-up. Cross-builds for both Windows targets from the release tree. The v0.10.3 features keep their `a012501` certification and are not re-run.

---

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
