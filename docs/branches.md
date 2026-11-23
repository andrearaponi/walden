# Parallel Branches

Walden keeps its state in the repository: the documents with their approval seals, and one evidence ledger per feature. When several features are in flight on parallel branches, as in a team or with a group of coding agents, every merge brings two versions of that state together. This page says what a merge does to it, how to resolve the conflicts it leaves under `.walden/`, and how to avoid most of them. Every command block was run with v0.15.2.

## What a merge does

| What you see | Why |
| --- | --- |
| The evidence of many features reads `stale-code` | The code identity covers the whole tree. A merge that brings in code makes every record older than the tree, also for features the merge did not touch. |
| A ledger is in conflict | Both branches ran `walden verify` on the same feature, and each run rewrote the same records. |
| The front matter of a document is in conflict | Both branches changed the state of one approved document: one approved it again, and the other completed its tasks or approved it again too. |

None of these means that work was lost. Proving again on the merged tree settles the first. The other two are conflicts in text the CLI owns, and the recipes below resolve them without typing a value by hand.

## After every merge

```bash
git diff --name-only --diff-filter=U   # the paths git left in conflict
walden validate --all                  # every document, and every ledger can be read
```

Since v0.15.2 `walden validate` fails on a ledger that cannot be read, such as one left with conflict markers. An earlier release answers VALID on it, so look at the output of the first command too.

Then run `walden status <feature>` for each feature whose documents the merge touched, and prove again as [the last recipe](#prove-again-after-the-merge) says.

## A ledger in conflict

Do not merge a ledger by hand. Take one side whole, finish the merge, and let `walden verify` rebuild the rest:

```bash
git diff --name-only --diff-filter=U -- .walden/evidence/ | while read -r ledger; do
  git checkout --theirs -- "$ledger" && git add "$ledger"
done
```

Either side works; `--ours` keeps the ledger of the branch you are on. After a merge that brought in code, the records of both sides are stale anyway. `walden verify` proves again every completed task that is not verified, including a task whose record was only on the side you dropped: it reads `unrecorded` until then.

Delivery proofs need care. A ledger that holds an attestation says `"schema_version": "v1alpha3"`. When only one side of the conflict holds attestations, take that side:

```bash
ledger=.walden/evidence/<feature>.json
git show ":2:$ledger" | grep -c '"attestation"'   # ours
git show ":3:$ledger" | grep -c '"attestation"'   # theirs
```

A delivery task whose attestation was on the dropped side is proven again by `walden verify` and attested at the merge commit. If the delivered text has moved on since, that proof fails, which is what the attestation was there to avoid: take the ledger of the other side instead.

## Front matter in conflict

Open the document first. When the conflict markers are all above the second `---` line, the conflict is in lines the CLI writes: approval time, last modification, fingerprints. Keep one side of the conflicting block and the changes both sides made to the body:

```bash
tmp=$(mktemp -d)
for doc in $(git diff --name-only --diff-filter=U -- .walden/specs/); do
  git show ":1:$doc" > "$tmp/base"
  git show ":2:$doc" > "$tmp/ours"
  git show ":3:$doc" > "$tmp/theirs"
  git merge-file --ours -p "$tmp/ours" "$tmp/base" "$tmp/theirs" > "$doc" && git add "$doc"
done
rm -r "$tmp"
walden status <feature>
```

`git checkout --ours` is not the same: it takes the whole file and drops what the other branch changed in the body. When the body is in conflict too, merge the body by hand before anything else, because the loop keeps your side of every conflicting block. Then keep one side of the front matter block whole, without mixing lines of the two sides.

`walden status <feature>` says what is left to do:

- **Approved and fresh.** Nothing. This is what you get when the other branch only completed tasks: a ticked checkbox does not change the fingerprint of the plan.
- **Stale**, with `content does not match approval fingerprint` and `upstream document is stale`. Both branches revised the document, and nobody has reviewed the merged text. Run `walden reconcile <feature>`, read the merged documents, and approve them again phase by phase.

## Avoid most of it

- **Persist evidence in one place.** On a feature branch, prove the features you carry with `walden verify <feature> --check`: it runs the proofs and writes nothing. Persist with `walden verify <feature>` for the feature the branch owns, and on the integration branch after each merge. A ledger that only one branch writes does not conflict.
- **Revise an approved specification on one branch at a time.** Two approvals of one document always collide, like two migrations with the same number. A second branch that must revise it merges the first one, then revises on top of it.
- **Keep a broad proof in one place.** A full test suite named in every task runs once per task whenever the feature is proven again, and again for every feature that names it. Give each task the narrow proof of its criteria, and keep the suite in one task, or in one feature that owns the regression baseline. In one portfolio of 21 features, four fifths of the time of a full re-prove went to two commands repeated across tasks.

## Prove again after the merge

`walden verify` takes one feature per run:

```bash
for dir in .walden/specs/*/; do
  walden verify "$(basename "$dir")" || break
done
walden release check
```

Commit the refreshed ledgers as your project's policy says. [CI Integration](ci.md) shows where the persisting run and the report-only run belong in a pipeline.

## What these recipes do not do

- They do not combine the evidence of two sides. One side wins, and `walden verify` rebuilds the rest.
- They do not review a specification for you. A document revised on two branches is approved again by a person.
- They are recipes, not commands: the CLI does not resolve these conflicts itself. What they keep out is hand edits of the text the CLI owns.
