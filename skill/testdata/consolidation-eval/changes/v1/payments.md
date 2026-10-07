# Requirements Document

## Introduction

Charge the driver's payment method when a session closes. Declined charges are no longer retried; unpaid sessions go to the collection agency.

## Requirements

### R1 Charges

**User Story:** As the city, I want closed sessions paid, so that parking revenue is collected.

#### Acceptance Criteria

1. `R1.AC1` WHEN a priced session closes, the system SHALL charge the driver's default payment method.
   - Acceptance check: closing a 0.70 EUR session creates one 0.70 EUR charge.
2. `R1.AC2` IF a charge is declined, THEN the system SHALL mark the session as unpaid.
   - Acceptance check: a declined card leaves the session unpaid and creates no receipt.

## Non-Functional Requirements

- `NFR1` Card data never leaves the payment provider (bridged by `R1.AC1`).

## Constraints And Dependencies

- `C1` Prices come from `session-pricing#R1.AC1`.

## Out Of Scope

- Anything not listed above.
