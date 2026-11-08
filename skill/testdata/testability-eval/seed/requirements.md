# Requirements Document

## Introduction

A small lending service for a community library: members borrow copies, receive reminders when a loan is overdue, pay late fees and read their loans on a member page. The service is one Go module with an in-memory store; mail goes out through the library's SMTP server.

## Requirements

### R1 Borrowing

**User Story:** As a member, I want to borrow copies within a fair limit, so that the collection stays available to everyone.

#### Acceptance Criteria

1. `R1.AC1` IF a borrow request names a member the library does not know, THEN the service SHALL reject it with an unknown-member error.
   - Acceptance check: a borrow request for an unknown member id returns the unknown-member error and records no loan.
2. `R1.AC2` The service SHALL keep every member at or below five active loans.
   - Acceptance check: after any sequence of borrow and return requests, no member holds more than five loans that are not returned.

### R2 Due Dates And Reminders

**User Story:** As a librarian, I want overdue loans flagged and their borrowers reminded, so that copies come back.

#### Acceptance Criteria

1. `R2.AC1` WHILE a loan is past its due date and not returned, the service SHALL report it as overdue.
   - Acceptance check: a loan whose due date has passed is reported overdue; the same loan one second before its due date is not.
2. `R2.AC2` WHEN the daily reminder run finds an overdue loan, the service SHALL send one reminder email to its borrower.
   - Acceptance check: after a run with one overdue loan, exactly one reminder addressed to its borrower has been sent.
3. `R2.AC3` IF the mail server rejects a reminder, THEN the service SHALL try the delivery again, up to three attempts in total.
   - Acceptance check: with a mail server that rejects every message, a reminder is attempted exactly three times.

### R3 Late Fees

**User Story:** As a librarian, I want late returns charged, so that members return copies on time.

#### Acceptance Criteria

1. `R3.AC1` WHEN a member returns a loan after its due date, the service SHALL charge the late fee for the days the loan was late.
   - Acceptance check: the fee recorded on a late return equals the late fee for its days late.

### R4 Member Page

**User Story:** As a member, I want to see my loans and which ones are overdue, so that I know what to return first.

#### Acceptance Criteria

1. `R4.AC1` WHERE overdue badges are enabled, the member page SHALL show a red overdue badge next to each overdue loan.
   - Acceptance check: with badges enabled, the page shows a red badge beside every overdue loan and beside no other loan.
2. `R4.AC2` WHEN a member opens the member page, the service SHALL display the page.
   - Acceptance check: the page feels responsive.

### R5 Statements

**User Story:** As a member, I want my monthly statement to list what I still owe, so that I can pay it.

#### Acceptance Criteria

1. `R5.AC1` WHEN the monthly statement runs for a member, the service SHALL list each unpaid fee of that member.
   - Acceptance check: a member with two unpaid fees from different months sees both on the statement, and no paid fee.

## Constraints And Dependencies

- `C1` Go standard library only.
- `C2` Reminders go out through the library's SMTP server at `mail.lending.internal:25`.
