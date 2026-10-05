package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/andrearaponi/walden/internal/consolidate"
	"github.com/andrearaponi/walden/internal/spec"
)

func consolidationBody(name string, criteria int, extra string) string {
	body := "# Requirements Document\n\n## Introduction\n\nSynthetic feature " + name + "." + extra + "\n\n## Requirements\n\n### R1 Behavior\n\n" +
		"**User Story:** As a user, I want " + name + ", so that it works.\n\n#### Acceptance Criteria\n\n"
	for i := 1; i <= criteria; i++ {
		body += fmt.Sprintf("%d. `R1.AC%d` WHEN request %d arrives, the system SHALL answer it.\n   - Acceptance check: request %d receives an answer.\n", i, i, i, i)
	}
	return body + "\n## Non-Functional Requirements\n\n- `NFR1` Answers are fast (bridged by `R1.AC1`).\n\n" +
		"## Constraints And Dependencies\n\n- `C1` Standard library only.\n\n## Out Of Scope\n\n- Anything else.\n"
}

func writeApproved(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, ".walden", "specs", name, "requirements.md")
	fields := map[string]string{"status": "approved", "approved_at": "2026-10-04T06:00:00Z", "last_modified": "2026-10-04T06:00:00Z",
		"approved_fingerprint": spec.Fingerprint(path, body)}
	if err := spec.SaveDocument(spec.Document{Path: path, Fields: fields, Body: body}); err != nil {
		t.Fatal(err)
	}
}

// TestConsolidationScale proves NFR1 on a 100-feature portfolio: the report
// and the status hook answer within two seconds, and the review scope outside
// widely cited files grows with the pending changes and their links.
func TestConsolidationScale(t *testing.T) {
	binary := diagnosticsNativeCLI(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "README.md"), []byte("# Docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	name := func(i int) string { return fmt.Sprintf("feature-%03d", i) }
	for i := 1; i <= 100; i++ {
		extra := fmt.Sprintf(" See `docs/README.md` and `docs/decisions/D%03d.md`.", i)
		if i < 100 {
			extra += " It relies on `" + name(i+1) + "#R1.AC1`."
		}
		writeApproved(t, root, name(i), consolidationBody(name(i), 3, extra))
	}
	if _, stderr, code := diagnosticsNativeRun(t, binary, root, "consolidate", "start"); code != 0 {
		t.Fatalf("start: exit %d %s", code, stderr)
	}
	writeApproved(t, root, name(10), consolidationBody(name(10), 4, " See `docs/README.md` and `docs/decisions/D010.md`. It relies on `"+name(11)+"#R1.AC1`."))
	writeApproved(t, root, name(50), consolidationBody(name(50), 4, " See `docs/README.md` and `docs/decisions/D050.md`. It relies on `"+name(51)+"#R1.AC1`."))
	writeApproved(t, root, name(101), consolidationBody(name(101), 2, " See `docs/README.md`. It relies on `"+name(20)+"#R1.AC1`."))

	timed := func(args ...string) time.Duration {
		started := time.Now()
		if _, stderr, code := diagnosticsNativeRun(t, binary, root, args...); code != 0 {
			t.Fatalf("%v: exit %d %s", args, code, stderr)
		}
		return time.Since(started)
	}
	for _, args := range [][]string{{"consolidate", "--json"}, {"status", name(1), "--json"}} {
		elapsed := timed(args...)
		t.Logf("%v took %v on 101 features", args, elapsed)
		if elapsed > 2*time.Second {
			t.Fatalf("%v took %v on 101 features, want under 2s", args, elapsed)
		}
	}

	result := diagnosticsNativeJSON(t, binary, root, 0, "consolidate")
	if result.Consolidation == nil || result.Consolidation.Threshold != consolidate.ThresholdDue || len(result.Consolidation.Pending) != 3 {
		t.Fatalf("consolidation = %+v, want three pending changes and a due threshold", result.Consolidation)
	}
	linked, hubOnly := []string{}, 0
	for _, entry := range result.Consolidation.Scope {
		if entry.HubOnly {
			hubOnly++
		} else {
			linked = append(linked, entry.Feature)
		}
	}
	sort.Strings(linked)
	want := []string{name(9), name(10), name(11), name(20), name(49), name(50), name(51), name(101)}
	if strings.Join(linked, ",") != strings.Join(want, ",") || hubOnly != 101-len(want) {
		t.Fatalf("linked scope = %v (hub-only %d), want %v with the rest hub-only", linked, hubOnly, want)
	}
	if len(result.Consolidation.Hubs) != 1 || result.Consolidation.Hubs[0].Path != "docs/README.md" {
		t.Fatalf("hubs = %+v", result.Consolidation.Hubs)
	}
	if len(result.Consolidation.Comparisons) != 3 {
		t.Fatalf("comparisons = %d, want one per pending change", len(result.Consolidation.Comparisons))
	}
	for _, comparison := range result.Consolidation.Comparisons {
		withText := 0
		for _, linked := range comparison.Linked {
			if !linked.HubOnly && len(linked.Statements) > 0 {
				withText++
			}
		}
		if len(comparison.Statements) == 0 || withText == 0 {
			t.Fatalf("comparison of %s carries no statements or no linked statements: %+v", comparison.Feature, comparison)
		}
	}
}

func cycleRequirements(t *testing.T, binary, root, feature, body string, initialize bool) {
	t.Helper()
	if initialize {
		if _, stderr, code := diagnosticsNativeRun(t, binary, root, "feature", "init", feature); code != 0 {
			t.Fatalf("feature init %s: %s", feature, stderr)
		}
	} else if _, stderr, code := diagnosticsNativeRun(t, binary, root, "reconcile", feature); code != 0 {
		t.Fatalf("reconcile %s: %s", feature, stderr)
	}
	path := filepath.Join(root, ".walden", "specs", feature, "requirements.md")
	document, err := spec.ParseDocument(path, mustReadFile(t, path))
	if err != nil {
		t.Fatal(err)
	}
	document.Body = body
	if err := spec.SaveDocument(document); err != nil {
		t.Fatal(err)
	}
	for _, step := range [][]string{{"review", "open", feature, "--phase", "requirements"}, {"review", "approve", feature, "--phase", "requirements"}} {
		if stdout, stderr, code := diagnosticsNativeRun(t, binary, root, step...); code != 0 {
			t.Fatalf("%v: exit %d\n%s\n%s", step, code, stdout, stderr)
		}
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func hasWarning(warnings []string, fragment string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, fragment) {
			return true
		}
	}
	return false
}

// TestConsolidationCycleEndToEnd walks the whole cycle through the real CLI:
// start, counting, suggested and due warnings, a refused and a successful
// open, approval with its checkpoint, zero pending, and a stale view.
func TestConsolidationCycleEndToEnd(t *testing.T) {
	binary := diagnosticsNativeCLI(t)
	root := t.TempDir()
	if _, stderr, code := diagnosticsNativeRun(t, binary, root, "repo", "init"); code != 0 {
		t.Fatalf("repo init: %s", stderr)
	}
	for _, feature := range []string{"alpha", "beta", "gamma"} {
		cycleRequirements(t, binary, root, feature, consolidationBody(feature, 2, ""), true)
	}
	if started := diagnosticsNativeJSON(t, binary, root, 0, "consolidate", "start"); !strings.Contains(started.Summary, "3 approved feature(s)") {
		t.Fatalf("start summary = %q", started.Summary)
	}

	cycleRequirements(t, binary, root, "alpha", consolidationBody("alpha", 3, " It relies on `beta#R1.AC1`."), false)
	cycleRequirements(t, binary, root, "delta", consolidationBody("delta", 2, ""), true)
	status := diagnosticsNativeJSON(t, binary, root, 0, "status", "gamma")
	if !hasWarning(status.Warnings, "consolidation suggested: 2 contract changes") {
		t.Fatalf("status warnings with two pending changes = %v", status.Warnings)
	}
	cycleRequirements(t, binary, root, "epsilon", consolidationBody("epsilon", 1, ""), true)
	status = diagnosticsNativeJSON(t, binary, root, 0, "status", "gamma")
	if !hasWarning(status.Warnings, "consolidation due: 3 contract changes since the last consolidation (alpha, delta, epsilon)") {
		t.Fatalf("status warnings with three pending changes = %v", status.Warnings)
	}
	if initialized := diagnosticsNativeJSON(t, binary, root, 0, "feature", "init", "zeta"); !hasWarning(initialized.Warnings, "consolidation due") {
		t.Fatalf("feature init warnings = %v", initialized.Warnings)
	}
	stdout, _, _ := diagnosticsNativeRun(t, binary, root, "release", "check", "--json")
	if !strings.Contains(stdout, `"repository_warnings"`) || !strings.Contains(stdout, "consolidation due") {
		t.Fatalf("release check does not carry the due warning:\n%s", stdout)
	}

	report := diagnosticsNativeJSON(t, binary, root, 0, "consolidate")
	sections := ""
	for _, feature := range []string{"alpha", "beta", "delta", "epsilon"} {
		ids := report.Consolidation.Identifiers[feature]
		if len(ids) == 0 {
			t.Fatalf("no identifiers reported for %s: %+v", feature, report.Consolidation.Identifiers)
		}
		sections += "## " + feature + "\n\nPurpose: Synthetic feature " + feature + ".\nActive: " + strings.Join(ids, ", ") + "\n\n"
	}
	viewPath := filepath.Join(root, consolidate.ViewPath)
	view, err := spec.ParseDocument(viewPath, mustReadFile(t, viewPath))
	if err != nil {
		t.Fatal(err)
	}
	view.Body = "# Current Contracts\n\n" + strings.Replace(sections, ", R1.AC3", "", 1)
	if err := spec.SaveDocument(view); err != nil {
		t.Fatal(err)
	}
	if refused := diagnosticsNativeJSON(t, binary, root, 1, "consolidate", "open"); !strings.Contains(strings.Join(refused.Blockers, "\n"), "alpha R1.AC3") {
		t.Fatalf("open with an omission: blockers %v", refused.Blockers)
	}
	view, _ = spec.ParseDocument(viewPath, mustReadFile(t, viewPath))
	view.Body = "# Current Contracts\n\n" + sections
	if err := spec.SaveDocument(view); err != nil {
		t.Fatal(err)
	}
	if refused := diagnosticsNativeJSON(t, binary, root, 1, "consolidate", "open"); !strings.Contains(strings.Join(refused.Blockers, "\n"), "coherence review") {
		t.Fatalf("open without a coherence review: blockers %v", refused.Blockers)
	}
	view, _ = spec.ParseDocument(viewPath, mustReadFile(t, viewPath))
	view.Body = "# Current Contracts\n\n" + sections + completeCoherenceSection(t, root)
	if err := spec.SaveDocument(view); err != nil {
		t.Fatal(err)
	}
	diagnosticsNativeJSON(t, binary, root, 0, "consolidate", "open")
	approved := diagnosticsNativeJSON(t, binary, root, 0, "consolidate", "approve")
	if approved.Consolidation == nil || len(approved.Consolidation.Pending) != 0 {
		t.Fatalf("approve result = %+v", approved.Consolidation)
	}
	record, err := consolidate.LoadRecord(root)
	if err != nil || record.Checkpoint == nil || record.Features["alpha"].State != consolidate.StateConsolidated || record.Features["gamma"].State != consolidate.StateBacklog {
		t.Fatalf("record after approval = %+v, %v", record, err)
	}
	after := diagnosticsNativeJSON(t, binary, root, 0, "consolidate")
	if len(after.Consolidation.Pending) != 0 || strings.Join(after.Consolidation.Backlog, ",") != "gamma" || after.Consolidation.View != consolidate.ViewApproved {
		t.Fatalf("after approval: %+v", after.Consolidation)
	}

	edited := strings.Replace(string(mustReadFile(t, viewPath)), "Purpose: Synthetic feature beta.", "Purpose: Synthetic feature beta, revised.", 1)
	if err := os.WriteFile(viewPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if status := diagnosticsNativeJSON(t, binary, root, 0, "status", "beta"); !hasWarning(status.Warnings, "contracts.md is stale") {
		t.Fatalf("status after editing the approved view = %v", status.Warnings)
	}
}

// completeCoherenceSection writes the coherence review a careful reviewer
// would leave: one entry per pending feature, citing a linked statement of its
// comparison when there is one.
func completeCoherenceSection(t *testing.T, root string) string {
	t.Helper()
	inputs, err := consolidate.LoadInputs(root)
	if err != nil {
		t.Fatal(err)
	}
	report := inputs.Report(root)
	section := "\n" + consolidate.CoherenceHeading + "\n\n"
	for _, change := range report.State.Pending {
		outcome := "No linked statements to compare."
		for _, comparison := range report.Comparisons {
			for _, linked := range comparison.Linked {
				if comparison.Feature == change.Feature && !linked.HubOnly && len(linked.Statements) > 0 && outcome == "No linked statements to compare." {
					outcome = "No inconsistency with `" + linked.Feature + "#" + linked.Statements[0].ID + "`."
				}
			}
		}
		section += "### " + change.Feature + "\n\n- " + outcome + "\n\n"
	}
	return section
}
