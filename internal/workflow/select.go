package workflow

import "github.com/andrearaponi/walden/internal/evidence"

// ProofSelection is what one run does with each completed task, in plan
// order. A task is in one list at most; a pending task is in none.
type ProofSelection struct {
	// Run holds the tasks whose proof the run executes.
	Run []string
	// Skipped holds the verified tasks a run leaves alone unless it is asked
	// for every task.
	Skipped []string
	// Kept holds the delivered tasks: their proof attested a delivery and is
	// not executed again.
	Kept []string
	// Left holds the tasks whose recorded pass awaits adoption: only a run
	// that names such a task replaces its pass.
	Left []string
}

// SelectProofs decides which proofs a run executes. It is the one place that
// decides, for walden verify and for the verify run of adoption, so the two
// cannot disagree on what is protected.
//
// all asks for every completed task, task names one, and attest says that the
// run is walden verify on a tree that is the tree of its commit. For each
// entry the first rule that matches decides:
//
//  1. a pending task is not selected;
//  2. with a named task, that task runs and no other;
//  3. a delivered task is kept;
//  4. a task whose pass awaits adoption is left for adoption;
//  5. with attest, a delivery task that holds no attestation runs, although
//     it may read verified: a pass on that tree attests it;
//  6. a verified task is skipped, unless every task is asked for;
//  7. anything else runs.
func SelectProofs(entries []evidence.TaskEvidence, all bool, task string, attest bool) ProofSelection {
	var selection ProofSelection
	for _, entry := range entries {
		switch {
		case entry.State == evidence.StatePending:
		case task != "":
			if entry.TaskID == task {
				selection.Run = append(selection.Run, entry.TaskID)
			}
		case entry.State == evidence.StateDelivered:
			selection.Kept = append(selection.Kept, entry.TaskID)
		case entry.AwaitsAdoption:
			selection.Left = append(selection.Left, entry.TaskID)
		case attest && entry.Delivery:
			selection.Run = append(selection.Run, entry.TaskID)
		case !all && entry.State == evidence.StateVerified:
			selection.Skipped = append(selection.Skipped, entry.TaskID)
		default:
			selection.Run = append(selection.Run, entry.TaskID)
		}
	}
	return selection
}
