# Requirements Document

## Introduction

Let drivers appeal fines.

## Requirements

### R1 Appeals

**User Story:** As a driver, I want to contest a fine, so that mistakes can be corrected.

#### Acceptance Criteria

1. `R1.AC1` WHEN a driver appeals a fine within 28 days, the system SHALL register the appeal.
   - Acceptance check: an appeal on day 20 is registered; one on day 29 is refused.
2. `R1.AC2` WHILE an appeal is open, the system SHALL suspend the fine's payment deadline.
   - Acceptance check: a fine under appeal is not overdue when its deadline passes.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Deadlines are those set by `fines#R1.AC2`.

## Out Of Scope

- Anything not listed above.
