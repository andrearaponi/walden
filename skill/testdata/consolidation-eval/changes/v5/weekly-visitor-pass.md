# Requirements Document

## Introduction

Sell weekly visitor passes to residents.

## Requirements

### R1 Weekly Passes

**User Story:** As a resident, I want a pass for a guest staying all week, so that I do not renew permits every day.

#### Acceptance Criteria

1. `R1.AC1` WHEN a resident buys a weekly visitor pass, the system SHALL let one visitor plate park free in the resident's home zone for seven consecutive days.
   - Acceptance check: a pass bought on Monday covers the visitor plate until Sunday.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Passes are bought by residents who can issue `visitor-permits#R1.AC1` permits.
- `C2` Prices follow decision P12 (`docs/decisions/P12-weekly-visitor-pass.md`).

## Out Of Scope

- Anything not listed above.
