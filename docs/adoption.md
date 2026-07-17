# Brownfield Adoption

Repositories that adopted Walden early — or that accumulated specs under older CLI versions — hold approvals recorded before the current contract existed: approved documents without fingerprints, completed tasks without evidence records. `walden adopt` brings them into the contract without re-running the human approval ceremony, and without ever fabricating evidence.

## Plan first

```bash
walden adopt            # whole portfolio
walden adopt user-auth  # one feature
```

The default is a **read-only plan** that classifies every feature:

| Class | Meaning | What apply would do |
| --- | --- | --- |
| `backfill` | Approved documents missing their fingerprints | Seal them, then re-prove unrecorded work |
| `re-prove` | Chain already sealed and fresh; completed tasks lack evidence | Re-prove them |
| `complete` | Nothing to adopt | Nothing |
| `blocked` | A *present* fingerprint contradicts the content | Nothing — ever |

```text
ADOPTION PLAN — 133 backfill, 0 re-prove, 2 complete, 0 blocked (399 doc(s) to seal, 931 task(s) to re-prove)
```

The honesty rule that shapes the classifier: an **absent** fingerprint is sealable — the approval is on record, only the seal is missing, and stamping the current body's fingerprint under it is a stated, reviewable trust assumption. A **present but wrong** fingerprint is evidence of drift after approval — human reconcile territory, and the lane never writes there.

Present the plan, review it, then:

## Apply

```bash
walden adopt --apply
```

Apply seals the backfill class (stamping fingerprints and repairing empty chain links), then re-proves unrecorded completed work through the same machinery as `walden verify` — real proof execution, purity contract included, execution profiles on every record. The result is an honest partition:

```text
ADOPTION — sealed 399 doc(s), verified 472, failed 441, skipped 0, blocked 0
```

Exit `1` when anything failed, with the full per-feature partition rendered. Interrupted or partially failed runs **resume by classification** — re-running apply picks up exactly what is still unsealed or unproven; a second apply over a finished adoption is a no-op. The adoption diff is ordinary git state: review it and commit it like any other change.

Plan-scale is cheap (a 135-feature portfolio classifies in under a second); apply costs what the proofs cost.

## Triaging the partition

A large failed partition is a work list, not a verdict on the adoption. In practice it decomposes into a few distinct populations:

- **Superseded eras.** Specs describing a product generation that no longer exists — an old frontend whose test files are gone, runtime code long since replaced. These proofs can never pass again, and fixing them would mean resurrecting dead code. They want [retirement](#retirement), not repair.
- **Mutating proofs.** Build or generate steps that write into the repository (`go build -o bin/…`, regenerated schemas). They fail the purity contract by design. Rewrite them as read-only assertions — `["go", "mod", "tidy", "-diff"]` instead of `["go", "mod", "tidy"]`, build outputs routed outside the tree.
- **Real failures.** The remainder — usually small — is the genuine work list: proofs that name actual regressions on living code.

## Retirement

Superseded specs are **deleted, not labeled**. History lives in git — `git show` recovers any retired spec at any commit — so keeping dead directories in `.walden/specs/` only forces every future gate run and every future reader to wade through a product generation that no longer exists.

The convention:

1. Delete the feature's spec directory (and its evidence file).
2. Record the retirement in `.walden/RETIRED.md` — one line per feature: name, date, reason, and the last commit where the spec was alive.
3. One commit for the whole ceremony, so the retirement is a single, findable event in history.

After retirement, `release check` judges only the living portfolio, and the narrative keeps its chapters: the repository ships a companion skill, [`walden-history`](../skill/walden-history/SKILL.md), that reconstructs sourced feature chronicles, product eras, and rework archaeology from committed `.walden/` history — and officiates the retirement ceremony itself.

## After adoption

The repository is on the current contract: fingerprinted chains, evidence with profiles, and a gate that means what it says. From here the [daily workflow](workflow.md) applies unchanged — and the first `walden release check` tells you exactly how far from certifiable the portfolio stands, one named blocker at a time.
