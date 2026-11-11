---
walden_schema_version: v1alpha1
status: approved
approved_at: 2026-11-10T18:56:12Z
last_modified: 2026-11-10T18:56:12Z
approved_fingerprint: sha256:85258c3a8d22cba6c38529d597cb6718a048db5ae5db63bca9b5bb5366e54914
source_requirements_approved_at: 2026-11-10T18:56:12Z
source_requirements_fingerprint: sha256:f6c9c8e40c3db122c994b68fad2381f1ba12d0a85011b94c5072c6ad70619b16
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
