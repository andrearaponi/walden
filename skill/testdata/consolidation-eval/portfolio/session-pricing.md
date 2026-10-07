# Requirements Document

## Introduction

Price parking sessions per minute and zone.

## Requirements

### R1 Pricing

**User Story:** As the city, I want sessions priced consistently, so that drivers pay the published tariff.

#### Acceptance Criteria

1. `R1.AC1` WHEN a session closes, the system SHALL price it per started minute at the zone tariff.
   - Acceptance check: 35 minutes at 0.02 EUR per minute cost 0.70 EUR.
2. `R1.AC2` WHEN a session's price would exceed the zone's daily cap, the system SHALL charge the cap.
   - Acceptance check: a 10-hour session in a zone with a 12 EUR cap costs 12 EUR.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Tariffs and caps come from decision P2 (`docs/decisions/P2-tariffs.md`).
- `C2` Free periods follow `parking-zones#R1.AC2`.

## Out Of Scope

- Anything not listed above.
