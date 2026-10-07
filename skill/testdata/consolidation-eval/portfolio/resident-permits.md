# Requirements Document

## Introduction

Issue parking permits to residents.

## Requirements

### R1 Resident Permits

**User Story:** As a resident, I want to park near home, so that I do not pay for daily parking.

#### Acceptance Criteria

1. `R1.AC1` WHEN a verified resident applies, the system SHALL issue a permit for the resident's home zone.
   - Acceptance check: a verified resident of zone B receives a zone B permit.
2. `R1.AC2` WHILE a resident permit is valid, the system SHALL let its plate park free in the home zone.
   - Acceptance check: a permit plate parked in zone B for 5 hours pays nothing.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Residency is verified by the city registry; this service stores only the permit.

## Out Of Scope

- Anything not listed above.
