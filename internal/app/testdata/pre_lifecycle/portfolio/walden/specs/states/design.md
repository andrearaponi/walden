---
status: draft
approved_at:
last_modified: 2026-11-10T00:00:00Z
approved_fingerprint:
source_requirements_approved_at:
source_requirements_fingerprint:
---

# Feature Design

## Architecture

One shell proof for each task. The fourth reads a flag under `.walden/`, which is outside the code identity.

## Options Considered

No alternative is meaningful: the fixture exists to be recorded.

## Simplicity And Elegance Review

Seven commands and nothing else.

## Failure Modes And Tradeoffs

The fourth proof fails once the flag exists. That is the purpose.

## Verification Plan

Each proof exits with status zero while the flag is absent.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| `R1` | the seven proofs |
