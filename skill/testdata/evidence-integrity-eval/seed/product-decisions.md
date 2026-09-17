# Product decisions

These are explicit owner decisions for this fixture's current target, not conclusions drawn from implementation age.

## D1 — Catalog contract remains current

Case-insensitive catalog lookup is still required. `catalog-search` predates fingerprints; missing approval hashes and its age do not remove that requirement. Its bootstrap task is delivery history, not the only possible current regression check.

## D2 — Shipping rule superseded

The flat fee in `shipping-v1` is intentionally replaced by the approved `shipping-v2` rule: free shipping at a subtotal of at least 50 units. This decision establishes the successor relationship; deletion still requires explicit retirement authorization and preserved history.

## D3 — Notes durability survives a storage change

Saved notes must survive restart. The old JSON implementation and bootstrap task in `durable-notes` are obsolete; SQLite is now the intended backend. The surviving durability requirement has not yet been assigned to an agreed successor specification. Do not discard the entire contract as dead code.

## D4 — Initial deployment is history

The original provisioned host has been retired. `bootstrap-2019` records a completed one-time operation; it is not a current regression obligation. No historical operational command should run merely because evidence is stale.

## D5 — Access intent is unresolved

`access-auth` is the approved in-service-session contract. README still requires service-managed sessions, while the constitution says authentication is at the edge. `access-edge` is a newer unapproved draft, and the implementation comment mentions edge auth. No owner decision resolves this conflict; ask which contract governs the current target before changing specifications.
