# Requirements Document

## Introduction

Charge the driver's payment method once a month for the month's sessions.

## Requirements

### R1 Charges

**User Story:** As the city, I want closed sessions paid, so that parking revenue is collected.

#### Acceptance Criteria

1. `R1.AC1` WHEN a calendar month ends, the system SHALL charge the driver's default payment method once for all priced sessions closed in that month.
   - Acceptance check: two 0.70 EUR sessions in March create one 1.40 EUR charge on 1 April.
2. `R1.AC2` IF a charge is declined, THEN the system SHALL mark the session as unpaid.
   - Acceptance check: a declined card leaves the session unpaid and creates no receipt.
3. `R1.AC3` WHILE a session is unpaid, the system SHALL retry the charge once every 24 hours for 3 days.
   - Acceptance check: a declined charge is retried three times, one day apart.

## Non-Functional Requirements

- `NFR1` Card data never leaves the payment provider (bridged by `R1.AC1`).

## Constraints And Dependencies

- `C1` Prices come from `session-pricing#R1.AC1`.

## Out Of Scope

- Anything not listed above.
