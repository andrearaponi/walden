package consolidate

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// coherenceFixture has two pending features: a, revised and citing b#R1.AC2,
// so its comparison shows b's statements; n, new and unlinked.
func coherenceFixture(t *testing.T) (State, []Comparison) {
	t.Helper()
	a := withStatements(approvedFeature("a", fp("a-2")), map[string]string{"R1.AC1": "WHEN a session closes, the system SHALL charge it."})
	a.Citations = Citations{References: []Reference{{Feature: "b", ID: "R1.AC2"}}}
	b := withStatements(approvedFeature("b", fp("b")), map[string]string{"R1.AC1": "B one.", "R1.AC2": "B two."})
	c := withStatements(approvedFeature("c", fp("c")), map[string]string{"R1.AC1": "C one."})
	n := withStatements(approvedFeature("n", fp("n")), map[string]string{"R1.AC1": "N one."})
	snapshot := Snapshot{Features: sortedFeatures([]FeatureSnapshot{a, b, c, n})}
	record := consolidatedRecord(map[string]RecordFeature{
		"a": {State: StateConsolidated, RequirementsFingerprint: fp("a-1"), ActiveIDs: []string{"R1.AC1"},
			Statements: map[string]string{"R1.AC1": "WHEN a session closes, the system SHALL record it."}},
	})
	consolidateOthers(record, snapshot, "a", "n")
	state := Derive(snapshot, record, nil, approvedView)
	if got := pendingFeatures(state); strings.Join(got, ",") != "a,n" {
		t.Fatalf("pending = %v, want a and n", got)
	}
	scope, _ := Scope(snapshot, state)
	return state, Comparisons(snapshot, record, state, scope)
}

func pendingFeatures(state State) []string {
	names := []string{}
	for _, change := range state.Pending {
		names = append(names, change.Feature)
	}
	sort.Strings(names)
	return names
}

func TestCoherenceReview(t *testing.T) {
	state, comparisons := coherenceFixture(t)
	view := func(entries string) string {
		return "# Current Contracts\n\n## a\n\nPurpose: A.\nActive: R1.AC1\n\n" + CoherenceHeading + "\n\nCompared on approval.\n\n" + entries + "\n## b\n\nPurpose: B.\nActive: R1.AC1, R1.AC2\n"
	}
	const cited = "### a\n\n- No inconsistency: the new charge rule agrees with `b#R1.AC2`.\n\n"
	const unlinked = "### n\n\n- No linked statements to compare.\n\n"

	t.Run("parsing", func(t *testing.T) {
		review := ParseCoherenceReview(view("### `a`\n\n- Conflicts with `b#R1.AC1` and `b#R1.AC2`.\n\n### n\n\n<!-- to do -->\n"))
		if !review.Present || len(review.Entries) != 2 {
			t.Fatalf("review = %+v, want a present section with two entries", review)
		}
		first, second := review.Entries[0], review.Entries[1]
		if first.Feature != "a" || !reflect.DeepEqual(first.References, []Reference{{Feature: "b", ID: "R1.AC1"}, {Feature: "b", ID: "R1.AC2"}}) {
			t.Fatalf("first entry = %+v", first)
		}
		if second.Feature != "n" || strings.TrimSpace(second.Text) != "" {
			t.Fatalf("a comment-only entry must be empty: %+v", second)
		}
		if absent := ParseCoherenceReview("# Current Contracts\n\n## a\n\nActive: R1.AC1\n"); absent.Present || len(absent.Entries) != 0 {
			t.Fatalf("a view without the section parsed as %+v", absent)
		}
	})

	cases := []struct {
		name, body string
		want       []string
	}{
		{"no section", "# Current Contracts\n\n## a\n\nPurpose: A.\nActive: R1.AC1\n", []string{"a|no entry", "n|no entry"}},
		{"missing entry", view(cited), []string{"n|no entry"}},
		{"empty entry", view(cited + "### n\n\n"), []string{"n|empty"}},
		{"comment-only entry", view(cited + "### n\n\n<!-- write the outcome -->\n"), []string{"n|empty"}},
		{"entry for a feature without pending changes", view(cited + unlinked + "### c\n\n- Unchanged.\n"), []string{"c|no pending change"}},
		{"duplicate entry", view(cited + unlinked + "### a\n\n- Again, see b#R1.AC1.\n"), []string{"a|more than one entry"}},
		{"no linked statement cited", view("### a\n\n- No inconsistency found.\n\n" + unlinked), []string{"a|cites none"}},
		{"citation only in a fenced block", view("### a\n\n```\nb#R1.AC2\n```\n\n" + unlinked), []string{"a|cites none"}},
		{"citation of an unlinked statement", view("### a\n\n- Agrees with `c#R1.AC1`.\n\n" + unlinked), []string{"a|cites none"}},
		{"reference outside inline code", view("### a\n\n- Agrees with b#R1.AC2.\n\n" + unlinked), []string{"a|cites none"}},
		{"complete review", view(cited + unlinked), nil},
		{"another linked statement cited", view("### a\n\n- Conflicts with `b#R1.AC1`: both charge the session.\n\n" + unlinked), nil},
	}
	keywords := []string{"no entry", "empty", "no pending change", "more than one entry", "cites none"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := []string{}
			for _, finding := range CheckCoherence(ParseCoherenceReview(tc.body), state, comparisons) {
				if finding.Kind != FindingCoherenceReview {
					t.Fatalf("finding kind %q, want %q", finding.Kind, FindingCoherenceReview)
				}
				label := finding.Feature + "|?"
				for _, keyword := range keywords {
					if strings.Contains(finding.Message, keyword) {
						label = finding.Feature + "|" + keyword
					}
				}
				got = append(got, label)
			}
			sort.Strings(got)
			want := append([]string{}, tc.want...)
			sort.Strings(want)
			if strings.Join(got, ";") != strings.Join(want, ";") {
				t.Fatalf("findings = %v, want %v", got, want)
			}
		})
	}

	t.Run("nothing pending", func(t *testing.T) {
		if findings := CheckCoherence(CoherenceReview{}, State{Tracking: TrackingActive}, nil); len(findings) != 0 {
			t.Fatalf("findings without pending changes: %+v", findings)
		}
	})
}
