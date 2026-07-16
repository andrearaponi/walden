package adopt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

func writeDoc(t *testing.T, root, feature, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".walden", "specs", feature)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func preFingerprintDocs(t *testing.T, root, feature string) {
	t.Helper()
	writeDoc(t, root, feature, "requirements.md", `---
status: approved
approved_at: 2026-06-09T14:00:00Z
last_modified: 2026-06-09T14:00:00Z
---

# Requirements Document
`)
	writeDoc(t, root, feature, "design.md", `---
status: approved
approved_at: 2026-06-09T14:10:00Z
last_modified: 2026-06-09T14:10:00Z
source_requirements_approved_at: 2026-06-09T14:00:00Z
---

# Feature Design
`)
	writeDoc(t, root, feature, "tasks.md", `---
status: approved
approved_at: 2026-06-09T14:20:00Z
last_modified: 2026-06-09T14:20:00Z
source_design_approved_at: 2026-06-09T14:10:00Z
---

# Implementation Plan

- [x] 1. Build it
  - [x] 1.1 Implement it
    - Requirements: `+"`R1`"+`
    - Design: Overview
    - Verification:
      - command: ["sh", "-c", "true"]
`)
}

func sealedDocs(t *testing.T, root, feature string, completed bool) {
	t.Helper()
	marker := " "
	if completed {
		marker = "x"
	}
	reqBody := "# Requirements Document\n"
	designBody := "# Feature Design\n"
	tasksBody := fmt.Sprintf(`# Implementation Plan

- [%s] 1. Build it
  - [%s] 1.1 Implement it
    - Requirements: `+"`R1`"+`
    - Design: Overview
    - Verification:
      - command: ["sh", "-c", "true"]
`, marker, marker)

	reqFP := spec.Fingerprint("requirements.md", reqBody)
	designFP := spec.Fingerprint("design.md", designBody)
	writeDoc(t, root, feature, "requirements.md", fmt.Sprintf("---\nstatus: approved\napproved_at: 2026-07-01T10:00:00Z\nlast_modified: 2026-07-01T10:00:00Z\napproved_fingerprint: %s\n---\n\n%s", reqFP, reqBody))
	writeDoc(t, root, feature, "design.md", fmt.Sprintf("---\nstatus: approved\napproved_at: 2026-07-01T10:10:00Z\nlast_modified: 2026-07-01T10:10:00Z\napproved_fingerprint: %s\nsource_requirements_approved_at: 2026-07-01T10:00:00Z\nsource_requirements_fingerprint: %s\n---\n\n%s", designFP, reqFP, designBody))
	writeDoc(t, root, feature, "tasks.md", fmt.Sprintf("---\nstatus: approved\napproved_at: 2026-07-01T10:20:00Z\nlast_modified: 2026-07-01T10:20:00Z\napproved_fingerprint: %s\nsource_design_approved_at: 2026-07-01T10:10:00Z\nsource_design_fingerprint: %s\n---\n\n%s", spec.Fingerprint("tasks.md", tasksBody), designFP, tasksBody))
}

func TestAdoptClassifier(t *testing.T) {
	root := t.TempDir()
	preFingerprintDocs(t, root, "old-era")
	sealedDocs(t, root, "needs-proofs", true)
	sealedDocs(t, root, "all-done", false)
	// Drifted: sealed, then the body changed.
	sealedDocs(t, root, "drifted", true)
	path := filepath.Join(root, ".walden", "specs", "drifted", "requirements.md")
	content, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(content), "# Requirements Document", "# Requirements Document\n\nEdited after approval.", 1)), 0o644); err != nil {
		t.Fatalf("drift the doc: %v", err)
	}

	report, err := Plan(context.Background(), root, "")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	classes := map[string]FeaturePlan{}
	for _, feature := range report.Features {
		classes[feature.Name] = feature
	}

	if got := classes["old-era"]; got.Class != ClassBackfill || len(got.SealableDocs) != 3 || got.ReproveCount != 1 {
		t.Fatalf("old-era = %+v, want backfill with 3 sealable docs and 1 task", got)
	}
	if got := classes["needs-proofs"]; got.Class != ClassReprove || got.ReproveCount != 1 {
		t.Fatalf("needs-proofs = %+v, want re-prove with 1 task", got)
	}
	if got := classes["all-done"]; got.Class != ClassComplete {
		t.Fatalf("all-done = %+v, want complete", got)
	}
	got := classes["drifted"]
	if got.Class != ClassBlocked || !strings.Contains(got.BlockReason, "requirements.md is stale") || !strings.Contains(got.BlockReason, "walden reconcile drifted") {
		t.Fatalf("drifted = %+v, want blocked naming the document and remedy", got)
	}

	if report.Totals.Backfill != 1 || report.Totals.Reprove != 1 || report.Totals.Complete != 1 || report.Totals.Blocked != 1 {
		t.Fatalf("totals = %+v", report.Totals)
	}
	if report.Totals.SealableDocs != 3 || report.Totals.ReproveTasks != 2 {
		t.Fatalf("work totals = %+v", report.Totals)
	}
}

func hashWaldenTree(t *testing.T, root string) string {
	t.Helper()
	sum := sha256.New()
	err := filepath.WalkDir(filepath.Join(root, ".walden"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(sum, "%s\n", path)
		sum.Write(content)
		return nil
	})
	if err != nil {
		t.Fatalf("walk .walden: %v", err)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func TestAdoptPlanIsReadOnly(t *testing.T) {
	root := t.TempDir()
	preFingerprintDocs(t, root, "old-era")
	sealedDocs(t, root, "needs-proofs", true)

	before := hashWaldenTree(t, root)
	if _, err := Plan(context.Background(), root, ""); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if after := hashWaldenTree(t, root); after != before {
		t.Fatal("the plan modified .walden/")
	}
}
