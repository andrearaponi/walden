package consolidate

import (
	"reflect"
	"testing"
)

func TestStatementTexts(t *testing.T) {
	got := StatementTexts(definitionsBody)
	want := map[string]string{
		"R1.AC1": "WHEN a member books a free desk, the system SHALL confirm it while `desk-occupancy#R1.AC2` holds (bridged by `NFR1`).",
		"R1.AC2": "WHEN a holder cancels, the system SHALL release the desk.",
		"R2.AC1": "The system SHALL keep one record per booking.",
		"NFR1":   "Privacy: no names in records (bridged by `R2.AC1`).",
		"C1":     "Standard library only; run `walden consolidate`, keep `go.mod`, ignore `https://example.com/a.md`, `/abs/path.md`, `../outside.md`, `cmd/walden@v0.10.4` and `docs/decisions/`.",
		"C2":     "Bookable hours follow decision K9 (`docs/decisions/K9-opening-hours.md`), cited again as `docs/decisions/K9-opening-hours.md`.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statement texts =\n%#v\nwant\n%#v", got, want)
	}

	root := t.TempDir()
	writeRequirements(t, root, "alpha", "approved", definitionsBody, true)
	snapshot, err := LoadSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if alpha, _ := snapshot.Feature("alpha"); !reflect.DeepEqual(alpha.Statements, want) {
		t.Fatalf("snapshot statements = %#v", alpha.Statements)
	}
}

func withStatements(feature FeatureSnapshot, statements map[string]string) FeatureSnapshot {
	feature.Statements = statements
	for id := range statements {
		switch {
		case len(id) > 3 && id[:3] == "NFR":
			feature.Definitions.NFRs = append(feature.Definitions.NFRs, id)
		case id[0] == 'C':
			feature.Definitions.Constraints = append(feature.Definitions.Constraints, id)
		default:
			feature.Definitions.Criteria = append(feature.Definitions.Criteria, id)
		}
	}
	sortIDs(feature.Definitions.Criteria)
	sortIDs(feature.Definitions.NFRs)
	sortIDs(feature.Definitions.Constraints)
	return feature
}

func sortIDs(ids []string) {
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
}

// consolidateOthers records every feature except the pending ones as consolidated at its current version.
func consolidateOthers(record *Record, snapshot Snapshot, pending ...string) {
	for _, feature := range snapshot.Features {
		if !containsString(pending, feature.Name) {
			record.Features[feature.Name] = RecordFeature{State: StateConsolidated, RequirementsFingerprint: feature.ApprovedFingerprint, ActiveIDs: feature.Definitions.Active()}
		}
	}
}

func comparisonOf(comparisons []Comparison, feature string) (Comparison, bool) {
	for _, comparison := range comparisons {
		if comparison.Feature == feature {
			return comparison, true
		}
	}
	return Comparison{}, false
}

func marks(statements []ComparedStatement) map[string]string {
	out := map[string]string{}
	for _, statement := range statements {
		out[statement.ID] = statement.Mark + ": " + statement.Text
	}
	return out
}

func TestComparisonMaterial(t *testing.T) {
	hub := "docs/README.md"
	a := withStatements(approvedFeature("a", fp("a-2")), map[string]string{
		"R1.AC1": "WHEN a session runs at night, the system SHALL charge the night tariff.",
		"R1.AC2": "WHEN a session closes, the system SHALL record it.",
		"R1.AC3": "WHEN a session is extended, the system SHALL notify the driver.",
	})
	a.Citations = Citations{Files: []FileCitation{{Path: hub}}, References: []Reference{{Feature: "b", ID: "R1.AC2"}}}
	b := withStatements(approvedFeature("b", fp("b")), map[string]string{
		"R1.AC1": "WHEN a location is queried, the system SHALL return its zone.",
		"R1.AC2": "WHILE a zone is closed, the system SHALL treat parking as free.",
	})
	features := []FeatureSnapshot{a, b, withStatements(approvedFeature("d", fp("d")), map[string]string{"R1.AC1": "Unrelated."})}
	for _, name := range []string{"c", "h1", "h2", "h3"} {
		feature := withStatements(approvedFeature(name, fp(name)), map[string]string{"R1.AC1": "Hub citer " + name + "."})
		feature.Citations.Files = []FileCitation{{Path: hub}}
		features = append(features, feature)
	}
	snapshot := Snapshot{Features: sortedFeatures(features)}
	record := consolidatedRecord(map[string]RecordFeature{
		"a": {State: StateConsolidated, RequirementsFingerprint: fp("a-1"), ActiveIDs: []string{"R1.AC1", "R1.AC2"},
			Statements: map[string]string{"R1.AC1": "WHEN a session runs at night, the system SHALL treat it as free.", "R1.AC2": "WHEN a session closes, the system SHALL record it."}},
	})
	consolidateOthers(record, snapshot, "a")
	state := Derive(snapshot, record, nil, approvedView)
	if len(state.Pending) != 1 || state.Pending[0].Feature != "a" {
		t.Fatalf("pending = %+v, want only a", state.Pending)
	}
	scope, _ := Scope(snapshot, state)
	comparison, ok := comparisonOf(Comparisons(snapshot, record, state, scope), "a")
	if !ok || comparison.Kind != ChangeRevised {
		t.Fatalf("comparison of a missing: %+v", comparison)
	}
	want := map[string]string{
		"R1.AC1": MarkChanged + ": WHEN a session runs at night, the system SHALL charge the night tariff.",
		"R1.AC2": MarkUnchanged + ": WHEN a session closes, the system SHALL record it.",
		"R1.AC3": MarkAdded + ": WHEN a session is extended, the system SHALL notify the driver.",
	}
	if got := marks(comparison.Statements); !reflect.DeepEqual(got, want) {
		t.Fatalf("statements of a =\n%#v\nwant\n%#v", got, want)
	}
	linked := map[string]LinkedStatements{}
	for _, entry := range comparison.Linked {
		linked[entry.Feature] = entry
	}
	if len(linked["b"].Statements) != 2 || linked["b"].HubOnly || linked["b"].Statements[1].Text != "WHILE a zone is closed, the system SHALL treat parking as free." {
		t.Fatalf("linked b = %+v, want both statements with text", linked["b"])
	}
	for _, name := range []string{"c", "h1", "h2", "h3"} {
		if entry, ok := linked[name]; !ok || !entry.HubOnly || len(entry.Statements) != 0 {
			t.Fatalf("hub-only %s = %+v (present %v), want named without statements", name, entry, ok)
		}
	}
	if _, ok := linked["d"]; ok {
		t.Fatal("an unrelated feature appears in the comparison")
	}

	t.Run("statements recorded without text are unverified", func(t *testing.T) {
		record := consolidatedRecord(map[string]RecordFeature{"a": {State: StateConsolidated, RequirementsFingerprint: fp("a-1"), ActiveIDs: []string{"R1.AC1", "R1.AC2"}}})
		consolidateOthers(record, snapshot, "a")
		state := Derive(snapshot, record, nil, approvedView)
		scope, _ := Scope(snapshot, state)
		comparison, _ := comparisonOf(Comparisons(snapshot, record, state, scope), "a")
		got := marks(comparison.Statements)
		if got["R1.AC1"][:len(MarkUnverified)] != MarkUnverified || got["R1.AC2"][:len(MarkUnverified)] != MarkUnverified || got["R1.AC3"][:len(MarkAdded)] != MarkAdded {
			t.Fatalf("marks without recorded texts = %#v", got)
		}
	})
}

func TestRemovedStatementsWithText(t *testing.T) {
	current := withStatements(approvedFeature("payments", fp("payments-2")), map[string]string{
		"R1.AC1": "WHEN a priced session closes, the system SHALL charge the default payment method.",
	})
	notifications := withStatements(approvedFeature("notifications", fp("notifications")), map[string]string{
		"R1.AC2": "WHEN a charge retry fails, the system SHALL notify the driver.",
	})
	notifications.Citations.References = []Reference{{Feature: "payments", ID: "R1.AC1"}, {Feature: "gone", ID: "R1.AC1"}}
	snapshot := Snapshot{Features: []FeatureSnapshot{notifications, current}}
	retry := "WHILE a session is unpaid, the system SHALL retry the charge once a day for 3 days."
	record := consolidatedRecord(map[string]RecordFeature{
		"payments": {State: StateConsolidated, RequirementsFingerprint: fp("payments-1"), ActiveIDs: []string{"R1.AC1", "R1.AC3"},
			Statements: map[string]string{"R1.AC1": "WHEN a priced session closes, the system SHALL charge the default payment method.", "R1.AC3": retry}},
		"notifications": {State: StateConsolidated, RequirementsFingerprint: fp("notifications"), ActiveIDs: []string{"R1.AC2"}},
		"gone":          {State: StateConsolidated, RequirementsFingerprint: fp("gone"), ActiveIDs: []string{"R1.AC1"}, Statements: map[string]string{"R1.AC1": "WHEN asked, the system SHALL answer."}},
	})
	state := Derive(snapshot, record, nil, approvedView)
	scope, _ := Scope(snapshot, state)
	comparisons := Comparisons(snapshot, record, state, scope)

	payments, _ := comparisonOf(comparisons, "payments")
	if got := marks(payments.Statements)["R1.AC3"]; got != MarkRemoved+": "+retry {
		t.Fatalf("removed statement = %q, want it with its recorded text", got)
	}
	if len(payments.Linked) != 1 || payments.Linked[0].Feature != "notifications" || len(payments.Linked[0].Statements) != 1 {
		t.Fatalf("payments linked = %+v, want notifications with its statement", payments.Linked)
	}
	gone, ok := comparisonOf(comparisons, "gone")
	if !ok || gone.Kind != ChangeRemoved || marks(gone.Statements)["R1.AC1"] != MarkRemoved+": WHEN asked, the system SHALL answer." {
		t.Fatalf("removed feature comparison = %+v", gone)
	}

	t.Run("record without texts", func(t *testing.T) {
		bare := consolidatedRecord(map[string]RecordFeature{
			"payments":      {State: StateConsolidated, RequirementsFingerprint: fp("payments-1"), ActiveIDs: []string{"R1.AC1", "R1.AC3"}},
			"notifications": {State: StateConsolidated, RequirementsFingerprint: fp("notifications"), ActiveIDs: []string{"R1.AC2"}},
		})
		state := Derive(snapshot, bare, nil, approvedView)
		scope, _ := Scope(snapshot, state)
		payments, _ := comparisonOf(Comparisons(snapshot, bare, state, scope), "payments")
		if got := marks(payments.Statements)["R1.AC3"]; got != MarkRemoved+": " {
			t.Fatalf("removed statement without recorded text = %q", got)
		}
	})
}

// TestComparisonPreviousText pins the recorded text next to every changed
// statement, and only there, so a dependent can be checked against what changed.
func TestComparisonPreviousText(t *testing.T) {
	before := "WHEN a fine is issued, the system SHALL set its payment deadline at 30 days."
	after := "WHEN a fine is issued, the system SHALL set its payment deadline at 14 days."
	fines := withStatements(approvedFeature("fines", fp("fines-2")), map[string]string{
		"R1.AC1": after,
		"R1.AC2": "WHEN a fine is paid, the system SHALL close it.",
		"R1.AC3": "WHEN a fine is overdue, the system SHALL notify the driver.",
		"R1.AC4": "WHEN a fine is cancelled, the system SHALL record why.",
	})
	snapshot := Snapshot{Features: sortedFeatures([]FeatureSnapshot{fines, withStatements(approvedFeature("appeals", fp("appeals")), map[string]string{"R1.AC1": "Appeals."})})}
	record := consolidatedRecord(map[string]RecordFeature{
		"fines": {State: StateConsolidated, RequirementsFingerprint: fp("fines-1"), ActiveIDs: []string{"R1.AC1", "R1.AC2", "R1.AC4", "R1.AC5"},
			Statements: map[string]string{"R1.AC1": before, "R1.AC2": "WHEN a fine is paid, the system SHALL close it.", "R1.AC5": "WHEN a fine is disputed, the system SHALL hold it."}},
	})
	consolidateOthers(record, snapshot, "fines")
	state := Derive(snapshot, record, nil, approvedView)
	scope, _ := Scope(snapshot, state)
	comparison, ok := comparisonOf(Comparisons(snapshot, record, state, scope), "fines")
	if !ok {
		t.Fatal("no comparison for fines")
	}
	got := map[string]string{}
	for _, statement := range comparison.Statements {
		got[statement.ID] = statement.Mark + " | " + statement.Previous
	}
	want := map[string]string{
		"R1.AC1": MarkChanged + " | " + before,
		"R1.AC2": MarkUnchanged + " | ",
		"R1.AC3": MarkAdded + " | ",
		"R1.AC4": MarkUnverified + " | ",
		"R1.AC5": MarkRemoved + " | ",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("marks and previous texts =\n%#v\nwant\n%#v", got, want)
	}
}
