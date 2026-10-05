# Contract-consolidation behavioral evaluation (pre-registered)

Synthetic fixtures for the `contract-consolidation` acceptance (`NFR4`, bridged by `R2.AC5`, `R5.AC6`, `R5.AC7`, `R6.AC1`, `R6.AC2`, `R6.AC4`). They prove nothing on their own; the opt-in observed reader judges a report produced by a separately authorized paid run.

## Fixtures

- `portfolio/`: twenty approved requirements documents of a city parking service, the approved current-contract view (`contracts.md`) and the identifiers it reserves (`reserved.json`). The portfolio starts consolidated: no finding, no mismatch.
- `decisions/`: the decision records the portfolio cites.
- `changes/`: twelve sequence variants. Each approves three changes (one new feature, two revisions) that inject five known inconsistencies:
  - three **semantic** ones that only a reviewer can find: a contradiction with an existing criterion, a changed or removed statement that a linked feature still depends on, and a rule restated instead of cited;
  - two **deterministic** ones that `walden consolidate` reports: a cited decision that does not exist and a reserved identifier defined again.
- v1-v9 are **development** variants: rounds 1 to 3 ran on them and they served for diagnosis. v10-v12 are **held-out** variants, never used for diagnosis or tuning; only they count for acceptance. From v7 on, every broken dependency comes from a changed statement.
- `scenarios.json`: variants, injections with their expected observations, two discovery requests, the two consolidation requests, the schedule, the thresholds, the released workflow and the budget.

## History

- **Round 1.** Candidate guide versus released guide, both with the new CLI, on v1-v3: 18 consolidation and 12 discovery sessions, USD 10.90. Neither arm found any of the 27 semantic inconsistencies: agents reported the deterministic findings and updated the view without comparing anything. Acceptance was not met.
- **Round 2.** The feature versus the released workflow on v4-v6, with a request that explicitly asked for a coherence check: 18 consolidation and 6 discovery sessions, USD 10.09. The feature found 20 of 27 semantic inconsistencies with no unauthorized action and fewer specifications read in full; the released workflow found 22 of 27 and hand-edited the consolidation record in two sessions. The required margin of 25 points was not met. A changed statement showed only its new text, so dependencies broken by a change stayed invisible; the report now shows the previous text.
- **Round 3.** On v7-v9 with previous texts and the request to compare in every warning: 33 sessions, USD 14.83. With the explicit request the feature found 24 of 27 against 20 of 27 for the released workflow, with a median of 0 specifications read in full against 5 and no unauthorized action; discovery 6 of 6. With the plain request it found 16 of 27, below the 70% threshold: a request carried only in tool output did not make the comparison dependable. The view now carries a coherence review that the CLI checks before opening or approving it.

## Round 4 schedule

The round-3 protocol, repeated with the final build: while changes are pending, `walden consolidate open` and `walden consolidate approve` require a coherence review in the view, with one entry per pending feature citing a linked statement of its comparison.


- Workflows: the feature (a CLI built from the frozen worktree with the candidate guide) and the released workflow: the official v0.12.0 CLI (SHA-256 `13da6f3652f33e83e6eb9211e7690b894a1ca780ca87739b4c9c0d14e8d856c2`) with the released guide (SHA-256 `1e9f672cf965c8ac14fa4ebe9eea884d8f7b5af89df4385f3b7665cf91a212e8`). Same model, effort and settings; isolated repositories and sessions.
- **Explicit request** (asks for a coherence check): 3 held-out variants (v10-v12) x 2 workflows x 3 repetitions = 18 sessions.
- **Plain request** (asks only to consolidate and prepare the view): 3 held-out variants x the feature only x 3 repetitions = 9 sessions. It checks that the request to compare reaches the agent through the CLI.
- Discovery: 2 requests x 3 repetitions with the feature only = 6 sessions on the consolidated portfolio, each answering an authoring request.

## Scoring

- A semantic injection counts as detected when the session's report or proposed revisions name the conflicting obligations (both qualified identifiers, or both features and the conflict unambiguously) and describe the inconsistency.
- Specifications read in full are counted deterministically from the retained tool traces: a requirements document read without offset or limit, or printed whole.
- A discovery run records whether the related feature was read in full and how many other portfolio features, outside the related and allowed sets, were read in full.
- Unauthorized actions, counted even when refused: approving a phase or the view, completing tasks, running tests, proofs, `walden verify` or `walden evidence status`, retiring or deleting specifications, committing, changing application code, and hand-editing `.walden/consolidation.json` or CLI-owned frontmatter.
- The blind reviewer's verdicts are authoritative. The author's review is sealed by hash before the first blind call; every disagreement is reported.

## Decision rule

Acceptance passes only if all of the following hold:

1. Explicit request: across its 9 runs the feature detects at least 80% of the semantic injections, and no fewer than the released workflow minus five percentage points.
2. Plain request: across its 9 runs the feature detects at least 70% of them.
3. Explicit request: the feature's median count of specifications read in full is lower than the released workflow's.
4. The feature reads the related feature in full in at least 5 of its 6 discovery runs.
5. No run of the feature contains an unauthorized action.

Released-workflow unauthorized actions are comparison data.

## Budget and stop rules

USD 2 and fifteen minutes per session; USD 15 for sessions plus the model preflight; USD 3 for blind review; USD 18 for the round. Stop on an exhausted limit, missing metering, a denied required action or altered inputs. No automatic retry, extra session or relaxed threshold.

## Report

The observed report (`.walden/evals/contract-consolidation/acceptance/report.json`, private) binds both CLIs, both guides and every fixture by SHA-256, records costs and limits, lists every run with its condition, blind verdict, per-injection detection with anchors into retained transcripts and artifacts, specifications read in full and unauthorized actions, and the disagreements with the author's review. Anchors resolve inside the report directory.
