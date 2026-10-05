# Requirements Document

## Introduction

Let families share one account for several vehicles.

## Requirements

### R1 Family Accounts

**User Story:** As a family, I want one account for all our vehicles, so that we manage parking in one place.

#### Acceptance Criteria

1. `R1.AC1` WHEN a driver turns an account into a family account, the system SHALL let it hold up to 5 plates.
   - Acceptance check: a family account accepts a fifth plate.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Family accounts are accounts created under `account-management#R1.AC1`.
- `C2` Eligibility follows decision P14 (`docs/decisions/P14-family-accounts.md`).

## Out Of Scope

- Anything not listed above.
