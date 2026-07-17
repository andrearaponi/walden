# CI Integration

Walden's commands are deterministic, exit-code honest, and split cleanly between producing evidence and judging it — which makes the CI story three small recipes rather than a framework.

## Spec validation on every PR

`walden repo init` generates `.github/workflows/validate-walden.yml`, pinned to the CLI version that generated it (`go install …/cmd/walden@vX.Y.Z`). It runs `walden validate <feature> --all --json` for every feature whenever `.walden/**` changes — structural validation, traceability, freshness — so a PR cannot merge a broken or stale spec chain unnoticed.

The workflow is ordinary generated code: review it, commit it, adjust triggers to taste. Regenerating after a CLI upgrade re-pins it.

## Evidence in pipelines: produce and judge

The division of labor — [invariant #2](lifecycle.md#the-invariants) — maps directly onto pipeline stages:

**Produce** (executes proofs, writes the ledger):

```bash
walden verify <feature> --check    # report-only: re-proves, writes nothing — the PR gate
walden verify <feature>            # persisting: refreshes evidence — the merge/main job
```

`--check` is the pull-request shape: it answers "would the evidence hold on this tree?" with an exit code and no side effects. The persisting form belongs where its output can be committed — evidence documents are shared repository state, reviewed like the specs they prove, so a main-branch job that runs `verify` should commit the refreshed ledger (a dirty `.walden/` legitimately precedes its own commit and only warns at the gate; it blocks under `--strict`).

Machines that run proofs should declare [environment probes](reference/spec-format.md#environmentmd): every record then carries the toolchain versions that produced it, and a later failure on a different machine diagnoses itself (`environment drift: go: recorded "go1.25.0" → current "go1.24.0"`).

**Judge** (reads everything, writes nothing):

```bash
walden release check --json
```

Gate the release pipeline on the exit code. The verdict carries `completion` (`complete` | `with-pending` | `with-waivers`) and `certified_commit` — archive the JSON envelope and every release has a reproducible certification record naming the exact commit it judged.

## The release job

```bash
walden verify <feature>            # re-prove execution on the current tree
walden release check --strict --json   # certify; strict = the verdict must be reproducible from the commit
```

Policy knobs, all recorded, none silent:

- **Pending work blocks by default.** A deliberate partial release is an explicit waiver — `--allow-pending --reason "<text>"` — and the reason plus waived task identifiers land in the archived verdict. Treat a waiver in CI as a decision that belongs to a human: pass it from a manually-set variable, never hardcode it into the pipeline.
- **`--strict`** requires committed `.walden/` state, so the certification is reproducible including its evidence.
- **No bypass exists** for uncommitted code outside `.walden/`, and a repository without usable git fails closed: certification requires a git-backed code identity.

## Parsing output

Every command takes `--json` and emits the [versioned envelope](reference/json.md) on success and error paths alike — parse `result` fields by name, gate on the process exit code, and treat unknown fields as future additions (the contract only grows within `schema_version: v0beta1`). Warnings ride `result.warnings`; do not drop them — purity violations during verify surface there, naming mutated paths and affected tasks.
