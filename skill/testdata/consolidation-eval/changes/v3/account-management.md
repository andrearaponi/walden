# Requirements Document

## Introduction

Manage driver accounts and plates. Payment methods are no longer stored: drivers enter them for each session.

## Requirements

### R1 Accounts

**User Story:** As a driver, I want one account for my cars, so that parking is quick.

#### Acceptance Criteria

1. `R1.AC1` WHEN a driver registers a plate, the system SHALL add it to the driver's account.
   - Acceptance check: registering AB123CD lists it on the account.
2. `R1.AC2` IF an account would hold more than 3 plates, THEN the system SHALL refuse the plate.
   - Acceptance check: a fourth plate is refused.
3. `R1.AC4` WHEN a driver enters a payment method for a session, the system SHALL use it only for that session.
   - Acceptance check: the next session asks for a payment method again.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- None.

## Out Of Scope

- Anything not listed above.
