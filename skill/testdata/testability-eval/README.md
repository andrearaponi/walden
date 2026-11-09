# Testability acceptance fixtures

These are synthetic inputs and a fixed review rubric for the testability companion, not a runtime dependency of the skill and not an automated judge of model behavior. The contract tests hold the skill's text; these fixtures let a reviewer observe its behavior.

## Prepare

1. Build one walden CLI from the checkout under review, outside the source tree or in an ignored directory, and record its version. Every scenario except S5 puts it first on PATH. S5 gets a PATH with `node` and the system tools but no walden executable, and its host is started by absolute path.
2. Create a fresh git repository per scenario and variant. Copy `seed/loans` and `seed/cmd` into it, and copy `seed/go-1.22.mod` (S1) or `seed/go-1.25.mod` (every other seed scenario) to `go.mod`.
3. Run `walden repo init` and `walden feature init loans`, then copy `seed/constitution.md` to `.walden/constitution.md`.
4. Replace only the scaffold bodies the scenario's setup names, keeping the CLI-created frontmatter: `seed/requirements.md` always, `seed/design.md` when the design is drafted.
5. Prime the state through the CLI, as the setup says: `walden review open` and `walden review approve` for approved requirements, and for the approved design of S3. These fixture operations are not approvals of any real feature.
6. Commit the prepared tree, so that a reviewer can diff every file the session touches.

Fixture sources do not live under a directory named `.walden`, which the development repository ignores. Generated state lives only in the temporary repositories.

The adopter scenarios, A1 and A2, run on a copy of the adopter's tracked tree in a fresh repository, never on the adopter itself. Install the companion there with the documented command, `npx skills add andrearaponi/walden --skill walden-testability`, pointed at a local export of the checkout under review instead of the published repository. Choose one feature with approved requirements in each adopter and record the choice and its reason in the report.

## Run

Use one host, version, model and settings for every run. Load skills explicitly and nothing else: the candidate loads the skills its scenario lists, S6 the companion alone, and the baselines of S1 and S2 the walden guide alone. Start a fresh session for every scenario and variant, and send each follow-up as a real user message in the same session after the previous answer; never place a future answer in the first prompt. Preflight the exact command forms before the first run. Keep exit status, timing and raw output before reading any transcript, and keep every transcript and every generated file.

## Review

Judge each scenario against its `expect` list in `scenarios.json` and the expected findings below. A finding's wording may differ; its substance may not. A baseline failure is comparison data. A missing or failed candidate observation blocks behavioral acceptance.

### Expected findings

| Case | Criterion | Location | Expected finding | Acceptable variants |
| --- | --- | --- | --- | --- |
| P1 | R2.AC1 | `seed/loans/overdue.go` | `IsOverdue` reads `time.Now` directly. S1, on Go 1.22: a controllability gap naming the clock, closed by a Go seam. S2 and S3, on Go 1.25: `testing/synctest` drives the clock, so no gap and no seam. | S1: a `func() time.Time` value, a one-method clock interface or a clock field, in Go form only. S2: synctest named as the control; a seam proposed beside it fails N2. |
| P2 | R2.AC2 | `seed/loans/reminders.go` | `SendReminders` sends through `smtp.SendMail` to a fixed address: a controllability gap naming the network or the mail service, closed by a mail port. In S2 the gap is anchored to `loans/reminders.go` and `SendReminders` or `deliver`. | A configurable mail address with a loopback listener in the proof, or a send port: both close the gap. |
| P3 | R2.AC3 | `seed/loans/reminders.go` | The attempt count stays in the unexported field `reminderRun.attempts`: an observability gap, closed by returning the attempts in a result or emitting them. | Counting the calls a fake mail port receives, when the port of P2 is proposed and the reply says so. Reading the unexported field fails. |
| P4 | R3.AC1 | `seed/loans/fees.go` | No requirement and no design section states the late-fee rule, so the only expected value would come from `lateFee` itself: an oracle gap, whose change is a question for the requirements phase. | A worked example agreed with the requirement's owner. Recording the current output as the oracle of new behavior fails. |
| P5 | R1.AC2 | `seed/loans/service.go` | An invariant over all active loans: a property proof over sequences of borrow and return requests, inner loop, no gap. | Examples with the stated reason that the domain is small enough to enumerate. |
| P6 | R4.AC1 | `seed/loans/web.go` | The badge is visible only in the rendered page: observed through an in-process render, the handler with `httptest` or the template executed to a buffer. Whatever needs the red color, a browser, carries the reason that only a browser computes the style. | The CSS class asserted in process with the stated limit; or an end-to-end checkpoint proof for the color with its reason. |
| P7 | R4.AC2 | requirements | "The page feels responsive" states no observable distinction between success and failure: unverifiable, routed to the requirements phase, no design change. | None. |
| P8 | R5.AC1 | `seed/loans/statement.go` | The design changes `MonthlyStatement`, which no test and no proof exercises: characterization proofs that pin its current output before the change, naming the search behind the claim. Expected in S2 and S3, where the design is drafted. | None. |
| P9 | R1.AC1 | `seed/loans/service.go` | No gap: the returned unknown-member error is the observation, the stated error the oracle, an example proof in the inner loop. | None. |
| N1 | R6.AC3 | `seed/loans/audit.go` | Decoy. `AppendAudit` reads the clock, the environment and the filesystem, but the design names neither it nor `cmd/lending/main.go`, and no named function calls it: no finding may cite it. | None. |
| N2 | R2.AC1 | `seed/go-1.25.mod` | On Go 1.25 the platform controls the clock: no clock seam in S2 or S3. | None. |
| N3 | R1.AC1 | `seed/loans/service_test.go` | The criterion without a gap shows none, gets no seam and no injection. | None. |

S2's follow-up accepts the changes for R2.AC2 and R2.AC3 and rejects the one for R3.AC1 with a reason. After it, the design holds the matrix under the Testability Matrix sub-heading of Verification Plan, the accepted changes in the sections they cite, one Failure Modes And Tradeoffs entry for R3.AC1 naming the gap and the reason, and the same frontmatter bytes.

## Report

Keep runs and the report under `temp/testability-skill/` in the development repository; the report used by the task proof is `temp/testability-skill/acceptance/report.json`. Never publish private run data or copy it into the product.

The test-only report contract is `testabilityReport` in `skill/testability_eval_test.go`:

- `kind`: `observed` for reviewed real sessions, never for fabricated data.
- `host`, `host_version`, `model`, `settings` and `walden_version`: the provenance of every run.
- `candidate`: the SHA-256 of `skill/walden-testability/SKILL.md` and `skill/walden-testability/references/seams.md` as run; they must equal the current files.
- `runs`: one candidate run per scenario and one baseline run for S1 and for S2. Each names its verdict, a report-relative transcript, the feature an adopter run reviewed, and one check per criterion its scenario lists.
- Each check has an `id`, a `verdict` and a report-relative `anchor` to an existing file, optionally with a `#L<number>` suffix; anchors stay inside the report directory.

Use `pass`, `fail` or `not-run` honestly. Acceptance rejects unexecuted runs, missing or escaping anchors, stale candidate hashes, and candidate criteria that did not pass. The checker validates the report's integrity, not the transcripts: the reviewer remains responsible for judging them. Run the acceptance test with `WALDEN_TESTABILITY_EVAL_REPORT` naming the real report; without it the ordinary suite skips, and no synthetic report may stand in for it.
