# Requirements Document

## Introduction

Offer a read-only API for zone information and location lookups.

## Requirements

### R1 Public API

**User Story:** As an app developer, I want zone data, so that I can show tariffs to drivers.

#### Acceptance Criteria

1. `R1.AC1` WHEN a client requests a zone, the system SHALL return its hours and tariff.
   - Acceptance check: a request for zone A returns its hours and per-minute tariff.
2. `R1.AC2` IF a client exceeds 60 requests per minute, THEN the system SHALL reject further requests for that minute.
   - Acceptance check: the 61st request in one minute is rejected.
3. `R1.AC3` WHEN a location is queried, the system SHALL return the zone that contains it.
   - Acceptance check: a point inside zone A returns zone A.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Zone data comes from `parking-zones#R1.AC1` and `session-pricing#R1.AC1`.

## Out Of Scope

- Anything not listed above.
