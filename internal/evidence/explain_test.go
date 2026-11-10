package evidence

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
)

// explainCodeFixture is a record made on a commit with one uncommitted path,
// and a current tree in which one path changed, one was added and one is
// gone. The record's identity is the digest of the tree its facts rebuild.
func explainCodeFixture() (Record, TaskEvidence, ExplainInput, string) {
	const commit = "1111111111111111111111111111111111111111"
	listing := lsTreeListing(
		"100644 blob aaa\tkept.go",
		"100644 blob bbb\tedited.go",
		"100644 blob ccc\tgone.go",
	)
	recordedPaths := map[string]string{"local.txt": "100644:ddd"}
	record := Record{
		CodeIdentity: RebuildManifest(listing, recordedPaths).Digest(),
		Tree:         &TreeFacts{Commit: commit, Paths: recordedPaths},
	}
	current := Manifest{
		"kept.go":   "100644:aaa",
		"edited.go": "100644:eee",
		"local.txt": "100644:ddd",
		"new.go":    "100644:fff",
	}
	input := ExplainInput{
		Feature:  "demo",
		Current:  current,
		Listings: map[string]CommitListing{commit: {Listing: listing}},
	}
	return record, TaskEvidence{TaskID: "1", CodeFreshness: "stale"}, input, commit
}

func TestExplainCode(t *testing.T) {
	t.Run("paths", func(t *testing.T) {
		record, entry, input, commit := explainCodeFixture()
		explanation := Explain(record, entry, input)
		if explanation == nil || explanation.Code == nil {
			t.Fatal("a stale-code record with its facts has an explanation")
		}
		code := explanation.Code
		if code.Unavailable != "" || code.RecordedCommit != commit {
			t.Fatalf("explanation = %+v, want the paths since %s", code, commit)
		}
		want := []PathChange{
			{Path: "edited.go", Kind: KindChanged},
			{Path: "gone.go", Kind: KindRemoved},
			{Path: "new.go", Kind: KindAdded},
		}
		if !reflect.DeepEqual(code.Paths, want) {
			t.Fatalf("paths = %v, want %v", code.Paths, want)
		}
	})

	t.Run("mode-only", func(t *testing.T) {
		record, entry, input, _ := explainCodeFixture()
		input.Current = Manifest{"kept.go": "100755:aaa", "edited.go": "100644:bbb", "gone.go": "100644:ccc", "local.txt": "100644:ddd"}
		code := Explain(record, entry, input).Code
		if want := []PathChange{{Path: "kept.go", Kind: KindChanged}}; !reflect.DeepEqual(code.Paths, want) {
			t.Fatalf("paths = %v, want %v: a mode change is a change", code.Paths, want)
		}
	})

	unavailable := func(t *testing.T, record Record, entry TaskEvidence, input ExplainInput, reason string, mentions ...string) {
		t.Helper()
		explanation := Explain(record, entry, input)
		if explanation == nil || explanation.Code == nil {
			t.Fatalf("want the reason %s, got no explanation", reason)
		}
		code := explanation.Code
		if code.Unavailable != reason {
			t.Fatalf("unavailable = %q, want %q", code.Unavailable, reason)
		}
		if len(code.Paths) != 0 {
			t.Fatalf("an unavailable explanation names no path, got %v", code.Paths)
		}
		for _, text := range mentions {
			if !strings.Contains(code.Detail, text) {
				t.Fatalf("detail %q does not mention %q", code.Detail, text)
			}
		}
	}

	t.Run("no-commit", func(t *testing.T) {
		record, entry, input, _ := explainCodeFixture()
		record.Tree = nil
		unavailable(t, record, entry, input, ReasonNoCommit, "no commit", "walden verify demo")
	})

	t.Run("paths-not-kept", func(t *testing.T) {
		record, entry, input, commit := explainCodeFixture()
		record.Tree = &TreeFacts{Commit: commit, PathsOmitted: 137}
		unavailable(t, record, entry, input, ReasonPathsNotKept, "137", "walden verify demo")
	})

	t.Run("commit-missing", func(t *testing.T) {
		record, entry, input, commit := explainCodeFixture()
		input.Listings = map[string]CommitListing{commit: {Missing: true}}
		unavailable(t, record, entry, input, ReasonCommitMissing, commit)

		// A commit nobody listed is not a commit that is there.
		input.Listings = nil
		unavailable(t, record, entry, input, ReasonCommitMissing, commit)
	})

	t.Run("identity-mismatch", func(t *testing.T) {
		record, entry, input, _ := explainCodeFixture()
		record.Tree.Paths = map[string]string{"local.txt": "100644:tampered"}
		unavailable(t, record, entry, input, ReasonIdentityMismatch, "do not reproduce")
	})

	t.Run("reason-order", func(t *testing.T) {
		record, entry, input, commit := explainCodeFixture()
		input.Listings = map[string]CommitListing{commit: {Missing: true}}

		// A missing commit comes before a mismatch: nothing can be rebuilt.
		record.CodeIdentity = "sha256:other"
		unavailable(t, record, entry, input, ReasonCommitMissing)

		// A list that was not kept comes before the commit: there is
		// nothing to apply to it.
		record.Tree = &TreeFacts{Commit: commit, PathsOmitted: 101}
		unavailable(t, record, entry, input, ReasonPathsNotKept)

		// No facts at all comes first.
		record.Tree = nil
		unavailable(t, record, entry, input, ReasonNoCommit)
	})

	t.Run("not-stale", func(t *testing.T) {
		record, entry, input, _ := explainCodeFixture()
		for _, freshness := range []string{"current", "unavailable", ""} {
			entry.CodeFreshness = freshness
			if explanation := Explain(record, entry, input); explanation != nil && explanation.Code != nil {
				t.Fatalf("code freshness %q has nothing to explain, got %+v", freshness, explanation.Code)
			}
		}
	})
}

// forbiddenGit fails the test when git is asked anything.
type forbiddenGit struct{ t *testing.T }

func (g forbiddenGit) Run(_ context.Context, _ string, args ...string) (shell.Response, error) {
	g.t.Fatalf("git was asked %v", args)
	return shell.Response{}, nil
}

func TestListCommit(t *testing.T) {
	commit := strings.Repeat("a", 40)

	t.Run("not-an-object-id", func(t *testing.T) {
		// A recorded commit is data read from a file. Only an object id
		// reaches git; anything else is missing without asking.
		for _, value := range []string{"", "HEAD", "--output=/tmp/x", "abc", strings.Repeat("g", 40)} {
			if listing := ListCommit(t.Context(), forbiddenGit{t}, t.TempDir(), value); !listing.Missing || listing.Listing != "" {
				t.Fatalf("ListCommit(%q) = %+v, want missing", value, listing)
			}
		}
	})

	t.Run("listed", func(t *testing.T) {
		git := &fakeGit{responses: map[string]shell.Response{
			"ls-tree": {ExitCode: 0, Stdout: lsTreeListing("100644 blob aaa\tmain.go")},
		}}
		listing := ListCommit(t.Context(), git, t.TempDir(), commit)
		if listing.Missing || !strings.Contains(listing.Listing, "main.go") {
			t.Fatalf("listing = %+v, want the commit's tree", listing)
		}
	})

	t.Run("absent", func(t *testing.T) {
		git := &fakeGit{responses: map[string]shell.Response{
			"ls-tree": {ExitCode: 128, Stderr: "fatal: not a valid object name"},
		}}
		if listing := ListCommit(t.Context(), git, t.TempDir(), commit); !listing.Missing {
			t.Fatalf("listing = %+v, want missing", listing)
		}
	})
}

// explainSpecFixture is a record whose task named three statements, and a
// current chain in which nothing changed yet.
func explainSpecFixture() (Record, TaskEvidence, ExplainInput) {
	record := Record{
		RequirementsFingerprint: "sha256:requirements",
		DesignFingerprint:       "sha256:design",
		Statements: &StatementFacts{
			Scheme:       spec.StatementScheme,
			Fingerprints: map[string]string{"R1.AC1": "sha256:a", "R2.AC1": "sha256:b", "NFR1": "sha256:c"},
		},
	}
	input := ExplainInput{
		Feature:    "demo",
		Chain:      ChainFingerprints{Requirements: "sha256:requirements", Design: "sha256:design"},
		Statements: map[string]string{"R1.AC1": "sha256:a", "R2.AC1": "sha256:b", "NFR1": "sha256:c"},
	}
	return record, TaskEvidence{TaskID: "1", Binding: BindingAssessment{State: "current"}}, input
}

func TestExplainSpec(t *testing.T) {
	specOf := func(t *testing.T, record Record, entry TaskEvidence, input ExplainInput) *SpecExplanation {
		t.Helper()
		explanation := Explain(record, entry, input)
		if explanation == nil || explanation.Spec == nil {
			t.Fatal("a changed approval has an explanation")
		}
		return explanation.Spec
	}

	t.Run("nothing-changed", func(t *testing.T) {
		record, entry, input := explainSpecFixture()
		if explanation := Explain(record, entry, input); explanation != nil {
			t.Fatalf("nothing changed, got %+v", explanation)
		}
	})

	t.Run("approvals", func(t *testing.T) {
		record, entry, input := explainSpecFixture()

		input.Chain.Design = "sha256:design-2"
		if got := specOf(t, record, entry, input); !reflect.DeepEqual(got.Approvals, []string{ApprovalDesign}) {
			t.Fatalf("approvals = %v, want the design alone", got.Approvals)
		} else if len(got.Statements) != 0 || got.StatementsUnchanged || got.Unavailable != "" {
			t.Fatalf("the requirements did not change: no statement is compared, got %+v", got)
		}

		entry.Binding.State = "changed"
		if got := specOf(t, record, entry, input); !reflect.DeepEqual(got.Approvals, []string{ApprovalDesign, ApprovalTask}) {
			t.Fatalf("approvals = %v, want design and task", got.Approvals)
		}

		input.Chain.Requirements = "sha256:requirements-2"
		if got := specOf(t, record, entry, input); !reflect.DeepEqual(got.Approvals, []string{ApprovalRequirements, ApprovalDesign, ApprovalTask}) {
			t.Fatalf("approvals = %v, want all three in order", got.Approvals)
		}

		// A fingerprint that is absent on either side establishes no change.
		record, entry, input = explainSpecFixture()
		record.DesignFingerprint = ""
		input.Chain.Requirements = ""
		if explanation := Explain(record, entry, input); explanation != nil {
			t.Fatalf("absent fingerprints are not changes, got %+v", explanation.Spec)
		}
	})

	t.Run("statements", func(t *testing.T) {
		record, entry, input := explainSpecFixture()
		// The requirements were approved again: one statement reworded, one
		// removed, one added.
		input.Chain.Requirements = "sha256:requirements-2"
		input.Statements = map[string]string{"R1.AC1": "sha256:a-reworded", "NFR1": "sha256:c", "R3.AC1": "sha256:new"}

		got := specOf(t, record, entry, input)
		want := []StatementChange{{ID: "R1.AC1", Kind: KindChanged}, {ID: "R2.AC1", Kind: KindRemoved}}
		if !reflect.DeepEqual(got.Statements, want) || got.StatementsUnchanged || got.Unavailable != "" {
			t.Fatalf("statements = %+v, want %v", got, want)
		}
	})

	t.Run("statements-unchanged", func(t *testing.T) {
		record, entry, input := explainSpecFixture()
		record.Statements.Fingerprints = map[string]string{"NFR1": "sha256:c"}
		input.Chain.Requirements = "sha256:requirements-2"
		input.Statements = map[string]string{"R1.AC1": "sha256:a-reworded", "NFR1": "sha256:c"}

		got := specOf(t, record, entry, input)
		if !got.StatementsUnchanged || len(got.Statements) != 0 || got.Unavailable != "" {
			t.Fatalf("none of the task's statements changed, got %+v", got)
		}

		// A task that names no statement has none that changed.
		record.Statements.Fingerprints = map[string]string{}
		if got := specOf(t, record, entry, input); !got.StatementsUnchanged {
			t.Fatalf("a task naming no statement, got %+v", got)
		}
	})

	t.Run("no-fingerprints", func(t *testing.T) {
		record, entry, input := explainSpecFixture()
		record.Statements = nil
		input.Chain.Requirements = "sha256:requirements-2"

		got := specOf(t, record, entry, input)
		if got.Unavailable != ReasonNoStatementFingerprints || len(got.Statements) != 0 || got.StatementsUnchanged {
			t.Fatalf("explanation = %+v, want %s and no statement", got, ReasonNoStatementFingerprints)
		}
		for _, text := range []string{"no statement fingerprints", "walden verify demo"} {
			if !strings.Contains(got.Detail, text) {
				t.Fatalf("detail %q does not mention %q", got.Detail, text)
			}
		}
	})

	t.Run("unknown-scheme", func(t *testing.T) {
		record, entry, input := explainSpecFixture()
		record.Statements.Scheme = "walden/statement-block/v9"
		input.Chain.Requirements = "sha256:requirements-2"
		input.Statements = map[string]string{}

		// Fingerprints this CLI cannot read are absent, never compared: a
		// comparison across schemes would call every statement changed.
		got := specOf(t, record, entry, input)
		if got.Unavailable != ReasonNoStatementFingerprints || len(got.Statements) != 0 || got.StatementsUnchanged {
			t.Fatalf("explanation = %+v, want %s and no statement", got, ReasonNoStatementFingerprints)
		}
		if !strings.Contains(got.Detail, "walden/statement-block/v9") {
			t.Fatalf("detail %q does not name the scheme", got.Detail)
		}
	})

	t.Run("identifier-order", func(t *testing.T) {
		record, entry, input := explainSpecFixture()
		record.Statements.Fingerprints = map[string]string{"R1.AC10": "x", "R1.AC2": "x", "R2.AC1": "x", "NFR1": "x", "C1": "x"}
		input.Chain.Requirements = "sha256:requirements-2"
		input.Statements = map[string]string{}

		ids := []string{}
		for _, change := range specOf(t, record, entry, input).Statements {
			ids = append(ids, change.ID)
		}
		if want := []string{"R1.AC2", "R1.AC10", "R2.AC1", "NFR1", "C1"}; !reflect.DeepEqual(ids, want) {
			t.Fatalf("order = %v, want %v: criteria by number, then NFRs, then constraints", ids, want)
		}
	})
}

func TestExplainDuration(t *testing.T) {
	ms := func(value int64) *int64 { return &value }
	steps := func(durations ...*int64) Record {
		record := Record{}
		for _, duration := range durations {
			record.Steps = append(record.Steps, StepResult{Command: []string{"prove"}, DurationMS: duration})
		}
		return record
	}

	t.Run("sum", func(t *testing.T) {
		got := RecordedDuration(steps(ms(200), ms(300)))
		if got == nil || *got != 500 {
			t.Fatalf("recorded duration = %v, want 500", got)
		}
	})

	t.Run("zero", func(t *testing.T) {
		// Zero is a measurement: a step can take less than a millisecond.
		got := RecordedDuration(steps(ms(0), ms(0)))
		if got == nil || *got != 0 {
			t.Fatalf("recorded duration = %v, want a measured zero", got)
		}
	})

	t.Run("absent", func(t *testing.T) {
		for name, record := range map[string]Record{
			"no duration":  steps(nil, nil),
			"one missing":  steps(ms(200), nil),
			"no step":      steps(),
			"empty record": {},
		} {
			if got := RecordedDuration(record); got != nil {
				t.Fatalf("%s: recorded duration = %d, want none", name, *got)
			}
		}
	})
}
