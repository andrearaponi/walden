package app

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

const invalidTasksForPortfolio = `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build feature
  - [ ] 1.1 Add implementation
    - Requirements: __BT__NFR1__BT__
    - Design: Todo flow
    - Verification: __BT__go test ./...__BT__
`

func chdirForPortfolio(t *testing.T, root string) {
	t.Helper()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory lookup to succeed, got %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("expected chdir to succeed, got %v", err)
	}
}

// TestRunValidatePortfolio proves R3: `walden validate` without a feature
// name validates every feature under .walden/specs/.
func TestRunValidatePortfolio(t *testing.T) {
	t.Run("AllValidExitsZero", func(t *testing.T) {
		root := t.TempDir()
		writeValidValidateFeature(t, root, "aaa-feature")
		writeValidValidateFeature(t, root, "bbb-feature")
		chdirForPortfolio(t, root)

		var stdout, stderr bytes.Buffer
		exitCode := Run([]string{"validate"}, &stdout, &stderr)

		if exitCode != 0 {
			t.Fatalf("expected portfolio validate to succeed, got %d, stderr %q", exitCode, stderr.String())
		}
		text := stdout.String()
		if !strings.Contains(text, "aaa-feature: VALID") {
			t.Fatalf("expected aaa-feature verdict, got %q", text)
		}
		if !strings.Contains(text, "bbb-feature: VALID") {
			t.Fatalf("expected bbb-feature verdict, got %q", text)
		}
		if strings.Index(text, "aaa-feature") > strings.Index(text, "bbb-feature") {
			t.Fatalf("expected sorted feature order, got %q", text)
		}
	})

	t.Run("OneInvalidExitsOne", func(t *testing.T) {
		root := t.TempDir()
		writeValidValidateFeature(t, root, "aaa-feature")
		writeValidateFeatureFile(t, root, "bbb-feature", "requirements.md", validRequirementsForValidateCommand)
		writeValidateFeatureFile(t, root, "bbb-feature", "design.md", validDesignForValidateCommand)
		writeValidateFeatureFile(t, root, "bbb-feature", "tasks.md", invalidTasksForPortfolio)
		chdirForPortfolio(t, root)

		var stdout, stderr bytes.Buffer
		exitCode := Run([]string{"validate"}, &stdout, &stderr)

		if exitCode != 1 {
			t.Fatalf("expected portfolio validate to fail, got %d", exitCode)
		}
		combined := stdout.String() + stderr.String()
		if !strings.Contains(combined, "aaa-feature: VALID") {
			t.Fatalf("expected valid verdict for aaa-feature, got %q", combined)
		}
		if !strings.Contains(combined, "bbb-feature: INVALID") {
			t.Fatalf("expected invalid verdict for bbb-feature, got %q", combined)
		}
	})

	t.Run("JSONReportsPerFeatureVerdicts", func(t *testing.T) {
		root := t.TempDir()
		writeValidValidateFeature(t, root, "aaa-feature")
		writeValidateFeatureFile(t, root, "bbb-feature", "requirements.md", validRequirementsForValidateCommand)
		writeValidateFeatureFile(t, root, "bbb-feature", "design.md", validDesignForValidateCommand)
		writeValidateFeatureFile(t, root, "bbb-feature", "tasks.md", invalidTasksForPortfolio)
		chdirForPortfolio(t, root)

		var stdout, stderr bytes.Buffer
		exitCode := Run([]string{"validate", "--json"}, &stdout, &stderr)

		if exitCode != 1 {
			t.Fatalf("expected portfolio validate to fail, got %d", exitCode)
		}
		var envelope output.Envelope
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatalf("expected valid json, got %v: %s", err, stdout.String())
		}
		features := envelope.Result.Features
		if len(features) != 2 {
			t.Fatalf("expected 2 feature verdicts, got %d", len(features))
		}
		if features[0].Feature != "aaa-feature" || !features[0].Valid {
			t.Fatalf("unexpected first verdict: %+v", features[0])
		}
		if features[1].Feature != "bbb-feature" || features[1].Valid {
			t.Fatalf("unexpected second verdict: %+v", features[1])
		}
		if envelope.Result.ExitCode != 1 {
			t.Fatalf("expected exit_code 1 in envelope, got %d", envelope.Result.ExitCode)
		}
	})

	t.Run("EmptyRepoNamesFeatureInit", func(t *testing.T) {
		root := t.TempDir()
		chdirForPortfolio(t, root)

		var stdout, stderr bytes.Buffer
		exitCode := Run([]string{"validate"}, &stdout, &stderr)

		if exitCode == 0 {
			t.Fatal("expected portfolio validate on empty repo to fail")
		}
		combined := stdout.String() + stderr.String()
		if !strings.Contains(combined, "feature init") {
			t.Fatalf("expected error naming feature init, got %q", combined)
		}
	})

	t.Run("AllFlagAppliesFullSpecScope", func(t *testing.T) {
		root := t.TempDir()
		// Current-phase validation passes (draft tasks are skipped);
		// full-spec validation must fail on the invalid tasks document.
		writeValidateFeatureFile(t, root, "aaa-feature", "requirements.md", validRequirementsForValidateCommand)
		writeValidateFeatureFile(t, root, "aaa-feature", "design.md", validDraftDesignForValidateCommand)
		writeValidateFeatureFile(t, root, "aaa-feature", "tasks.md", invalidDraftTasksForValidateCommand)
		chdirForPortfolio(t, root)

		var stdout, stderr bytes.Buffer
		if exitCode := Run([]string{"validate"}, &stdout, &stderr); exitCode != 0 {
			t.Fatalf("expected current-phase portfolio to pass, got %d: %s%s", exitCode, stdout.String(), stderr.String())
		}

		stdout.Reset()
		stderr.Reset()
		if exitCode := Run([]string{"validate", "--all"}, &stdout, &stderr); exitCode != 1 {
			t.Fatalf("expected full-spec portfolio to fail, got %d: %s%s", exitCode, stdout.String(), stderr.String())
		}
		combined := stdout.String() + stderr.String()
		if !strings.Contains(combined, "aaa-feature: INVALID") {
			t.Fatalf("expected full-spec invalid verdict, got %q", combined)
		}
	})
}
