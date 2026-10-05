package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/consolidate"
	"github.com/andrearaponi/walden/internal/spec"
)

type statusEnvelope struct {
	OK     bool `json:"ok"`
	Result struct {
		CurrentPhase  string            `json:"current_phase"`
		Documents     []json.RawMessage `json:"documents"`
		Blockers      []string          `json:"blockers"`
		NextAction    string            `json:"next_action"`
		Warnings      []string          `json:"warnings"`
		ExitCode      int               `json:"exit_code"`
		Consolidation *struct {
			Tracking       string `json:"tracking"`
			Threshold      string `json:"threshold"`
			Unconsolidated int    `json:"unconsolidated"`
			View           string `json:"view"`
		} `json:"consolidation"`
	} `json:"result"`
}

func statusJSON(t *testing.T, feature string) (statusEnvelope, int) {
	t.Helper()
	stdout, stderr, code := runCommand(t, []string{"status", feature, "--json"})
	var envelope statusEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("status %s: invalid envelope (exit %d): %v\n%s\n%s", feature, code, err, stdout, stderr)
	}
	return envelope, code
}

func consolidationWarningsOf(warnings []string) []string {
	found := []string{}
	for _, warning := range warnings {
		if strings.Contains(warning, "consolidation") || strings.Contains(warning, "contracts.md") {
			found = append(found, warning)
		}
	}
	return found
}

func startedConsolidationRepo(t *testing.T) string {
	t.Helper()
	root := consolidationRepo(t)
	if _, code := consolidationJSON(t, "consolidate", "start"); code != 0 {
		t.Fatal("start failed")
	}
	return root
}

func TestStatusConsolidationWarnings(t *testing.T) {
	root := startedConsolidationRepo(t)
	expectWarnings := func(want ...string) {
		t.Helper()
		for _, feature := range []string{"desk-booking", "member-invoicing"} {
			envelope, code := statusJSON(t, feature)
			got := consolidationWarningsOf(envelope.Result.Warnings)
			if code != 0 || len(got) != len(want) {
				t.Fatalf("status %s: exit %d, consolidation warnings %v, want %d matching %v", feature, code, got, len(want), want)
			}
			for i, fragment := range want {
				if !strings.Contains(got[i], fragment) {
					t.Fatalf("status %s warning %q does not contain %q", feature, got[i], fragment)
				}
			}
		}
	}

	expectWarnings()
	writeRequirementsDocument(t, root, "waitlist", "approved", "# Requirements Document\n\n### R1 Waitlist\n\n1. `R1.AC1` WHEN all desks are taken, the system SHALL queue the member.\n")
	expectWarnings()
	writeRequirementsDocument(t, root, "desk-occupancy", "approved", consolidationOccupancy+"- `C2` Sites are configured.\n")
	expectWarnings("consolidation suggested: 2 contract changes since the last consolidation (desk-occupancy, waitlist)")
	if envelope, _ := statusJSON(t, "desk-booking"); envelope.Result.Consolidation == nil || envelope.Result.Consolidation.Threshold != consolidate.ThresholdSuggested {
		t.Fatalf("suggested threshold missing from status JSON: %+v", envelope.Result.Consolidation)
	}
	writeRequirementsDocument(t, root, "member-invoicing", "approved", consolidationInvoicing+"2. `R1.AC2` WHEN a month closes, the system SHALL invoice memberships.\n")
	expectWarnings("consolidation due: 3 contract changes since the last consolidation (desk-occupancy, member-invoicing, waitlist)")
	stdout, _, code := runCommand(t, []string{"status", "desk-booking"})
	if code != 0 || !strings.Contains(stdout, "consolidation due") {
		t.Fatalf("text status does not report the due consolidation:\n%s", stdout)
	}

	t.Run("stale view", func(t *testing.T) {
		root := openConsolidationView(t)
		if _, code := consolidationJSON(t, "consolidate", "approve"); code != 0 {
			t.Fatal("approve failed")
		}
		viewPath := filepath.Join(root, consolidate.ViewPath)
		edited := strings.Replace(string(mustRead(t, viewPath)), "Report free desks.", "Report free desks per site.", 1)
		if err := os.WriteFile(viewPath, []byte(edited), 0o644); err != nil {
			t.Fatal(err)
		}
		expectWarnings("contracts.md is stale")
	})

	t.Run("unknown record", func(t *testing.T) {
		root := startedConsolidationRepo(t)
		before, _ := statusJSON(t, "desk-booking")
		if err := os.WriteFile(filepath.Join(root, consolidate.RecordPath), []byte("{broken"), 0o644); err != nil {
			t.Fatal(err)
		}
		after, code := statusJSON(t, "desk-booking")
		got := consolidationWarningsOf(after.Result.Warnings)
		if code != 0 || len(got) != 1 || !strings.Contains(got[0], "consolidation state unknown") || !strings.Contains(got[0], consolidate.RecordPath) || !strings.Contains(got[0], "restore") {
			t.Fatalf("unknown record: exit %d, warnings %v", code, got)
		}
		if after.Result.Consolidation.Tracking != consolidate.TrackingUnknown || after.Result.Consolidation.Threshold != consolidate.ThresholdNone {
			t.Fatalf("unknown record consolidation = %+v", after.Result.Consolidation)
		}
		if before.Result.CurrentPhase != after.Result.CurrentPhase || before.Result.NextAction != after.Result.NextAction ||
			!reflect.DeepEqual(before.Result.Documents, after.Result.Documents) || !reflect.DeepEqual(before.Result.Blockers, after.Result.Blockers) {
			t.Fatal("an unknown consolidation record changed the rest of the status output")
		}
	})
}

func TestStatusConsolidationNotStarted(t *testing.T) {
	consolidationRepo(t)
	envelope, code := statusJSON(t, "desk-booking")
	if code != 0 || envelope.Result.Consolidation == nil || envelope.Result.Consolidation.Tracking != consolidate.TrackingNotStarted ||
		envelope.Result.Consolidation.Unconsolidated != 3 || envelope.Result.Consolidation.Threshold != consolidate.ThresholdNone {
		t.Fatalf("not started: exit %d, %+v", code, envelope.Result.Consolidation)
	}
	if got := consolidationWarningsOf(envelope.Result.Warnings); len(got) != 0 {
		t.Fatalf("the not-started note became a warning: %v", got)
	}
	stdout, _, code := runCommand(t, []string{"status", "member-invoicing"})
	if code != 0 || !strings.Contains(stdout, "Consolidation: not started; 3 approved feature(s) not consolidated (walden consolidate start)") || strings.Contains(stdout, "due") {
		t.Fatalf("text status:\n%s", stdout)
	}

	root := t.TempDir()
	t.Chdir(root)
	writeRequirementsDocument(t, root, "draft-only", "draft", consolidationInvoicing)
	stdout, _, _ = runCommand(t, []string{"status", "draft-only"})
	if strings.Contains(stdout, "Consolidation:") {
		t.Fatalf("a repository without approved features got a consolidation note:\n%s", stdout)
	}
}

func makeDue(t *testing.T, root string) {
	t.Helper()
	for _, feature := range []string{"alpha-extra", "beta-extra", "gamma-extra"} {
		writeRequirementsDocument(t, root, feature, "approved", "# Requirements Document\n\n### R1 Extra\n\n1. `R1.AC1` WHEN asked, the system SHALL answer.\n")
	}
}

func TestFeatureInitConsolidationWarning(t *testing.T) {
	type initEnvelope struct {
		OK     bool `json:"ok"`
		Result struct {
			CreatedFiles []string `json:"created_files"`
			Warnings     []string `json:"warnings"`
			ExitCode     int      `json:"exit_code"`
		} `json:"result"`
	}
	initFeature := func(name string) (initEnvelope, int) {
		stdout, _, code := runCommand(t, []string{"feature", "init", name, "--json"})
		var envelope initEnvelope
		if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
			t.Fatalf("feature init %s: %v\n%s", name, err, stdout)
		}
		return envelope, code
	}

	root := startedConsolidationRepo(t)
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	quiet, quietCode := initFeature("first-new")
	if quietCode != 0 || len(consolidationWarningsOf(quiet.Result.Warnings)) != 0 {
		t.Fatalf("feature init without a due consolidation: exit %d, warnings %v", quietCode, quiet.Result.Warnings)
	}

	makeDue(t, root)
	due, dueCode := initFeature("second-new")
	if dueCode != quietCode || due.OK != quiet.OK || len(due.Result.CreatedFiles) != len(quiet.Result.CreatedFiles) {
		t.Fatalf("a due consolidation changed feature init: exit %d vs %d, files %v vs %v", dueCode, quietCode, due.Result.CreatedFiles, quiet.Result.CreatedFiles)
	}
	if _, err := os.Stat(filepath.Join(root, ".walden", "specs", "second-new", "requirements.md")); err != nil {
		t.Fatalf("scaffold not created: %v", err)
	}
	got := consolidationWarningsOf(due.Result.Warnings)
	if len(got) != 1 || !strings.Contains(got[0], "consolidation due") {
		t.Fatalf("feature init warnings = %v, want the due consolidation", due.Result.Warnings)
	}
}

func releaseJSON(t *testing.T, args ...string) (map[string]any, int) {
	t.Helper()
	stdout, _, code := runCommand(t, append(append([]string{"release", "check"}, args...), "--json"))
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("release check %v: %v\n%s", args, err, stdout)
	}
	return envelope, code
}

func TestReleaseCheckConsolidationWarning(t *testing.T) {
	root := startedConsolidationRepo(t)
	record, err := consolidate.LoadRecord(root)
	if err != nil {
		t.Fatal(err)
	}
	clean := *record
	changed := consolidate.Record{Schema: record.Schema, StartedAt: record.StartedAt, Features: map[string]consolidate.RecordFeature{}}
	for name, feature := range record.Features {
		feature.RequirementsFingerprint = spec.Fingerprint("requirements.md", "an older revision of "+name)
		changed.Features[name] = feature
	}

	for _, args := range [][]string{{}, {"--strict"}} {
		if err := consolidate.SaveRecord(root, clean); err != nil {
			t.Fatal(err)
		}
		without, withoutCode := releaseJSON(t, args...)
		if err := consolidate.SaveRecord(root, changed); err != nil {
			t.Fatal(err)
		}
		with, withCode := releaseJSON(t, args...)

		release, _ := with["result"].(map[string]any)["release"].(map[string]any)
		warnings, _ := release["repository_warnings"].([]any)
		if len(warnings) != 1 || !strings.Contains(warnings[0].(string), "consolidation due") {
			t.Fatalf("release check %v: repository warnings %v, want the due consolidation", args, warnings)
		}
		delete(release, "repository_warnings")
		if withCode != withoutCode || !reflect.DeepEqual(with, without) {
			t.Fatalf("release check %v changed beyond the warning: exit %d vs %d\nwith: %v\nwithout: %v", args, withCode, withoutCode, with, without)
		}
		if _, present := without["result"].(map[string]any)["release"].(map[string]any)["repository_warnings"]; present {
			t.Fatalf("release check %v reported repository warnings without a due consolidation", args)
		}
	}
}
