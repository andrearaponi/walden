# Requirements Document

## Introduction

Turn EV bays over faster.

## Requirements

### R1 Fast Turnover

**User Story:** As the city, I want EV bays freed sooner, so that more vehicles can charge.

#### Acceptance Criteria

1. `R1.AC1` WHEN a charging session exceeds 2 hours, the system SHALL close the parking session in the EV bay.
   - Acceptance check: a charging session closes the parking session at 2 hours.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Charging sessions are those of `ev-charging#R1.AC1`.

## Out Of Scope

- Anything not listed above.
