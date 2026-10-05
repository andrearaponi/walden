# Requirements Document

## Introduction

Charge a flat fee per EV charging session.

## Requirements

### R1 Flat Fee

**User Story:** As an EV driver, I want a predictable charging price, so that I know the cost in advance.

#### Acceptance Criteria

1. `R1.AC1` WHEN a vehicle finishes charging in an EV bay, the system SHALL charge a flat 5 EUR for the charging session.
   - Acceptance check: a 12 kWh charging session costs 5 EUR.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Charging sessions are those of `ev-charging#R1.AC1`.
- `C2` The flat fee follows decision P11 (`docs/decisions/P11-ev-flat-fee.md`).

## Out Of Scope

- Anything not listed above.
