# Requirements Document

## Introduction

Honour drivers' erasure requests.

## Requirements

### R1 Erasure Requests

**User Story:** As a driver, I want my personal data erased on request, so that the service keeps nothing about me it does not need.

#### Acceptance Criteria

1. `R1.AC1` WHEN a driver requests erasure, the system SHALL remove the driver's plates from audit entries within 30 days.
   - Acceptance check: 30 days after an erasure request, no audit entry shows the driver's plates.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Audit entries are those appended under `audit-log#R1.AC1`.
- `C2` The procedure follows decision P16 (`docs/decisions/P16-erasure-requests.md`).

## Out Of Scope

- Anything not listed above.
