# Requirements Document

## Introduction

Register residents' plates for free parking in their home zone.

## Requirements

### R1 Resident Permits

**User Story:** As a resident, I want to park near home, so that I do not pay for daily parking.

#### Acceptance Criteria

1. `R1.AC1` WHEN a verified resident applies, the system SHALL register one plate of the resident for free parking in the home zone.
   - Acceptance check: a verified resident of zone B has one plate registered for zone B.
2. `R1.AC2` WHILE a resident's plate is registered, the system SHALL let it park free in the home zone.
   - Acceptance check: a registered plate parked in zone B for 5 hours pays nothing.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Residency is verified by the city registry; this service stores only the registration.

## Out Of Scope

- Anything not listed above.
