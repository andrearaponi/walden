package consolidate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func withCitations(feature FeatureSnapshot, files []string, references ...Reference) FeatureSnapshot {
	for _, path := range files {
		feature.Citations.Files = append(feature.Citations.Files, FileCitation{Path: path})
	}
	feature.Citations.References = append(feature.Citations.References, references...)
	return feature
}

func defining(feature FeatureSnapshot, ids ...string) FeatureSnapshot {
	feature.Definitions = DefinedIdentifiers(definitionLines(ids...))
	return feature
}

func definitionLines(ids ...string) string {
	body := ""
	for i, id := range ids {
		switch {
		case len(id) > 3 && id[:3] == "NFR":
			body += "- `" + id + "` Quality.\n"
		case id[0] == 'C':
			body += "- `" + id + "` Constraint.\n"
		case len(id) > 1 && !containsDot(id):
			body += "### " + id + " Title\n"
		default:
			body += string(rune('1'+i%9)) + ". `" + id + "` WHEN something happens, the system SHALL respond.\n"
		}
	}
	return body
}

func containsDot(value string) bool {
	for _, r := range value {
		if r == '.' {
			return true
		}
	}
	return false
}

func TestConsolidationScope(t *testing.T) {
	hub := "docs/README.md"
	features := []FeatureSnapshot{
		withCitations(approvedFeature("a", fp("a")), []string{"docs/decisions/D1.md", hub}, Reference{Feature: "b", ID: "R1.AC1"}),
		defining(approvedFeature("b", fp("b")), "R1.AC1"),
		withCitations(approvedFeature("c", fp("c")), nil, Reference{Feature: "a", ID: "R2.AC1"}),
		withCitations(approvedFeature("d", fp("d")), []string{"docs/decisions/D1.md"}),
		withCitations(approvedFeature("e", fp("e")), []string{"docs/other.md"}, Reference{Feature: "b", ID: "R1.AC1"}),
		withCitations(approvedFeature("y", fp("y")), nil, Reference{Feature: "z", ID: "R1.AC1"}),
	}
	for _, name := range []string{"h1", "h2", "h3", "h4"} {
		features = append(features, withCitations(approvedFeature(name, fp(name)), []string{hub}))
	}
	state := State{Tracking: TrackingActive, Pending: []PendingChange{{Feature: "a", Kind: ChangeNew}, {Feature: "z", Kind: ChangeRemoved}}}

	scope, hubs := Scope(Snapshot{Features: sortedFeatures(features)}, state)

	names := []string{}
	byName := map[string]ScopeEntry{}
	for _, entry := range scope {
		names = append(names, entry.Feature)
		byName[entry.Feature] = entry
	}
	if want := []string{"a", "z", "b", "c", "d", "h1", "h2", "h3", "h4", "y"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("scope = %v, want %v (e excluded)", names, want)
	}
	if byName["a"].Pending != ChangeNew || byName["z"].Pending != ChangeRemoved {
		t.Errorf("pending kinds lost: %+v %+v", byName["a"], byName["z"])
	}
	expectLink := func(feature string, link ScopeLink) {
		t.Helper()
		for _, got := range byName[feature].Links {
			if got == link {
				return
			}
		}
		t.Errorf("%s links = %+v, want %+v", feature, byName[feature].Links, link)
	}
	expectLink("b", ScopeLink{Kind: LinkCitedBy, Feature: "a"})
	expectLink("c", ScopeLink{Kind: LinkCites, Feature: "a"})
	expectLink("d", ScopeLink{Kind: LinkSharedFile, Feature: "a", Path: "docs/decisions/D1.md"})
	expectLink("y", ScopeLink{Kind: LinkCites, Feature: "z"})
	if !byName["h1"].HubOnly || byName["d"].HubOnly || byName["b"].HubOnly {
		t.Errorf("hub-only marking wrong: h1=%v d=%v b=%v", byName["h1"].HubOnly, byName["d"].HubOnly, byName["b"].HubOnly)
	}
	if want := []Hub{{Path: hub, Features: []string{"a", "h1", "h2", "h3", "h4"}}}; !reflect.DeepEqual(hubs, want) {
		t.Errorf("hubs = %+v, want %+v", hubs, want)
	}
}

func sortedFeatures(features []FeatureSnapshot) []FeatureSnapshot {
	sorted := append([]FeatureSnapshot{}, features...)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Name < sorted[i].Name {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

func TestMissingCitedFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "decisions", "K2-day-passes.md"), []byte("# K2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	booking := approvedFeature("desk-booking", fp("booking"))
	booking.Citations.Files = []FileCitation{{Path: "docs/decisions/K2-day-passes.md"}, {Path: "docs/decisions/K9-opening-hours.md", CitedBy: "C3"}}
	draft := FeatureSnapshot{Name: "waitlist", HasRequirements: true, Status: "draft"}
	draft.Citations.Files = []FileCitation{{Path: "docs/notes/waitlist.md", CitedBy: "R1.AC1"}}

	got := MissingCitedFiles(root, Snapshot{Features: []FeatureSnapshot{booking, draft}})
	want := []Finding{
		{Kind: FindingMissingFile, Feature: "desk-booking", ID: "C3", Subject: "docs/decisions/K9-opening-hours.md"},
		{Kind: FindingMissingFile, Feature: "waitlist", ID: "R1.AC1", Subject: "docs/notes/waitlist.md"},
	}
	if !reflect.DeepEqual(stripMessages(got), want) {
		t.Fatalf("missing files = %+v, want %+v", got, want)
	}
	for _, finding := range got {
		if finding.Message == "" {
			t.Errorf("finding without a message: %+v", finding)
		}
	}
}

func stripMessages(findings []Finding) []Finding {
	stripped := []Finding{}
	for _, finding := range findings {
		finding.Message = ""
		stripped = append(stripped, finding)
	}
	return stripped
}

func TestDanglingQualifiedReferences(t *testing.T) {
	occupancy := defining(approvedFeature("desk-occupancy", fp("occupancy")), "R1", "R1.AC1", "R1.AC2", "NFR1")
	booking := withCitations(approvedFeature("desk-booking", fp("booking")), nil,
		Reference{Feature: "desk-occupancy", ID: "R1.AC2", CitedBy: "R3.AC1"},
		Reference{Feature: "desk-occupancy", ID: "R1"},
		Reference{Feature: "desk-occupancy", ID: "R1.AC3", CitedBy: "R3.AC1"},
		Reference{Feature: "retired-feature", ID: "R1.AC1"},
	)
	got := DanglingReferences(Snapshot{Features: []FeatureSnapshot{booking, occupancy}})
	want := []Finding{
		{Kind: FindingDanglingReference, Feature: "desk-booking", ID: "R3.AC1", Subject: "desk-occupancy#R1.AC3"},
		{Kind: FindingDanglingReference, Feature: "desk-booking", Subject: "retired-feature#R1.AC1"},
	}
	if !reflect.DeepEqual(stripMessages(got), want) {
		t.Fatalf("dangling references = %+v, want %+v", got, want)
	}
}

func TestReservedIdentifierReuse(t *testing.T) {
	record := consolidatedRecord(map[string]RecordFeature{
		"desk-booking": {State: StateConsolidated, RequirementsFingerprint: fp("booking-1"), ActiveIDs: []string{"R1.AC1", "R2.AC1"}, ReservedIDs: []string{"R1.AC2"}},
	})
	reused := defining(approvedFeature("desk-booking", fp("booking-2")), "R1.AC1", "R1.AC2", "R2.AC1")
	got := ReservedReuse(Snapshot{Features: []FeatureSnapshot{reused}}, record)
	if want := []Finding{{Kind: FindingReservedReuse, Feature: "desk-booking", ID: "R1.AC2"}}; !reflect.DeepEqual(stripMessages(got), want) {
		t.Fatalf("reserved reuse = %+v, want %+v", got, want)
	}
	extended := defining(approvedFeature("desk-booking", fp("booking-3")), "R1.AC1", "R1.AC3", "R2.AC1")
	if got := ReservedReuse(Snapshot{Features: []FeatureSnapshot{extended}}, record); len(got) != 0 {
		t.Fatalf("a new identifier was reported as reuse: %+v", got)
	}
	if got := ReservedReuse(Snapshot{Features: []FeatureSnapshot{reused}}, nil); len(got) != 0 {
		t.Fatalf("reuse reported without a record: %+v", got)
	}
}
