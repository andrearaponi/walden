# Requirements Document

## Introduction

Define the city's parking districts and zones with their operating hours and tariffs.

## Requirements

### R1 Zones

**User Story:** As a driver, I want to know the rules of the zone I park in, so that I pay correctly.

#### Acceptance Criteria

1. `R1.AC1` WHEN a location is queried, the system SHALL return the district that contains it, where a district groups several zones.
   - Acceptance check: a point inside district North, which groups zones A, B and C, returns North.
2. `R1.AC2` WHILE a zone is outside its operating hours, the system SHALL treat parking there as free.
   - Acceptance check: a session at 21:00 in a zone open 08:00-20:00 costs nothing.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Operating hours come from decision P1 (`docs/decisions/P1-zone-hours.md`).

## Out Of Scope

- Anything not listed above.
