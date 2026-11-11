package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
)

const viewCommit = "0123456789abcdef0123456789abcdef01234567"

func deliveryEntry(id, state string) evidence.TaskEvidence {
	return evidence.TaskEvidence{
		TaskID: id, State: state, CodeFreshness: "stale", RecordedResult: evidence.ResultPassed,
		Binding:   evidence.BindingAssessment{State: "current"},
		Execution: evidence.ExecutionAssessment{State: "supported"},
	}
}

// TestEvidenceViewDelivery holds the three fields an entry gains and the
// forms its text line takes.
func TestEvidenceViewDelivery(t *testing.T) {
	delivered := deliveryEntry("3", evidence.StateDelivered)
	delivered.Delivery, delivered.Attestation = true, &evidence.AttestedCommit{Commit: viewCommit}
	sealed := deliveryEntry("4", evidence.StateDelivered)
	sealed.Delivery, sealed.Attestation = true, &evidence.AttestedCommit{Commit: viewCommit, Sealed: true}
	unattested := deliveryEntry("5", evidence.StateVerified)
	unattested.Delivery = true
	unattested.Explanation = &evidence.Explanation{Attestation: &evidence.AttestationExplanation{
		Unavailable: evidence.ReasonUncommittedPaths, Detail: "the proof ran on a tree with 2 uncommitted path(s); a remedy",
	}}
	awaiting := deliveryEntry("6", evidence.StateStaleSpec)
	awaiting.Delivery, awaiting.AwaitsAdoption = true, true
	regression := deliveryEntry("7", evidence.StateVerified)

	views := []EvidenceStatus{}
	for _, entry := range []evidence.TaskEvidence{delivered, sealed, unattested, awaiting, regression} {
		views = append(views, EvidenceView(entry))
	}

	// JSON: additive fields, absent where they say nothing.
	fields := func(view EvidenceStatus) map[string]any {
		t.Helper()
		data, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		return decoded
	}
	attestation := func(view EvidenceStatus) map[string]any {
		value, _ := fields(view)["attestation"].(map[string]any)
		return value
	}
	if got := fields(views[0]); got["delivery"] != true || got["state"] != "delivered" {
		t.Errorf("delivered entry: %v", got)
	}
	if got := attestation(views[0]); got["commit"] != viewCommit || got["sealed"] != false {
		t.Errorf("attestation written by an execution: %v, want the commit and sealed=false", got)
	}
	if got := attestation(views[1]); got["commit"] != viewCommit || got["sealed"] != true {
		t.Errorf("sealed attestation: %v", got)
	}
	if got := fields(views[2]); got["delivery"] != true || got["attestation"] != nil || got["awaits_adoption"] != nil {
		t.Errorf("unattested delivery entry: %v", got)
	}
	if got := fields(views[3]); got["delivery"] != true || got["awaits_adoption"] != true {
		t.Errorf("awaiting entry: %v", got)
	}
	plain := fields(views[4])
	for _, key := range []string{"delivery", "attestation", "awaits_adoption"} {
		if _, present := plain[key]; present {
			t.Errorf("a regression entry carries %q: %v", key, plain)
		}
	}

	// Text.
	var out bytes.Buffer
	printEvidence(&out, views, "")
	text := out.String()
	for _, want := range []string{
		"- 3: delivered at " + viewCommit + " [binding=current code=stale execution=supported]\n",
		"- 4: delivered at " + viewCommit + " (sealed by adoption) [binding=current code=stale execution=supported]\n",
		"- 5: verified (delivery proof) [binding=current code=stale execution=supported]\n",
		"  not attested: the proof ran on a tree with 2 uncommitted path(s); a remedy\n",
		"- 6: stale-spec (delivery proof, awaits adoption) [binding=current code=stale execution=supported]\n",
		"- 7: verified [binding=current code=stale execution=supported]\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the text does not hold %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "sealed by adoption") != 1 || strings.Count(text, "delivery proof") != 2 {
		t.Errorf("marks on the wrong entries:\n%s", text)
	}
}
