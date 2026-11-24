---
walden_schema_version: v1alpha1
status: approved
approved_at: 2026-11-24T14:25:06Z
last_modified: 2026-11-24T14:25:06Z
approved_fingerprint: sha256:1390cf9dbfff0cc389a34812bf1c2f761e7647887e24ee1bd910e4b46d1e185f
---

# Requirements Document

## Introduction

A program with release notes.

## Requirements

### R1 Ship the program

**User Story:** As a maintainer, I want the notes published and the status checked, so that a release can be judged.

#### Acceptance Criteria

1. `R1.AC1` WHEN the notes are published, the system SHALL name the release.
   - Acceptance check: the notes hold the word released.
2. `R1.AC2` WHEN the status is read, the system SHALL report green.
   - Acceptance check: the status file holds the word green.

## Non-Functional Requirements

- `NFR1` Determinism: the same input SHALL give the same output. (Bridged by `R1.AC2`.)

## Constraints And Dependencies

- `C1` Standard library only.

## Out Of Scope

- Nothing else.
