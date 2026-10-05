# Requirements Document

## Introduction

Limit how long personal data is kept.

## Requirements

### R1 Retention

**User Story:** As a driver, I want my data deleted when it is no longer needed, so that my movements are not tracked.

#### Acceptance Criteria

1. `R1.AC1` WHEN a closed session is older than 13 months, the system SHALL remove its plate.
   - Acceptance check: a session closed 14 months ago has no plate.
2. `R1.AC2` WHEN an account is closed, the system SHALL delete its plates and payment tokens within 30 days.
   - Acceptance check: 30 days after closing, the account has no plates or tokens.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Audit entries are kept as required by `audit-log#R1.AC2`.
- `C2` Accounts are those of `account-management#R1.AC1`.

## Out Of Scope

- Anything not listed above.
