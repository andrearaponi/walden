# Feature Design

## Architecture

One shell script validates its argument and prints the greeting.

## Options Considered

Use shell rather than a compiled service: no deployment or dependency is needed for this local command.

## Simplicity And Elegance Review

One executable and a small assertion script suffice.

## Failure Modes And Tradeoffs

Reject missing or empty names. No network or storage dependency exists.

## Verification Plan

Run the assertion script against a valid name and invalid invocations.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| `R1` | src/greet.sh and tests/test_greet.sh |
