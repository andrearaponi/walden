package validation

import (
	"fmt"
	"strings"
	"testing"
)

// failureSignalRequirements is a draft requirements document whose criteria
// are the given texts, numbered R1.AC1, R1.AC2 and so on.
func failureSignalRequirements(criteria ...string) string {
	var list strings.Builder
	for i, criterion := range criteria {
		fmt.Fprintf(&list, "%d. __BT__R1.AC%d__BT__ %s\n", i+1, i+1, criterion)
	}
	return `---
status: draft
approved_at:
last_modified: 2026-10-10T16:00:00Z
---

# Requirements Document

## Requirements

### R1 Feature

#### Acceptance Criteria

` + list.String() + `
## Out Of Scope

- None.
`
}

func hasMissingFailureWarning(warnings []string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, "no unwanted-behavior") {
			return true
		}
	}
	return false
}

// TestMissingFailureModeSignalCountsComplexUnwantedClause pins R5.AC1: an
// unwanted-behavior clause counts as failure handling whatever the form of
// its criterion, and a document without any keeps the warning.
func TestMissingFailureModeSignalCountsComplexUnwantedClause(t *testing.T) {
	root := t.TempDir()
	writeFeatureFile(t, root, "complex-failure", "requirements.md", failureSignalRequirements(
		"WHEN the user saves, the system SHALL store the draft",
		"WHILE a turn is running, IF the container exits, THEN the system SHALL report the exit",
	))
	writeFeatureFile(t, root, "no-failure", "requirements.md", failureSignalRequirements(
		"WHEN the user saves, the system SHALL store the draft",
		"WHILE a sync runs, the system SHALL show a progress bar",
	))

	complexFailure, err := ValidateFeature(root, "complex-failure")
	if err != nil {
		t.Fatalf("validate complex-failure: %v", err)
	}
	if !complexFailure.Valid {
		t.Fatalf("complex-failure: want valid, got %s", complexFailure.Message)
	}
	if form := complexFailure.EARSResults[1].Form; form != "complex" {
		t.Fatalf("complex-failure: want R1.AC2 complex, got %q", form)
	}
	if hasMissingFailureWarning(complexFailure.Warnings) {
		t.Fatalf("complex-failure: want no missing-failure warning, got %v", complexFailure.Warnings)
	}

	noFailure, err := ValidateFeature(root, "no-failure")
	if err != nil {
		t.Fatalf("validate no-failure: %v", err)
	}
	if !noFailure.Valid {
		t.Fatalf("no-failure: want valid, got %s", noFailure.Message)
	}
	if !hasMissingFailureWarning(noFailure.Warnings) {
		t.Fatalf("no-failure: want the missing-failure warning, got %v", noFailure.Warnings)
	}
}
