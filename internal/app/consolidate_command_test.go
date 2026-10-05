package app

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/consolidate"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

const consolidationBooking = "# Requirements Document\n\n## Introduction\n\nBook desks.\n\n## Requirements\n\n### R1 Booking\n\n" +
	"1. `R1.AC1` WHEN a member books a free desk, the system SHALL confirm it while `desk-occupancy#R1.AC2` holds.\n" +
	"   - Acceptance check: a booking is confirmed.\n" +
	"2. `R1.AC2` WHEN a day-pass holder books, the system SHALL confirm it.\n" +
	"   - Acceptance check: a day-pass booking is confirmed.\n\n" +
	"### R2 Records\n\n1. `R2.AC1` The system SHALL record each booking.\n   - Acceptance check: one record per booking.\n\n" +
	"## Non-Functional Requirements\n\n- `NFR1` Privacy (bridged by `R2.AC1`).\n\n" +
	"## Constraints And Dependencies\n\n- `C1` Standard library only.\n- `C2` Hours follow decision K9 (`docs/decisions/K9-opening-hours.md`).\n"

const consolidationOccupancy = "# Requirements Document\n\n## Introduction\n\nFree desks.\n\n## Requirements\n\n### R1 Status\n\n" +
	"1. `R1.AC1` WHEN a desk is booked, the system SHALL report it occupied.\n   - Acceptance check: occupied after booking.\n" +
	"2. `R1.AC2` WHEN a booking is cancelled, the system SHALL report the desk free.\n   - Acceptance check: free after cancelling.\n\n" +
	"## Constraints And Dependencies\n\n- `C1` Occupancy derives from bookings only.\n"

const consolidationInvoicing = "# Requirements Document\n\n## Introduction\n\nInvoices; see `desk-booking#R9.AC9`.\n\n## Requirements\n\n### R1 Charges\n\n" +
	"1. `R1.AC1` WHEN a month closes, the system SHALL invoice day passes.\n   - Acceptance check: one line per day pass.\n"

const consolidationView = "# Current Contracts\n\n" +
	"## desk-booking\n\nPurpose: Book one desk per holder per day.\nActive: R1.AC1, R1.AC2, R2.AC1, NFR1, C1, C2\nRelated: desk-occupancy\n\n" +
	"## desk-occupancy\n\nPurpose: Report free desks.\nActive: R1.AC1, R1.AC2, C1\n"

type consolidationEnvelope struct {
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	Result  struct {
		Summary       string   `json:"summary"`
		Warnings      []string `json:"warnings"`
		Blockers      []string `json:"blockers"`
		NextAction    string   `json:"next_action"`
		CreatedFiles  []string `json:"created_files"`
		UpdatedFiles  []string `json:"updated_files"`
		ExitCode      int      `json:"exit_code"`
		Consolidation *struct {
			Tracking       string `json:"tracking"`
			Pending        []struct{ Feature, Kind string }
			Backlog        []string `json:"backlog"`
			Unconsolidated int      `json:"unconsolidated"`
			Threshold      string   `json:"threshold"`
			View           string   `json:"view"`
			Problem        string   `json:"problem"`
			Remedy         string   `json:"remedy"`
			Scope          []struct {
				Feature string `json:"feature"`
				Pending string `json:"pending"`
				HubOnly bool   `json:"hub_only"`
				Links   []struct{ Kind, Feature, Path string }
			} `json:"scope"`
			Findings    []struct{ Kind, Feature, ID, Subject, Message string }
			Identifiers map[string][]string `json:"identifiers"`
		} `json:"consolidation"`
	} `json:"result"`
}

func consolidationRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	writeRequirementsDocument(t, root, "desk-booking", "approved", consolidationBooking)
	writeRequirementsDocument(t, root, "desk-occupancy", "approved", consolidationOccupancy)
	writeRequirementsDocument(t, root, "member-invoicing", "approved", consolidationInvoicing)
	return root
}

func writeRequirementsDocument(t *testing.T, root, feature, status, body string) {
	t.Helper()
	path := filepath.Join(root, ".walden", "specs", feature, "requirements.md")
	fields := map[string]string{"status": status, "approved_at": "", "last_modified": "2026-10-04T06:00:00Z", "approved_fingerprint": ""}
	if status == "approved" {
		fields["approved_at"] = "2026-10-04T06:00:00Z"
		fields["approved_fingerprint"] = spec.Fingerprint(path, body)
	}
	if err := spec.SaveDocument(spec.Document{Path: path, Fields: fields, Body: body}); err != nil {
		t.Fatal(err)
	}
}

func writeViewBody(t *testing.T, root, body string) {
	t.Helper()
	path := filepath.Join(root, consolidate.ViewPath)
	document, err := spec.ParseDocument(path, mustRead(t, path))
	if err != nil {
		t.Fatal(err)
	}
	document.Body = body
	if err := spec.SaveDocument(document); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func consolidationJSON(t *testing.T, args ...string) (consolidationEnvelope, int) {
	t.Helper()
	stdout, stderr, code := runCommand(t, append(args, "--json"))
	var envelope consolidationEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("walden %v --json: invalid envelope (exit %d): %v\nstdout: %s\nstderr: %s", args, code, err, stdout, stderr)
	}
	return envelope, code
}

func pendingNames(envelope consolidationEnvelope) []string {
	names := []string{}
	if envelope.Result.Consolidation == nil {
		return names
	}
	for _, change := range envelope.Result.Consolidation.Pending {
		names = append(names, change.Feature+":"+change.Kind)
	}
	return names
}

func TestConsolidateStart(t *testing.T) {
	root := consolidationRepo(t)

	before, code := consolidationJSON(t, "consolidate")
	if code != 0 || before.Result.Consolidation == nil || before.Result.Consolidation.Tracking != consolidate.TrackingNotStarted || before.Result.Consolidation.Unconsolidated != 3 {
		t.Fatalf("before start: exit %d, %+v", code, before.Result.Consolidation)
	}

	started, code := consolidationJSON(t, "consolidate", "start")
	if code != 0 || !started.OK || started.Command != "consolidate-start" {
		t.Fatalf("start: exit %d, envelope %+v", code, started)
	}
	record, err := consolidate.LoadRecord(root)
	if err != nil || record == nil || len(record.Features) != 3 {
		t.Fatalf("record after start = %+v, %v", record, err)
	}
	for name, feature := range record.Features {
		if feature.State != consolidate.StateBacklog || !spec.ValidFingerprint(feature.RequirementsFingerprint) || len(feature.ActiveIDs) == 0 {
			t.Errorf("%s recorded as %+v, want backlog with fingerprint and identifiers", name, feature)
		}
	}
	view, err := consolidate.LoadView(root)
	if err != nil || view.Status().State != consolidate.ViewDraft {
		t.Fatalf("view after start = %+v, %v; want a draft scaffold", view.Status(), err)
	}

	after, _ := consolidationJSON(t, "consolidate")
	if got := after.Result.Consolidation; got.Tracking != consolidate.TrackingActive || len(got.Pending) != 0 || len(got.Backlog) != 3 {
		t.Fatalf("after start: %+v, want nothing pending and three backlog features", got)
	}

	writeRequirementsDocument(t, root, "waitlist", "approved", "# Requirements Document\n\n### R1 Waitlist\n\n1. `R1.AC1` WHEN all desks are taken, the system SHALL queue the member.\n")
	if got := pendingNames(mustConsolidation(t)); strings.Join(got, ",") != "waitlist:new" {
		t.Fatalf("after approving a new feature: pending %v", got)
	}
	writeRequirementsDocument(t, root, "desk-occupancy", "approved", consolidationOccupancy+"- `C2` Sites are listed in configuration.\n")
	if got := pendingNames(mustConsolidation(t)); strings.Join(got, ",") != "desk-occupancy:revised,waitlist:new" {
		t.Fatalf("after revising a backlog feature: pending %v", got)
	}

	recordBytes := mustRead(t, filepath.Join(root, consolidate.RecordPath))
	again, code := consolidationJSON(t, "consolidate", "start")
	if code == 0 || again.OK || !strings.Contains(again.Result.Summary, "already") {
		t.Fatalf("second start: exit %d, %+v", code, again.Result)
	}
	if !bytes.Equal(recordBytes, mustRead(t, filepath.Join(root, consolidate.RecordPath))) {
		t.Fatal("a refused start changed the record")
	}
}

func mustConsolidation(t *testing.T) consolidationEnvelope {
	t.Helper()
	envelope, code := consolidationJSON(t, "consolidate")
	if code != 0 || envelope.Result.Consolidation == nil {
		t.Fatalf("consolidate report: exit %d, %+v", code, envelope.Result)
	}
	return envelope
}

func TestConsolidateOpen(t *testing.T) {
	root := consolidationRepo(t)
	if _, code := consolidationJSON(t, "consolidate", "start"); code != 0 {
		t.Fatal("start failed")
	}
	writeViewBody(t, root, strings.Replace(consolidationView, "Active: R1.AC1, R1.AC2, C1", "Active: R1.AC1, C1", 1))
	viewBefore := mustRead(t, filepath.Join(root, consolidate.ViewPath))

	refused, code := consolidationJSON(t, "consolidate", "open")
	if code == 0 || refused.OK || refused.Command != "consolidate-open" {
		t.Fatalf("open with a mismatch: exit %d, %+v", code, refused)
	}
	if !strings.Contains(strings.Join(refused.Result.Blockers, "\n"), "desk-occupancy R1.AC2") {
		t.Fatalf("refusal blockers %v do not name the omission", refused.Result.Blockers)
	}
	if !bytes.Equal(viewBefore, mustRead(t, filepath.Join(root, consolidate.ViewPath))) {
		t.Fatal("a refused open changed the view")
	}

	writeViewBody(t, root, consolidationView)
	opened, code := consolidationJSON(t, "consolidate", "open")
	if code != 0 || !opened.OK {
		t.Fatalf("open after the fix: exit %d, %+v", code, opened.Result)
	}
	if view, _ := consolidate.LoadView(root); view.Status().State != consolidate.ViewInReview {
		t.Fatalf("view state after open = %q", view.Status().State)
	}
}

func openConsolidationView(t *testing.T) string {
	t.Helper()
	root := consolidationRepo(t)
	if _, code := consolidationJSON(t, "consolidate", "start"); code != 0 {
		t.Fatal("start failed")
	}
	writeViewBody(t, root, consolidationView)
	if _, code := consolidationJSON(t, "consolidate", "open"); code != 0 {
		t.Fatal("open failed")
	}
	return root
}

func TestConsolidateApprove(t *testing.T) {
	t.Run("refuses a view that is not in review", func(t *testing.T) {
		root := consolidationRepo(t)
		if _, code := consolidationJSON(t, "consolidate", "start"); code != 0 {
			t.Fatal("start failed")
		}
		writeViewBody(t, root, consolidationView)
		if refused, code := consolidationJSON(t, "consolidate", "approve"); code == 0 || refused.OK {
			t.Fatalf("approving a draft view: exit %d", code)
		}
		if record, _ := consolidate.LoadRecord(root); record.Checkpoint != nil {
			t.Fatal("a refused approval recorded a checkpoint")
		}
	})

	root := openConsolidationView(t)
	recordBefore := mustRead(t, filepath.Join(root, consolidate.RecordPath))
	writeRequirementsDocument(t, root, "desk-occupancy", "in-review", consolidationOccupancy)
	refused, code := consolidationJSON(t, "consolidate", "approve")
	if code == 0 || refused.OK || !strings.Contains(strings.Join(refused.Result.Blockers, "\n"), "desk-occupancy") {
		t.Fatalf("approval with a covered feature under review: exit %d, %+v", code, refused.Result)
	}
	if !bytes.Equal(recordBefore, mustRead(t, filepath.Join(root, consolidate.RecordPath))) {
		t.Fatal("a refused approval changed the record")
	}

	writeRequirementsDocument(t, root, "desk-occupancy", "approved", consolidationOccupancy)
	approved, code := consolidationJSON(t, "consolidate", "approve")
	if code != 0 || !approved.OK || approved.Command != "consolidate-approve" {
		t.Fatalf("approval: exit %d, %+v", code, approved.Result)
	}
	view, _ := consolidate.LoadView(root)
	record, err := consolidate.LoadRecord(root)
	if err != nil || view.Status().State != consolidate.ViewApproved || record.Checkpoint == nil || record.Checkpoint.ViewFingerprint != view.Status().ApprovedFingerprint {
		t.Fatalf("after approval: view %+v, record %+v, %v", view.Status(), record, err)
	}
	for _, name := range []string{"desk-booking", "desk-occupancy"} {
		feature := record.Features[name]
		if feature.State != consolidate.StateConsolidated || !spec.ValidFingerprint(feature.RequirementsFingerprint) {
			t.Errorf("%s recorded as %+v, want consolidated", name, feature)
		}
	}
	report := mustConsolidation(t)
	if got := report.Result.Consolidation; len(got.Pending) != 0 || strings.Join(got.Backlog, ",") != "member-invoicing" {
		t.Fatalf("after approval: pending %+v, backlog %v; want only member-invoicing in the backlog", got.Pending, got.Backlog)
	}

	t.Run("repairs a record left behind by an interrupted approval", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(root, consolidate.RecordPath), recordBefore, 0o644); err != nil {
			t.Fatal(err)
		}
		if repaired, code := consolidationJSON(t, "consolidate", "approve"); code != 0 || !repaired.OK {
			t.Fatalf("repair: exit %d, %+v", code, repaired.Result)
		}
		if record, _ := consolidate.LoadRecord(root); record.Checkpoint == nil || record.Features["desk-booking"].State != consolidate.StateConsolidated {
			t.Fatalf("record not repaired: %+v", record)
		}
	})
}

func TestConsolidateReport(t *testing.T) {
	root := openConsolidationView(t)
	if _, code := consolidationJSON(t, "consolidate", "approve"); code != 0 {
		t.Fatal("approve failed")
	}

	report := mustConsolidation(t)
	got := report.Result.Consolidation
	if got.View != consolidate.ViewApproved || report.Command != "consolidate" {
		t.Fatalf("view state = %q, command %q", got.View, report.Command)
	}
	kinds := map[string]bool{}
	for _, finding := range got.Findings {
		kinds[finding.Kind+" "+finding.Subject] = true
	}
	if !kinds["missing-file docs/decisions/K9-opening-hours.md"] || !kinds["dangling-reference desk-booking#R9.AC9"] {
		t.Fatalf("findings = %+v", got.Findings)
	}

	writeRequirementsDocument(t, root, "desk-booking", "approved", consolidationBooking+"- `C3` Desks are numbered per site.\n")
	writeRequirementsDocument(t, root, "waitlist", "approved", "# Requirements Document\n\n### R1 Waitlist\n\n1. `R1.AC1` WHEN all desks are taken, the system SHALL queue the member while `desk-booking#R1.AC1` holds.\n")
	report = mustConsolidation(t)
	got = report.Result.Consolidation
	if got.Threshold != consolidate.ThresholdSuggested || strings.Join(pendingNames(report), ",") != "desk-booking:revised,waitlist:new" {
		t.Fatalf("threshold %q, pending %v; want suggested with two changes", got.Threshold, pendingNames(report))
	}
	scope := []string{}
	for _, entry := range got.Scope {
		scope = append(scope, entry.Feature)
	}
	if strings.Join(scope, ",") != "desk-booking,waitlist,desk-occupancy,member-invoicing" {
		t.Fatalf("scope = %v", scope)
	}
	if ids := got.Identifiers["desk-booking"]; strings.Join(ids, ",") != "R1.AC1,R1.AC2,R2.AC1,NFR1,C1,C2,C3" {
		t.Fatalf("identifiers of desk-booking = %v", ids)
	}

	viewPath := filepath.Join(root, consolidate.ViewPath)
	edited := strings.Replace(string(mustRead(t, viewPath)), "Report free desks.", "Report free desks per site.", 1)
	if err := os.WriteFile(viewPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	report = mustConsolidation(t)
	if report.Result.Consolidation.View != consolidate.ViewStale || !strings.Contains(strings.Join(report.Result.Warnings, "\n"), "stale") {
		t.Fatalf("edited view: state %q, warnings %v", report.Result.Consolidation.View, report.Result.Warnings)
	}
	stdout, _, code := runCommand(t, []string{"consolidate"})
	if code != 0 || !strings.Contains(stdout, "stale") {
		t.Fatalf("text report: exit %d\n%s", code, stdout)
	}
	writeRequirementsDocument(t, root, "desk-booking", "approved", consolidationBooking)
	current, _ := consolidate.LoadView(root)
	writeViewBody(t, root, current.Document.Body+completeCoherenceSection(t, root))
	if _, code := consolidationJSON(t, "consolidate", "open"); code != 0 {
		t.Fatal("reopening the edited view failed")
	}
	if _, code := consolidationJSON(t, "consolidate", "approve"); code != 0 {
		t.Fatal("approving the edited view failed")
	}
	if report = mustConsolidation(t); report.Result.Consolidation.View != consolidate.ViewApproved {
		t.Fatalf("after re-approval the view is %q", report.Result.Consolidation.View)
	}
}

func treeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		files[path] = string(mustRead(t, path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestConsolidateReportReadOnly(t *testing.T) {
	root := openConsolidationView(t)
	if _, code := consolidationJSON(t, "consolidate", "approve"); code != 0 {
		t.Fatal("approve failed")
	}
	ledger := filepath.Join(root, ".walden", "evidence", "desk-booking.json")
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledger, []byte("{\"schema\":\"sentinel\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".walden", "environment.md"), []byte("# Environment Probes\n\n- probe: [\"sh\", \"-c\", \"touch probe-ran\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := testutil.NewFakeRunner()
	previous := commandRunner
	commandRunner = runner
	t.Cleanup(func() { commandRunner = previous })

	before := treeBytes(t, root)
	for _, args := range [][]string{{"consolidate"}, {"consolidate", "--json"}, {"consolidate", "report", "--json"}} {
		if _, _, code := runCommand(t, args); code != 0 {
			t.Fatalf("walden %v exited %d", args, code)
		}
	}
	after := treeBytes(t, root)
	if len(before) != len(after) {
		t.Fatalf("the report created or removed files: %d before, %d after", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("the report modified %s", path)
		}
	}
	if calls := runner.Calls(); len(calls) != 0 {
		t.Fatalf("the report executed commands: %+v", calls)
	}
}

func TestConsolidateHelp(t *testing.T) {
	t.Chdir(t.TempDir())
	stdout, _, code := runCommand(t, []string{"--help"})
	for _, syntax := range []string{"consolidate report [--json]", "also plain walden consolidate", "consolidate start [--json]", "consolidate open [--json]", "consolidate approve [--json]"} {
		if code != 0 || !strings.Contains(stdout, syntax) {
			t.Errorf("global help does not list %q", syntax)
		}
	}
	stdout, _, code = runCommand(t, []string{"consolidate", "--help"})
	if code != 0 || !strings.Contains(stdout, "consolidate start [--json]") || !strings.Contains(stdout, "Subcommands:") {
		t.Errorf("consolidate --help:\n%s", stdout)
	}
	if _, stderr, code := runCommand(t, []string{"consolidate", "bogus"}); code == 0 || !strings.Contains(stderr, "unknown command") {
		t.Errorf("unknown subcommand accepted: exit %d, %s", code, stderr)
	}

	// The help is the agent's map of the cycle: the group describes it in
	// order and each subcommand states its preconditions, effects, refusals
	// and next step (R7.AC1, R7.AC2).
	t.Run("the help describes the cycle", func(t *testing.T) {
		help := func(args ...string) string {
			t.Helper()
			stdout, stderr, code := runCommand(t, append(args, "--help"))
			if code != 0 {
				t.Fatalf("walden %v --help: exit %d\n%s", args, code, stderr)
			}
			return stdout
		}
		require := func(name, text string, fragments ...string) {
			t.Helper()
			for _, fragment := range fragments {
				if !strings.Contains(text, fragment) {
					t.Errorf("%s help does not mention %q", name, fragment)
				}
			}
		}

		group := help("consolidate")
		order := []string{"walden status suggests a consolidation", "walden consolidate (read-only)", "## Coherence Review", "walden consolidate open",
			"walden consolidate approve, only after the user's explicit approval"}
		last := -1
		for _, step := range order {
			index := strings.Index(group, step)
			if index < 0 || index < last {
				t.Errorf("consolidate help does not describe %q in cycle order", step)
			}
			last = index
		}
		require("consolidate", group, "comparisons", "`feature#ID` in inline code", "walden consolidate start",
			"Never edit .walden/consolidation.json or the view frontmatter by hand")

		require("consolidate report", help("consolidate", "report"), "Read-only", "comparisons", "coherence-review", "next action")
		require("consolidate start", help("consolidate", "start"), "once per repository", "refused once tracking has started", ".walden/consolidation.json",
			".walden/contracts.md in draft")
		require("consolidate open", help("consolidate", "open"), "Refused, changing nothing", "omits a covered feature or identifier", "## Coherence Review",
			"`feature#ID` in inline code", "present the view to the user")
		require("consolidate approve", help("consolidate", "approve"), "only after the user's explicit approval", "the view in review",
			"a complete coherence review", "approved, fresh requirements", "a revision still in review must be approved first")
	})
}
