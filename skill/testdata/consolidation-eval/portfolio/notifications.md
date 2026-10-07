# Requirements Document

## Introduction

Notify drivers about sessions, charges and fines.

## Requirements

### R1 Notifications

**User Story:** As a driver, I want timely notices, so that I avoid fines and unpaid sessions.

#### Acceptance Criteria

1. `R1.AC1` WHEN a session is 10 minutes from its zone's maximum duration, the system SHALL notify the driver.
   - Acceptance check: a session in a 2-hour zone triggers a notice at 1 hour 50 minutes.
2. `R1.AC2` WHEN a charge retry fails, the system SHALL notify the driver that the session is still unpaid.
   - Acceptance check: each failed retry produces one unpaid notice.
3. `R1.AC3` WHEN a fine is issued, the system SHALL notify the plate's account holder.
   - Acceptance check: issuing a fine sends one notice to the account holder.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Sessions come from `parking-sessions#R1.AC1`, declined charges from `payments#R1.AC2` and fines from `fines#R1.AC1`.

## Out Of Scope

- Anything not listed above.
