# What walden v0.14.5 derives

This directory holds what the release before proof kinds wrote and printed
for a portfolio that declares no proof kind. Tests compare the current code
with it, so a plan without a declaration keeps deriving what it derived.

- `portfolio/` is the fixture before any run: draft documents and two files
  the fixture treats as code. Its Walden tree is stored as `walden/`, because
  `.walden/` is ignored at any depth in this repository.
- `recorded/` is written by walden v0.14.5 and must not be edited by hand:
  - `version.txt`: the output of `walden version` of the binary that made it;
  - `complete/`: the documents and the ledgers after every leaf was
    completed, so each leaf has a recorded fingerprint;
  - `scenario/`: the tree on which the read commands ran. Its plan `states`
    holds one task for each evidence state;
  - `results/`: the JSON results of `evidence status`, `release check`,
    `adopt` and `task status` on that tree.
- `record.sh` regenerates `recorded/`. It needs the released v0.14.5 binary
  and refuses any other release.

The recorder is `TestRecordPreLifecycle` in `internal/app`. Two states have no
command that reaches them, so the recorder edits the scenario ledger: it
removes the execution facts of task 1.5 and the record of task 1.6. Every
other record is as the release wrote it.
