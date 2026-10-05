# Requirements Document

## Introduction

Charge residents a small daily fee in their home zone.

## Requirements

### R1 Resident Surcharge

**User Story:** As the city, I want residents to contribute to street maintenance, so that costs are shared.

#### Acceptance Criteria

1. `R1.AC1` WHEN a resident permit holder parks in the home zone, the system SHALL charge 1 EUR per day.
   - Acceptance check: a permit plate parked in zone B on Monday is charged 1 EUR.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Resident permits are those of `resident-permits#R1.AC1`.
- `C2` The fee follows decision P10 (`docs/decisions/P10-resident-surcharge.md`).

## Out Of Scope

- Anything not listed above.
