package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/testutil"
)

// taskProofKind runs a task command in JSON and in text mode and returns the
// proof kind each shows for its task: the value of task.proof, and the value
// of the "Task proof:" line. Absence is the empty string in both.
func taskProofKind(t *testing.T, args ...string) (inJSON, inText string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := Run(append(append([]string{}, args...), "--json"), &stdout, &stderr); code != 0 {
		t.Fatalf("walden %v --json exited %d: %s%s", args, code, stdout.String(), stderr.String())
	}
	var envelope struct {
		Result struct {
			Task map[string]any `json:"task"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("walden %v printed no JSON: %v", args, err)
	}
	if envelope.Result.Task == nil {
		t.Fatalf("walden %v reports no task: %s", args, stdout.String())
	}
	if value, present := envelope.Result.Task["proof"]; present {
		inJSON, _ = value.(string)
		if inJSON == "" {
			t.Errorf("walden %v: task.proof is present and empty", args)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("walden %v exited %d: %s%s", args, code, stdout.String(), stderr.String())
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if value, found := strings.CutPrefix(line, "Task proof: "); found {
			inText = value
		}
	}
	return inJSON, inText
}

// TestTaskOutputShowsProofKind: the task commands say which task holds a
// delivery proof, in JSON and in text, and say nothing for any other task.
func TestTaskOutputShowsProofKind(t *testing.T) {
	root := chdirContract(t)
	const feature = "proof-kind-output"
	// Leaf 1 declares a delivery proof; leaf 2.1 does not.
	tasks := strings.Replace(deliveryDeclarationTasks, "    - Proof: delivery\n", "", 1)
	if strings.Count(tasks, "- Proof: delivery") != 1 {
		t.Fatalf("the fixture must declare one delivery proof:\n%s", tasks)
	}
	addDeliveryFeature(t, root, feature, tasks)

	// task status and task start run no proof. task complete runs one in
	// each mode, on a fresh copy of the feature.
	for _, step := range []struct {
		task string
		want string
	}{{"1", "delivery"}, {"2.1", ""}} {
		for _, args := range [][]string{
			{"task", "status", feature},
			{"task", "start", feature, step.task},
		} {
			inJSON, inText := taskProofKind(t, args...)
			if inJSON != step.want || inText != step.want {
				t.Errorf("walden %v for task %s: proof %q in JSON, %q in text, want %q", args, step.task, inJSON, inText, step.want)
			}
		}
		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}))
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"task", "complete", feature, step.task, "--json"}, &stdout, &stderr); code != 0 {
			t.Fatalf("task complete %s exited %d: %s%s", step.task, code, stdout.String(), stderr.String())
		}
		var envelope struct {
			Result struct {
				Task map[string]any `json:"task"`
			} `json:"result"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatalf("task complete printed no JSON: %v", err)
		}
		got, _ := envelope.Result.Task["proof"].(string)
		if got != step.want {
			t.Errorf("task complete %s: task.proof = %q, want %q", step.task, got, step.want)
		}
	}

	// The text of task complete, on a second copy of the same feature.
	const again = "proof-kind-output-text"
	addDeliveryFeature(t, root, again, tasks)
	for _, step := range []struct {
		task string
		want bool
	}{{"1", true}, {"2.1", false}} {
		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}))
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"task", "complete", again, step.task}, &stdout, &stderr); code != 0 {
			t.Fatalf("task complete %s exited %d: %s%s", step.task, code, stdout.String(), stderr.String())
		}
		if got := strings.Contains(stdout.String(), "Task proof: delivery\n"); got != step.want {
			t.Errorf("task complete %s: the text holds the proof line = %t, want %t:\n%s", step.task, got, step.want, stdout.String())
		}
	}
}
