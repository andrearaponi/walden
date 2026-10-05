# Requirements Document

## Introduction

Record operator actions.

## Requirements

### R1 Audit

**User Story:** As an auditor, I want every operator action recorded, so that misuse can be investigated.

#### Acceptance Criteria

1. `R1.AC1` WHEN an operator acts on a session, refund or fine, the system SHALL append an audit entry with operator, action and time.
   - Acceptance check: each operator action yields one entry with those three fields.
2. `R1.AC2` WHILE an audit entry is younger than 2 years, the system SHALL keep it unchanged.
   - Acceptance check: an entry from last year cannot be edited or deleted.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Retention follows decision P5 (`docs/decisions/P5-audit-retention.md`).

## Out Of Scope

- Anything not listed above.
