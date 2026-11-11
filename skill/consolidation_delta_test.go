package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// evaluatedGuideSHA256 is the candidate guide the round-4 evaluation measured.
const evaluatedGuideSHA256 = "0bc704eeddadd529b33b0bf34ec3b855399db300716203f8c57b3e83852e1958"

type guideReplacement struct{ Old, New string }

// approvedGuideDelta is the documentation-only change approved after the
// round-4 evaluation (R7.AC4): the shipped guide is the evaluated guide with
// exactly these replacements and nothing else.
var approvedGuideDelta = []guideReplacement{
	{Old: "| `walden consolidate` | Read-only report: contract changes since the last consolidation, review scope, deterministic findings and identifiers. |",
		New: "| `walden consolidate` | Read-only report: contract changes since the last consolidation, review scope, deterministic findings, comparisons of each change with its linked statements, and identifiers. |"},
	{Old: "| `walden consolidate open` | Open `.walden/contracts.md` for review; refused while it disagrees with the specifications. |",
		New: "| `walden consolidate open` | Open `.walden/contracts.md` for review; refused while it disagrees with the specifications or, with changes pending, its coherence review is incomplete. |"},
	{Old: "| `walden consolidate approve` | Seal the reviewed view and record the checkpoint, only after explicit user approval. |",
		New: "| `walden consolidate approve` | Seal the reviewed view and record the checkpoint, only after explicit user approval; refused while a covered feature has a revision in review. |"},
	{Old: "citing the linked statements you compared as `feature#ID`;",
		New: "citing the linked statements you compared as `feature#ID` in inline code;"},
	{Old: "Run `walden consolidate open`, present the view, and run `walden consolidate approve` only after the user's explicit approval.",
		New: "Run `walden consolidate open`, present the view, and run `walden consolidate approve` only after the user's explicit approval. If `status` reports the view stale, review it and run `walden consolidate open` again."},
	// Approved separately, for CLI v0.14.2: no evaluation measured this row.
	{Old: "| `walden verify <feature> --check` | Report without persisting the ledger; add `--all` to force all completed proofs. |",
		New: "| `walden verify <feature> --check` | Report without persisting the ledger; add `--all` to force all completed proofs. |\n| `walden verify <feature> --task <id>` | On CLI v0.14.2 or newer, re-prove one completed leaf task after a targeted repair; `release check` still judges every task, so finish with `walden verify <feature>`. |"},
	// Approved with the plan that introduced delivery proofs: no evaluation
	// measured these three changes. The guide names no version for them: the
	// help of the installed CLI says whether it knows delivery proofs.
	{Old: "Do not retrofit approved proofs outside an authorized revision and re-review cycle.",
		New: "Do not retrofit approved proofs outside an authorized revision and re-review cycle.\n\n" +
			"A proof that describes a delivery, not a behavior, was true when the work shipped and is not expected to hold forever: a version string in a document, a file that a one-time bootstrap task created. " +
			"Where `walden verify --help` describes delivered tasks, a leaf can declare the metadata line `- Proof: delivery`, at the indentation of `Requirements:`; an earlier CLI skips the line and judges the proof as a regression. " +
			"Once its proof passes on a committed tree, usually at the first `walden verify` after the work is committed, the task reads `delivered` and is not proven again. " +
			"Declare it only for a fact about what shipped. " +
			"A check that must keep holding gets a regression leaf of its own, and the declaration is never a way to silence a failing regression proof. " +
			"It is a plan change that the user approves through the task-plan review: propose it, never add it on your own. " +
			"After a declaration on a completed task, its recorded pass goes through `walden adopt <feature>` before any `walden verify`: the plan says whether apply can seal the pass without running the proof, and a run that proves the task again replaces the pass, with a failure when the delivered text has moved on."},
	{Old: "Evidence states are derived: `verified`, `stale-spec`, `stale-code`, `failed`, `unattested`, `unrecorded`, and `pending`.",
		New: "Evidence states are derived: `verified`, `delivered`, `stale-spec`, `stale-code`, `failed`, `unattested`, `unrecorded`, and `pending`. `delivered` means that a delivery proof passed on the tree of the commit the report names: no run proves it again, with or without `--all`, and it is never `verified`."},
	{Old: "New `v1alpha2` ledgers can contain untouched legacy records; use a compatible CLI and never delete or downgrade evidence to satisfy an old reader.",
		New: "New `v1alpha2` ledgers can contain untouched legacy records, and a ledger that holds the attestation of a delivery proof is written as `v1alpha3`, which earlier CLIs refuse; use a compatible CLI and never delete or downgrade evidence to satisfy an old reader."},
}

// applyGuideDelta applies the replacements, or reverts them, requiring each
// source text to occur exactly once.
func applyGuideDelta(text string, delta []guideReplacement, reverse bool) (string, error) {
	for _, replacement := range delta {
		from, to := replacement.Old, replacement.New
		if reverse {
			from, to = replacement.New, replacement.Old
		}
		if count := strings.Count(text, from); count != 1 {
			return "", fmt.Errorf("%q occurs %d times, want exactly once", from, count)
		}
		text = strings.Replace(text, from, to, 1)
	}
	return text, nil
}

// checkGuideDelta accepts a shipped guide only when it is the evaluated guide
// with exactly the approved replacements.
func checkGuideDelta(evaluated, shipped string, delta []guideReplacement) error {
	applied, err := applyGuideDelta(evaluated, delta, false)
	if err != nil {
		return err
	}
	if applied != shipped {
		return errors.New("the shipped guide differs from the evaluated guide beyond the approved delta")
	}
	return nil
}

func TestEvaluatedGuideDelta(t *testing.T) {
	t.Run("the shipped guide is the evaluated guide plus the approved delta", func(t *testing.T) {
		shipped := string(canonicalGuide(t))
		evaluated, err := applyGuideDelta(shipped, approvedGuideDelta, true)
		if err != nil {
			t.Fatalf("reverting the approved delta: %v", err)
		}
		sum := sha256.Sum256([]byte(evaluated))
		if got := hex.EncodeToString(sum[:]); got != evaluatedGuideSHA256 {
			t.Fatalf("the reverted guide hashes to %s, want the evaluated %s", got, evaluatedGuideSHA256)
		}
		if err := checkGuideDelta(evaluated, shipped, approvedGuideDelta); err != nil {
			t.Fatal(err)
		}
	})

	evaluated := "A one.\nB two.\nC three.\n"
	delta := []guideReplacement{{Old: "B two.", New: "B two, clarified."}}
	if err := checkGuideDelta(evaluated, "A one.\nB two, clarified.\nC three.\n", delta); err != nil {
		t.Fatalf("synthetic delta rejected: %v", err)
	}
	for name, tc := range map[string]struct{ evaluated, shipped string }{
		"another change":          {evaluated, "A one!\nB two, clarified.\nC three.\n"},
		"replacement not applied": {evaluated, evaluated},
		"different new text":      {evaluated, "A one.\nB two, changed.\nC three.\n"},
		"old text absent":         {"A one.\nC three.\n", "A one.\nB two, clarified.\nC three.\n"},
		"old text present twice":  {"B two.\nB two.\n", "B two, clarified.\nB two.\n"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := checkGuideDelta(tc.evaluated, tc.shipped, delta); err == nil {
				t.Fatal("accepted")
			} else {
				t.Logf("rejected as intended: %v", err)
			}
		})
	}
}
