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

`--check` answers whether the selected proofs satisfy verification policy without persisting the ledger. It does not prevent command side effects or sandbox the proof. Mutation or an unavailable required identity rejects verification and contaminates later executions in that invocation. The persisting form belongs where its output can be committed — evidence documents are shared repository state, reviewed like the specs they prove, so a main-branch job that runs `verify` should commit the refreshed ledger (a dirty `.walden/` legitimately precedes its own commit and only warns at the gate; it blocks under `--strict`).

Machines that run proofs should declare [environment probes](reference/spec-format.md#environmentmd): every record then carries the toolchain versions that produced it, and a later failure on a different machine diagnoses itself (`environment drift: go: recorded "go1.25.0" → current "go1.24.0"`).

**Judge** (reads everything, writes nothing):

```bash
walden release check --json
```

Gate the release pipeline on its exit code and retain the scope, assurance gaps, `completion`, and `certified_commit`. A selected-feature verdict is not a whole-repository claim. Reproducibility of spec/evidence inputs is attested only when strict committed-input binding succeeds; default mode can judge local metadata.

## The release job

```bash
walden verify <feature>                  # refresh the explicitly reviewed scope
# Review and commit its evidence when authorized and permitted by project policy.
walden release check <feature> --strict --json  # exact committed-input judgment
```

Policy knobs, all recorded, none silent:

- **Pending work blocks by default.** A deliberate partial release is an explicit waiver — `--allow-pending --reason "<text>"` — and the reason plus waived task identifiers land in the archived verdict. Treat a waiver in CI as a decision that belongs to a human: pass it from a manually-set variable, never hardcode it into the pipeline.
- **`--strict`** compares the actual consumed spec/evidence bytes and presence against one captured commit, independently of ignore rules. A pending waiver cannot bypass missing inputs or legacy assurance.
- **No bypass exists** for uncommitted code outside `.walden/`, and a repository without usable git fails closed: certification requires a git-backed code identity.

## Parsing output

Every command takes `--json` and emits the [versioned envelope](reference/json.md) on success and error paths alike — parse `result` fields by name, gate on the process exit code, and treat unknown fields as future additions (the contract only grows within `schema_version: v0beta1`). Warnings supplement failed outcomes; do not drop them. Known purity violations reject verification, with assertion and execution-policy facts distinguished. Handle `unattested` and unknown future assurance/state values fail-closed. Use probe-free `adopt <feature>` planning and reviewed current-contract scope before applying legacy migrations; a tool upgrade must not trigger an automatic historical replay.
