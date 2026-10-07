# Requirements Document

## Introduction

Let residents issue short visitor permits as one-time codes.

## Requirements

### R1 Visitor Permits

**User Story:** As a resident, I want my guests to park near my home, so that visits are easy.

#### Acceptance Criteria

1. `R1.AC1` WHEN a resident issues a visitor permit, the system SHALL send the visitor a one-time parking code valid in the resident's home zone for the requested hours.
   - Acceptance check: a 3-hour permit sends one code valid for 3 hours in the home zone.
2. `R1.AC2` IF a resident's visitor permits would exceed 10 hours in a calendar week, THEN the system SHALL refuse the permit.
   - Acceptance check: a request bringing the week to 11 hours is refused.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Only holders of `resident-permits#R1.AC1` permits can issue visitor permits.

## Out Of Scope

- Anything not listed above.
