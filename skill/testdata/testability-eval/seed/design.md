# Feature Design

## Architecture

The lending service stays one Go package, `loans`, with an in-memory store. This feature works on six existing files and adds no package:

- `loans/service.go`: `Service.Borrow` rejects unknown members with `ErrUnknownMember` and enforces `MaxActiveLoans`; `Service.Return` records the return instant it is given and charges `lateFee` from `loans/fees.go` when the loan is late (`R1`, `R3`).
- `loans/overdue.go`: `IsOverdue` decides whether a loan is past its due date; `Service.OverdueLoans` lists those loans (`R2.AC1`).
- `loans/reminders.go`: `Service.SendReminders` is the daily reminder run. It emails the borrower of each overdue loan through the library's SMTP server and retries a rejected delivery (`R2.AC2`, `R2.AC3`).
- `loans/web.go`: `Service.MemberPage` renders the member page, with the overdue badge when `Features.OverdueBadges` is set (`R4`).
- `loans/statement.go`: `Service.MonthlyStatement` changes. Today it lists the loans borrowed in the given month with their fees; it will list every unpaid fee of the member, whatever the month of the loan, under the heading "Unpaid fees". `Loan` gains a `FeePaid` field in `loans/service.go` (`R5.AC1`).

## Options Considered

- Chosen: extend the existing package in place; the service is small and has one deployment.
- Alternative: a separate statements package reading the store through an interface; rejected because the statement needs only the member's loans, which the service already holds.

## Simplicity And Elegance Review

No new package and no new dependency. The statement change reuses the loan list the service keeps, and the reminder run keeps its current shape.

## Failure Modes And Tradeoffs

- The SMTP server may be down during the daily run: a rejected delivery is retried up to three times, then reported in the run's error.
- A member page request for an unknown member answers 404.

## Verification Plan

- Unit tests in `loans/` for borrowing, overdue detection and the late fee.
- A manual check of the member page in a browser.
- A run of the reminder job against the staging mail server.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| `R1` | `loans/service.go` `Service.Borrow`; unit tests |
| `R2` | `loans/overdue.go` `IsOverdue`; `loans/reminders.go` `Service.SendReminders`; unit tests and a staging run |
| `R3` | `loans/service.go` `Service.Return`; `loans/fees.go` `lateFee`; unit tests |
| `R4` | `loans/web.go` `Service.MemberPage`; manual browser check |
| `R5` | `loans/statement.go` `Service.MonthlyStatement`; unit tests |
