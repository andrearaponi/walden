# Requirements Document

## Introduction

Let officers check whether a parked plate is covered and route doubtful cases to the back office.

## Requirements

### R1 Plate Checks

**User Story:** As an officer, I want to check a plate on the street, so that I fine only uncovered vehicles.

#### Acceptance Criteria

1. `R1.AC1` WHEN an officer checks a plate, the system SHALL report whether a paid session, a permit or a blue badge covers it.
   - Acceptance check: a plate with an open session is reported covered by that session.
2. `R1.AC2` IF no session, permit or badge covers the plate, THEN the system SHALL send the plate to back-office review.
   - Acceptance check: a plate with nothing active appears in the back-office review queue.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Coverage combines `parking-sessions#R1.AC1`, `resident-permits#R1.AC2`, `visitor-permits#R1.AC1` and `accessible-parking#R1.AC1`.

## Out Of Scope

- Anything not listed above.
