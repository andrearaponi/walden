---
walden_schema_version: v1alpha1
status: approved
approved_at: 2026-11-10T18:56:12Z
last_modified: 2026-11-10T18:56:13Z
approved_fingerprint: sha256:8be86cc837cf980ee878663b1d0f597f9a5c4c212d61b60960f58670eca5f1ae
source_design_approved_at: 2026-11-10T18:56:12Z
source_design_fingerprint: sha256:a192fdfadc01f2ef3fb00c06d8889f7af536498fdeefded622b28ee4ec300df2
---

# Implementation Plan

- [x] 1. A top-level leaf with two steps and every attribute
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
- [x] 2. A group
  - [x] 2.1 A child leaf
    - Requirements: `R1.AC3`
    - Design: Architecture; Verification Plan
    - Verification:
      - command: ["sh", "-c", "test 21 -eq 21"]
        covers: ["R1.AC3"]
  - [x] 2.2 A proof in the legacy single-line format
    - Requirements: `R1.AC4`
    - Design: Architecture
    - Verification: `sh -c true`
