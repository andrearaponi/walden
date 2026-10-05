# Requirements Document

## Introduction

Free parking for blue-badge holders.

## Requirements

### R1 Accessible Parking

**User Story:** As a blue-badge holder, I want to park without paying, so that mobility is not taxed.

#### Acceptance Criteria

1. `R1.AC1` WHILE a vehicle carries a registered blue badge, the system SHALL let it park free in every zone.
   - Acceptance check: a registered badge plate parked 4 hours in zone A pays nothing.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Eligibility follows decision P3 (`docs/decisions/P3-accessibility.md`).

## Out Of Scope

- Anything not listed above.
