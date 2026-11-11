package evidence

import (
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

const (
	verifyRemedy = "a pass of walden verify markers on a tree without uncommitted changes attests it"
	adoptRemedy  = "walden adopt markers shows whether the pass can be sealed and --apply seals it; a pass left unsealed is proven again with walden verify markers --task 3"
)

// factsOf is a pass that carries the statement facts every record written
// since v0.14.5 holds, and the tree facts given.
func factsOf(definition string, tree *TreeFacts) Record {
	record := passOf(definition)
	record.Tree = tree
	record.Statements = &StatementFacts{Scheme: spec.StatementScheme, Fingerprints: map[string]string{}}
	return record
}

func TestExplainAttestation(t *testing.T) {
	// Entries as Derive gives them: a delivery task whose pass is bound to
	// the declared definition, and one whose pass awaits adoption.
	bound := TaskEvidence{TaskID: "3", State: StateVerified, Delivery: true}
	awaiting := TaskEvidence{TaskID: "3", State: StateStaleSpec, Delivery: true, AwaitsAdoption: true}

	committed := &TreeFacts{Commit: deliveredCommit}
	differing := &TreeFacts{Commit: deliveredCommit, Paths: map[string]string{"a.go": "100644:aaa", "gone.go": ""}}
	omitted := &TreeFacts{Commit: deliveredCommit, PathsOmitted: 150}
	old := passOf(declaredDefinition) // written before records kept tree or statement facts
	oldUndeclared := passOf(undeclaredDefinition)

	cases := []struct {
		name   string
		record Record
		entry  TaskEvidence
		reason string
		detail string
	}{
		{"differing paths", factsOf(declaredDefinition, differing), bound, ReasonUncommittedPaths,
			"the proof ran on a tree with 2 uncommitted path(s); " + verifyRemedy},
		{"paths not kept", factsOf(declaredDefinition, omitted), bound, ReasonUncommittedPaths,
			"the proof ran on a tree with 150 uncommitted path(s); " + verifyRemedy},
		{"no commit", factsOf(declaredDefinition, nil), bound, ReasonNoCommit,
			"no commit could be read when the proof ran; " + verifyRemedy},
		{"no tree facts", old, bound, ReasonNoTree,
			"the record keeps no commit and no paths, so it does not show whether its tree was committed; " + verifyRemedy},
		{"a commit and no declaration, awaiting", factsOf(undeclaredDefinition, committed), awaiting, ReasonUndeclaredRun,
			"the pass was recorded on commit " + deliveredCommit + " by a run that did not read the task's proof as a delivery proof; " + adoptRemedy},
		{"differing paths, awaiting", factsOf(undeclaredDefinition, differing), awaiting, ReasonUncommittedPaths,
			"the proof ran on a tree with 2 uncommitted path(s); " + adoptRemedy},
		{"no tree facts, awaiting", oldUndeclared, awaiting, ReasonNoTree,
			"the record keeps no commit and no paths, so it does not show whether its tree was committed; " + adoptRemedy},
		// A pass whose proof changed too is not awaiting: verify proves it.
		{"a commit and another definition", factsOf("sha256:something-else", committed), TaskEvidence{TaskID: "3", State: StateStaleSpec, Delivery: true}, ReasonUndeclaredRun,
			"the pass was recorded on commit " + deliveredCommit + " by a run that did not read the task's proof as a delivery proof; " + verifyRemedy},
	}
	for _, tc := range cases {
		got := ExplainAttestation(tc.record, tc.entry, "markers")
		want := &AttestationExplanation{Unavailable: tc.reason, Detail: tc.detail}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got %+v\nwant %+v", tc.name, got, want)
		}
	}

	failed := factsOf(declaredDefinition, differing)
	failed.Result = ResultFailed
	for name, tc := range map[string]struct {
		record Record
		entry  TaskEvidence
	}{
		"a delivered task":                    {attestedPass(), TaskEvidence{TaskID: "3", State: StateDelivered, Delivery: true, Attestation: &AttestedCommit{Commit: deliveredCommit}}},
		"a task read through its attestation": {attestedPass(), TaskEvidence{TaskID: "3", State: StateStaleSpec, Delivery: true}},
		"a failed record":                     {failed, TaskEvidence{TaskID: "3", State: StateFailed, Delivery: true}},
		"a regression task":                   {factsOf(siblingDefinition, differing), TaskEvidence{TaskID: "3", State: StateVerified}},
		"a pending task":                      {factsOf(declaredDefinition, differing), TaskEvidence{TaskID: "3", State: StatePending, Delivery: true}},
	} {
		if got := ExplainAttestation(tc.record, tc.entry, "markers"); got != nil {
			t.Errorf("%s is explained: %+v", name, *got)
		}
	}

	// Explain carries the line beside the others, and says nothing at all
	// under a delivered task, whose later changes are not a defect.
	record := factsOf(declaredDefinition, differing)
	explanation := Explain(record, bound, ExplainInput{Feature: "markers"})
	if explanation == nil || explanation.Attestation == nil || explanation.Attestation.Unavailable != ReasonUncommittedPaths {
		t.Errorf("Explain does not carry the attestation line: %+v", explanation)
	}
	delivered := attestedPass()
	delivered.Tree = committed
	stale := TaskEvidence{TaskID: "3", State: StateDelivered, Delivery: true, CodeFreshness: "stale", Attestation: &AttestedCommit{Commit: deliveredCommit}}
	if got := Explain(delivered, stale, ExplainInput{Feature: "markers", Listings: map[string]CommitListing{deliveredCommit: {Listing: ""}}}); got != nil {
		t.Errorf("a delivered task is explained: %+v", got)
	}
}

// TestExplainSpecReadsAttestedStatements: an entry read through its
// attestation is explained from the statements its state came from, also
// when the record keeps none of its own.
func TestExplainSpecReadsAttestedStatements(t *testing.T) {
	record := sealedPass() // attests R1.AC1 and R1.AC2; the record keeps no statement facts
	if record.Statements != nil {
		t.Fatal("the fixture record must keep no statement facts")
	}
	chain := deliveryChain()
	chain.Requirements = "sha256:req-again"
	chain.Statements["R1.AC2"] = "sha256:reworded"

	document := Document{Feature: "f", Tasks: map[string]Record{"1": record}}
	entry := Derive(document, chain, "sha256:code", true, []LeafTask{deliveryLeaf()})[0]
	if entry.State != StateStaleSpec {
		t.Fatalf("the fixture derives %s, want stale-spec", entry.State)
	}

	explanation := Explain(record, entry, ExplainInput{Feature: "markers", Chain: chain, Statements: chain.Statements})
	if explanation == nil || explanation.Spec == nil {
		t.Fatalf("no specification explanation: %+v", explanation)
	}
	spec := explanation.Spec
	if spec.Unavailable != "" || !reflect.DeepEqual(spec.Approvals, []string{ApprovalRequirements}) {
		t.Errorf("explanation = %+v, want the requirements as the changed approval and no missing fact", spec)
	}
	if want := []StatementChange{{ID: "R1.AC2", Kind: KindChanged}}; !reflect.DeepEqual(spec.Statements, want) {
		t.Errorf("statements = %+v, want %+v", spec.Statements, want)
	}
	if explanation.Attestation != nil {
		t.Errorf("a task that holds an attestation is told why it holds none: %+v", explanation.Attestation)
	}

	// The same record judged by the regression reading keeps the answer of
	// today: its own statement facts are missing.
	plain := sealedPass()
	plain.Attestation = nil
	regression := Derive(Document{Feature: "f", Tasks: map[string]Record{"1": plain}}, chain, "sha256:code", true, []LeafTask{deliveryLeaf()})[0]
	explanation = Explain(plain, regression, ExplainInput{Feature: "markers", Chain: chain, Statements: chain.Statements})
	if explanation == nil || explanation.Spec == nil || explanation.Spec.Unavailable != ReasonNoStatementFingerprints {
		t.Errorf("without an attestation the missing fact is not reported: %+v", explanation)
	}
	if explanation == nil || explanation.Attestation == nil || !strings.Contains(explanation.Attestation.Detail, "walden adopt markers") {
		t.Errorf("an awaiting pass is not pointed to adoption: %+v", explanation)
	}
}
