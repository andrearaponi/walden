---
status: draft
approved_at:
last_modified: 2026-11-10T00:00:00Z
approved_fingerprint:
---

# Requirements Document

## Introduction

A fixture for the recording of the release before proof kinds: a feature that stays verified, with every attribute a proof can carry.

## Requirements

### R1 Steady

**User Story:** As a maintainer, I want a feature whose proofs use every attribute, so that a recording holds the fingerprint of each form.

#### Acceptance Criteria

1. `R1.AC1` WHEN the first step runs, the system SHALL print its marker.
2. `R1.AC2` WHEN the second step runs, the system SHALL exit with the declared status.
3. `R1.AC3` WHEN the last proof runs, the system SHALL succeed.
4. `R1.AC4` WHEN the legacy proof runs, the system SHALL succeed.
