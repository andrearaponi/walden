# Requirements Document

## Introduction

Send overdue fines to collection.

## Requirements

### R1 Fast Collection

**User Story:** As the city, I want overdue fines collected quickly, so that unpaid fines do not pile up.

#### Acceptance Criteria

1. `R1.AC1` WHEN a fine's payment deadline passes, the system SHALL send the fine to the collection agency even if an appeal is open.
   - Acceptance check: a fine with an open appeal is sent to collection the day after its deadline.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Appeals are those registered under `fine-appeals#R1.AC1`.
- `C2` Collection follows decision P19 (`docs/decisions/P19-fast-collection.md`).

## Out Of Scope

- Anything not listed above.
