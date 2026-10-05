# Requirements Document

## Introduction

Let residents issue short visitor permits.

## Requirements

### R1 Visitor Permits

**User Story:** As a resident, I want my guests to park near my home, so that visits are easy.

#### Acceptance Criteria

1. `R1.AC1` WHEN a resident issues a visitor permit, the system SHALL let the visitor's plate park free in the resident's home zone for the requested hours.
   - Acceptance check: a 3-hour visitor permit covers the guest plate in zone B for 3 hours.
2. `R1.AC2` IF a resident's visitor permits would exceed 10 hours in a calendar week, THEN the system SHALL refuse the permit.
   - Acceptance check: a request bringing the week to 11 hours is refused.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Only holders of `resident-permits#R1.AC1` permits can issue visitor permits.

## Out Of Scope

- Anything not listed above.
