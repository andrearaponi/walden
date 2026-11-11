package adopt

import (
	"context"
	"fmt"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/spec"
)

// PassToSeal is the recorded pass of a completed delivery task that adoption
// can seal as an attestation, with the commit that holds the tree it ran on.
type PassToSeal struct {
	TaskID string
	Commit string
}

// Reasons a recorded pass is left unsealed. They are checked in this order,
// cheapest first, and the first that holds is reported.
const (
	// The execution provenance of the pass is not supported: apply proves
	// the task again, as any task that is not verified.
	ReasonNoProvenance = "no-provenance"
	// The recorded proof differs from the approved one in more than the
	// declaration: apply proves the task again.
	ReasonProofChanged = "proof-changed"
	// It cannot be established that the statements the task names read as
	// they did at the pass: the pass stays as recorded.
	ReasonStatementsUnknown = "statements-unknown"
	// No commit the search covers holds the tree of the pass: the pass stays
	// as recorded.
	ReasonNoCommit = "no-commit"
)

// UnsealedPass is a recorded pass of a delivery task that adoption leaves
// unsealed. Detail says why and what proves the task again.
type UnsealedPass struct {
	TaskID string
	Reason string
	Detail string
}

// passAssessment is what adoption can do with the recorded passes of the
// delivery tasks of one feature.
type passAssessment struct {
	sealable []PassToSeal
	unsealed []UnsealedPass
	// attestations holds, by task, the attestation apply adds to each
	// sealable record.
	attestations map[string]evidence.Attestation
}

// assessPasses looks at each completed delivery leaf whose record holds a
// pass and no attestation. A pass bound to the current definition was
// recorded after the plan declared the delivery: walden verify attests it,
// and it is not offered here. For every other pass the checks run cheapest
// first, and git is asked only for a pass that passed the others.
func assessPasses(ctx context.Context, name string, assessed featureEvidence, search *evidence.TreeSearch) passAssessment {
	result := passAssessment{attestations: map[string]evidence.Attestation{}}
	for _, leaf := range assessed.leafs {
		if !leaf.Delivery || !leaf.Completed || leaf.Definition == nil {
			continue
		}
		record, recorded := assessed.ledger.Tasks[leaf.ID]
		if !recorded || record.Result != evidence.ResultPassed || record.Attestation != nil {
			continue
		}
		if record.TaskFingerprintScheme == spec.TaskFingerprintScheme && record.TaskFingerprint == leaf.Fingerprint {
			continue
		}

		leave := func(reason, detail string) {
			result.unsealed = append(result.unsealed, UnsealedPass{TaskID: leaf.ID, Reason: reason, Detail: detail})
		}
		// A pass that fails one of the first two checks does not await
		// adoption: its task is proven again by apply, as any task that is
		// not verified. A pass that fails one of the last two still awaits
		// adoption, so no run replaces it unless it names the task.
		byApply := fmt.Sprintf("walden adopt %s --apply proves the task again", name)
		byTask := fmt.Sprintf("the pass stays as recorded, and walden verify %s --task %s proves the task again", name, leaf.ID)

		if !evidence.SupportedExecution(record) {
			leave(ReasonNoProvenance, "the recorded pass has no supported execution provenance; "+byApply)
			continue
		}
		if !evidence.AwaitsAdoption(record, leaf) {
			leave(ReasonProofChanged, "the recorded proof differs from the approved one in more than the declaration; "+byApply)
			continue
		}
		named := evidence.NamedStatements(assessed.current.Statements, leaf.Definition.Requirements, leaf.Definition.Proof)
		if !statementsEstablished(record, assessed.current, named) {
			leave(ReasonStatementsUnknown, "it cannot be established that the statements the task names read as they did at the pass; "+byTask)
			continue
		}
		commit, found := search.Commit(ctx, record.CodeIdentity)
		if !found {
			leave(ReasonNoCommit, noCommitDetail(search)+"; "+byTask)
			continue
		}
		result.sealable = append(result.sealable, PassToSeal{TaskID: leaf.ID, Commit: commit})
		result.attestations[leaf.ID] = evidence.Attestation{
			Commit: commit, TaskFingerprint: leaf.Fingerprint, Statements: *named, Sealed: true,
		}
	}
	return result
}

// statementsEstablished reports that the statements a task names read now as
// they did at the pass. Either the pass was recorded under the current
// approval of the requirements, so every statement reads as it did, or the
// record keeps the fingerprints of the statements it named and they equal the
// current ones. This is the one place that reads the statement fingerprints
// of a record to decide something.
func statementsEstablished(record evidence.Record, current evidence.ChainFingerprints, named *evidence.StatementFacts) bool {
	if record.RequirementsFingerprint != "" && record.RequirementsFingerprint == current.Requirements {
		return true
	}
	facts := record.Statements
	if facts == nil || facts.Scheme != spec.StatementScheme || len(facts.Fingerprints) != len(named.Fingerprints) {
		return false
	}
	for id, fingerprint := range named.Fingerprints {
		if facts.Fingerprints[id] != fingerprint {
			return false
		}
	}
	return true
}

// noCommitDetail says what the search covered when it found no commit.
func noCommitDetail(search *evidence.TreeSearch) string {
	switch covered := search.Covered(); {
	case covered == 0:
		return "no commit could be read, so none holds the tree of the pass"
	case search.Truncated():
		return fmt.Sprintf("no commit among the last %d commit(s) reachable from HEAD holds the tree of the pass, and the search stops at that limit", covered)
	default:
		return fmt.Sprintf("no commit among the %d commit(s) reachable from HEAD holds the tree of the pass", covered)
	}
}

// sealPasses adds the attestation to each recorded pass of the feature that
// can be sealed, and saves the ledger once. It executes no proof and touches
// no other field of a record: the result, the steps, the date and the
// execution facts stay as they were written. The checks are those of the
// plan, made again on the ledger as it is now, so a second run finds an
// attestation and seals nothing.
func sealPasses(ctx context.Context, root, name string, search *evidence.TreeSearch) ([]PassToSeal, []UnsealedPass, error) {
	feature, err := spec.LoadFeature(root, name)
	if err != nil {
		return nil, nil, err
	}
	if !feature.Tasks.Exists || feature.Tasks.Status != "approved" {
		return nil, nil, nil
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		return nil, nil, fmt.Errorf("tasks assessment unavailable: %w", err)
	}
	ledger, err := evidence.Load(root, feature.Name)
	if err != nil {
		return nil, nil, err
	}
	current, leafs := evidence.FeatureInputs(feature, tree)
	passes := assessPasses(ctx, name, featureEvidence{ledger: ledger, current: current, leafs: leafs}, search)
	if len(passes.sealable) == 0 {
		return nil, passes.unsealed, nil
	}
	for _, pass := range passes.sealable {
		record := ledger.Tasks[pass.TaskID]
		attestation := passes.attestations[pass.TaskID]
		record.Attestation = &attestation
		ledger.Tasks[pass.TaskID] = record
	}
	ledger.Feature = feature.Name
	if err := evidence.Save(root, ledger); err != nil {
		return nil, passes.unsealed, fmt.Errorf("seal recorded passes: %w", err)
	}
	return passes.sealable, passes.unsealed, nil
}
