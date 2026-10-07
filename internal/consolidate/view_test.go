package consolidate

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

const cleanViewBody = "# Current Contracts\n\n" +
	"<!-- One section per consolidated feature. -->\n\n" +
	"## desk-booking\n\n" +
	"Purpose: Book one desk per holder per day.\n" +
	"Active: R1.AC1, R2.AC1, NFR1, C1\n" +
	"Reserved: R1.AC2 (removed by decision K5)\n" +
	"Sources: docs/decisions/K2-day-passes.md\n" +
	"Related: desk-occupancy\n\n" +
	"## desk-occupancy\n\n" +
	"Purpose: Report free desks per site and day.\n" +
	"Active: `R1.AC1`, `R1.AC2`, `C1`\n" +
	"Reserved: R1.AC3\n"

func viewPortfolio() (Snapshot, *Record) {
	snapshot := Snapshot{Features: []FeatureSnapshot{
		defining(approvedFeature("desk-booking", fp("booking-2")), "R1.AC1", "R2.AC1", "NFR1", "C1"),
		defining(approvedFeature("desk-occupancy", fp("occupancy-2")), "R1.AC1", "R1.AC2", "C1"),
	}}
	record := consolidatedRecord(map[string]RecordFeature{
		"desk-booking":   {State: StateConsolidated, RequirementsFingerprint: fp("booking-1"), ActiveIDs: []string{"R1.AC1", "R1.AC2", "R2.AC1", "NFR1", "C1"}},
		"desk-occupancy": {State: StateConsolidated, RequirementsFingerprint: fp("occupancy-1"), ActiveIDs: []string{"R1.AC1", "R1.AC2", "R1.AC3", "C1"}},
	})
	return snapshot, record
}

func mismatchKeys(findings []Finding) []string {
	keys := []string{}
	for _, finding := range findings {
		if finding.Kind != FindingViewMismatch || finding.Message == "" {
			keys = append(keys, "malformed finding "+finding.Kind)
			continue
		}
		keys = append(keys, finding.Feature+" "+finding.ID)
	}
	return keys
}

func TestViewMismatches(t *testing.T) {
	snapshot, record := viewPortfolio()
	view := View{Exists: true, Sections: ParseViewBody(cleanViewBody)}
	if got := CheckView(view, snapshot, record); len(got) != 0 {
		t.Fatalf("clean view reported mismatches: %+v", got)
	}
	if len(view.Sections) != 2 {
		t.Fatalf("view sections = %+v, want desk-booking and desk-occupancy", view.Sections)
	}
	if booking := view.Sections[0]; booking.Feature != "desk-booking" || !reflect.DeepEqual(booking.Reserved, []string{"R1.AC2"}) || !reflect.DeepEqual(booking.Related, []string{"desk-occupancy"}) {
		t.Fatalf("section not parsed: %+v", booking)
	}

	t.Run("criterion added without updating the view", func(t *testing.T) {
		snapshot, record := viewPortfolio()
		snapshot.Features[0] = defining(approvedFeature("desk-booking", fp("booking-3")), "R1.AC1", "R2.AC1", "R2.AC2", "NFR1", "C1")
		if got := mismatchKeys(CheckView(view, snapshot, record)); !reflect.DeepEqual(got, []string{"desk-booking R2.AC2"}) {
			t.Fatalf("mismatches = %v, want one naming desk-booking R2.AC2", got)
		}
	})

	t.Run("retired feature kept in the view", func(t *testing.T) {
		snapshot, record := viewPortfolio()
		retired := View{Exists: true, Sections: ParseViewBody(cleanViewBody + "\n## retired-feature\n\nPurpose: Gone.\nActive: R1.AC1\n")}
		if got := mismatchKeys(CheckView(retired, snapshot, record)); !reflect.DeepEqual(got, []string{"retired-feature "}) {
			t.Fatalf("mismatches = %v, want one naming retired-feature", got)
		}
	})

	t.Run("reservation kept across consolidations", func(t *testing.T) {
		snapshot, record := viewPortfolio()
		booking := record.Features["desk-booking"]
		booking.ActiveIDs = []string{"R1.AC1", "R2.AC1", "NFR1", "C1"}
		booking.ReservedIDs = []string{"R1.AC2"}
		record.Features["desk-booking"] = booking
		forgotten := View{Exists: true, Sections: ParseViewBody(strings.Replace(cleanViewBody, "Reserved: R1.AC2 (removed by decision K5)\n", "", 1))}
		if got := mismatchKeys(CheckView(forgotten, snapshot, record)); !reflect.DeepEqual(got, []string{"desk-booking R1.AC2"}) {
			t.Fatalf("mismatches = %v, want one naming the dropped reservation desk-booking R1.AC2", got)
		}
	})

	t.Run("every mismatch kind", func(t *testing.T) {
		snapshot, record := viewPortfolio()
		snapshot.Features = append(snapshot.Features, defining(approvedFeature("member-invoicing", fp("invoicing")), "R1.AC1"))
		record.Features["member-invoicing"] = RecordFeature{State: StateConsolidated, RequirementsFingerprint: fp("invoicing"), ActiveIDs: []string{"R1.AC1"}}
		body := "## desk-booking\n\nPurpose: Book desks.\nActive: R1.AC1, NFR1, C1, R9.AC9, bogus\nReserved: R1.AC2, R2.AC1\n\n" +
			"## desk-occupancy\n\nActive: R1.AC1, R1.AC2, C1\n\n" +
			"## desk-occupancy\n\nPurpose: Duplicate.\nActive: R1.AC1, R1.AC2, C1\nReserved: R1.AC3\n"
		got := mismatchKeys(CheckView(View{Exists: true, Sections: ParseViewBody(body)}, snapshot, record))
		want := []string{
			"desk-booking R2.AC1",   // defined, but missing from Active
			"desk-booking R9.AC9",   // listed as active but undefined
			"desk-booking bogus",    // not an identifier
			"desk-booking R2.AC1",   // reserved, but defined
			"desk-occupancy ",       // no purpose
			"desk-occupancy R1.AC3", // dropped since the checkpoint, not reserved
			"desk-occupancy ",       // duplicate section
			"member-invoicing ",     // consolidated feature without a section
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("mismatches =\n%v\nwant\n%v", got, want)
		}
	})
}

func writeView(t *testing.T, root, status, body string, approvedBody string) {
	t.Helper()
	path := filepath.Join(root, ViewPath)
	fields := map[string]string{"status": status, "approved_at": "", "last_modified": "2026-10-04T08:00:00Z", "approved_fingerprint": ""}
	if approvedBody != "" {
		fields["approved_at"] = "2026-10-04T08:00:00Z"
		fields["approved_fingerprint"] = spec.Fingerprint(path, approvedBody)
	}
	if err := spec.SaveDocument(spec.Document{Path: path, Fields: fields, Body: body}); err != nil {
		t.Fatal(err)
	}
}

func TestViewStaleness(t *testing.T) {
	root := t.TempDir()
	view, err := LoadView(root)
	if err != nil || view.Exists || view.Status().State != ViewAbsent {
		t.Fatalf("absent view = %+v, %v", view, err)
	}

	cases := []struct {
		name, status, body, approvedBody, want string
	}{
		{"draft", "draft", cleanViewBody, "", ViewDraft},
		{"in review", "in-review", cleanViewBody, "", ViewInReview},
		{"approved", "approved", cleanViewBody, cleanViewBody, ViewApproved},
		{"edited after approval", "approved", strings.Replace(cleanViewBody, "Book one desk", "Book desks", 1), cleanViewBody, ViewStale},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeView(t, root, tc.status, tc.body, tc.approvedBody)
			view, err := LoadView(root)
			if err != nil {
				t.Fatal(err)
			}
			status := view.Status()
			if status.State != tc.want {
				t.Fatalf("view state = %q, want %q", status.State, tc.want)
			}
			if tc.approvedBody != "" && status.ApprovedFingerprint != spec.Fingerprint(filepath.Join(root, ViewPath), tc.approvedBody) {
				t.Errorf("approved fingerprint = %q", status.ApprovedFingerprint)
			}
			if len(view.Sections) != 2 {
				t.Errorf("sections = %+v", view.Sections)
			}
		})
	}

	if err := os.WriteFile(filepath.Join(root, ViewPath), []byte("---\nstatus: approved\nsurprise: 1\n---\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadView(root); err == nil || !strings.Contains(err.Error(), "contracts.md") {
		t.Fatalf("malformed view frontmatter error = %v, want one naming the view", err)
	}
}
