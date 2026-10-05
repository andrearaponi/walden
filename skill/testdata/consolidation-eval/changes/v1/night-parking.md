# Requirements Document

## Introduction

Charge a night tariff in central zones.

## Requirements

### R1 Night Tariff

**User Story:** As the city, I want central zones paid at night, so that residents find space in the evening.

#### Acceptance Criteria

1. `R1.AC1` WHEN a session runs between 20:00 and 08:00 in a central zone, the system SHALL price it at the night tariff.
   - Acceptance check: a session from 21:00 to 22:00 in zone A is priced at the night tariff.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Central zones are the zones of `parking-zones#R1.AC1` marked central.
- `C2` The night tariff follows decision P9 (`docs/decisions/P9-night-tariff.md`).

## Out Of Scope

- Anything not listed above.
