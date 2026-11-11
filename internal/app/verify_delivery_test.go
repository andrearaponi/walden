package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/testutil"
)

// verifyBoth runs `walden verify` on the delivery feature in JSON and in text
// mode, with a runner that passes every proof, and returns the JSON result as
// a generic object beside the text. Check mode keeps the two runs identical:
// the first does not change what the second sees.
func verifyBoth(t *testing.T, proofs int) (result map[string]any, text string) {
	t.Helper()
	responses := make([]testutil.Response, proofs)
	for i := range responses {
		responses[i] = testutil.Response{Stdout: "ok"}
	}
	var stdout, stderr bytes.Buffer
	overrideCommandRunner(t, testutil.NewFakeRunner(responses...))
	if code := Run([]string{"verify", verifyTaskFeature, "--check", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("verify --json exited %d: %s%s", code, stdout.String(), stderr.String())
	}
	var envelope struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v (%s)", err, stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	overrideCommandRunner(t, testutil.NewFakeRunner(responses...))
	if code := Run([]string{"verify", verifyTaskFeature, "--check"}, &stdout, &stderr); code != 0 {
		t.Fatalf("verify exited %d: %s%s", code, stdout.String(), stderr.String())
	}
	return envelope.Result, stdout.String()
}

func textLine(text, prefix string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

func TestVerifyReportsKeptAndLeft(t *testing.T) {
	t.Run("kept", func(t *testing.T) {
		root, head := deliveredStatusRepo(t)
		diagnosticsFile(t, root, "notes.txt", "superseded\n")
		integrityGit(t, root, "commit", "-qam", "the notes move on")

		result, text := verifyBoth(t, 1)
		kept, _ := result["kept"].([]any)
		if len(kept) != 1 {
			t.Fatalf("kept = %v, want one task", result["kept"])
		}
		if entry := kept[0].(map[string]any); entry["task_id"] != "1.1" || entry["commit"] != head || entry["sealed"] != false {
			t.Errorf("kept entry = %v, want task 1.1 at %s, not sealed", entry, head)
		}
		// The kept task is apart from the proven ones: the evidence of the
		// run holds the regression task alone.
		proven, _ := result["evidence"].([]any)
		if len(proven) != 1 || proven[0].(map[string]any)["task_id"] != "1.2" {
			t.Errorf("proven tasks = %v, want 1.2 alone", result["evidence"])
		}
		if _, present := result["left_for_adoption"]; present {
			t.Errorf("a run with nothing left for adoption carries the field: %v", result["left_for_adoption"])
		}
		if want := "Kept as delivered: 1.1 at " + head; textLine(text, "Kept as delivered:") != want {
			t.Errorf("text line %q, want %q", textLine(text, "Kept as delivered:"), want)
		}
		summary := "verification passed for " + verifyTaskFeature + ": 1 task(s) re-proven, 0 skipped as verified, 1 kept as delivered (check mode: nothing persisted)"
		if result["summary"] != summary || textLine(text, "Summary: ") != "Summary: "+summary {
			t.Errorf("summary %q / %q, want %q", result["summary"], textLine(text, "Summary: "), summary)
		}

		// A sealed attestation keeps its mark.
		rewriteLedger(t, root, func(tasks map[string]any) {
			tasks["1.1"].(map[string]any)["attestation"].(map[string]any)["sealed"] = true
		})
		result, text = verifyBoth(t, 1)
		if entry := result["kept"].([]any)[0].(map[string]any); entry["sealed"] != true {
			t.Errorf("kept entry = %v, want sealed", entry)
		}
		if want := "Kept as delivered: 1.1 at " + head + " (sealed by adoption)"; textLine(text, "Kept as delivered:") != want {
			t.Errorf("text line %q, want %q", textLine(text, "Kept as delivered:"), want)
		}
	})

	t.Run("left", func(t *testing.T) {
		root := deliveryStatusFixture(t, strings.Replace(deliveryStatusTasks, deliveryDeclarationLine, "", 1))
		gitify(t, root)
		completeStatusTask(t, "1.1")
		completeStatusTask(t, "1.2")
		reapprove(t, root, "tasks.md", func(content string) string {
			return strings.Replace(content, "    - Design: Task Store\n    - Verification:\n      - command: [\"grep\"", "    - Design: Task Store\n"+deliveryDeclarationLine+"    - Verification:\n      - command: [\"grep\"", 1)
		})
		if entry := statusEntries(t)["1.1"]; !entry.AwaitsAdoption || entry.State != evidence.StateStaleSpec {
			t.Fatalf("the fixture pass must await adoption: %+v", entry)
		}

		result, text := verifyBoth(t, 0)
		left, _ := result["left_for_adoption"].([]any)
		if len(left) != 1 || left[0] != "1.1" {
			t.Errorf("left_for_adoption = %v, want task 1.1", result["left_for_adoption"])
		}
		if _, present := result["kept"]; present {
			t.Errorf("a run that kept nothing carries the field: %v", result["kept"])
		}
		if want := "Left for adoption: 1.1 — walden adopt " + verifyTaskFeature; textLine(text, "Left for adoption:") != want {
			t.Errorf("text line %q, want %q", textLine(text, "Left for adoption:"), want)
		}
		summary := "nothing to verify for " + verifyTaskFeature + ": 1 completed task(s) already verified, 1 left for adoption (check mode: nothing persisted)"
		if result["summary"] != summary || textLine(text, "Summary: ") != "Summary: "+summary {
			t.Errorf("summary %q / %q, want %q", result["summary"], textLine(text, "Summary: "), summary)
		}
		action, _ := result["next_action"].(string)
		if !strings.Contains(action, "walden adopt "+verifyTaskFeature) || !strings.Contains(textLine(text, "Next action: "), "walden adopt "+verifyTaskFeature) {
			t.Errorf("the next action does not name walden adopt: %q / %q", action, textLine(text, "Next action: "))
		}
	})

	t.Run("worded as before", func(t *testing.T) {
		// Without delivery tasks the result holds neither list and the
		// summary is worded as in v0.14.5.
		setupCompletedMarkers(t)
		result, text := verifyBoth(t, 0)
		for _, key := range []string{"kept", "left_for_adoption"} {
			if _, present := result[key]; present {
				t.Errorf("a feature without delivery tasks carries %q: %v", key, result[key])
			}
		}
		summary := "nothing to verify for " + verifyTaskFeature + ": 2 completed task(s) already verified (check mode: nothing persisted)"
		if result["summary"] != summary {
			t.Errorf("summary %q, want %q", result["summary"], summary)
		}
		if strings.Contains(text, "Kept as delivered") || strings.Contains(text, "Left for adoption") {
			t.Errorf("the text holds a delivery line:\n%s", text)
		}
	})
}
