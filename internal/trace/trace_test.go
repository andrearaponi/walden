package trace

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/andrearaponi/walden/internal/spec"
)

// repeats is how many times each case runs. A join that iterated a map would
// give its entries in an order that changes between runs, and two runs over
// a few entries often agree by chance.
const repeats = 50

func repeat(t *testing.T, check func()) {
	t.Helper()
	for run := 0; run < repeats && !t.Failed(); run++ {
		check()
	}
}

func step(covers []string, argv ...string) spec.VerificationStep {
	return spec.VerificationStep{Argv: argv, Covers: covers}
}

func leaf(id string, requirements []string, steps ...spec.VerificationStep) *spec.Task {
	return &spec.Task{ID: id, Requirements: requirements, DesignRefs: []string{"Architecture"}, Proof: spec.VerificationSpec{Steps: steps}}
}

func statementIDs(report Report) []string {
	ids := []string{}
	for _, statement := range report.Statements {
		ids = append(ids, statement.ID)
	}
	return ids
}

func TestBuildListsStatements(t *testing.T) {
	t.Run("every criterion in the order of the document", func(t *testing.T) {
		// Thirteen criteria, declared out of numeric order.
		criteria := []string{"R2.AC1", "R1.AC1", "R1.AC2", "R1.AC10", "R1.AC3", "R3.AC2", "R3.AC1", "R4.AC1", "R4.AC2", "R4.AC3", "R5.AC1", "R5.AC2", "R5.AC3"}
		// One leaf names every criterion but R4.AC2, in another order.
		named := []string{"R5.AC3", "R1.AC1", "R3.AC1", "R2.AC1", "R1.AC10", "R1.AC2", "R1.AC3", "R3.AC2", "R4.AC1", "R4.AC3", "R5.AC1", "R5.AC2"}
		in := Inputs{
			Feature:   "demo",
			Criteria:  criteria,
			Leaves:    []*spec.Task{leaf("1.1", named, step(named, "go", "test", "./..."))},
			States:    map[string]string{"1.1": "verified"},
			Unnamed:   []string{"R4.AC2"},
			Uncovered: []string{"R4.AC2"},
		}
		repeat(t, func() {
			report := Build(in)
			if got := statementIDs(report); !reflect.DeepEqual(got, criteria) {
				t.Fatalf("statements = %v, want the thirteen criteria as the document declares them: %v", got, criteria)
			}
			for _, statement := range report.Statements {
				if statement.Kind != KindCriterion {
					t.Errorf("%s has the kind %q, want %q", statement.ID, statement.Kind, KindCriterion)
				}
			}
			unnamed := report.Statements[8]
			if want := (Statement{ID: "R4.AC2", Kind: KindCriterion, NamedBy: []string{}, Proofs: []Proof{}}); !reflect.DeepEqual(unnamed, want) {
				t.Errorf("the criterion no leaf names = %+v, want %+v", unnamed, want)
			}
		})
	})

	t.Run("the leaves that name a statement and the steps that cover it", func(t *testing.T) {
		in := Inputs{
			Feature:  "demo",
			Criteria: []string{"R1.AC1", "R1.AC2"},
			Leaves: []*spec.Task{
				leaf("1.1", []string{"R1.AC1"}, step([]string{"R1.AC1"}, "go", "test", "./a")),
				// 1.2 names R1.AC1, and its first step does not cover it.
				leaf("1.2", []string{"R1.AC2", "R1.AC1"},
					step([]string{"R1.AC2"}, "go", "test", "./b"),
					step([]string{"R1.AC1", "R1.AC2"}, "go", "vet", "./...")),
			},
			States: map[string]string{"1.1": "verified", "1.2": "stale-code"},
		}
		want := []Statement{
			{ID: "R1.AC1", Kind: KindCriterion, NamedBy: []string{"1.1", "1.2"}, Proofs: []Proof{
				{TaskID: "1.1", Step: 1, Command: []string{"go", "test", "./a"}, State: "verified"},
				{TaskID: "1.2", Step: 2, Command: []string{"go", "vet", "./..."}, State: "stale-code"},
			}},
			{ID: "R1.AC2", Kind: KindCriterion, NamedBy: []string{"1.2"}, Proofs: []Proof{
				{TaskID: "1.2", Step: 1, Command: []string{"go", "test", "./b"}, State: "stale-code"},
				{TaskID: "1.2", Step: 2, Command: []string{"go", "vet", "./..."}, State: "stale-code"},
			}},
		}
		repeat(t, func() {
			if got := Build(in).Statements; !reflect.DeepEqual(got, want) {
				t.Fatalf("statements = %+v, want %+v", got, want)
			}
		})
	})

	t.Run("statements that are not criteria come after them", func(t *testing.T) {
		in := Inputs{
			Feature:     "demo",
			Criteria:    []string{"R1.AC1"},
			NFRs:        []string{"NFR2", "NFR1", "NFR3"},
			Constraints: []string{"C1", "C2"},
			Leaves: []*spec.Task{
				// R2 is a requirement heading and R9.AC9 is not defined:
				// neither adds an entry.
				leaf("1.1", []string{"R1.AC1", "NFR1", "R2", "R9.AC9"}, step([]string{"R1.AC1", "NFR1", "C2"}, "go", "test", "./...")),
				leaf("1.2", []string{"NFR3"}, step(nil, "go", "vet", "./...")),
			},
			States: map[string]string{"1.1": "verified", "1.2": "pending"},
		}
		covering := Proof{TaskID: "1.1", Step: 1, Command: []string{"go", "test", "./..."}, State: "verified"}
		want := []Statement{
			{ID: "R1.AC1", Kind: KindCriterion, NamedBy: []string{"1.1"}, Proofs: []Proof{covering}},
			// NFR2 and C1 are named by no leaf and covered by no step.
			{ID: "NFR1", Kind: KindNFR, NamedBy: []string{"1.1"}, Proofs: []Proof{covering}},
			{ID: "NFR3", Kind: KindNFR, NamedBy: []string{"1.2"}, Proofs: []Proof{}},
			{ID: "C2", Kind: KindConstraint, NamedBy: []string{}, Proofs: []Proof{covering}},
		}
		repeat(t, func() {
			report := Build(in)
			if !reflect.DeepEqual(report.Statements, want) {
				t.Fatalf("statements = %+v, want %+v", report.Statements, want)
			}
			if len(report.Holes) != 0 {
				t.Errorf("holes = %+v, want none: the pending leaf covers nothing", report.Holes)
			}
		})
	})

	t.Run("a leaf or a step that repeats an identifier counts once", func(t *testing.T) {
		in := Inputs{
			Feature:  "demo",
			Criteria: []string{"R1.AC1"},
			Leaves:   []*spec.Task{leaf("1.1", []string{"R1.AC1", "R1.AC1"}, step([]string{"R1.AC1", "R1.AC1"}, "go", "test", "./..."))},
			States:   map[string]string{"1.1": "verified"},
		}
		want := []Statement{{ID: "R1.AC1", Kind: KindCriterion, NamedBy: []string{"1.1"}, Proofs: []Proof{
			{TaskID: "1.1", Step: 1, Command: []string{"go", "test", "./..."}, State: "verified"},
		}}}
		repeat(t, func() {
			if got := Build(in).Statements; !reflect.DeepEqual(got, want) {
				t.Fatalf("statements = %+v, want %+v", got, want)
			}
		})
	})

	t.Run("the leaves with their design references, delivery and state", func(t *testing.T) {
		delivery := leaf("1.1", []string{"R1.AC1"}, step([]string{"R1.AC1"}, "grep", "-q", "released", "notes.txt"))
		delivery.Proof.Delivery = true
		regression := leaf("1.2", []string{"R1.AC1"}, step([]string{"R1.AC1"}, "go", "test", "./..."))
		regression.DesignRefs = []string{"Architecture", "Verification Plan; a free note"}
		silent := leaf("1.3", []string{"R1.AC1"}, step(nil, "go", "vet", "./..."))
		in := Inputs{
			Feature:  "demo",
			Criteria: []string{"R1.AC1"},
			Leaves:   []*spec.Task{delivery, regression, silent},
			States:   map[string]string{"1.1": "delivered", "1.2": "stale-code", "1.3": "pending"},
		}
		wantLeaves := []Leaf{
			{ID: "1.1", DesignRefs: []string{"Architecture"}, Delivery: true, State: "delivered"},
			{ID: "1.2", DesignRefs: []string{"Architecture", "Verification Plan; a free note"}, State: "stale-code"},
			// A leaf whose steps cover nothing is still a leaf of the plan.
			{ID: "1.3", DesignRefs: []string{"Architecture"}, State: "pending"},
		}
		wantProofs := []Proof{
			{TaskID: "1.1", Step: 1, Command: []string{"grep", "-q", "released", "notes.txt"}, Delivery: true, State: "delivered"},
			{TaskID: "1.2", Step: 1, Command: []string{"go", "test", "./..."}, State: "stale-code"},
		}
		repeat(t, func() {
			report := Build(in)
			if !reflect.DeepEqual(report.Leaves, wantLeaves) {
				t.Fatalf("leaves = %+v, want %+v", report.Leaves, wantLeaves)
			}
			if got := report.Statements[0].Proofs; !reflect.DeepEqual(got, wantProofs) {
				t.Fatalf("proofs = %+v, want %+v", got, wantProofs)
			}
		})
	})

	t.Run("a report of nothing holds empty lists", func(t *testing.T) {
		report := Build(Inputs{Feature: "demo"})
		if report.Feature != "demo" || report.Statements == nil || report.Leaves == nil || report.Holes == nil {
			t.Errorf("report = %+v, want the feature and three empty lists that are not nil", report)
		}
		if len(report.Statements)+len(report.Leaves)+len(report.Holes) != 0 {
			t.Errorf("report = %+v, want nothing listed", report)
		}
	})
}

func TestBuildHoles(t *testing.T) {
	covered := func(id string, ids ...string) *spec.Task {
		return leaf(id, ids, step(ids, "go", "test", "./..."))
	}
	cases := []struct {
		name string
		in   Inputs
		want []Hole
	}{
		{
			name: "a criterion no leaf names has one hole, not two",
			in: Inputs{
				Criteria:  []string{"R1.AC1", "R1.AC2"},
				Leaves:    []*spec.Task{covered("1.1", "R1.AC1")},
				States:    map[string]string{"1.1": "verified"},
				Unnamed:   []string{"R1.AC2"},
				Uncovered: []string{"R1.AC2"},
			},
			want: []Hole{{Kind: HoleUnnamed, Statement: "R1.AC2"}},
		},
		{
			name: "a criterion leaves name and no step covers carries those leaves",
			in: Inputs{
				Criteria: []string{"R1.AC1", "R1.AC2"},
				Leaves: []*spec.Task{
					leaf("1.1", []string{"R1.AC1", "R1.AC2"}, step([]string{"R1.AC1"}, "go", "test", "./...")),
					leaf("1.2", []string{"R1.AC2"}, step(nil, "go", "vet", "./...")),
				},
				States:    map[string]string{"1.1": "verified", "1.2": "verified"},
				Uncovered: []string{"R1.AC2"},
			},
			want: []Hole{{Kind: HoleUncovered, Statement: "R1.AC2", NamedBy: []string{"1.1", "1.2"}}},
		},
		{
			name: "one hole for each step on a leaf that is not standing",
			in: Inputs{
				Criteria: []string{"R1.AC1", "R1.AC2", "R1.AC3"},
				NFRs:     []string{"NFR1"},
				Leaves: []*spec.Task{
					// Delivered and verified leaves add no hole.
					covered("1.1", "R1.AC1"),
					leaf("1.2", []string{"R1.AC2", "R1.AC3", "NFR1"},
						step([]string{"R1.AC2"}, "go", "test", "./a"),
						step([]string{"R1.AC3", "NFR1"}, "go", "test", "./b")),
					covered("1.3", "R1.AC1"),
					// R1.AC1 is covered by a verified leaf and by this
					// pending one: one hole, for this one.
					covered("1.4", "R1.AC1"),
				},
				States: map[string]string{"1.1": "delivered", "1.2": "stale-code", "1.3": "verified", "1.4": "pending"},
			},
			want: []Hole{
				{Kind: HoleNotStanding, Statement: "R1.AC1", TaskID: "1.4", Step: 1, State: "pending"},
				{Kind: HoleNotStanding, Statement: "R1.AC2", TaskID: "1.2", Step: 1, State: "stale-code"},
				{Kind: HoleNotStanding, Statement: "R1.AC3", TaskID: "1.2", Step: 2, State: "stale-code"},
				{Kind: HoleNotStanding, Statement: "NFR1", TaskID: "1.2", Step: 2, State: "stale-code"},
			},
		},
		{
			name: "every leaf verified or delivered leaves no hole",
			in: Inputs{
				Criteria: []string{"R1.AC1", "R1.AC2"},
				Leaves:   []*spec.Task{covered("1.1", "R1.AC1"), covered("1.2", "R1.AC2")},
				States:   map[string]string{"1.1": "delivered", "1.2": "verified"},
			},
			want: []Hole{},
		},
		{
			name: "every other state is not standing",
			in: Inputs{
				Criteria: []string{"R1.AC1"},
				Leaves:   []*spec.Task{covered("1.1", "R1.AC1"), covered("1.2", "R1.AC1"), covered("1.3", "R1.AC1"), covered("1.4", "R1.AC1")},
				States:   map[string]string{"1.1": "failed", "1.2": "stale-spec", "1.3": "unattested", "1.4": "unrecorded"},
			},
			want: []Hole{
				{Kind: HoleNotStanding, Statement: "R1.AC1", TaskID: "1.1", Step: 1, State: "failed"},
				{Kind: HoleNotStanding, Statement: "R1.AC1", TaskID: "1.2", Step: 1, State: "stale-spec"},
				{Kind: HoleNotStanding, Statement: "R1.AC1", TaskID: "1.3", Step: 1, State: "unattested"},
				{Kind: HoleNotStanding, Statement: "R1.AC1", TaskID: "1.4", Step: 1, State: "unrecorded"},
			},
		},
		{
			name: "the kinds in order, the two corners of the readers, and criteria that are mentioned and not defined",
			in: Inputs{
				Criteria: []string{"R1.AC1", "R1.AC2", "R1.AC3", "R1.AC4", "R1.AC5"},
				Leaves: []*spec.Task{
					// R1.AC9 is mentioned by the requirements and not defined.
					leaf("1.1", []string{"R1.AC1", "R1.AC2", "R1.AC9"}, step([]string{"R1.AC1"}, "go", "test", "./a")),
					// The parser reads R1.AC4 on this leaf, written without
					// backticks; the validator does not count it.
					covered("1.2", "R1.AC4"),
				},
				States: map[string]string{"1.1": "pending", "1.2": "pending"},
				// The lists of the validator, in its order. R1.AC5 is named
				// for it by a line of prose, and by no leaf for the parser.
				Unnamed:   []string{"R1.AC3", "R1.AC4", "R1.AC8"},
				Uncovered: []string{"R1.AC2", "R1.AC3", "R1.AC5", "R1.AC8", "R1.AC9"},
			},
			want: []Hole{
				{Kind: HoleUnnamed, Statement: "R1.AC3"},
				{Kind: HoleUnnamed, Statement: "R1.AC4", NamedBy: []string{"1.2"}},
				{Kind: HoleUnnamed, Statement: "R1.AC8"},
				{Kind: HoleUncovered, Statement: "R1.AC2", NamedBy: []string{"1.1"}},
				{Kind: HoleUncovered, Statement: "R1.AC5"},
				{Kind: HoleUncovered, Statement: "R1.AC9", NamedBy: []string{"1.1"}},
				{Kind: HoleNotStanding, Statement: "R1.AC1", TaskID: "1.1", Step: 1, State: "pending"},
				{Kind: HoleNotStanding, Statement: "R1.AC4", TaskID: "1.2", Step: 1, State: "pending"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.in.Feature = "demo"
			repeat(t, func() {
				if got := Build(tc.in).Holes; !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("holes = %+v, want %+v", got, tc.want)
				}
			})
		})
	}
}

func TestSummary(t *testing.T) {
	sixteen := Report{Feature: "demo", Statements: make([]Statement, 16), Holes: []Hole{}}
	if got, want := sixteen.Summary(), "trace for demo: 16 statement(s), 0 hole(s)"; got != want {
		t.Errorf("summary without holes = %q, want %q", got, want)
	}
	if got, want := sixteen.HoleBreakdown(), "0 unnamed, 0 uncovered, 0 not-standing"; got != want {
		t.Errorf("breakdown without holes = %q, want %q", got, want)
	}

	mixed := Report{Feature: "demo", Statements: make([]Statement, 4), Holes: []Hole{
		{Kind: HoleUnnamed}, {Kind: HoleUnnamed},
		{Kind: HoleUncovered},
		{Kind: HoleNotStanding}, {Kind: HoleNotStanding}, {Kind: HoleNotStanding},
	}}
	repeat(t, func() {
		if got, want := mixed.HoleBreakdown(), "2 unnamed, 1 uncovered, 3 not-standing"; got != want {
			t.Fatalf("breakdown = %q, want %q", got, want)
		}
		if got, want := mixed.Summary(), "trace for demo: 4 statement(s), 6 hole(s): 2 unnamed, 1 uncovered, 3 not-standing"; got != want {
			t.Fatalf("summary = %q, want %q", got, want)
		}
	})

	onlyStale := Report{Feature: "demo", Statements: make([]Statement, 2), Holes: []Hole{{Kind: HoleNotStanding}}}
	if got, want := onlyStale.Summary(), "trace for demo: 2 statement(s), 1 hole(s): 0 unnamed, 0 uncovered, 1 not-standing"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

func TestShown(t *testing.T) {
	if CommandLimit != 120 {
		t.Fatalf("the limit is %d characters, want the 120 the help states", CommandLimit)
	}

	written := []struct {
		name string
		argv []string
		want string
	}{
		{"arguments separated by one space", []string{"go", "test", "-run", "^TestExample$", "./pkg"}, "go test -run ^TestExample$ ./pkg"},
		{"an argument that holds a space is a literal", []string{"sh", "-c", "test -f main.go | cat"}, `sh -c "test -f main.go | cat"`},
		{"an argument that holds a double quote is a literal", []string{"echo", `say "hi"`}, `echo "say \"hi\""`},
		{"a line break reads as an escape", []string{"printf", "a\nb"}, `printf "a\nb"`},
		{"a tab reads as an escape", []string{"printf", "a\tb"}, `printf "a\tb"`},
		{"a backslash and a backtick stay as written", []string{"grep", `a\|b`, "`date`"}, "grep a\\|b `date`"},
		{"a backslash inside a literal is doubled", []string{"sh", "-c", `grep 'a\|b' f`}, `sh -c "grep 'a\\|b' f"`},
	}
	for _, tc := range written {
		t.Run(tc.name, func(t *testing.T) {
			if got := Shown(tc.argv); got != tc.want {
				t.Errorf("Shown(%q) = %q, want %q", tc.argv, got, tc.want)
			}
		})
	}

	lengths := []struct {
		name  string
		char  string
		count int
		cut   bool
	}{
		{"119 characters are whole", "a", 119, false},
		{"120 characters are whole", "a", 120, false},
		{"121 characters are cut", "a", 121, true},
		{"400 characters are cut", "a", 400, true},
		{"120 characters of two bytes each are whole", "è", 120, false},
		{"121 characters of two bytes each are cut", "è", 121, true},
	}
	for _, tc := range lengths {
		t.Run(tc.name, func(t *testing.T) {
			command := strings.Repeat(tc.char, tc.count)
			got := Shown([]string{command})
			if !tc.cut {
				if got != command {
					t.Errorf("a command of %d characters is shown as %d", tc.count, utf8.RuneCountInString(got))
				}
				return
			}
			// The first 119 characters, then the mark: 120 in all.
			if want := strings.Repeat(tc.char, 119) + "…"; got != want {
				t.Errorf("a command of %d characters is shown as %q (%d characters), want 119 of them and the mark", tc.count, got, utf8.RuneCountInString(got))
			}
		})
	}

	t.Run("the cut counts the whole command, arguments and spaces", func(t *testing.T) {
		argv := []string{"sh", "-c", strings.Repeat("true && ", 60) + "true"}
		got := Shown(argv)
		if utf8.RuneCountInString(got) != 120 || !strings.HasPrefix(got, `sh -c "true && true`) || !strings.HasSuffix(got, "…") {
			t.Errorf("Shown = %q (%d characters), want 120 that open with the command and end with the mark", got, utf8.RuneCountInString(got))
		}
	})
}
