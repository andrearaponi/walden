# Requirements Document

## Introduction

Issue receipts for paid sessions.

## Requirements

### R1 Receipts

**User Story:** As a driver, I want a receipt for each paid session, so that I can claim expenses.

#### Acceptance Criteria

1. `R1.AC1` WHEN a charge succeeds, the system SHALL issue a receipt with zone, duration and amount.
   - Acceptance check: a successful 0.70 EUR charge yields one receipt showing zone A, 35 minutes and 0.70 EUR.
2. `R1.AC2` WHEN a driver requests a receipt again, the system SHALL send the same receipt by email.
   - Acceptance check: a second request emails an identical receipt.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Receipts reference charges made under `payments#R1.AC1`.

## Out Of Scope

- Anything not listed above.
