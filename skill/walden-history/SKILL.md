---
name: walden-history
description: "Walden History narrates a Walden repository's past from git history: feature chronicles built from frontmatter transitions, the product story of eras and supersessions, rework reports measuring churn after approvals, and the retirement ceremony for superseded specifications. Use when the user asks how a feature evolved, for a spec timeline or contract history, for rework or churn analysis, for the portfolio's story, or to retire superseded specs from `.walden/specs/`."
metadata:
  short-description: Walden spec history and retirement
---

# Walden History

Use this skill to narrate a Walden repository's past: how a feature evolved, what story the portfolio tells, how much rework followed each approval — and to retire superseded specifications properly. The walden skill drafts and executes the present; this skill reads time. Reply in the user's preferred language when possible.

## When To Use

- A user asks how a feature evolved — its timeline, chronicle, or contract history
- A user asks for the product's story: eras, supersessions, living versus retired work
- A user asks how much rework followed an approval, or wants churn analysis
- A user decides to retire superseded specifications

## Prerequisites

A git repository whose history contains committed `.walden/` documents. The walden CLI is optional and used only to cross-check current state: every reconstruction below works with git alone. A repository that gitignores `.walden/` has no spec history to chronicle — say so plainly instead of improvising one.

## The Grounding Rule

Every historical statement carries exactly one of two labels, and the label is not decoration — it is the statement's evidence class:

- `[sealed]` — a string comparison over recorded values at a named commit. Chain coherence is sealed when, at that commit, `design.source_requirements_fingerprint` equals `requirements.approved_fingerprint`, `tasks.source_design_fingerprint` equals `design.approved_fingerprint`, and evidence records' chain fields equal the document seals they name. Incoherent seals at a commit are findings — report them explicitly, never smooth them over.
- `[inferred]` — commit messages, authorship, temporal causality, and every "because". Inference is legitimate narration; unlabeled inference is confabulation.

Four hard rules:

1. Retrieve historical content with `git show <sha>:<path>` — never from recollection. If you have not run the command in this conversation, you do not know what the document said.
2. Anchor every historical claim to the commit identifier it derives from. A claim without a sha is not a claim; it is an impression.
3. Content verification is kernel territory: comparing recorded fingerprints to each other is yours; recomputing a fingerprint from a body requires the walden CLI's normalization rules. When asked to verify content against a seal, say exactly that and suggest checking out the commit and running `walden validate`.
4. Git authorship is a provenance proxy: write "committed by", never "approved by". Walden does not yet record approver identity, and the chronicle must not invent it.

## Recipe: Feature Chronicle

Reconstruct one feature's timeline. Work per document (`requirements.md`, `design.md`, `tasks.md`), oldest first:

```bash
# Revision list for one document (oldest first)
git log --follow --reverse --format='%h|%aI|%an|%s' -- .walden/specs/<feature>/requirements.md

# Frontmatter as of one revision (flat key: value — trivially parseable)
git show <sha>:.walden/specs/<feature>/requirements.md | sed -n '2,/^---$/p'
```

Derive events from frontmatter transitions between consecutive revisions using the event derivation table — never from commit messages alone:

| Transition observed between consecutive revisions | Event |
| --- | --- |
| `status: draft` → `in-review` | review opened |
| `status: in-review` → `approved`, `approved_at` set | approval |
| `status: approved` → `draft`, fingerprints cleared | reconciliation |
| `approved_fingerprint` appears, status stays `approved` | seal (adoption backfill) |
| `source_*_fingerprint` appears or changes on an approved document | chain repair |
| body changed and a new `approved_fingerprint` + `approved_at` | re-approval cycle |
| `walden_schema_version` appears | format migration stamp |

Then read the evidence ledger's own history the same way:

```bash
git log --reverse --format='%h|%aI' -- .walden/evidence/<feature>.json
git show <sha>:.walden/evidence/<feature>.json
```

Record appearances are first proofs; `result` flips are verification history; `profile` differences between revisions are environment shifts worth narrating. Close the timeline with the retirement lookup: if the feature has an entry in `.walden/RETIRED.md`, report the retirement commit, the recorded reason, and the superseding feature.

Output: a chronological table — one row per event, columns for commit, date, event, and label (`[sealed]`/`[inferred]`) — followed by the narrative, which may interpret but must cite the rows.

## Recipe: Product Story

The portfolio's eras. This recipe has a hard scoping rule: whole-portfolio passes read only the retirement index and `git log` metadata — never deep-reconstruct every feature. Deep dives are per named feature, via the Feature Chronicle.

```bash
# First appearance of each feature (creation event)
git log --reverse --format='%h %aI' -- .walden/specs/<feature> | head -1

# All feature directories ever deleted (retirement candidates for the integrity check)
git log --diff-filter=D --format= --name-only -- '.walden/specs/*/requirements.md' | sort -u
```

Assemble: creation dates cluster features into generations; `.walden/RETIRED.md` entries close them with explicit supersessions; the census (living / retired / in-flight) is the story's present tense. Group eras by supersession chains, not by dates alone — dates suggest `[inferred]`, index entries state `[sealed by convention]`.

## Recipe: Rework Report

The raw material for rework analysis: what happened to each document *after* its approvals.

1. Run the Feature Chronicle's approval extraction to get the approval events per document.
2. Split history into inter-approval segments; for each segment summarize what changed:

```bash
git diff <approvalShaA>..<approvalShaB> -- .walden/specs/<feature>/<doc>
```

3. Report reconciliation events (approved → draft) separately from content evolution inside a draft — a reconcile is a contract reopening; draft churn is authorship.
4. End with the metrics block, numbers before narrative:

```text
rework metrics — <feature>
  re-approval cycles: requirements=N design=N tasks=N
  reconciliations: N
  first approval → last recorded change: N days
```

## The Retirement Ceremony

The skill's only write path. Superseded specifications are deleted — history lives in git — and recorded in the retirement index, so the gate judges only the living portfolio while the narrative keeps its chapters.

Preconditions: the user has named the features to retire and their supersession in the current conversation. Confirm the exact list and the successor back to the user and wait for explicit approval — never treat silence, or an earlier musing, as consent.

Steps, in one commit:

```bash
git rm -r .walden/specs/<feature>            # per retired feature
# append one index entry per feature to .walden/RETIRED.md (create it on first use)
git add .walden/RETIRED.md
git commit -m "chore: retire <scope> (superseded by <successor>)"
```

Never push. After the commit, suggest `walden validate` — the living portfolio is now the whole portfolio.

## The Retirement Index Convention

`.walden/RETIRED.md` lives beside `constitution.md` and `lessons.md`, outside `specs/`, so no walden command ever judges it. One line per retired feature — parseable with a single pattern, readable without one:

```markdown
# Retired Specifications

History lives in git: `git show <retirement-commit>^:.walden/specs/<name>/requirements.md`
restores any document exactly as sealed.

- <feature-name> — retired <short-sha> — superseded by <feature-or-era> — <reason>
```

## Integrity Check

The index and git history must agree. Report every mismatch:

- An index entry whose spec directory still exists — a retirement recorded but never executed.
- A deleted feature (from the `--diff-filter=D` listing) with no index entry — an unrecorded retirement; offer to backfill the index line with the deletion commit.
- A malformed index line — name the line number.

## Output Standards

- Timelines as tables, one commit anchor per row, labels inline; narrative after the table, citing its rows.
- Short shas in prose, full paths in commands.
- When history is missing (squash merges, shallow clones, gitignored `.walden/`), say what was lost and narrate only what git preserved — never interpolate.
- Numbers before narrative in every rework report.
