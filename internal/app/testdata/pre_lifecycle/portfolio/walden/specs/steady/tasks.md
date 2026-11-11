---
status: draft
approved_at:
last_modified: 2026-11-10T00:00:00Z
approved_fingerprint:
source_design_approved_at:
source_design_fingerprint:
---

# Implementation Plan

- [ ] 1. A top-level leaf with two steps and every attribute
  - Requirements: `R1.AC1`, `R1.AC2`
  - Design: Architecture
  - Verification:
    - command: ["sh", "-c", "echo steady"]
      expect_output: "steady"
      covers: ["R1.AC1"]
    - command: ["sh", "-c", "exit 3"]
      expect_exit: 3
      timeout: 30s
      covers: ["R1.AC2"]
- [ ] 2. A group
  - [ ] 2.1 A child leaf
    - Requirements: `R1.AC3`
    - Design: Architecture; Verification Plan
    - Verification:
      - command: ["sh", "-c", "test 21 -eq 21"]
        covers: ["R1.AC3"]
  - [ ] 2.2 A proof in the legacy single-line format
    - Requirements: `R1.AC4`
    - Design: Architecture
    - Verification: `sh -c true`
