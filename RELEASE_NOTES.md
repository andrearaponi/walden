## Walden v0.15.0

A plan can now say that a proof attests a delivery. Until this release every proof was judged as a regression that must keep passing. A proof that describes what shipped, such as a version string in a document or an evaluation observed once, failed as soon as the delivered text moved on, and the feature read NOT RELEASABLE until someone rewrote the proof. A delivery proof is attested once, on the tree of a commit, and is not proven again. **This changes the criteria of the release gate.**

### Declare a delivery proof

```markdown
- [ ] 1. Name the release in the README
  - Requirements: `R1.AC1`
  - Design: Architecture
  - Proof: delivery
  - Verification:
    - command: ["grep", "-q", "version 1.2", "README.md"]
      covers: ["R1.AC1"]
```

- `- Proof: delivery` is a metadata line of a leaf task, and `delivery` is its only value. A task without the line holds a regression proof, as before.
- The declaration is part of the task-definition fingerprint, so it is a change of the plan that needs the plan's approval. The CLI never adds or suggests it.
- `walden validate` warns about the statements that only delivery proofs cover, before the plan is approved:

```
- statement(s) covered only by delivery proofs, which are not proven again once attested: R1.AC1
```

### Attested, then `delivered`

A delivery proof that passes on a tree with no uncommitted path outside `.walden/` is attested: its record names the commit of that tree. `task complete` usually runs on work that is not committed yet, so the first `walden verify` after the commit proves the task again and attests it. With commit ids shortened here:

```
$ walden verify demo
Summary: verification passed for demo: 1 task(s) re-proven, 1 skipped as verified
- 1: delivered at 97a4c63a… [binding=current code=current execution=supported]
```

The README then moves on to the next version. The delivered task is kept, with or without `--all`, and `--task` refuses it:

```
$ walden verify demo --all
Summary: verification passed for demo: 1 task(s) re-proven, 0 skipped as verified, 1 kept as delivered
- 2: verified [binding=current code=current execution=supported]
Kept as delivered: 1 at 97a4c63a…

$ walden verify demo --task 1
task "1" is delivered: its proof was attested at commit 97a4c63a… and is not run again — a delivered task is proven again only after its declaration or its proof changes in the approved plan, or after a statement it names changes
```

- **What keeps the state.** The task's definition is the attested one, and the statements it names read as they did. Later changes to the code do not touch it, and neither does a re-approval of the requirements or of the design that leaves those statements alone.
- **What loses it.** A statement the task names is reworded or removed, or the task's definition changes, the removal of the declaration included. The task then reads `stale-spec` and is proven again.
- **Without an attestation** a delivery task is judged as a regression task. `walden evidence status` says why its recorded pass is no attestation and what attests it.

### The release gate names what it did not prove again

```
$ walden release check demo
Summary: RELEASABLE — 1 feature(s) certified [feature: demo], 1 delivered task(s) not proven again, commit d40acb19b3c7
- demo: chain=pass validation=pass decisions=pass evidence=pass
  - 1: delivered at 97a4c63a… [binding=current code=stale execution=supported]
  - 2: verified [binding=current code=current execution=supported]
  only delivery proofs cover: R1.AC1
```

**Breaking (gate criteria contract): a delivered task no longer blocks.** `walden release check` accepts a `delivered` task under the `evidence` criterion without its proof being run again. The verdict never reports it as `verified`: it lists the task with the commit it was attested on, a releasable verdict states the number of delivered tasks, and each feature lists the statements that only delivery proofs cover. Nothing changes for a plan that declares no delivery proof.

### A recorded pass is sealed, not replayed

A plan written before this release often holds proofs that describe a delivery. Once the plan declares one, the pass its record already holds is the fact worth keeping: running the proof again would at best repeat it and at worst replace it with a failure. `walden verify` leaves that pass alone unless `--task` names its task, and `walden adopt --apply` seals it as an attestation without executing the proof:

```
$ walden evidence status legacy
- 1: stale-spec (delivery proof, awaits adoption) [binding=changed code=stale execution=supported]

$ walden verify legacy
Summary: verification passed for legacy: 1 task(s) re-proven, 0 skipped as verified, 1 left for adoption
Left for adoption: 1 — walden adopt legacy

$ walden adopt legacy
- legacy: backfill (1 recorded pass(es) to seal)
  seal 1 at 11e75372…

$ walden adopt legacy --apply
Summary: ADOPTION — sealed 0 doc(s), verified 0, failed 0, skipped 1, blocked 0, errors 0, sealed 1 recorded pass(es)

$ walden evidence status legacy
- 1: delivered at 11e75372… (sealed by adoption) [binding=current code=stale execution=supported]
```

Four checks run in order, and the first that fails leaves the pass unsealed with its reason:

| Reason | Meaning | What proves the task again |
| --- | --- | --- |
| `no-provenance` | The execution provenance of the pass is not supported. | Apply, as for any task that is not verified. |
| `proof-changed` | The recorded proof differs from the approved one in more than the declaration. | Apply, as above. |
| `statements-unknown` | It cannot be established that the statements the task names read as they did at the pass. | `walden verify <feature> --task <id>` |
| `no-commit` | No commit among the last 200 reachable from `HEAD` holds the tree the pass ran on. | `walden verify <feature> --task <id>` |

Sealing adds the attestation to the record and changes nothing else in it. The commit is searched by code identity, most recent first, so the pass need not have run on a committed tree: what counts is that a commit holds exactly the tree it ran on.

### What it found

On an isolated copy of this repository's own portfolio, 40 features, with the compiled CLI of this release:

- **Nothing changes without a declaration.** 244 read commands gave the same states, verdicts and adoption plans as v0.14.5. The one difference was the version label of the binary in the profile diagnostics.
- **Seven recorded passes were sealed**, each on a commit found between 13 and 69 commits back in the history, and each record was identical to before in every other field. Three features that only a delivery proof kept from certification became releasable.
- **Ten other delivery proofs were not helped by the declaration.** Four passes had legacy provenance and six tasks held no recorded pass, and all ten proofs fail today. They need a rewritten proof or a retirement, as before.
- **Seal before revising the requirements.** Records written before v0.14.5 keep no statement fingerprints. For such a record, a re-approval of the requirements before the sealing left the pass unsealed with `statements-unknown`, also when only the introduction had changed.
- **Mixed leaves.** Most of the seven leaves held lasting checks beside the one-time observation. Declared whole, they moved up to 29 statements of a feature under "only delivery proofs cover". Splitting such a leaf changes its recorded proof, so the lasting checks belong in a new regression leaf.

### Limits

- A delivered task is not proven again. The verdict says what it accepted without proving, not that the proof still passes.
- The attested commit is not checked against the history of what is certified: after a squash merge or in a shallow clone it may not be there.
- A pass on a tree with uncommitted changes is no attestation, whatever its result.
- A sealed attestation names the most recent commit that holds the tree the pass ran on. That can be a later commit with the same code, such as the merge of the branch.
- The declaration is for a fact about what shipped. A check that must keep holding gets a regression leaf of its own, and the declaration is never a way to silence a failing regression proof.

### Compatibility and distribution

- **Gate change.** The acceptance of delivered tasks is a breaking change of the gate criteria, labeled as such in the changelog. Verdicts for plans that declare no delivery proof are those of v0.14.5.
- **A ledger that holds an attestation is written as `v1alpha3`.** Any other ledger stays `v1alpha2`, and a ledger whose last attestation is gone goes back to it. An earlier CLI refuses a `v1alpha3` ledger, naming the version, so that it cannot prove a delivery again and replace its attestation. It also skips the declaration and judges a delivery task as a regression task. Once a plan declares a delivery proof, use this release or a newer one everywhere that repository is read, locally and in CI.
- **JSON stays additive within `v0beta1`.** Evidence entries gain `delivery`, `attestation` and `awaits_adoption`; `verify` gains `kept` and `left_for_adoption`; `release check` gains `delivery_only` on each feature; `adopt` gains `sealable_passes`, `sealed_passes` and `unsealed_passes`; `evidence status` gains `explanation.attestation`; the task of `task status`, `task start` and `task complete` gains `proof`. The state vocabulary gains `delivered`: exhaustive clients must handle it, and it is not `verified`.
- No command and no flag is added. The help of `verify`, `adopt`, `release check` and `evidence status` describes the behavior.
- **The guide** says which proofs to declare as delivery proofs, that the declaration never silences a failing regression proof, and that a recorded pass goes through `walden adopt` before any `walden verify`. It tells the agent to read `walden verify --help` for the capability, names no version, and still works with CLI v0.10.4 or newer.
- Update the binary with `walden update` on supported systems. Windows users replace the executable from the release assets.

### Verification

- All 20 tasks of the approved `proof-lifecycle` specification re-proven against the release tree.
- An end-to-end run with the compiled CLI on a small repository, which produced the reports above, a second apply that left the ledger byte-identical, and a portfolio verdict with two delivered tasks.
- The run on a copy of this repository's portfolio described under "What it found". The portfolio itself was not touched.
- The released v0.14.5 on ledgers written by this release: it refuses each ledger that holds an attestation, naming the version, and validates the plans that carry the declaration.
- Guards recorded from the released v0.14.5 binary before any change: fingerprints, states, verdicts and JSON of a portfolio without declarations, each failing under a mutation.
- `go vet ./...`, `go build ./...`, `go test ./...`, with `go test -race ./...` in CI.

## Walden v0.14.5

Stale evidence now says what changed. Until this release a `stale-code` task reported that the recorded and current code identities differ, and a `stale-spec` task that the recorded bindings differ from the current chain: true, and no help in deciding whether the change concerns the proof. `walden evidence status` now names the paths, the approvals and the statements that changed since each proof ran, and shows what the current code identity is made of. The release also corrects the code identity of a renamed file.

### `evidence status` says what changed

After three edits to a clean tree, with commit ids shortened here:

```
Current identity: commit 46a53dd3… plus 3 uncommitted path(s)
- changed README.md
- removed SECURITY.md
- added (untracked) vendor-copy/SKILL.md
Evidence:
- 1.1: stale-code [binding=current code=stale execution=supported]
  code-stale: recorded and current code identities differ
  since commit 46a53dd3…: changed README.md, removed SECURITY.md, added vendor-copy/SKILL.md
  recorded duration: 4406 ms
```

After the requirements were approved again with one criterion reworded:

```
- 1.2: stale-spec [binding=current code=current execution=supported]
  chain-changed: recorded requirements/design bindings differ from the current chain
  changed approvals: requirements
  changed statements: R1.AC5 changed
  recorded duration: 845 ms
- 1.3: stale-spec [binding=current code=current execution=supported]
  chain-changed: recorded requirements/design bindings differ from the current chain
  changed approvals: requirements
  changed statements: none of the statements this task names
  recorded duration: 517 ms
```

- **Stale code.** The paths that changed, were added or were removed since the task's proof ran.
- **Stale specification.** The approvals that changed (the requirements, the design, the task's own definition) and, when the requirements did, which of the statements the task names changed or are no longer defined, or that none did. A reworded criterion is told from an edit elsewhere in the document.
- **Current identity.** The commit the identity rests on and each uncommitted path, untracked ones marked. A stray copy or a forgotten edit is visible as an input, where before it only moved a digest.
- **Recorded duration.** How long each recorded proof took, as the measurement of that one execution.

`walden evidence status --help` describes the report. `--json` carries the same facts in `current_tree`, `explanation` and `recorded_duration_ms`, with complete lists; text shows the first 10 items of a list and the number omitted.

### What a record keeps

New records keep three optional groups of facts: the commit `HEAD` named with the paths that differed from it, up to 100; a fingerprint of each requirement statement the task names in its `Requirements:` line or in a `covers:` attribute, acceptance check included; and the duration of each proof step. They hold paths, content identities, fingerprints and durations, never file content or proof output.

### Exact or withheld

An explanation names a path only when the recorded facts establish it. The recorded tree is rebuilt from the commit and the paths, and accepted only when its digest equals the code identity the record was bound to. When a record lacks a fact, names a commit that is not in the repository (rewritten history, a shallow clone, a commit that was never pushed), or keeps facts that do not rebuild its identity, the report says so and names no path or statement. Re-proving the task writes a record with the facts available then.

### The explanations decide nothing

What makes evidence stale does not change. The code identity still covers the whole tree outside `.walden/`, every state is derived as before, and no proof is skipped or reused. State derivation never reads the new facts, so states, exit statuses and release verdicts are the same with and without them. A measurement on a real portfolio showed that narrowing the identity to declared inputs recovers almost no evidence and can keep a false green: this release explains the whole-tree identity instead of narrowing it.

### A renamed file no longer leaves its old path in the code identity

After `git mv`, the identity kept the old path, with the content the last commit had there, until the rename was committed. Committing the rename then moved the identity although no content had changed, so evidence recorded in between went `stale-code` for no reason. And a file created again at the old path with that same content did not move the identity at all, so evidence stayed `verified` on a tree that had gained a file. The old path now leaves the identity as soon as git reports the rename, in the index or, after `git add -N`, in the working tree; a copy keeps its source. In the `git add -N` case `walden release check` also listed a path that does not exist among the uncommitted ones, and no longer does.

### Also in this release

- **`walden update --version` rejects a tag whose numbers carry a sign or a leading zero.** `v+1.2.3`, `v-0.2.3` and `v01.2.3` passed the format check and the update then failed later with a download error for a release that cannot exist. Such a tag is now refused at once.
- **`walden-testability`, a companion skill for the design phase.** For every acceptance criterion it asks what a proof must provoke, where it sees the response from outside, which oracle decides pass or fail without trusting the implementation, and which proof is cheapest; each gap comes with the smallest change that closes it. Install it with `npx skills add andrearaponi/walden --skill walden-testability`. The CLI and the walden guide do not change.

### What it found

- On 9 November the portfolio of this repository held 155 `stale-code` tasks, each with the same sentence and nothing to tell them apart.
- In a run on a copy of this repository with 25 paths already uncommitted when the proofs ran, three later edits were named for every task, and none of the 25.
- The rename defect above was found while writing the tests for the recorded tree: it had been in every release since the code identity was introduced.

### Limits

- A record written before this release lacks the facts until its task is re-proven. `walden verify <feature> --all` does it for work that is still verified.
- Naming the changed paths needs the recorded commit to be in the repository.
- Over 100 differing paths a record keeps their number, not the paths.
- A duration is the measurement of one execution. This release computes no estimate and changes neither the order nor the concurrency of proofs.
- The explanations are in `evidence status`. `release check`, `status`, `task status` and `verify` keep their output and their remedies.
- A statement fingerprint follows the text of the statement and its acceptance check: rewording either changes it, reordering other statements does not.

### Compatibility and distribution

- No gate changes. States, exit statuses and release verdicts are those of v0.14.4 for the same ledger and the same committed tree.
- JSON stays additive within `v0beta1`: `current_tree` on the result of `evidence status`, `explanation` and `recorded_duration_ms` on its entries. No existing field changes.
- The ledger stays `v1alpha2`, with optional fields: `tree`, `statements` and `duration_ms` on each step. **The ledger now keeps step durations**, which v0.12.0 said it did not. `elapsed_ms` of adoption output is still not written.
- An earlier CLI reads a ledger that holds the new fields and derives the same states. When it rewrites the ledger it drops them, and this release then reports them missing until the task is re-proven.
- The text of `walden evidence status` gains a section before `Evidence:` and lines under its entries, and its line in `walden --help` changes. No command and no flag is added.
- Evidence recorded while a rename was uncommitted reads `stale-code` until `walden verify` runs again, as it already did once the rename was committed. `walden release check` never certified a tree with uncommitted changes, so no verdict changes.
- The guide does not change and still works with CLI v0.10.4 or newer.
- Update the binary with `walden update` on supported systems. Windows users replace the executable from the release assets.

### Verification

- All 12 tasks of the approved `staleness-diagnostics` specification re-proven against the release tree.
- An end-to-end run with the compiled CLI on a copy of this repository, which produced the two reports above: the three edits named for each of the 12 tasks, and every task `verified` again once they were undone; a file renamed with `git mv`, proven and then committed, with every task still `verified`; a reworded criterion named for the one task that proves it, and "none of the statements this task names" for the other 11.
- The released v0.14.4 run on a ledger written by this release: the same states for every task. After it rewrote the ledger, this release reported the facts missing and named no path.
- Targeted mutations, each caught by the test meant to catch it: state derivation made to read the new facts, a read command made to write the ledger, an existing JSON field renamed, a recorded commit listed once for each task, three mutations of the rename fix, and the usage line of another command changed.
- `go vet ./...`, `go build ./...`, `go test ./...`, with `go test -race ./...` in CI.

## Walden v0.14.4

A decision checkpoint can no longer be sealed unresolved. Until this release `review open` and `review approve` accepted a document still holding an open `[decision: …]` marker, and only `release check` noticed it, after approval. `release check` in turn read the marker literally, so a document that only quoted it could not be certified.

### The review gates refuse unresolved content

`walden review open` and `walden review approve` refuse a document that still holds an open `[decision: …]` marker or an HTML comment that never closes, before changing anything. The refusal names the document, the body line of each finding and how to clear it:

```
review refused: design.md holds unresolved content at body line 51 (unresolved [decision:] marker). To clear an unresolved [decision:] marker, resolve it, or quote it in a code span.
```

Only the document under review is read, and drafts stay free to hold open checkpoints: `walden validate` and `walden status` do not report them. A document left in review by an earlier release is approved once its findings are gone, with no new `review open`.

### A quoted marker no longer blocks

The review gates and `walden release check` read a document with the same scan, in order. A code span between two equal runs of backticks on one line, a fenced code block, a proof step or attribute of `tasks.md` and a terminated HTML comment are quotations, so a document that explains the marker can be certified. A comment, a fence or a code span that never closes hides nothing. `release check` reports each finding as a `decisions` blocker of its own, with its body line and remedy.

### What it found

The scan read 948 specification documents from this repository and 32 others, on isolated copies, and reported one: a draft holding six open checkpoints. A literal reading of the same documents reports four, three of which only quote the marker. Across the 40 features of this repository, one loses its three `decisions` blockers, because its approved documents only quote the marker, and none gains one.

### Limits

- The scan is textual, not a Markdown parser. Indented code blocks and code spans that cross a line are ordinary text, which can only add a finding, and a backslash before a backtick does not escape it.
- The marker is matched as written: `[Decision:` is ordinary text.
- The gates check that no checkpoint is left open, not how it was settled.

### Compatibility and distribution

- A document with no finding keeps its review behavior and output.
- **Breaking (approval contract):** a document under review that holds an open marker or a comment that never closes is now refused, where v0.14.3 opened or approved it. The refusal changes nothing on disk: resolve or quote the finding and run the command again.
- **Breaking (gate criteria contract):** a marker quoted in a code span, a fenced block or a proof line of `tasks.md` no longer blocks `release check`. A comment opener quoted in a code span no longer hides the text after it, so a marker there now blocks.
- JSON stays additive within `v0beta1`: no new field, the refusal travels in the `summary` of the failure envelope.
- No ledger field, document status, proof keyword, evidence state or release guarantee changes. Approval fingerprints are unchanged.
- The guide does not change and still works with CLI v0.10.4 or newer: it already keeps unresolved checkpoints in draft.
- Update the binary with `walden update` on supported systems. Windows users replace the executable from the release assets.

### Verification

- All 5 tasks of the approved `review-marker-gate` specification re-proven against the release tree, including an end-to-end cycle with the compiled CLI: `review open` and `review approve` refuse a document holding unresolved content and leave it unchanged, then accept it once the content is cleared, with no new `review open` before the approval.
- The scan tested with one subtest per criterion, and on the field corpus above with the same result as a prototype written for the design.
- Three targeted mutations of the implementation, all caught by the tests meant to catch them.
- `go vet ./...`, `go build ./...`, `go test ./...`, with `go test -race ./...` in CI.

## Walden v0.14.3

Empty proofs now say so: a proof that passes without running any test warns. `go test` exits 0 when its selector matches nothing, so until this release a proof without `expect_output` stayed green after its test was renamed or removed, and the evidence read `verified` for code no test exercised.

### A passing proof that ran no test warns

When every package line of a `go test` step says `[no tests to run]` or `[no test files]`, the command adds a warning naming the task and the step, and proposing the fix: assert the test by name with `-v` and `expect_output`. `task complete`, `task complete-all`, `verify` with every selection and `adopt --apply` all report it.

The warning decides nothing. The task completes, the exit code stays, the evidence record is that of any passing proof, the task reads `verified` and `release check` judges it as any other. A proof that fails gives no warning, because the failure already stops it.

### Declaring an empty run

A step that means to run no test stays quiet in two ways: the compile-only selector `-run '^$'`, or an `expect_output` naming `no tests to run` or `no test files`. `expect_output: "ok"` does not silence it, because `ok` also matches an empty run.

### What it found

The built binary replayed real proofs from this repository and three others, on isolated copies: 726 tasks executed, 679 passed. 8 steps warned, in 6 tasks, and all 8 were true: no test matched their selector. Three are in this repository, on tasks whose tests were removed in v0.11.0. No warning was false.

### Limits

- A package that ran a test clears the step, so a selector that misses in one of several packages goes unnoticed.
- The check reads only the plain output of `go test` that the step prints: `-json`, other test runners and a step that drops the package lines get no warning.
- A pipeline that keeps only the line of an empty package warns even when another package ran a test, as `go test ./... | tail -1` does when the last package has no test files. None of the measured proofs has that form.

An `expect_output` on the named `--- PASS:` line remains the safeguard, because it fails the proof.

### Compatibility and distribution

- A run with no empty test run keeps its behavior, output and exit code, with two exceptions. `task complete-all` now reports the warnings of its tasks, also when a later proof fails, including the code identity warning it used to drop. `verify` prints its warnings on stderr when a text run fails.
- JSON stays additive within `v0beta1`: no new field, the warning travels in the existing `warnings` array. `task complete-all` keeps `warnings: null` with nothing to report.
- No ledger field, document status, proof keyword, evidence state or release guarantee changes. Approval fingerprints are unchanged.
- The guide does not change and still works with CLI v0.10.4 or newer.
- Update the binary with `walden update` on supported systems. Windows users replace the executable from the release assets.

### Verification

- All 5 tasks of the approved `vacuous-proof-warning` specification re-proven against the release tree, including an end-to-end cycle with the compiled CLI, the real `go test` and a real Git repository: a proof completes silently, its test is renamed and committed, `verify` warns and exits 0, `evidence status` reads `verified`, `release check` passes.
- Recognition tested on real `go test` output from a temporary module, beside literal cases.
- Four targeted mutations of the implementation, all caught by the tests meant to catch them.
- `go vet ./...`, `go build ./...`, `go test ./...`, with `go test -race ./...` in CI.

## Walden v0.14.2

Single-task verification: a targeted repair no longer pays for the whole feature. Until this release `walden verify` selected completed tasks by state, or all of them with `--all`, and `task complete` refused a completed task, so re-proving one repaired task meant running the proofs of the whole feature. `walden verify <feature> --task <id>` runs the proof of that task alone.

### Re-prove one completed task

`--task <id>` selects one completed leaf task, whatever its state, and runs its proof through the path a plain `verify` uses: the same purity rules, the same record. Every other record stays as it is, a `failed` one included, and orphaned entries wait for the next feature-level run. `--check` works as it does without the flag.

A wrong selection is refused before any environment probe or proof runs: an id that is not a completed leaf task, a repeated or empty `--task`, or `--task` together with `--all`. An empty variable in `--task "$ID"` is an error, not a run of the whole feature.

### A green run is a repair step, not a verified feature

The result states what it covers. `scope.kind` is `task` with the id in `scope.tasks`, the summary names the task and the feature, and the run warns about the completed tasks left unverified, with their number and the first ten ids. The next action points to `walden verify <feature>`, and the exit status follows the selected task alone.

The code identity still covers the whole tree, so any later change can stale the other tasks, and `release check` still judges every task.

### What it saves

On a real feature of 14 proofs, `verify --all` took 37.5 s. A selected run took 21.8 s on the proof running `go test ./...` and 0.4 s on a proof running one test. The saving is the cost of the proofs skipped, so it depends on which proof you repeat. Environment probes and identity captures still run.

### Compatibility and distribution

- Every invocation without `--task` keeps its behavior, output and exit code. `verify --help` gains the flag and three lines.
- JSON stays additive within `v0beta1`: `scope.kind` gains the value `task` and `scope` gains `tasks`, both only under a selection. A client that switches exhaustively on `kind` must handle the new value.
- No ledger field, document status, proof keyword, evidence state or release guarantee changes. Approval fingerprints are unchanged.
- In text mode a failing command prints only its summary and next action, so on a failing selected run the warning about open tasks and the scope line appear in JSON alone.
- The guide still works with CLI v0.10.4 or newer, and its command table names `--task` for CLI v0.14.2 or newer. No evaluation measured that row. Update the guide with `npx skills update walden`.
- Update the binary with `walden update` on supported systems. Windows users replace the executable from the release assets.

### Verification

- All 4 tasks of the approved `verify-task` specification re-proven against the release tree, including an end-to-end cycle with the compiled CLI in a real Git repository: a commit stales two tasks, `--task` proves one, `release check` blocks on the other, `verify` proves it, and `release check` passes.
- Each rejection checked on a repository whose environment probe creates a sentinel file, with a valid selection on the same fixture as the positive control.
- Ten targeted mutations of the implementation, all caught by the tests meant to catch them.
- `go vet ./...`, `go build ./...`, `go test ./...`, with `go test -race ./...` in CI.

## Walden v0.14.1

Proof failure tail: a failed proof now shows what it printed. Until this release an output mismatch and a timeout reported no output at all, so the cause could only be found by running the proof again by hand, and an exit-code mismatch showed standard error alone, or standard output when standard error was empty, with no limit.

### A failed proof ends with its output

An output mismatch, a timeout and an exit-code mismatch now end with the last 4096 bytes of the step's combined output, standard output first, preceded by the number of bytes omitted. A Go test that fails on standard output next to a linker warning on standard error now shows the failure, where it used to show only the warning. A step that prints 2 MB no longer produces a 2 MB JSON result.

The excerpt is diagnosis, never evidence. `expect_output` and the output digest still read the complete output, and the ledger stores only the digest.

### `walden feature init` is proposed only for a missing feature

`task complete` and `task complete-all` looked for the words `does not exist` and `feature` in the error text, which can hold proof output, so a failing proof that printed both was told to initialize a feature that already existed. A missing feature keeps its message and its hint.

### What changes for existing proofs

- Nothing changes in what passes or fails: assessment, evidence states and the ledger are as they were.
- The failure text carries an excerpt where it carried none, and gets shorter where a step printed more than 4096 bytes.
- Standard output and standard error are concatenated, not interleaved: standard error longer than 4096 bytes fills the excerpt.
- Proof output is not sanitized. Output mismatch and timeout diagnostics gain the exposure exit-code mismatch diagnostics already had, bounded to 4096 bytes, and nothing new is persisted. Whoever redirects CLI output remains responsible for its confidentiality.

### Compatibility and distribution

- No command, flag, JSON field, document status, proof keyword, evidence state or release guarantee changes. Approval fingerprints are unchanged.
- The guide is unchanged and still works with CLI v0.10.4 or newer.
- Update the binary with `walden update` on supported systems. Windows users replace the executable from the release assets.

### Verification

- All 3 tasks of the approved `proof-failure-tail` specification re-proven against the release tree, including a real runner test for the output printed before a timeout and a check that the excerpt stays out of the ledger.
- A one-off comparison of v0.14.0 and v0.14.1 on real failures: a step printing 2 MB, whose JSON result drops from 2,000,347 to 4,467 bytes, an output mismatch, a timeout, a proof printing the misleading words, and a cgo test package in one of the maintainer's repositories.
- `go vet ./...`, `go build ./...`, `go test ./...`, with `go test -race ./...` in CI.

## Walden v0.14.0

EARS canonical grammar: the validator reads acceptance criteria the way EARS writes them. Until this release the classifier looked for the condition keywords anywhere before `SHALL`, in any letter case, and accepted as a combined form only a `WHILE` clause with a `WHEN` clause. An ordinary word inside a condition could fail validation or change a criterion's form, and combinations that EARS defines were rejected or misclassified.

### A keyword counts where it opens a clause

`WHERE`, `WHILE` (or `DURING`), `WHEN` and `IF` open a clause at the start of the criterion or after a comma, in any letter case, and anywhere before `SHALL` when written in capitals. Anywhere else they are text of the clause around them: `WHEN the user asks if the file exists, the system SHALL …` is event-driven instead of failing for a missing `THEN`, and `WHEN a download completes while offline, …` is no longer counted as complex.

### Every EARS combination is a complex criterion

The form follows from the kinds of clause: none is ubiquitous, one kind gives that kind's form, and any two or more kinds form a complex criterion. `WHERE …, WHEN …` and `WHERE …, WHILE …` no longer fail as an ambiguous combination, and `WHILE …, IF …, THEN …` is complex instead of unwanted; an `IF` clause inside a complex criterion counts as failure handling for the "no unwanted-behavior" warning. Every clause needs text, and `IF` still needs its `THEN`.

### Two wording warnings

`walden validate --json` warns, on the criterion, when its clauses are out of the EARS order `WHERE` → `WHILE` → `WHEN`/`IF`, and when its subject is the pronoun `it` or `they`, which leaves the reader to guess the component that responds. Neither fails validation.

### What changes for existing requirements

- No criterion that v0.13.1 validates becomes invalid, except one with an empty or unclosed clause that the old classifier never checked. Across 27,127 distinct criteria in the maintainer's repositories, none changes validity.
- Criteria counted in the wrong form change form: 91 in that read, mostly criteria counted complex because of a word inside an event clause. `ears_distribution` can change for documents that validated before; approvals and evidence do not, because forms are derived at each validation and stored nowhere.
- A doubled keyword such as `WHENWHEN` no longer crashes `walden validate`.

### Compatibility and distribution

- No command, flag, JSON field, document status, proof keyword, evidence state or release guarantee changes. `ears_validation` and `ears_distribution` keep their fields, and the new warnings appear in each criterion's existing `warnings`. Approval fingerprints are unchanged.
- The guide and the requirements template are unchanged and still work with CLI v0.10.4 or newer.
- Update the binary with `walden update` on supported systems. Windows users replace the executable from the release assets.

### Verification

- All 8 tasks of the approved `ears-canonical-grammar` specification re-proven against the release tree, including a regression fixture of 65 synthetic criteria whose v0.13.1 verdicts were recorded before the change and an end-to-end cycle from validation to task completion.
- A one-off comparison of v0.13.1 and v0.14.0 over 27,127 criteria: no validity change, 91 form changes, 7 order and 119 subject warnings.
- `go vet ./...`, `go build ./...`, `go test ./...`, with `go test -race ./...` in CI.

## Walden v0.13.1

Proof parsing hardening: every line of a proof block is now read or rejected. Until this release the tasks parser ended a structured `Verification:` block, without any report, at the first line it did not recognize, and read every step and attribute after it as free text. Those steps never ran, while `validate`, `task complete` and `verify` reported success. The defect dates back to v0.1.0.

### Every line of a proof block counts

A proof line is recognized by its keyword alone, and its value is parsed afterwards. An unrecognized line indented under `Verification:`, a value outside its form — coverage in backticks instead of a JSON array, `expect_exit: zero`, an empty `timeout:` — or a step or attribute line outside a block now fails, naming the task, the line and its text. Blank lines and single-line HTML comments, such as an inline assumption, are skipped. `covers: []` is valid and equivalent to omitting `covers`. Draft validation reads proof blocks with the same rules and messages as execution, so a draft that validates cannot hide a line the executor would drop.

### What changes for existing plans

- A plan whose blocks the previous parser read in full parses unchanged, with the same task-definition fingerprints: its evidence stays verified. Across 2,159 plans in the maintainer's repositories, 2,150 read identically.
- A plan containing a line the previous parser dropped fails until the line is fixed; an approved plan then goes through `walden reconcile` and a new approval. 7 plans in that read.
- A completed task whose proof gains the steps or attributes that were dropped, typically after `covers: []`, gets a new definition fingerprint: its evidence becomes `stale-spec` until `walden verify` runs the full proof, and a step that never ran may fail. 2 plans in that read.

### Also since v0.13.0

`install.sh --remove-legacy-skill` removes the guide copies Walden itself placed before v0.11.0 and reports the rest, and after an install the script warns when another `walden` on PATH shadows the one it installed. Both ship from `main`, independently of the binary. `walden-soundings` was never a published skill: the v0.11.0 texts are corrected.

### Compatibility and distribution

- No command, flag, JSON field, document status, proof keyword, evidence state or release guarantee changes. Approval fingerprints are unchanged, and error line numbers keep counting from the first line after the frontmatter; the new errors quote the line's text.
- The guide is unchanged and still works with CLI v0.10.4 or newer.
- Update the binary with `walden update` on supported systems. Windows users replace the executable from the release assets.

### Verification

- All 7 tasks of the approved `proof-parsing-hardening` specification re-proven against the release tree, including a characterization test that pins what v0.13.0 reads from a well-formed plan and a negative control on task completion after `covers: []`.
- A one-off comparison of v0.13.0 and v0.13.1 over 2,159 plans: 2,150 identical, 7 rejected at the expected lines, 2 regaining dropped steps or timeouts.
- `go vet ./...`, `go build ./...`, `go test ./...`, with `go test -race ./...` in CI.

## Walden v0.13.0

Contract consolidation: a growing portfolio stays coherent through small, regular steps instead of an audit after dozens of specifications. Each change is still reviewed against the requirements it touches; `walden consolidate` adds the step nobody could hold in mind — comparing what changed with the contracts linked to it — in a scope the CLI bounds and with an outcome the CLI checks.

### Count and remind

A feature whose approved requirements differ from the version recorded at the last consolidation is a pending change. With two pending changes `walden status` suggests a consolidation; with three or more it reports one as due, `walden feature init` says so before a new specification is opened, and `walden release check` adds a repository note. All of them are warnings: no verdict, blocker, completion class or exit code changes.

### Review a bounded scope with both sides in view

`walden consolidate` is read-only. It lists the pending features and every feature linked to them in either direction by a `feature#ID` reference or a shared cited file, with widely cited files listed apart; the deterministic findings — cited files that do not exist, references to missing features or identifiers, identifiers defined again after being reserved — and, for each pending feature, its criteria, NFRs and constraints marked added, changed, unchanged or removed, each changed one with its recorded text beside the current one.

### Record the outcome where the CLI can check it

The reviewing agent writes the outcome per pending feature under `## Coherence Review` in `.walden/contracts.md`, the current-contract view: one section per consolidated feature with its purpose, active and reserved identifiers, sources and related features. The CLI checks the view against the specifications and the review for presence, coverage and citations, never for correctness, which stays with the reviewer. `walden consolidate open` refuses while the view disagrees with the specifications or the review is incomplete; `walden consolidate approve`, only after the user's explicit approval, seals the view and records the checkpoint in `.walden/consolidation.json`. A reserved identifier stays reserved: a specification that defines it again is reported.

### Existing portfolios

`walden consolidate start` records today's approved features as an unconsolidated backlog and counts pending changes from that point. The backlog is consolidated in batches when the user asks, never all at once.

### Compatibility and distribution

- New command group `walden consolidate` (`report`, `start`, `open`, `approve`). No existing command, flag, document status, proof kind, evidence state or release guarantee changes; the reminders are warnings.
- JSON grows additively within `v0beta1`: `result.consolidation` on the consolidate commands, consolidation warnings in `result.warnings` of `status` and the report, and `release.repository_warnings` on `release check`. Document schema `v1alpha1` and ledger schema `v1alpha2` are unchanged. `.walden/consolidation.json` and the view's frontmatter are written only by the CLI.
- The guide carries the consolidation procedure and its four command rows, which need v0.13.0; everything else in the guide still works with CLI v0.10.4 or newer. The guide remains distributed independently through the Skills CLI.
- Update the binary with `walden update` on supported systems and the guide with `npx skills update walden`. Windows users replace the executable from the release assets.

### Verification

- All 19 tasks of the approved `contract-consolidation` specification re-proven against the release tree, including the full suite with real temporary Git repositories and the end-to-end consolidation cycle.
- Behavioral acceptance on held-out variants with blind review: with an explicit coherence request 22 of 27 inconsistencies found against 20 of 27 for the v0.12.0 workflow; with a plain request 24 of 27; discovery of the linked specifications 6 of 6; median specifications read in full 3 against 5; no unauthorized action in any session.
- Field use on this repository with the release candidate: tracking started on 32 approved features; a first batch of five adoption and evidence specifications consolidated, opened and approved through the CLI; a false positive found in the field — a model identifier read as a cited file — fixed under task 19 with a regression test.
- `go vet ./...`, `go build ./...`, `go test ./...`, `go mod tidy -diff`.

## Walden v0.12.0

Adoption becomes observable before, during and after execution. This release makes `walden adopt` explain its workload, preserve the verifier's diagnosis and report measured durations, without changing which proofs run or which evidence is accepted.

### Before: declared work, not a guess

The read-only plan reports selected completed tasks and declared proof steps. Known zero stays distinct from unavailable work: blocked features retain their reasons and appear in `unassessed_features`, outside the assessed totals. These counts are not test counts, expanded shell processes, an ETA or permission to replay historical delivery work.

### During: task progress and measured duration

Text-mode apply reports each selected task before its proof starts and its execution outcome afterward, with feature/task identifiers and position. Durations use monotonic elapsed time: task time covers attempted proofs; feature time includes preparation and saving; invocation time includes planning. The enclosing measurements are not sums of nested timings. JSON mode still emits one final envelope, with no progress text on stdout.

### After: preserve the diagnosis and its limits

Apply now retains the verifier's failure detail, assertion/integrity facts, final feature-level evidence assessment and warnings. A passing assertion can still be rejected for mutation or contamination; an accepted pure prefix can end stale-code after a later mutation.

If a ledger write fails after proofs ran, their observed results remain available with `evidence_persisted: false` and the feature error. They are not promoted into durable evidence or the legacy successful-feature partitions. Entry failures and no-op runs invent no task executions or durations.

### Compatibility and distribution

- No command, flag, adoption class, selection rule, timeout/purity policy or release guarantee changes.
- JSON reporting grows additively within `v0beta1`. Document schema `v1alpha1` and ledger schema `v1alpha2` are unchanged; no timing or raw-output fields are added to storage.
- This is not a waiver for an untestable legacy specification. A required proof still has to pass, or its contract needs an explicit reviewed change.
- The skill remains independently distributed through the Skills CLI and supports CLI v0.10.4 or newer compatible releases. The new reporting is available in v0.12.0; absent fields on older binaries are not zero measurements.
- Update the binary with `walden update` on supported systems, and update the guide separately with `npx skills update walden`. Windows users replace the executable or use the Go installation path. Neither artifact installs the other.

The release also aligns the README, roadmap and site with the current skill distribution channel. The guide published on the repository's default branch is the source consumed by skills.sh; previously installed copies require an explicit Skills CLI update.

### Verification

The feature has focused tests for workload availability, non-executing inspection, diagnostic propagation, mutation/contamination, measured intervals, no-op storage and output compatibility. Native-CLI end-to-end tests use real temporary Git repositories and exercise actual failure, retry and late persistence errors. The full suite runs with the race detector; documentation examples are checked against production projections.

An operator-run pilot on an existing repository reported three completed tasks and four declared proof steps, with all three accepted and pure. The reported 17.600-second invocation matched the external wall-clock measurement. Only the selected evidence ledger changed; a pending production proof remained excluded. This demonstrates observability on a real workload, not a performance improvement, a diagnosis of an older failure, or certification of the entire adopter repository.

---

## Walden v0.11.0

The binary no longer distributes the AI skill. The Skills CLI is the only channel for the guide; GitHub releases are the only channel for the binary; neither knows about the other. This closes the dual-channel period that started when `walden skill install` was added in v0.5.0 and that the last two patches spent reconciling.

### Removed: `walden skill …`, skill re-sync, installer skill handoff

`walden skill install|uninstall|status|show` no longer exist — `walden skill …` is an unknown command, indistinguishable from a typo, and the binary embeds no copy of `SKILL.md`, knows no agent path and names no channel. `walden update` replaces the executable and nothing else; `changed_files` has one entry. `install.sh` installs the binary, rejects `--skill <agent>` with a pointer to the Skills CLI, and `--uninstall` removes only `~/.local/bin/walden`. `--no-skill` is accepted as a no-op so older instructions keep working; it is not advertised and will be removed later.

### Changed: one channel, one sentence

Install the guide with `npx skills add andrearaponi/walden` (project-level by default, `--global` for user scope, `--copy` for a committable file) and update it with `npx skills update walden`. This is the same model the companion skill `walden-history` already used. The guide's CLI Prerequisite section replaces two paragraphs of ownership protocol with one sentence naming the two update paths, and its command table loses the `skill show` / `status` row. The floor stays **v0.10.4**: nothing in the guide depends on new CLI behavior, and raising it would have forced the very users being migrated to update first. The four per-agent install pages collapse into `skill/walden/install.md`.

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
