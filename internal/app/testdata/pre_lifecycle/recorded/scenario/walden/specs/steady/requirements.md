---
walden_schema_version: v1alpha1
status: approved
approved_at: 2026-11-10T18:56:13Z
last_modified: 2026-11-10T18:56:13Z
approved_fingerprint: sha256:f8b7540e46ef9e71c8c0aa0de7f51ed30c5241e1e95ce6e3a6d6c00445d8bf91
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
