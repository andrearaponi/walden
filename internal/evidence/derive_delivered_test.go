package evidence

import (
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

// The fixture of the delivery tests: a delivery leaf whose definition is
// "declared", the same definition without the declaration, a regression
// sibling, and the three statements the requirements define now.
const (
	deliveredCommit      = "0123456789abcdef0123456789abcdef01234567"
	declaredDefinition   = "sha256:declared"
	undeclaredDefinition = "sha256:undeclared"
	siblingDefinition    = "sha256:sibling"
)

func deliveryLeaf() LeafTask {
	return LeafTask{ID: "1", Completed: true, Fingerprint: declaredDefinition, Delivery: true, Undeclared: undeclaredDefinition}
}

func siblingLeaf() LeafTask {
	return LeafTask{ID: "2", Completed: true, Fingerprint: siblingDefinition}
}

func deliveryChain() ChainFingerprints {
	return ChainFingerprints{
		Requirements: "sha256:req", Design: "sha256:des",
		Statements: map[string]string{"R1.AC1": "sha256:first", "R1.AC2": "sha256:second", "R2.AC1": "sha256:other"},
	}
}

// passOf is a pass with supported provenance, recorded for a definition on
// the code "sha256:code" under the approvals of deliveryChain.
func passOf(definition string) Record {
	return Record{
		TaskFingerprint: definition, TaskFingerprintScheme: spec.TaskFingerprintScheme,
		Execution: &ExecutionFacts{
			Origin: "verify", Policy: VerifyPolicy, AssertionResult: ResultPassed, Integrity: "pure",
			BeforeCodeIdentity: "sha256:code", AfterCodeIdentity: "sha256:code",
		},
		RequirementsFingerprint: "sha256:req", DesignFingerprint: "sha256:des", TasksFingerprint: "sha256:tasks",
		CodeIdentity: "sha256:code", Steps: []StepResult{{Command: []string{"prove"}}}, Result: ResultPassed,
	}
}

// attestedPass is a pass with the attestation an execution writes: the
// record is bound to the declared definition, which is the attested one.
func attestedPass() Record {
	record := passOf(declaredDefinition)
	record.Attestation = &Attestation{
		Commit: deliveredCommit, TaskFingerprint: declaredDefinition,
		Statements: StatementFacts{Scheme: spec.StatementScheme, Fingerprints: map[string]string{"R1.AC1": "sha256:first", "R1.AC2": "sha256:second"}},
	}
	return record
}

// sealedPass is a pass adoption sealed: recorded for the definition without
// the declaration, attested for the declared one.
func sealedPass() Record {
	record := attestedPass()
	record.TaskFingerprint = undeclaredDefinition
	record.Attestation.Sealed = true
	return record
}

// deriveDelivery derives the delivery leaf and its regression sibling, whose
// record is a plain pass of its own definition.
func deriveDelivery(record Record, leaf LeafTask, chain ChainFingerprints, identity string) (delivery, sibling TaskEvidence) {
	document := Document{Feature: "f", Tasks: map[string]Record{"1": record, "2": passOf(siblingDefinition)}}
	derived := Derive(document, chain, identity, true, []LeafTask{leaf, siblingLeaf()})
	return derived[0], derived[1]
}

func gapOf(entry TaskEvidence, kind string) string {
	for _, gap := range entry.Gaps {
		if gap.Kind == kind {
			return gap.Message
		}
	}
	return ""
}

func TestDeriveDelivered(t *testing.T) {
	t.Run("delivered", func(t *testing.T) {
		for name, record := range map[string]Record{"attested by an execution": attestedPass(), "sealed": sealedPass()} {
			// The identity is still the recorded one: the task would read
			// verified as a regression task, and must not.
			entry, sibling := deriveDelivery(record, deliveryLeaf(), deliveryChain(), "sha256:code")
			if entry.State != StateDelivered {
				t.Errorf("%s: state %s, want delivered (gaps %v)", name, entry.State, entry.Gaps)
				continue
			}
			if entry.Attestation == nil || entry.Attestation.Commit != deliveredCommit || entry.Attestation.Sealed != record.Attestation.Sealed {
				t.Errorf("%s: attestation %+v, want commit %s sealed=%t", name, entry.Attestation, deliveredCommit, record.Attestation.Sealed)
			}
			if len(entry.Gaps) != 0 {
				t.Errorf("%s: a delivered task carries gaps: %v", name, entry.Gaps)
			}
			if !entry.Delivery || entry.AwaitsAdoption {
				t.Errorf("%s: delivery=%t awaits=%t", name, entry.Delivery, entry.AwaitsAdoption)
			}
			if entry.Binding.State != "current" || entry.Binding.EffectiveFingerprint != declaredDefinition {
				t.Errorf("%s: binding %+v", name, entry.Binding)
			}
			if sibling.State != StateVerified || sibling.Delivery || sibling.Attestation != nil {
				t.Errorf("%s: the regression sibling reads %s, delivery=%t", name, sibling.State, sibling.Delivery)
			}
		}

		// A task that is not completed is pending, whatever its record holds.
		pending := deliveryLeaf()
		pending.Completed = false
		if entry, _ := deriveDelivery(attestedPass(), pending, deliveryChain(), "sha256:code"); entry.State != StatePending || entry.Attestation != nil {
			t.Errorf("an uncompleted task with an attestation reads %s", entry.State)
		}
	})

	t.Run("code-changed", func(t *testing.T) {
		for name, record := range map[string]Record{"attested by an execution": attestedPass(), "sealed": sealedPass()} {
			entry, sibling := deriveDelivery(record, deliveryLeaf(), deliveryChain(), "sha256:moved")
			if entry.State != StateDelivered || len(entry.Gaps) != 0 {
				t.Errorf("%s: after a code change the task reads %s with gaps %v, want delivered", name, entry.State, entry.Gaps)
			}
			if sibling.State != StateStaleCode {
				t.Errorf("%s: the regression sibling reads %s, want stale-code", name, sibling.State)
			}
		}
		// Without a current identity at all the state stands too.
		document := Document{Feature: "f", Tasks: map[string]Record{"1": attestedPass()}}
		if derived := Derive(document, deliveryChain(), "", false, []LeafTask{deliveryLeaf()}); derived[0].State != StateDelivered {
			t.Errorf("without a current identity the task reads %s, want delivered", derived[0].State)
		}
	})

	t.Run("reapproved", func(t *testing.T) {
		// The requirements and the design were approved again, and a
		// statement the task does not name reads differently.
		chain := deliveryChain()
		chain.Requirements, chain.Design = "sha256:req-again", "sha256:des-again"
		chain.Statements["R2.AC1"] = "sha256:reworded"
		for name, record := range map[string]Record{"attested by an execution": attestedPass(), "sealed": sealedPass()} {
			entry, sibling := deriveDelivery(record, deliveryLeaf(), chain, "sha256:code")
			if entry.State != StateDelivered || len(entry.Gaps) != 0 {
				t.Errorf("%s: after the re-approval the task reads %s with gaps %v, want delivered", name, entry.State, entry.Gaps)
			}
			if sibling.State != StateStaleSpec {
				t.Errorf("%s: the regression sibling reads %s, want stale-spec", name, sibling.State)
			}
		}
	})

	t.Run("statement-changed", func(t *testing.T) {
		reworded := deliveryChain()
		reworded.Requirements = "sha256:req-again"
		reworded.Statements["R1.AC2"] = "sha256:reworded"
		removed := deliveryChain()
		removed.Requirements = "sha256:req-again"
		delete(removed.Statements, "R1.AC1")
		both := deliveryChain()
		both.Statements["R1.AC2"] = "sha256:reworded"
		delete(both.Statements, "R1.AC1")

		for name, tc := range map[string]struct {
			chain ChainFingerprints
			wants []string
			lacks string
		}{
			"reworded": {reworded, []string{"R1.AC2 changed"}, "R1.AC1"},
			"removed":  {removed, []string{"R1.AC1 no longer defined"}, "R1.AC2"},
			"both":     {both, []string{"R1.AC1 no longer defined, R1.AC2 changed"}, "R2.AC1"},
		} {
			for kind, record := range map[string]Record{"attested by an execution": attestedPass(), "sealed": sealedPass()} {
				entry, _ := deriveDelivery(record, deliveryLeaf(), tc.chain, "sha256:code")
				if entry.State != StateStaleSpec || entry.Attestation != nil {
					t.Errorf("%s, %s: state %s, attestation %+v, want stale-spec and none", name, kind, entry.State, entry.Attestation)
				}
				message := gapOf(entry, "attested-statements-changed")
				for _, want := range append(tc.wants, deliveredCommit) {
					if !strings.Contains(message, want) {
						t.Errorf("%s, %s: the gap %q does not hold %q", name, kind, message, want)
					}
				}
				if strings.Contains(message, tc.lacks) {
					t.Errorf("%s, %s: the gap %q names %s, which reads as it did", name, kind, message, tc.lacks)
				}
			}
		}
	})

	t.Run("definition-changed", func(t *testing.T) {
		changedStep := deliveryLeaf()
		changedStep.Fingerprint, changedStep.Undeclared = "sha256:declared-with-another-step", "sha256:undeclared-with-another-step"
		// Without the declaration the leaf has the definition the plan held
		// before it: for a sealed record that is the one it is bound to.
		undeclared := LeafTask{ID: "1", Completed: true, Fingerprint: undeclaredDefinition}

		for name, leaf := range map[string]LeafTask{"a changed step": changedStep, "the declaration removed": undeclared} {
			for kind, record := range map[string]Record{"attested by an execution": attestedPass(), "sealed": sealedPass()} {
				entry, sibling := deriveDelivery(record, leaf, deliveryChain(), "sha256:code")
				if entry.State != StateStaleSpec || entry.Attestation != nil {
					t.Errorf("%s, %s: state %s, attestation %+v, want stale-spec and none", name, kind, entry.State, entry.Attestation)
				}
				if entry.Binding.State != "changed" || !strings.Contains(gapOf(entry, "binding-changed"), deliveredCommit) {
					t.Errorf("%s, %s: binding %+v, gaps %v: the gap must name the attested commit", name, kind, entry.Binding, entry.Gaps)
				}
				if sibling.State != StateVerified {
					t.Errorf("%s, %s: the sibling reads %s, want verified as before", name, kind, sibling.State)
				}
			}
		}

		// The attested definition is the current one, but the record is a
		// pass of something else: it contradicts its own attestation.
		foreign := attestedPass()
		foreign.TaskFingerprint = "sha256:something-else"
		unsealed := sealedPass()
		unsealed.Attestation.Sealed = false
		for name, record := range map[string]Record{"a record of another definition": foreign, "an unsealed record of the undeclared definition": unsealed} {
			entry, _ := deriveDelivery(record, deliveryLeaf(), deliveryChain(), "sha256:code")
			if entry.State != StateStaleSpec || entry.Binding.State != "contradictory" {
				t.Errorf("%s: state %s, binding %+v, want stale-spec and contradictory", name, entry.State, entry.Binding)
			}
		}
	})

	t.Run("not-supported", func(t *testing.T) {
		failed := attestedPass()
		failed.Result = ResultFailed
		failed.Execution.AssertionResult = ResultFailed
		rejected := attestedPass()
		rejected.Execution.Integrity = "mutated"
		// A record without execution facts, with an attestation written in
		// by hand: the attestation is not read.
		legacy := attestedPass()
		legacy.Execution = nil
		unknown := attestedPass()
		unknown.Execution.Policy = "something-else/v9"

		for name, tc := range map[string]struct {
			record Record
			want   string
		}{
			"a failed result":    {failed, StateFailed},
			"a rejected run":     {rejected, StateFailed},
			"legacy provenance":  {legacy, StateUnattested},
			"unknown provenance": {unknown, StateUnattested},
		} {
			entry, _ := deriveDelivery(tc.record, deliveryLeaf(), deliveryChain(), "sha256:code")
			if entry.State != tc.want || entry.Attestation != nil {
				t.Errorf("%s: state %s, attestation %+v, want %s and none", name, entry.State, entry.Attestation, tc.want)
			}
		}

		// The same records after a code change read what a regression task
		// reads, never delivered.
		if entry, _ := deriveDelivery(legacy, deliveryLeaf(), deliveryChain(), "sha256:moved"); entry.State != StateStaleCode {
			t.Errorf("legacy provenance after a code change reads %s, want stale-code", entry.State)
		}

		// An attestation this binary cannot read is no attestation: another
		// statement scheme, or no commit.
		otherScheme := attestedPass()
		otherScheme.Attestation.Statements.Scheme = "walden/statement-block/v9"
		noCommit := attestedPass()
		noCommit.Attestation.Commit = ""
		for name, record := range map[string]Record{"another statement scheme": otherScheme, "no commit": noCommit} {
			if entry, _ := deriveDelivery(record, deliveryLeaf(), deliveryChain(), "sha256:moved"); entry.State != StateStaleCode || entry.Attestation != nil {
				t.Errorf("%s: state %s, want the regression reading stale-code", name, entry.State)
			}
		}
	})

	t.Run("no-attestation", func(t *testing.T) {
		// Three worked examples. A pass written by an execution for the
		// declared definition, on a tree with uncommitted paths: the task
		// reads as a regression task reads.
		unattested := passOf(declaredDefinition)
		if entry, _ := deriveDelivery(unattested, deliveryLeaf(), deliveryChain(), "sha256:code"); entry.State != StateVerified || !entry.Delivery || entry.AwaitsAdoption {
			t.Errorf("identity current: state %s, delivery=%t awaits=%t, want verified", entry.State, entry.Delivery, entry.AwaitsAdoption)
		}
		if entry, _ := deriveDelivery(unattested, deliveryLeaf(), deliveryChain(), "sha256:moved"); entry.State != StateStaleCode {
			t.Errorf("identity changed: state %s, want stale-code", entry.State)
		}

		// A sealed record whose attestation is removed is bound to the
		// definition without the declaration: it awaits adoption again.
		removed := sealedPass()
		removed.Attestation = nil
		entry, _ := deriveDelivery(removed, deliveryLeaf(), deliveryChain(), "sha256:code")
		if entry.State != StateStaleSpec || !entry.AwaitsAdoption || entry.Attestation != nil {
			t.Errorf("a sealed record without its attestation: state %s, awaits=%t, want stale-spec awaiting adoption", entry.State, entry.AwaitsAdoption)
		}

		// Removing the attestation of an execution gives the first example.
		stripped := attestedPass()
		stripped.Attestation = nil
		if entry, _ := deriveDelivery(stripped, deliveryLeaf(), deliveryChain(), "sha256:code"); entry.State != StateVerified {
			t.Errorf("an execution's attestation removed: state %s, want verified", entry.State)
		}
	})
}

func TestAwaitsAdoption(t *testing.T) {
	definition := &spec.Task{ID: "1", Proof: spec.VerificationSpec{Steps: []spec.VerificationStep{{Argv: []string{"prove"}}}, Delivery: true}}
	leaf := deliveryLeaf()
	leaf.Definition = definition
	base := passOf(undeclaredDefinition)

	if !AwaitsAdoption(base, leaf) {
		t.Fatal("a pass recorded before the declaration, with supported provenance, does not await adoption")
	}

	regression := leaf
	regression.Delivery, regression.Undeclared = false, ""
	uncompleted := leaf
	uncompleted.Completed = false

	sealed := sealedPass()
	failed := passOf(undeclaredDefinition)
	failed.Result, failed.Execution.AssertionResult = ResultFailed, ResultFailed
	legacy := passOf(undeclaredDefinition)
	legacy.Execution = nil
	rejected := passOf(undeclaredDefinition)
	rejected.Execution.Integrity = "mutated"
	otherScheme := passOf(undeclaredDefinition)
	otherScheme.TaskFingerprintScheme = ""
	otherSteps := passOf(undeclaredDefinition)
	otherSteps.Steps = []StepResult{{Command: []string{"something", "else"}}}

	for name, tc := range map[string]struct {
		record Record
		leaf   LeafTask
	}{
		"a regression task":                 {base, regression},
		"an uncompleted task":               {base, uncompleted},
		"a record with an attestation":      {sealed, leaf},
		"a failed result":                   {failed, leaf},
		"legacy provenance":                 {legacy, leaf},
		"a rejected run":                    {rejected, leaf},
		"another fingerprint scheme":        {otherScheme, leaf},
		"a pass of the declared definition": {passOf(declaredDefinition), leaf},
		"a pass of another definition":      {passOf("sha256:something-else"), leaf},
		"steps that are not the proof's":    {otherSteps, leaf},
	} {
		if AwaitsAdoption(tc.record, tc.leaf) {
			t.Errorf("%s awaits adoption", name)
		}
	}

	// The predicate reads no tree fact and no statement fingerprint.
	withFacts := passOf(undeclaredDefinition)
	withFacts.Tree = &TreeFacts{Commit: deliveredCommit, Paths: map[string]string{"a.go": "100644:aaa"}}
	withFacts.Statements = &StatementFacts{Scheme: "walden/statement-block/v9", Fingerprints: map[string]string{"R1.AC1": "sha256:else"}}
	if !AwaitsAdoption(withFacts, leaf) {
		t.Error("the tree and statement facts of the record changed the answer")
	}

	// Derive reports it on the entry.
	document := Document{Feature: "f", Tasks: map[string]Record{"1": base}}
	entry := Derive(document, deliveryChain(), "sha256:code", true, []LeafTask{leaf})[0]
	if !entry.AwaitsAdoption || entry.State != StateStaleSpec {
		t.Errorf("Derive: awaits=%t state %s, want an awaiting pass that reads stale-spec", entry.AwaitsAdoption, entry.State)
	}
}

func TestFeatureInputsDelivery(t *testing.T) {
	const requirements = "# Requirements Document\n\n## Requirements\n\n### R1 First\n\n#### Acceptance Criteria\n\n" +
		"1. `R1.AC1` WHEN the first event occurs, the system SHALL respond.\n" +
		"2. `R1.AC2` WHEN the second event occurs, the system SHALL respond.\n"
	const plan = "# Implementation Plan\n\n" +
		"- [x] 1. A delivery\n  - Requirements: `R1.AC1`\n  - Design: Architecture\n  - Proof: delivery\n  - Verification:\n    - command: [\"true\"]\n      covers: [\"R1.AC1\"]\n" +
		"- [ ] 2. A regression\n  - Requirements: `R1.AC2`\n  - Design: Architecture\n  - Verification:\n    - command: [\"true\"]\n      covers: [\"R1.AC2\"]\n"
	feature := spec.Feature{
		Name:         "f",
		Requirements: spec.Document{Exists: true, Path: "requirements.md", Body: requirements, ApprovedFingerprint: "sha256:req"},
		Design:       spec.Document{Exists: true, Path: "design.md", ApprovedFingerprint: "sha256:des"},
		Tasks:        spec.Document{Exists: true, Path: "tasks.md", Body: plan},
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		t.Fatalf("parse the plan: %v", err)
	}

	chain, leaves := FeatureInputs(feature, tree)
	want := spec.StatementFingerprints(requirements)
	if len(want) != 2 || len(chain.Statements) != 2 || chain.Statements["R1.AC1"] != want["R1.AC1"] || chain.Statements["R1.AC2"] != want["R1.AC2"] {
		t.Errorf("statements = %v, want %v", chain.Statements, want)
	}
	if chain.Requirements != "sha256:req" || chain.Design != "sha256:des" {
		t.Errorf("the approval fingerprints changed: %+v", chain)
	}

	delivery, regression := leaves[0], leaves[1]
	if !delivery.Delivery || !delivery.Completed || delivery.Undeclared != spec.UndeclaredTaskDefinitionFingerprint(tree.LeafTasks()[0]) || delivery.Undeclared == delivery.Fingerprint {
		t.Errorf("delivery leaf: %+v", delivery)
	}
	if regression.Delivery || regression.Undeclared != "" {
		t.Errorf("regression leaf: delivery=%t undeclared=%q", regression.Delivery, regression.Undeclared)
	}
}
