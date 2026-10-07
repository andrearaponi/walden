# Requirements Document

## Introduction

Let officers check whether a parked plate is covered by a session or a permit.

## Requirements

### R1 Plate Checks

**User Story:** As an officer, I want to check a plate on the street, so that I fine only uncovered vehicles.

#### Acceptance Criteria

1. `R1.AC1` WHEN an officer checks a plate, the system SHALL report whether a paid session or a permit covers it.
   - Acceptance check: a plate with an open session is reported covered by that session.
2. `R1.AC2` IF no session or permit covers the plate, THEN the system SHALL report it as uncovered.
   - Acceptance check: a plate with nothing active is reported uncovered.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Coverage combines `parking-sessions#R1.AC1`, `resident-permits#R1.AC2`, `visitor-permits#R1.AC1` and `accessible-parking#R1.AC1`.

## Out Of Scope

- Anything not listed above.
