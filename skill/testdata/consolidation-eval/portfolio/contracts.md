# Current Contracts

## accessible-parking

Purpose: Free parking for blue-badge holders.
Active: R1.AC1, C1
Sources: docs/decisions/P3-accessibility.md
Related: enforcement-checks

## account-management

Purpose: Manage driver accounts, plates and payment methods.
Active: R1.AC1, R1.AC2, R1.AC3
Reserved: R1.AC4 (removed: shared family accounts)
Related: privacy-retention, payments

## audit-log

Purpose: Record operator actions.
Active: R1.AC1, R1.AC2, C1
Sources: docs/decisions/P5-audit-retention.md
Related: operator-console, privacy-retention

## enforcement-checks

Purpose: Let officers check whether a parked plate is covered.
Active: R1.AC1, R1.AC2, C1
Related: fines

## ev-charging

Purpose: Bill electric-vehicle charging in EV bays.
Active: R1.AC1, R1.AC2, C1
Reserved: R1.AC3 (removed: reserved charging bays)
Related: session-pricing

## fine-appeals

Purpose: Let drivers appeal fines.
Active: R1.AC1, R1.AC2, C1
Related: fines

## fines

Purpose: Fine uncovered vehicles.
Active: R1.AC1, R1.AC2, C1, C2
Reserved: R1.AC3 (removed: fine instalments)
Sources: docs/decisions/P4-fines.md
Related: enforcement-checks, fine-appeals, notifications

## notifications

Purpose: Notify drivers about sessions, charges and fines.
Active: R1.AC1, R1.AC2, R1.AC3, C1
Reserved: R1.AC4 (removed: SMS reminders)
Related: parking-sessions, payments, fines

## operator-console

Purpose: Give operators a console for sessions and refunds.
Active: R1.AC1, R1.AC2, C1
Related: refunds, audit-log

## parking-sessions

Purpose: Start and stop paid parking sessions from the app.
Active: R1.AC1, R1.AC2, R1.AC3, C1
Related: parking-zones, notifications

## parking-zones

Purpose: Define the city's parking zones with their operating hours and tariffs.
Active: R1.AC1, R1.AC2, C1
Reserved: R1.AC3 (removed: zone colours)
Sources: docs/decisions/P1-zone-hours.md
Related: session-pricing, public-api

## payments

Purpose: Charge the driver's payment method when a session closes.
Active: R1.AC1, R1.AC2, R1.AC3, NFR1, C1
Related: session-pricing, receipts, refunds, notifications

## privacy-retention

Purpose: Limit how long personal data is kept.
Active: R1.AC1, R1.AC2, C1, C2
Reserved: R1.AC3 (removed: anonymised exports)
Related: account-management, audit-log, reporting

## public-api

Purpose: Offer a read-only API for zone information.
Active: R1.AC1, R1.AC2, C1
Reserved: R1.AC3 (removed: zone availability endpoint)
Related: parking-zones, session-pricing

## receipts

Purpose: Issue receipts for paid sessions.
Active: R1.AC1, R1.AC2, C1
Reserved: R1.AC3 (removed: paper receipts)
Related: payments

## refunds

Purpose: Refund mistaken sessions.
Active: R1.AC1, R1.AC2, C1
Reserved: R1.AC3 (removed: automatic refunds after app outages)
Related: payments, operator-console

## reporting

Purpose: Publish monthly occupancy reports.
Active: R1.AC1, C1
Reserved: R1.AC2 (removed: daily hourly reports)
Related: privacy-retention

## resident-permits

Purpose: Issue parking permits to residents.
Active: R1.AC1, R1.AC2, C1
Reserved: R1.AC3 (removed: second-car permits)
Related: visitor-permits, enforcement-checks

## session-pricing

Purpose: Price parking sessions per minute and zone.
Active: R1.AC1, R1.AC2, C1, C2
Reserved: R1.AC3 (removed: weekend surcharge)
Sources: docs/decisions/P2-tariffs.md
Related: parking-zones, payments

## visitor-permits

Purpose: Let residents issue short visitor permits.
Active: R1.AC1, R1.AC2, C1
Related: resident-permits
