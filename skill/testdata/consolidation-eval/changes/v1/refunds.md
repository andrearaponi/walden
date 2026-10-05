# Requirements Document

## Introduction

Refund mistaken sessions and record operator approvals.

## Requirements

### R1 Refunds

**User Story:** As a driver, I want mistaken sessions refunded, so that I do not pay for parking I did not use.

#### Acceptance Criteria

1. `R1.AC1` WHEN a driver stops a session within 2 minutes of starting it, the system SHALL refund the charge in full.
   - Acceptance check: a session stopped after 90 seconds is refunded in full.
2. `R1.AC2` WHEN an operator approves a refund request, the system SHALL refund the requested amount.
   - Acceptance check: an approved request for 3 EUR refunds 3 EUR.
3. `R1.AC3` WHEN an operator approves a refund request, the system SHALL record the approval in the audit log.
   - Acceptance check: approving a refund adds one audit entry for that approval.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Refunds reverse charges made under `payments#R1.AC1`.

## Out Of Scope

- Anything not listed above.
