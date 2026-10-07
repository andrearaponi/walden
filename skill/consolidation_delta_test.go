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
