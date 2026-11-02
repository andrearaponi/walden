package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/testutil"
)

const verifyTaskFeature = "markers"

// setupSelectionRepo returns a committed repository holding group 1 with the
// completed leaf 1.1 and the pending leaf 1.2, and the path of a file that only
// a declared environment probe creates.
func setupSelectionRepo(t *testing.T) (root, sentinel string) {
	t.Helper()
	first := setupReleasableFeature(t, verifyTaskFeature)
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}))
	integrityMustCLI(t, "task", "complete", verifyTaskFeature, "1.1")

	// A process captures the profile once per root, so the completion above
	// would keep the probe from ever running here. Moving the repository hands
	// the command under test a root this process has not profiled.
	root = filepath.Join(t.TempDir(), "repo")
	if err := os.Rename(first, root); err != nil {
		t.Fatalf("move repository: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("enter moved repository: %v", err)
	}

	sentinel = filepath.Join(t.TempDir(), "probe-ran")
	probes := fmt.Sprintf("- sentinel: [\"touch\", %q]\n", sentinel)
	if err := os.WriteFile(filepath.Join(root, ".walden", "environment.md"), []byte(probes), 0o644); err != nil {
		t.Fatalf("declare probe: %v", err)
	}
	return root, sentinel
}

// setupCompletedMarkers returns a committed repository whose leaves 1.1 and 1.2
// are completed and verified.
func setupCompletedMarkers(t *testing.T) string {
	t.Helper()
	root := setupReleasableFeature(t, verifyTaskFeature)
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"}))
	integrityMustCLI(t, "task", "complete-all", verifyTaskFeature)
	return root
}

// staleTree changes the code identity under every completed task.
func staleTree(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "new-code"), []byte("changed"), 0o644); err != nil {
		t.Fatalf("change the tree: %v", err)
	}
}

// wireScope runs a passing command and returns result.scope as the wire holds it.
func wireScope(t *testing.T, args ...string) map[string]any {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := Run(append(args, "--json"), &stdout, &stderr); code != 0 {
		t.Fatalf("walden %v exited %d: %s%s", args, code, stdout.String(), stderr.String())
	}
	var envelope struct {
		Result struct {
			Scope map[string]any `json:"scope"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	return envelope.Result.Scope
}

func readLedger(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(evidence.DocumentPath(root, verifyTaskFeature))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	return data
}

func TestVerifyTaskRejections(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		wants []string
	}{
		{name: "unknown id", args: []string{"--task", "9.9"}, wants: []string{`task "9.9" is not a leaf task of markers`}},
		{name: "group id", args: []string{"--task", "1"}, wants: []string{`task "1" is not a leaf task of markers`}},
		{name: "pending leaf", args: []string{"--task", "1.2"}, wants: []string{`task "1.2" is not completed`, "walden task complete markers 1.2"}},
		{name: "task before all", args: []string{"--task", "1.1", "--all"}, wants: []string{"--task", "--all", "walden verify --help"}},
		{name: "all before task", args: []string{"--all", "--task", "1.1"}, wants: []string{"--task", "--all", "walden verify --help"}},
		{name: "repeated", args: []string{"--task", "1.1", "--task", "1.1"}, wants: []string{"--task", "walden verify --help"}},
		{name: "empty", args: []string{"--task", ""}, wants: []string{"--task", "walden verify --help"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, sentinel := setupSelectionRepo(t)
			runner := overrideCommandRunner(t, testutil.NewFakeRunner(
				testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"},
			))
			before := readLedger(t, root)

			var stdout, stderr bytes.Buffer
			code := Run(append([]string{"verify", verifyTaskFeature}, tc.args...), &stdout, &stderr)

			if code == 0 {
				t.Errorf("exit 0, want a rejection: %s", stdout.String())
			}
			for _, want := range tc.wants {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("error %q does not hold %q", stderr.String(), want)
				}
			}
			if calls := runner.Calls(); len(calls) != 0 {
				t.Errorf("proofs ran: %v", calls)
			}
			if !bytes.Equal(before, readLedger(t, root)) {
				t.Error("the ledger changed")
			}
			if _, err := os.Stat(sentinel); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the environment probe ran (stat: %v)", err)
			}
		})
	}

	// Positive control: the same repository, with a valid selection, reaches
	// the probe, the proof and the ledger, so the checks above can fail.
	t.Run("valid selection", func(t *testing.T) {
		root, sentinel := setupSelectionRepo(t)
		runner := overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}))
		before := readLedger(t, root)

		integrityMustCLI(t, "verify", verifyTaskFeature, "--task", "1.1")

		if calls := runner.Calls(); len(calls) != 1 {
			t.Errorf("proofs run = %v, want one", calls)
		}
		if bytes.Equal(before, readLedger(t, root)) {
			t.Error("the ledger did not change")
		}
		if _, err := os.Stat(sentinel); err != nil {
			t.Errorf("the environment probe did not run: %v", err)
		}
	})
}

func TestVerifyHelpListsTask(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"verify", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("verify --help exited %d: %s", code, stderr.String())
	}
	help := stdout.String()
	for _, want := range []string{
		"verify <feature> [--all] [--check] [--task <id>] [--json]",
		"--task <id>",
		"Re-prove one completed task",
		"--task re-proves one completed leaf task for a repair and leaves every other record as it is.",
		"Refused, running nothing, with --all, when repeated or empty, or for an id that is not a completed leaf task.",
		"It proves nothing about the feature: release check still judges every task, so finish with walden verify <feature>.",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("help does not hold %q:\n%s", want, help)
		}
	}
}

func TestVerifyTaskScope(t *testing.T) {
	setupCompletedMarkers(t)
	overrideCommandRunner(t, testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"},
	))

	selected := wireScope(t, "verify", verifyTaskFeature, "--task", "1.1")
	plain := wireScope(t, "verify", verifyTaskFeature, "--all")

	if selected["kind"] != "task" {
		t.Errorf("selected kind = %v, want task", selected["kind"])
	}
	if !reflect.DeepEqual(selected["tasks"], []any{"1.1"}) {
		t.Errorf("selected tasks = %v, want [1.1]", selected["tasks"])
	}
	if plain["kind"] != "feature" {
		t.Errorf("plain kind = %v, want feature", plain["kind"])
	}
	if _, exists := plain["tasks"]; exists {
		t.Errorf("plain scope holds tasks: %v", plain)
	}
	if !reflect.DeepEqual(plain["features"], []any{verifyTaskFeature}) {
		t.Errorf("plain features = %v, want the feature", plain["features"])
	}
	if plain["guarantee"] != evidence.Guarantee {
		t.Errorf("plain guarantee = %v, want %s", plain["guarantee"], evidence.Guarantee)
	}
	for _, key := range []string{"features", "guarantee"} {
		if !reflect.DeepEqual(selected[key], plain[key]) {
			t.Errorf("%s differs: selected %v, plain %v", key, selected[key], plain[key])
		}
	}
}

func TestVerifyTaskSummaryAndNextAction(t *testing.T) {
	passing := testutil.Response{Stdout: "ok"}
	failing := testutil.Response{Stderr: "boom", ExitCode: 1}
	cases := []struct {
		name     string
		stale    bool
		response testutil.Response
		flags    []string
		exit     int
		summary  string
		next     string
		warning  string
	}{
		{
			name: "failing proof", response: failing, exit: 1,
			summary: "verification failed for task 1.1 of markers",
			next:    "Fix the failing proof and rerun walden verify markers --task 1.1",
		},
		{
			name: "passing with an open task", stale: true, response: passing,
			summary: "verification passed for task 1.1 of markers",
			next:    "Run walden verify markers to re-prove the other tasks",
			warning: "1 other completed task(s) not verified: 1.2",
		},
		{
			name: "passing with nothing open", response: passing,
			summary: "verification passed for task 1.1 of markers",
		},
		{
			name: "check mode", stale: true, response: passing, flags: []string{"--check"},
			summary: "verification passed for task 1.1 of markers (check mode: nothing persisted)",
			next:    "Run walden verify markers to re-prove the other tasks",
			warning: "1 other completed task(s) not verified: 1.2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := setupCompletedMarkers(t)
			if tc.stale {
				staleTree(t, root)
			}
			overrideCommandRunner(t, testutil.NewFakeRunner(tc.response, tc.response))
			args := append([]string{"verify", verifyTaskFeature, "--task", "1.1"}, tc.flags...)

			result, code := integrityCLI(t, args...)
			if code != tc.exit {
				t.Errorf("JSON exit = %d, want %d", code, tc.exit)
			}
			if result.Summary != tc.summary {
				t.Errorf("JSON summary = %q, want %q", result.Summary, tc.summary)
			}
			if result.NextAction != tc.next {
				t.Errorf("JSON next action = %q, want %q", result.NextAction, tc.next)
			}
			if tc.warning != "" && !slices.Contains(result.Warnings, tc.warning) {
				t.Errorf("JSON warnings = %q, want %q", result.Warnings, tc.warning)
			}

			var stdout, stderr bytes.Buffer
			if code := Run(args, &stdout, &stderr); code != tc.exit {
				t.Errorf("text exit = %d, want %d", code, tc.exit)
			}
			text := stdout.String() + stderr.String()
			wants := []string{tc.summary}
			if tc.next != "" {
				wants = append(wants, "Next action: "+tc.next)
			} else if strings.Contains(text, "Next action:") {
				t.Errorf("text proposes a next action:\n%s", text)
			}
			if tc.exit == 0 {
				wants = append(wants, "Scope: task: 1.1 of markers; guarantee: "+evidence.Guarantee, tc.warning)
			}
			for _, want := range wants {
				if !strings.Contains(text, want) {
					t.Errorf("text does not hold %q:\n%s", want, text)
				}
			}
		})
	}

	// Control: without a selection the next action names the feature alone.
	t.Run("plain verify", func(t *testing.T) {
		setupCompletedMarkers(t)
		overrideCommandRunner(t, testutil.NewFakeRunner(failing))

		result, code := integrityCLI(t, "verify", verifyTaskFeature, "--all")
		if code != 1 {
			t.Fatalf("exit = %d, want 1", code)
		}
		if want := "Fix the failing proofs and rerun walden verify markers"; result.NextAction != want {
			t.Fatalf("next action = %q, want %q", result.NextAction, want)
		}
	})
}

func TestVerifyTaskExitStatus(t *testing.T) {
	setupCompletedMarkers(t)
	failing := testutil.Response{Stderr: "boom", ExitCode: 1}
	overrideCommandRunner(t, testutil.NewFakeRunner(failing, testutil.Response{Stdout: "ok"}, failing))

	if _, code := integrityCLI(t, "verify", verifyTaskFeature, "--task", "1.2"); code != 1 {
		t.Fatalf("exit = %d with the selected proof failing, want 1", code)
	}

	result, code := integrityCLI(t, "verify", verifyTaskFeature, "--task", "1.1")
	if code != 0 {
		t.Fatalf("exit = %d with only another task failed, want 0: %+v", code, result)
	}
	if want := "1 other completed task(s) not verified: 1.2"; !slices.Contains(result.Warnings, want) {
		t.Fatalf("warnings = %q, want %q", result.Warnings, want)
	}

	// Control: the same repository fails a plain run, whose status covers 1.2.
	if _, code := integrityCLI(t, "verify", verifyTaskFeature); code != 1 {
		t.Fatalf("plain exit = %d with 1.2 failing, want 1", code)
	}
}
