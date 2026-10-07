# Requirements Document

## Introduction

Mask plates in the operator console.

## Requirements

### R1 Masked Plates

**User Story:** As a driver, I want operators to see only part of my plate, so that my movements stay private.

#### Acceptance Criteria

1. `R1.AC1` WHEN an operator opens a session, the system SHALL show its plate masked except for the last two characters.
   - Acceptance check: an operator opening the session of plate AB123CD sees *****CD.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Sessions are those shown under `operator-console#R1.AC1`.
- `C2` Masking follows decision P18 (`docs/decisions/P18-masked-plates.md`).

## Out Of Scope

- Anything not listed above.
