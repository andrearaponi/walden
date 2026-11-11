package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const declareDeliveryAnchor = "    - Design: Task Store\n    - Verification:\n      - command: [\"grep\""

// declareDelivery adds the declaration to task 1.1 of the delivery feature
// and seals the plan again, as an approved revision of the plan does.
func declareDelivery(t *testing.T, root string) {
	t.Helper()
	reapprove(t, root, "tasks.md", func(content string) string {
		return strings.Replace(content, declareDeliveryAnchor, "    - Design: Task Store\n"+deliveryDeclarationLine+"    - Verification:\n      - command: [\"grep\"", 1)
	})
}

// awaitingAdoptionRepo is a committed repository whose task 1.1 was completed
// before the plan declared its proof as a delivery proof. With committed set
// the pass ran on the committed tree; otherwise it ran with an uncommitted
// file that was changed before it was committed, so no commit holds its tree.
func awaitingAdoptionRepo(t *testing.T, committed bool) (root, head string) {
	t.Helper()
	root = deliveryStatusFixture(t, strings.Replace(deliveryStatusTasks, deliveryDeclarationLine, "", 1))
	gitify(t, root)
	head = integrityGit(t, root, "rev-parse", "HEAD")
	if !committed {
		diagnosticsFile(t, root, "draft.txt", "as proven\n")
	}
	completeStatusTask(t, "1.1")
	if !committed {
		diagnosticsFile(t, root, "draft.txt", "changed before the commit\n")
		integrityGit(t, root, "add", "-A")
		integrityGit(t, root, "commit", "-qm", "the draft, changed")
	}
	completeStatusTask(t, "1.2")
	declareDelivery(t, root)
	return root, head
}

// adoptBoth runs `walden adopt` on the delivery feature in JSON and in text
// mode, with the extra arguments given, and returns the feature entry of the
// JSON result beside the whole result and the text.
func adoptBoth(t *testing.T, extra ...string) (feature, result map[string]any, text string) {
	t.Helper()
	args := append([]string{"adopt", verifyTaskFeature}, extra...)
	var stdout, stderr bytes.Buffer
	Run(append(append([]string{}, args...), "--json"), &stdout, &stderr)
	var envelope struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("walden %v --json printed no envelope: %v (%s%s)", args, err, stdout.String(), stderr.String())
	}
	adoption, _ := envelope.Result["adoption"].(map[string]any)
	features, _ := adoption["features"].([]any)
	if len(features) != 1 {
		t.Fatalf("the adoption result holds %d feature(s), want one: %v", len(features), envelope.Result)
	}
	stdout.Reset()
	stderr.Reset()
	Run(args, &stdout, &stderr)
	return features[0].(map[string]any), envelope.Result, stdout.String() + stderr.String()
}

func TestAdoptPlanShowsPasses(t *testing.T) {
	t.Run("a pass to seal", func(t *testing.T) {
		_, head := awaitingAdoptionRepo(t, true)
		feature, result, text := adoptBoth(t)

		passes, _ := feature["sealable_passes"].([]any)
		if len(passes) != 1 {
			t.Fatalf("sealable_passes = %v, want one", feature["sealable_passes"])
		}
		if pass := passes[0].(map[string]any); pass["task_id"] != "1.1" || pass["commit"] != head {
			t.Errorf("sealable pass = %v, want task 1.1 at %s", pass, head)
		}
		if feature["class"] != "backfill" || feature["unsealed_passes"] != nil || feature["sealed_passes"] != nil {
			t.Errorf("feature = %v", feature)
		}
		summary := "ADOPTION PLAN — 1 backfill, 0 re-prove, 0 complete, 0 blocked (0 doc(s) to seal, 0 task(s) to re-prove, 1 recorded pass(es) to seal)"
		if result["summary"] != summary || textLine(text, "Summary: ") != "Summary: "+summary {
			t.Errorf("summary %q / %q, want %q", result["summary"], textLine(text, "Summary: "), summary)
		}
		for _, want := range []string{
			"- " + verifyTaskFeature + ": backfill (1 recorded pass(es) to seal)\n",
			"  seal 1.1 at " + head + "\n",
			"Next action: Review this scope and its trust assumptions before explicitly running walden adopt " + verifyTaskFeature + " --apply\n",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("the text does not hold %q:\n%s", want, text)
			}
		}
	})

	t.Run("a pass left unsealed", func(t *testing.T) {
		awaitingAdoptionRepo(t, false)
		// The regression task 1.2 was completed on the final tree: the pass
		// is the only open work of the feature.
		feature, result, text := adoptBoth(t)

		passes, _ := feature["unsealed_passes"].([]any)
		if len(passes) != 1 {
			t.Fatalf("unsealed_passes = %v, want one", feature["unsealed_passes"])
		}
		pass := passes[0].(map[string]any)
		detail, _ := pass["detail"].(string)
		if pass["task_id"] != "1.1" || pass["reason"] != "no-commit" || !strings.Contains(detail, "walden verify "+verifyTaskFeature+" --task 1.1") {
			t.Errorf("unsealed pass = %v", pass)
		}
		if feature["class"] != "re-prove" || feature["sealable_passes"] != nil {
			t.Errorf("feature = %v", feature)
		}
		summary := "ADOPTION PLAN — 0 backfill, 1 re-prove, 0 complete, 0 blocked (0 doc(s) to seal, 0 task(s) to re-prove, 1 recorded pass(es) left unsealed)"
		if result["summary"] != summary || textLine(text, "Summary: ") != "Summary: "+summary {
			t.Errorf("summary %q / %q, want %q", result["summary"], textLine(text, "Summary: "), summary)
		}
		for _, want := range []string{
			"- " + verifyTaskFeature + ": re-prove\n",
			"  unsealed 1.1 (no-commit): " + detail + "\n",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("the text does not hold %q:\n%s", want, text)
			}
		}
		// Apply would execute nothing here, so the plan does not propose it:
		// it points to the command each unsealed pass names.
		action, _ := result["next_action"].(string)
		if strings.Contains(action, "--apply") || !strings.Contains(action, "walden verify") {
			t.Errorf("next action %q", action)
		}
	})
}

func TestAdoptApplyShowsSealedPasses(t *testing.T) {
	t.Run("sealed", func(t *testing.T) {
		// JSON and text are taken on two repositories built alike: the first
		// apply seals, so a second one on the same repository has nothing
		// left to report.
		_, head := awaitingAdoptionRepo(t, true)
		feature, result, _ := adoptBoth(t, "--apply")
		passes, _ := feature["sealed_passes"].([]any)
		if len(passes) != 1 {
			t.Fatalf("sealed_passes = %v, want one", feature["sealed_passes"])
		}
		if pass := passes[0].(map[string]any); pass["task_id"] != "1.1" || pass["commit"] != head {
			t.Errorf("sealed pass = %v, want task 1.1 at %s", pass, head)
		}
		if feature["verified"] != nil || feature["failed"] != nil || feature["unsealed_passes"] != nil || feature["sealable_passes"] != nil {
			t.Errorf("feature = %v: nothing is proven and nothing is left", feature)
		}
		summary := "ADOPTION — sealed 0 doc(s), verified 0, failed 0, skipped 1, blocked 0, errors 0, sealed 1 recorded pass(es)"
		if result["summary"] != summary || result["exit_code"] != float64(0) {
			t.Errorf("summary %q, exit %v, want %q and 0", result["summary"], result["exit_code"], summary)
		}

		// The reports that name the attestation mark it as sealed.
		entry := statusEntries(t)["1.1"]
		if entry.State != "delivered" || entry.Attestation == nil || entry.Attestation.Commit != head || !entry.Attestation.Sealed {
			t.Errorf("evidence status: task 1.1 reads %s at %+v, want delivered at %s, sealed", entry.State, entry.Attestation, head)
		}
		if want := "- 1.1: delivered at " + head + " (sealed by adoption) ["; !strings.HasPrefix(statusLine(t, statusText(t), "1.1"), want) {
			t.Errorf("evidence status line %q", statusLine(t, statusText(t), "1.1"))
		}
		verdict, verdictText := releaseBoth(t, verifyTaskFeature)
		sealed := releaseEntries(t, verdict)["1.1"]["attestation"].(map[string]any)
		if sealed["sealed"] != true || sealed["commit"] != head {
			t.Errorf("release check: attestation %v", sealed)
		}
		if want := "  - 1.1: delivered at " + head + " (sealed by adoption) ["; !strings.Contains(verdictText, want) {
			t.Errorf("release check text does not hold %q:\n%s", want, verdictText)
		}

		// The text of the same run, on a second repository.
		_, head = awaitingAdoptionRepo(t, true)
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"adopt", verifyTaskFeature, "--apply"}, &stdout, &stderr); code != 0 {
			t.Fatalf("adopt --apply exited %d: %s%s", code, stdout.String(), stderr.String())
		}
		for _, want := range []string{"Summary: " + summary + "\n", "  sealed 1.1 at " + head + "\n"} {
			if !strings.Contains(stdout.String(), want) {
				t.Errorf("the text does not hold %q:\n%s", want, stdout.String())
			}
		}
		if strings.Contains(stdout.String(), "starting proof") {
			t.Errorf("apply reports a proof it did not run:\n%s", stdout.String())
		}
	})

	t.Run("unsealed", func(t *testing.T) {
		awaitingAdoptionRepo(t, false)
		feature, result, _ := adoptBoth(t, "--apply")
		passes, _ := feature["unsealed_passes"].([]any)
		if len(passes) != 1 {
			t.Fatalf("unsealed_passes = %v, want one", feature["unsealed_passes"])
		}
		pass := passes[0].(map[string]any)
		detail, _ := pass["detail"].(string)
		if pass["task_id"] != "1.1" || pass["reason"] != "no-commit" || !strings.Contains(detail, "walden verify "+verifyTaskFeature+" --task 1.1") {
			t.Errorf("unsealed pass = %v", pass)
		}
		if feature["sealed_passes"] != nil || feature["verified"] != nil || feature["failed"] != nil {
			t.Errorf("feature = %v: the task of an unsealed pass is not proven", feature)
		}
		// An unsealed pass is no failed execution: the exit status is the
		// one of a run without failures, and the summary names the pass.
		summary := "ADOPTION — sealed 0 doc(s), verified 0, failed 0, skipped 1, blocked 0, errors 0, 1 recorded pass(es) left unsealed"
		if result["summary"] != summary || result["exit_code"] != float64(0) {
			t.Errorf("summary %q, exit %v, want %q and 0", result["summary"], result["exit_code"], summary)
		}
		if action, _ := result["next_action"].(string); !strings.Contains(action, "walden verify") {
			t.Errorf("next action %q does not point to the command that proves the task again", action)
		}

		var stdout, stderr bytes.Buffer
		if code := Run([]string{"adopt", verifyTaskFeature, "--apply"}, &stdout, &stderr); code != 0 {
			t.Fatalf("adopt --apply exited %d: %s%s", code, stdout.String(), stderr.String())
		}
		for _, want := range []string{"Summary: " + summary + "\n", "  unsealed 1.1 (no-commit): " + detail + "\n"} {
			if !strings.Contains(stdout.String(), want) {
				t.Errorf("the text does not hold %q:\n%s", want, stdout.String())
			}
		}
	})
}
