# Requirements Document

## Introduction

Keep receipts in the app only.

## Requirements

### R1 Paperless Receipts

**User Story:** As the city, I want to stop emailing receipts, so that personal data leaves the service less often.

#### Acceptance Criteria

1. `R1.AC1` WHEN a charge succeeds, the system SHALL make its receipt available only in the app, never by email.
   - Acceptance check: a paid session shows its receipt in the app and no email is sent.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Receipts are those issued under `receipts#R1.AC1`.
- `C2` The policy follows decision P13 (`docs/decisions/P13-paperless-receipts.md`).

## Out Of Scope

- Anything not listed above.
