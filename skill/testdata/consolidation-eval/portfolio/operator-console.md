# Requirements Document

## Introduction

Give operators a console for sessions and refunds.

## Requirements

### R1 Console

**User Story:** As an operator, I want to inspect sessions and handle refunds, so that I can help drivers.

#### Acceptance Criteria

1. `R1.AC1` WHEN an operator opens a session, the system SHALL show its plate, zone, duration and price.
   - Acceptance check: opening a session shows its four fields.
2. `R1.AC2` WHEN an operator approves a refund request, the system SHALL record the approval in the audit log.
   - Acceptance check: approving a refund adds one audit entry for that approval.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Refunds follow `refunds#R1.AC2`; approvals are recorded under `audit-log#R1.AC1`.

## Out Of Scope

- Anything not listed above.
