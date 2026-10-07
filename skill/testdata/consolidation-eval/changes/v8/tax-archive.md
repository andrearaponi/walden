# Requirements Document

## Introduction

Archive session data for tax audits.

## Requirements

### R1 Tax Archive

**User Story:** As the city's finance office, I want closed sessions archived, so that tax audits can trace every payment.

#### Acceptance Criteria

1. `R1.AC1` WHEN a session closes, the system SHALL archive its plate, zone and amount for 10 years.
   - Acceptance check: a session closed today stays in the archive with its plate for 10 years.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Personal data of closed accounts is handled under `privacy-retention#R1.AC2`.
- `C2` The archive period follows decision P15 (`docs/decisions/P15-tax-archive.md`).

## Out Of Scope

- Anything not listed above.
