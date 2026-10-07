# Requirements Document

## Introduction

Bill electric-vehicle charging in EV bays.

## Requirements

### R1 EV Charging

**User Story:** As an EV driver, I want to charge while parked, so that I leave with a charged car.

#### Acceptance Criteria

1. `R1.AC1` WHEN a vehicle starts charging in an EV bay, the system SHALL open a charging session billed per kWh.
   - Acceptance check: charging 12 kWh bills 12 kWh at the bay price.
2. `R1.AC2` WHEN a charging session exceeds 4 hours, the system SHALL notify the driver to move the vehicle.
   - Acceptance check: the driver is notified once the charging session reaches 4 hours.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Parking in EV bays is still priced by `session-pricing#R1.AC1`.

## Out Of Scope

- Anything not listed above.
