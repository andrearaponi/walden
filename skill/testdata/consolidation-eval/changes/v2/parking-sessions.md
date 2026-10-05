# Requirements Document

## Introduction

Start and stop paid parking sessions from the app. Sessions now run until the driver stops them; the daily cap bounds their price.

## Requirements

### R1 Sessions

**User Story:** As a driver, I want to start and stop parking from my phone, so that I pay only for the time I use.

#### Acceptance Criteria

1. `R1.AC1` WHEN a driver starts a session with a registered plate in a valid zone, the system SHALL open the session.
   - Acceptance check: a start with plate AB123CD in zone A opens exactly one session.
2. `R1.AC2` WHEN a driver stops a session, the system SHALL close it and record its duration.
   - Acceptance check: stopping after 35 minutes records 35 minutes.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Zones and their hours come from `parking-zones#R1.AC1` and `parking-zones#R1.AC2`.
- `C2` Open-ended sessions follow decision P8 (`docs/decisions/P8-open-ended-sessions.md`).

## Out Of Scope

- Anything not listed above.
