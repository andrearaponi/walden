# Requirements Document

## Introduction

Price parking during city events.

## Requirements

### R1 Event Pricing

**User Story:** As the city, I want higher parking prices during events, so that visitors use public transport.

#### Acceptance Criteria

1. `R1.AC1` WHILE a city event is active in a zone, the system SHALL price sessions there at double the zone tariff without a daily cap.
   - Acceptance check: a 10-hour session during an event in a zone with a 12 EUR cap costs twice the tariff for 10 hours, more than 12 EUR.

## Non-Functional Requirements

- None specific to this feature.

## Constraints And Dependencies

- `C1` Event sessions are priced from the tariffs of `session-pricing#R1.AC1`.
- `C2` Events and their zones follow decision P17 (`docs/decisions/P17-event-pricing.md`).

## Out Of Scope

- Anything not listed above.
