package app

import (
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/consolidate"
)

// TestConsolidationViewScaffold pins the example coherence review in the view
// created by start, and checks that the commented example never counts as a
// real review entry.
func TestConsolidationViewScaffold(t *testing.T) {
	root := startedConsolidationRepo(t)
	view, err := consolidate.LoadView(root)
	if err != nil {
		t.Fatal(err)
	}
	scaffold := view.Document.Body
	for _, fragment := range []string{"## Coherence Review", "### feature-name", "`other-feature#R1.AC2`"} {
		if !strings.Contains(scaffold, fragment) {
			t.Errorf("the created view does not show %q", fragment)
		}
	}

	revised := strings.Replace(consolidationBooking, "WHEN a member books a free desk, the system SHALL confirm it", "WHEN a member books a free desk up to 14 days ahead, the system SHALL confirm it", 1)
	writeRequirementsDocument(t, root, "desk-booking", "approved", revised)
	sections := "\n## desk-booking\n\nPurpose: Book one desk per holder per day.\nActive: R1.AC1, R1.AC2, R2.AC1, NFR1, C1, C2\nRelated: desk-occupancy\n"
	writeViewBody(t, root, scaffold+sections)
	refused, code := consolidationJSON(t, "consolidate", "open")
	if code == 0 || strings.Contains(strings.Join(refused.Result.Blockers, "\n"), "feature-name") {
		t.Fatalf("open with only the commented example: exit %d, blockers %v", code, refused.Result.Blockers)
	}
	writeViewBody(t, root, scaffold+sections+completeCoherenceSection(t, root))
	if opened, code := consolidationJSON(t, "consolidate", "open"); code != 0 {
		t.Fatalf("open with the example comment and a complete review: exit %d, blockers %v", code, opened.Result.Blockers)
	}
}
