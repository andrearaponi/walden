package spec

import (
	"fmt"
	"sort"
)

// ProofKindDelivery is the one proof kind a plan can declare, on the metadata
// line `- Proof: delivery` of a leaf task. A task without that line holds a
// regression proof: there is no value for the default.
const ProofKindDelivery = "delivery"

// proofKindPrefix starts the declaration once its indentation is trimmed. The
// colon belongs to it: `- Proof scope:` is not a declaration.
const proofKindPrefix = "- Proof:"

// parseProofKind reads the value of a `- Proof:` line. The parser and the
// draft layout check share it, so both report the same defect.
func parseProofKind(value string, index int, taskID string) error {
	if value != ProofKindDelivery {
		return fmt.Errorf(
			"line %d: unknown proof kind %q for task %q: expected %q; a task without a Proof line holds a regression proof",
			index+1, value, taskID, ProofKindDelivery,
		)
	}
	return nil
}

// nonLeafProofKindError names a task with subtasks that declares a proof
// kind: the declaration belongs to the leaf that holds the proof.
func nonLeafProofKindError(taskID string) error {
	return fmt.Errorf("non-leaf task %q cannot declare a proof kind", taskID)
}

// DeliveryOnlyStatements returns the statements that only delivery proofs
// cover: those a step of a delivery leaf names in covers and no step of a
// regression leaf does, in the order of a requirements document. Once such a
// task is delivered, nothing proves these statements again.
func DeliveryOnlyStatements(tree TaskTree) []string {
	delivery, regression := map[string]bool{}, map[string]bool{}
	for _, task := range tree.LeafTasks() {
		covered := regression
		if task.Proof.Delivery {
			covered = delivery
		}
		for _, step := range task.Proof.Steps {
			for _, id := range step.Covers {
				covered[id] = true
			}
		}
	}
	var only []string
	for id := range delivery {
		if !regression[id] {
			only = append(only, id)
		}
	}
	sort.Slice(only, func(i, j int) bool { return StatementLess(only[i], only[j]) })
	return only
}
