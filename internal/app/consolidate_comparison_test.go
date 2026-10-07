package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/consolidate"
)

type comparisonEnvelope struct {
	Result struct {
		Consolidation *struct {
			Comparisons []struct {
				Feature    string `json:"feature"`
				Kind       string `json:"kind"`
				Statements []struct{ ID, Text, Mark, Previous string }
				Linked     []struct {
					Feature    string `json:"feature"`
					HubOnly    bool   `json:"hub_only"`
					Statements []struct{ ID, Text, Mark string }
				} `json:"linked"`
			} `json:"comparisons"`
		} `json:"consolidation"`
	} `json:"result"`
}

func TestConsolidateReportComparisons(t *testing.T) {
	root := startedConsolidationRepo(t)
	record, err := consolidate.LoadRecord(root)
	if err != nil || record.Features["desk-booking"].Statements["R1.AC2"] != "WHEN a day-pass holder books, the system SHALL confirm it." {
		t.Fatalf("start did not record statement texts: %+v, %v", record.Features["desk-booking"], err)
	}

	revised := strings.Replace(consolidationBooking, "WHEN a member books a free desk, the system SHALL confirm it", "WHEN a member books a free desk up to 14 days ahead, the system SHALL confirm it", 1)
	revised = strings.Replace(revised, "2. `R1.AC2` WHEN a day-pass holder books, the system SHALL confirm it.\n   - Acceptance check: a day-pass booking is confirmed.\n", "", 1)
	writeRequirementsDocument(t, root, "desk-booking", "approved", revised)
	stdout, _, code := runCommand(t, []string{"consolidate", "--json"})
	var envelope comparisonEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil || code != 0 || envelope.Result.Consolidation == nil {
		t.Fatalf("report: exit %d, %v\n%s", code, err, stdout)
	}
	var booking *struct {
		Feature    string `json:"feature"`
		Kind       string `json:"kind"`
		Statements []struct{ ID, Text, Mark, Previous string }
		Linked     []struct {
			Feature    string `json:"feature"`
			HubOnly    bool   `json:"hub_only"`
			Statements []struct{ ID, Text, Mark string }
		} `json:"linked"`
	}
	for i := range envelope.Result.Consolidation.Comparisons {
		if envelope.Result.Consolidation.Comparisons[i].Feature == "desk-booking" {
			booking = &envelope.Result.Consolidation.Comparisons[i]
		}
	}
	if booking == nil || booking.Kind != consolidate.ChangeRevised {
		t.Fatalf("no comparison for desk-booking: %+v", envelope.Result.Consolidation.Comparisons)
	}
	byID := map[string]string{}
	previous := map[string]string{}
	for _, statement := range booking.Statements {
		byID[statement.ID] = statement.Mark + ": " + statement.Text
		previous[statement.ID] = statement.Previous
	}
	if want := "WHEN a member books a free desk, the system SHALL confirm it while `desk-occupancy#R1.AC2` holds."; previous["R1.AC1"] != want ||
		previous["R1.AC2"] != "" || previous["R2.AC1"] != "" {
		t.Fatalf("previous texts = %#v, want only R1.AC1 = %q", previous, want)
	}
	if !strings.HasPrefix(byID["R1.AC1"], consolidate.MarkChanged+": WHEN a member books a free desk up to 14 days ahead") ||
		byID["R1.AC2"] != consolidate.MarkRemoved+": WHEN a day-pass holder books, the system SHALL confirm it." ||
		!strings.HasPrefix(byID["R2.AC1"], consolidate.MarkUnchanged+": ") {
		t.Fatalf("desk-booking statements = %#v", byID)
	}
	linked := map[string]int{}
	for _, entry := range booking.Linked {
		linked[entry.Feature] = len(entry.Statements)
	}
	if linked["desk-occupancy"] != 3 || linked["member-invoicing"] != 1 {
		t.Fatalf("linked statements = %v, want desk-occupancy (3) and member-invoicing (1)", linked)
	}

	text, _, code := runCommand(t, []string{"consolidate"})
	if code != 0 || !strings.Contains(text, "Comparisons:") || !strings.Contains(text, "[removed] R1.AC2 WHEN a day-pass holder books") ||
		!strings.Contains(text, "was: WHEN a member books a free desk, the system SHALL confirm it while") ||
		!strings.Contains(text, "desk-occupancy") {
		t.Fatalf("text report:\n%s", text)
	}

	writeViewBody(t, root, "# Current Contracts\n\n## desk-booking\n\nPurpose: Book one desk per holder per day.\nActive: R1.AC1, R2.AC1, NFR1, C1, C2\nReserved: R1.AC2\n"+completeCoherenceSection(t, root))
	if _, code := consolidationJSON(t, "consolidate", "open"); code != 0 {
		t.Fatal("open failed")
	}
	if _, code := consolidationJSON(t, "consolidate", "approve"); code != 0 {
		t.Fatal("approve failed")
	}
	record, _ = consolidate.LoadRecord(root)
	if got := record.Features["desk-booking"].Statements["R1.AC1"]; !strings.Contains(got, "up to 14 days ahead") {
		t.Fatalf("approval did not record the current statement texts: %q", got)
	}
	if _, kept := record.Features["desk-booking"].Statements["R1.AC2"]; kept {
		t.Fatal("approval kept the text of a statement that is no longer active")
	}
}
