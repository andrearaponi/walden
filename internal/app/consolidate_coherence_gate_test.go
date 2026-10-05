package app

import (
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/consolidate"
)

// TestConsolidateCoherenceGate pins the CLI-checked coherence review: while
// changes are pending, the view opens and is approved only with a non-empty
// entry per pending feature that cites a linked statement of its comparison.
func TestConsolidateCoherenceGate(t *testing.T) {
	root := startedConsolidationRepo(t)
	revised := strings.Replace(consolidationBooking, "WHEN a member books a free desk, the system SHALL confirm it", "WHEN a member books a free desk up to 14 days ahead, the system SHALL confirm it", 1)
	writeRequirementsDocument(t, root, "desk-booking", "approved", revised)
	contracts := "# Current Contracts\n\n## desk-booking\n\nPurpose: Book one desk per holder per day.\nActive: R1.AC1, R1.AC2, R2.AC1, NFR1, C1, C2\nRelated: desk-occupancy\n"
	review := func(entry string) string {
		return contracts + "\n" + consolidate.CoherenceHeading + "\n\n### desk-booking\n\n" + entry + "\n"
	}
	blockers := func(envelope consolidationEnvelope) string { return strings.Join(envelope.Result.Blockers, "\n") }

	report, code := consolidationJSON(t, "consolidate")
	listed := false
	for _, finding := range report.Result.Consolidation.Findings {
		listed = listed || (finding.Kind == consolidate.FindingCoherenceReview && finding.Feature == "desk-booking")
	}
	if code != 0 || !listed {
		t.Fatalf("the report does not list the missing coherence entry: exit %d, findings %+v", code, report.Result.Consolidation.Findings)
	}

	writeViewBody(t, root, contracts)
	refused, code := consolidationJSON(t, "consolidate", "open")
	if code == 0 || refused.OK || !strings.Contains(blockers(refused), "desk-booking") {
		t.Fatalf("open without a coherence review: exit %d, blockers %v", code, refused.Result.Blockers)
	}
	if view, _ := consolidate.LoadView(root); view.Status().State == consolidate.ViewInReview {
		t.Fatal("a refused open moved the view to review")
	}

	writeViewBody(t, root, review("- No inconsistency found."))
	if refused, code := consolidationJSON(t, "consolidate", "open"); code == 0 || !strings.Contains(blockers(refused), "cites none") {
		t.Fatalf("open with an entry citing no linked statement: exit %d, blockers %v", code, refused.Result.Blockers)
	}

	complete := review("- The longer booking window agrees with `desk-occupancy#R1.AC2`.")
	writeViewBody(t, root, complete)
	if opened, code := consolidationJSON(t, "consolidate", "open"); code != 0 {
		t.Fatalf("open with a complete coherence review: exit %d, blockers %v", code, opened.Result.Blockers)
	}

	writeViewBody(t, root, contracts)
	if refused, code := consolidationJSON(t, "consolidate", "approve"); code == 0 || !strings.Contains(blockers(refused), "desk-booking") {
		t.Fatalf("approve after the coherence entry was removed: exit %d, blockers %v", code, refused.Result.Blockers)
	}
	if record, _ := consolidate.LoadRecord(root); record.Checkpoint != nil {
		t.Fatal("a refused approval recorded a checkpoint")
	}

	writeViewBody(t, root, complete)
	if approved, code := consolidationJSON(t, "consolidate", "approve"); code != 0 {
		t.Fatalf("approve with a complete coherence review: exit %d, blockers %v", code, approved.Result.Blockers)
	}
	after, _ := consolidationJSON(t, "consolidate")
	if len(after.Result.Consolidation.Pending) != 0 {
		t.Fatalf("pending after approval: %+v", after.Result.Consolidation.Pending)
	}

	writeViewBody(t, root, contracts+"\nA note.\n")
	if opened, code := consolidationJSON(t, "consolidate", "open"); code != 0 {
		t.Fatalf("with nothing pending the coherence review is not required: exit %d, blockers %v", code, opened.Result.Blockers)
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
