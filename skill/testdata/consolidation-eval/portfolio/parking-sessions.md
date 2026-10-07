# Requirements Document

## Introduction

Start and stop paid parking sessions from the app.

## Requirements

### R1 Sessions

**User Story:** As a driver, I want to start and stop parking from my phone, so that I pay only for the time I use.

#### Acceptance Criteria

1. `R1.AC1` WHEN a driver starts a session with a registered plate in a valid zone, the system SHALL open the session.
   - Acceptance check: a start with plate AB123CD in zone A opens exactly one session.
2. `R1.AC2` WHEN a driver stops a session, the system SHALL close it and record its duration.
   - Acceptance check: stopping after 35 minutes records 35 minutes.
3. `R1.AC3` WHEN a session reaches the maximum duration of its zone, the system SHALL close it automatically.
   - Acceptance check: a session in a zone with a 2-hour maximum closes at 2 hours.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Zones and their hours come from `parking-zones#R1.AC1` and `parking-zones#R1.AC2`.

## Out Of Scope

- Anything not listed above.
