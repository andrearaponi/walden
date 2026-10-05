# Requirements Document

## Introduction

Publish monthly occupancy reports and keep them free of old plates.

## Requirements

### R1 Reports

**User Story:** As a planner, I want occupancy figures, so that zones can be redesigned.

#### Acceptance Criteria

1. `R1.AC1` WHEN a month ends, the system SHALL publish occupancy per zone and hour without plates.
   - Acceptance check: the March report lists occupancy for every zone and hour and contains no plate.
2. `R1.AC2` WHEN a closed session is older than 13 months, the system SHALL remove its plate.
   - Acceptance check: a session closed 14 months ago has no plate.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Reports use only data permitted by `privacy-retention#R1.AC1`.

## Out Of Scope

- Anything not listed above.
