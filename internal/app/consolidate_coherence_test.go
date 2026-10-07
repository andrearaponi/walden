package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConsolidationCoherenceRequest pins the request to compare the pending
// changes with their linked statements in every suggested or due warning and
// in the report's next action, so the request reaches the agent even when the
// user only asks to consolidate.
func TestConsolidationCoherenceRequest(t *testing.T) {
	const request = "compare each pending change with the statements linked to it"
	root := startedConsolidationRepo(t)
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	expect := func(where, threshold string, warnings []string) {
		t.Helper()
		got := consolidationWarningsOf(warnings)
		if len(got) != 1 || !strings.Contains(got[0], threshold) || !strings.Contains(got[0], request) {
			t.Errorf("%s warnings = %v, want one %q warning asking to %q", where, got, threshold, request)
		}
	}
	expectNextAction := func(where string) {
		t.Helper()
		report, code := consolidationJSON(t, "consolidate")
		if code != 0 || !strings.Contains(report.Result.NextAction, request) {
			t.Errorf("%s report next action = %q (exit %d), want it to ask to %q", where, report.Result.NextAction, code, request)
		}
	}

	writeRequirementsDocument(t, root, "waitlist", "approved", "# Requirements Document\n\n### R1 Waitlist\n\n1. `R1.AC1` WHEN all desks are taken, the system SHALL queue the member.\n")
	expectNextAction("one pending change")
	writeRequirementsDocument(t, root, "desk-occupancy", "approved", consolidationOccupancy+"- `C2` Sites are configured.\n")
	suggested, _ := statusJSON(t, "desk-booking")
	expect("status (suggested)", "consolidation suggested", suggested.Result.Warnings)
	expectNextAction("suggested")

	writeRequirementsDocument(t, root, "member-invoicing", "approved", consolidationInvoicing+"2. `R1.AC2` WHEN a month closes, the system SHALL invoice memberships.\n")
	due, _ := statusJSON(t, "desk-booking")
	expect("status (due)", "consolidation due", due.Result.Warnings)
	expectNextAction("due")

	stdout, _, code := runCommand(t, []string{"feature", "init", "coherence-new", "--json"})
	var created struct {
		Result struct {
			Warnings []string `json:"warnings"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &created); err != nil || code != 0 {
		t.Fatalf("feature init: exit %d, %v\n%s", code, err, stdout)
	}
	expect("feature init", "consolidation due", created.Result.Warnings)

	release, _ := releaseJSON(t)
	var warnings []string
	if result, ok := release["result"].(map[string]any); ok {
		if section, ok := result["release"].(map[string]any); ok {
			if list, ok := section["repository_warnings"].([]any); ok {
				for _, item := range list {
					warnings = append(warnings, item.(string))
				}
			}
		}
	}
	expect("release check", "consolidation due", warnings)
}
