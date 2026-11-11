package app

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/testutil"
)

// TestDeliveryTasksAgreeAcrossCommands: validation, task status, evidence
// status and release check read the plan through different entry points, and
// must agree on which leaves are delivery tasks. The plan holds one declared
// leaf and one undeclared leaf, each covering a statement of its own.
func TestDeliveryTasksAgreeAcrossCommands(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := chdirContract(t)
	const feature = "delivery-agreement"
	// Leaf 1 declares a delivery proof and covers R1.AC1. Leaf 2.1 declares
	// nothing and covers R1.AC2.
	tasks := strings.Replace(deliveryDeclarationTasks, "    - Proof: delivery\n", "", 1)
	addDeliveryFeature(t, root, feature, tasks)
	diagnosticsFile(t, root, "main.go", "package main\n")
	gitify(t, root)

	// Validation shows its reading through the statements only delivery
	// proofs cover: the statement of leaf 1, and not the one of leaf 2.1.
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"validate", feature, "--all", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("validate exited %d: %s%s", code, stdout.String(), stderr.String())
	}
	var validation struct {
		Result struct {
			Warnings []string `json:"warnings"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &validation); err != nil {
		t.Fatalf("decode: %v", err)
	}
	fromValidation := []string{}
	for _, warning := range validation.Result.Warnings {
		if list, found := strings.CutPrefix(warning, "statement(s) covered only by delivery proofs, which are not proven again once attested: "); found {
			fromValidation = strings.Split(list, ", ")
		}
	}
	if !reflect.DeepEqual(fromValidation, []string{"R1.AC1"}) {
		t.Errorf("validation reads %v as covered by delivery proofs alone, want R1.AC1 (warnings %v)", fromValidation, validation.Result.Warnings)
	}

	// task status names the proof kind of the next task: leaf 1, then 2.1.
	fromTaskStatus := map[string]string{}
	for _, id := range []string{"1", "2.1"} {
		inJSON, inText := taskProofKind(t, "task", "status", feature)
		if inJSON != inText {
			t.Errorf("task status for %s: %q in JSON, %q in text", id, inJSON, inText)
		}
		fromTaskStatus[id] = inJSON
		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}))
		integrityMustCLI(t, "task", "complete", feature, id)
	}

	// evidence status and release check mark each entry.
	deliveryOf := func(entries []any) map[string]string {
		kinds := map[string]string{}
		for _, item := range entries {
			entry := item.(map[string]any)
			kind := ""
			if entry["delivery"] == true {
				kind = "delivery"
			}
			kinds[entry["task_id"].(string)] = kind
		}
		return kinds
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"evidence", "status", feature, "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("evidence status exited %d: %s", code, stderr.String())
	}
	var status struct {
		Result struct {
			Evidence []any `json:"evidence"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	fromEvidenceStatus := deliveryOf(status.Result.Evidence)

	verdict, _ := releaseBoth(t, feature)
	fromReleaseCheck := deliveryOf(releaseFeature(t, verdict)["evidence"].([]any))

	want := map[string]string{"1": "delivery", "2.1": ""}
	for name, got := range map[string]map[string]string{
		"task status": fromTaskStatus, "evidence status": fromEvidenceStatus, "release check": fromReleaseCheck,
	} {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s reads %v, want leaf 1 as the delivery task and leaf 2.1 as a regression task", name, got)
		}
	}
}
