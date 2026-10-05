# Requirements Document

## Introduction

Define the city's parking zones with their operating hours, tariffs and daily caps.

## Requirements

### R1 Zones

**User Story:** As a driver, I want to know the rules of the zone I park in, so that I pay correctly.

#### Acceptance Criteria

1. `R1.AC1` WHEN a location is queried, the system SHALL return the zone that contains it.
   - Acceptance check: a point inside zone A returns zone A; a point outside every zone returns no zone.
2. `R1.AC2` WHILE a zone is outside its operating hours, the system SHALL treat parking there as free.
   - Acceptance check: a session at 21:00 in a zone open 08:00-20:00 costs nothing.
3. `R1.AC3` WHEN a session's price would exceed the zone's daily cap, the system SHALL charge the cap.
   - Acceptance check: a 10-hour session in a zone with a 12 EUR cap costs 12 EUR.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Operating hours come from decision P1 (`docs/decisions/P1-zone-hours.md`).

## Out Of Scope

- Anything not listed above.
