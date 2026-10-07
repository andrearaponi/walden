# Requirements Document

## Introduction

Fine uncovered vehicles with a shorter payment deadline.

## Requirements

### R1 Fines

**User Story:** As the city, I want uncovered parking fined, so that the rules are respected.

#### Acceptance Criteria

1. `R1.AC1` WHEN an officer confirms an uncovered plate, the system SHALL issue a fine at the amount of the zone.
   - Acceptance check: confirming an uncovered plate in zone A issues one fine at the zone A amount.
2. `R1.AC2` WHEN a fine is issued, the system SHALL set a payment deadline 14 days later.
   - Acceptance check: a fine issued on 1 March is due on 15 March.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Fine amounts come from decision P4 (`docs/decisions/P4-fines.md`).
- `C2` Only plates reported uncovered by `enforcement-checks#R1.AC2` can be fined.

## Out Of Scope

- Anything not listed above.
