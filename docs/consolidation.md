# Contract Consolidation

A long-lived repository accumulates specifications. Each change is reviewed against the requirements it touches, but nobody can hold dozens of specifications in mind at once, so overlaps, silent contradictions, removals that other specifications still cite, reused identifiers and citations of missing sources build up unnoticed. A single audit after many specifications is unreliable for the same reason.

Contract consolidation keeps the portfolio consistent in small, regular steps. Walden counts the specifications whose contract changed since the last consolidation, asks for a bounded review every two or three of them, runs checks whose cost does not depend on anyone's memory, and keeps a compact, approved view of the contracts that hold today. Later authoring reads that view instead of the whole portfolio.

## The cycle

1. **Count.** A feature is a pending contract change when its approved requirements differ from the version recorded at the last consolidation (by approval fingerprint, not by date), or when a recorded feature no longer exists. Design, task and evidence changes do not count, and re-approving identical content does not count.
2. **Remind.** With two pending changes `walden status` suggests a consolidation; with three or more it reports the consolidation as due, and `walden feature init` and `walden release check` repeat the due warning. Each of these warnings, and the report's next action, asks to compare each pending change with the statements linked to it, so the semantic check is requested even when someone only asks to consolidate. The warning never blocks: exit statuses, verdicts and blockers stay what they were.
3. **Review the bounded scope.** `walden consolidate` lists the pending features plus every feature linked to them in either direction by a feature-qualified reference (`feature#R1.AC1`) or a shared cited file. Files cited by many features are listed apart as widely cited, with the features linked only through them. For each pending feature the report also puts its statements next to the linked statements (see Comparisons below). The agent compares them, proposes fixes as ordinary specification revisions, and updates the view.
4. **Approve.** `walden consolidate open` puts the updated view in review only when it matches the specifications and its coherence review is complete. After the user's explicit approval, `walden consolidate approve` seals the view and records a checkpoint; the pending count starts again from zero.

## Comparisons

Finding a contradiction needs both sides in front of the reviewer. For each pending feature the report shows the text of its criteria, NFRs and constraints, each marked `added`, `changed` or `unchanged` against the version recorded at the last consolidation (`unverified` when only the identifier was recorded), plus its `removed` statements with the wording they had. A `changed` statement also shows its previous text, so a dependent can be checked against what actually changed. Next to them come the current statement texts of every linked feature; features linked only through widely cited files are named without statements. A reviewer compares every added, changed or removed statement with the linked statements beside it and reports each contradiction, duplicated rule or dependency on a removed or changed statement with both qualified identifiers.

The record keeps the statement texts of every consolidated or backlog feature for this purpose. Records written before statement texts existed still report removals, without wording.

## Coherence review

While changes are pending, the view carries a `## Coherence Review` section: one `### <feature>` entry per pending feature with the outcome of comparing that change with its linked statements, the inconsistencies found or none, citing the linked statements compared as `feature#ID` in inline code. Until it is complete the report lists `coherence-review` findings as the remaining work, and `walden consolidate open` and `walden consolidate approve` refuse, naming each feature whose entry is missing, empty, duplicated or cites none of its linked statements, and any entry for a feature without pending changes. With nothing pending, the section is not checked. You read the review together with the contracts it explains when you approve the view.

The CLI checks the review's presence, coverage and citations, never its correctness: whether the comparison was right stays with the reviewing agent and with you.

## The current-contract view

`.walden/contracts.md` holds one section per consolidated feature. The agent writes the body; the CLI owns its frontmatter, exactly as for specification documents.

```markdown
## feature-name

Purpose: One line on what the feature guarantees today.
Active: R1.AC1, R1.AC2, NFR1, C1
Reserved: R1.AC3 (removed by decision D4)
Sources: docs/decisions/D1-example.md
Related: other-feature (why they interact)
```

The CLI checks the view against the specifications and reports every mismatch: a consolidated feature without a section, a section for a feature that does not exist, an `Active:` list that omits a defined identifier or names an undefined one, an identifier that disappeared since the last checkpoint without moving to `Reserved:`, a reservation that was dropped, and a missing `Purpose:`. A view with mismatches cannot be opened or approved. Editing an approved view makes it stale until it is opened and approved again.

`Reserved:` is the memory of removed identifiers. Once recorded, a reserved identifier stays reserved, and the report flags any specification that defines it again.

## The record

`.walden/consolidation.json` is written only by `walden consolidate start` and `walden consolidate approve`. It stores, per feature, whether it is consolidated or still in the backlog, the requirements fingerprint it was consolidated at, its active and reserved identifiers and the statement texts of its active identifiers, plus the checkpoint that binds the approved view's fingerprint. Do not edit it by hand. If it becomes unreadable or disagrees with the approved view, every command reports the consolidation state as unknown with the path and a remedy, and otherwise behaves as before.

## Starting on an existing portfolio

`walden consolidate start` records today's approved features as an **unconsolidated backlog** and counts pending changes only from that point. Nothing in the backlog is reported as consolidated until an approved consolidation covers it, so starting never pretends that older specifications were reviewed. Consolidate the backlog in small batches, by area, when you choose to.

## Deterministic findings

The report is read-only: it writes nothing, runs no proofs and no environment probes. Its findings are advisory leads:

| Kind | Meaning |
| --- | --- |
| `missing-file` | A requirements document cites a repository file that does not exist, such as a decision record. |
| `dangling-reference` | A feature-qualified reference names a feature or identifier that does not exist. |
| `reserved-reuse` | A specification defines again an identifier the last consolidation reserved. |
| `view-mismatch` | The view disagrees with the specifications. |
| `coherence-review` | While changes are pending, the coherence review lacks an entry, has an empty, duplicated or extra entry, or an entry cites none of its linked statements. |

Citations are read from inline code outside fenced blocks and outside acceptance-check lines, where examples usually live: a cited file is a relative path with a directory and an extension that begins with a letter (so `github-copilot/claude-opus-5.5` is a model, not a file), and a qualified reference is `feature#ID`. These heuristics can miss a citation or flag an example; that is why findings never block anything.

## Limits

- The thresholds are fixed in this release and the due state is a warning, never a gate.
- The CLI checks structure and identifiers, not meaning: overlaps and contradictions are found by the reviewing agent and confirmed by you.
- Scope links come from explicit citations only. Related features that cite nothing are found through the view's `Related:` lines.
- CLI ownership of the record and the view is a convention plus consistency checks, not cryptographic protection, as for approvals.
