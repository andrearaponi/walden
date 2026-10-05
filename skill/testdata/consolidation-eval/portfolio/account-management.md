# Requirements Document

## Introduction

Manage driver accounts, plates and payment methods.

## Requirements

### R1 Accounts

**User Story:** As a driver, I want one account for my cars and cards, so that parking is quick.

#### Acceptance Criteria

1. `R1.AC1` WHEN a driver registers a plate, the system SHALL add it to the driver's account.
   - Acceptance check: registering AB123CD lists it on the account.
2. `R1.AC2` IF an account would hold more than 3 plates, THEN the system SHALL refuse the plate.
   - Acceptance check: a fourth plate is refused.
3. `R1.AC3` WHEN a driver adds a payment method, the system SHALL store only the provider token.
   - Acceptance check: adding a card stores a token and no card number.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- None.

## Out Of Scope

- Anything not listed above.
