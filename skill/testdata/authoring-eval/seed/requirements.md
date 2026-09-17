# Requirements Document

## Introduction

A local single-user POSIX shell greeting command.

## Requirements

### R1 Greeting

**User Story:** As a user, I want a greeting by name, so that I know my input was accepted.

#### Acceptance Criteria

1. `R1.AC1` WHEN the user supplies one non-empty name, the system SHALL print `Hello, <name>!`.
2. `R1.AC2` IF a single non-empty name is not supplied, THEN the system SHALL reject the invocation with the error `a name is required` and a non-zero exit.

## Constraints And Dependencies

- `C1` POSIX shell and standard local utilities only.
