package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

// The recording under testdata/pre_lifecycle holds what the release before
// proof kinds wrote and printed for a fixture portfolio that declares none.
// It is the reference for "a plan without the declaration derives what it
// derived before": the values come from that binary, not from this tree.
const preLifecycleVersion = "v0.14.5"

var preLifecycleFeatures = []string{"states", "steady"}

// preLifecycleCommands are the read commands whose JSON results are recorded
// and compared, by the name of the file that holds each result.
var preLifecycleCommands = []struct {
	name string
	args []string
}{
	{"evidence-status-states", []string{"evidence", "status", "states"}},
	{"evidence-status-steady", []string{"evidence", "status", "steady"}},
	{"release-check", []string{"release", "check"}},
	{"release-check-steady", []string{"release", "check", "steady"}},
	{"release-check-strict", []string{"release", "check", "--strict"}},
	{"adopt", []string{"adopt"}},
	{"task-status-states", []string{"task", "status", "states"}},
	{"task-status-steady", []string{"task", "status", "steady"}},
}

// preLifecycleRoot is resolved when the package loads, while the working
// directory is still the package: the tests leave it before they read.
var preLifecycleRoot = func() string {
	absolute, err := filepath.Abs(filepath.Join("testdata", "pre_lifecycle"))
	if err != nil {
		panic(err)
	}
	return absolute
}()

func preLifecyclePath(parts ...string) string {
	return filepath.Join(append([]string{preLifecycleRoot}, parts...)...)
}

func preLifecycleJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v — regenerate the recording with testdata/pre_lifecycle/record.sh", path, err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return decoded
}

// preLifecycleRecorded returns the result a recorded command printed.
func preLifecycleRecorded(t *testing.T, name string) map[string]any {
	t.Helper()
	envelope := preLifecycleJSON(t, preLifecyclePath("recorded", "results", name+".json"))
	result, ok := envelope["result"].(map[string]any)
	if !ok {
		t.Fatalf("recorded %s holds no result", name)
	}
	return result
}

// preLifecycleLedger returns the records of one recorded ledger by task id.
func preLifecycleLedger(t *testing.T, phase, feature string) map[string]map[string]any {
	t.Helper()
	ledger := preLifecycleJSON(t, preLifecyclePath("recorded", phase, "walden", "evidence", feature+".json"))
	records := map[string]map[string]any{}
	tasks, _ := ledger["tasks"].(map[string]any)
	for id, record := range tasks {
		records[id], _ = record.(map[string]any)
	}
	return records
}

// rebuildPreLifecycleScenario copies the recorded scenario into an isolated
// temporary repository, commits it and enters it. The tree is the one the
// recorded read commands ran on, so its code identity is the recorded one.
func rebuildPreLifecycleScenario(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	root := chdirContract(t)
	scenario := preLifecyclePath("recorded", "scenario")
	for source, target := range map[string]string{"code": "code", "walden": ".walden"} {
		if err := os.CopyFS(filepath.Join(root, target), os.DirFS(filepath.Join(scenario, source))); err != nil {
			t.Fatalf("copy the recorded scenario: %v — regenerate the recording with testdata/pre_lifecycle/record.sh", err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "fixture@walden.test"},
		{"config", "user.name", "Fixture"},
		{"add", "-A"},
		{"commit", "-qm", "scenario"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	return root
}

// preLifecycleCurrent runs a read command in process and returns its result.
func preLifecycleCurrent(t *testing.T, args ...string) map[string]any {
	t.Helper()
	var stdout, stderr bytes.Buffer
	Run(append(append([]string{}, args...), "--json"), &stdout, &stderr)
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("walden %v printed no JSON envelope: %v (%s; %s)", args, err, stdout.String(), stderr.String())
	}
	result, ok := envelope["result"].(map[string]any)
	if !ok {
		t.Fatalf("walden %v printed no result: %s", args, stdout.String())
	}
	return result
}

// preLifecycleLeftOut names the fields that belong to one run or one machine:
// commit ids, durations of the run, the environment profile and the code
// explanation, which is built on a recorded commit this repository lacks.
func preLifecycleLeftOut(path string) bool {
	plain := regexp.MustCompile(`\[\d+\]`).ReplaceAllString(path, "")
	switch plain[strings.LastIndex(plain, ".")+1:] {
	case "certified_commit", "profile", "profile_drift", "elapsed_ms":
		return true
	}
	return strings.HasSuffix(plain, "current_tree.commit") || strings.HasSuffix(plain, "explanation.code")
}

var preLifecycleCommitSuffix = regexp.MustCompile(`, commit [0-9a-f]{12}\b`)

func preLifecycleNormalize(value any) any {
	if text, ok := value.(string); ok {
		return preLifecycleCommitSuffix.ReplaceAllString(text, ", commit <id>")
	}
	return value
}

// preLifecycleDiff lists every recorded field the current result lacks or
// holds with another value. Fields the current result adds are not reported:
// additions are allowed, changes are not.
func preLifecycleDiff(path string, recorded, current any, problems *[]string) {
	switch want := recorded.(type) {
	case map[string]any:
		got, ok := current.(map[string]any)
		if !ok {
			*problems = append(*problems, fmt.Sprintf("%s: recorded an object, now %T", path, current))
			return
		}
		keys := make([]string, 0, len(want))
		for key := range want {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			field := path + "." + key
			if preLifecycleLeftOut(field) {
				continue
			}
			value, present := got[key]
			if !present {
				*problems = append(*problems, field+": recorded, now absent")
				continue
			}
			preLifecycleDiff(field, want[key], value, problems)
		}
	case []any:
		got, ok := current.([]any)
		if !ok || len(got) != len(want) {
			*problems = append(*problems, fmt.Sprintf("%s: recorded %d item(s), now %v", path, len(want), current))
			return
		}
		for index := range want {
			preLifecycleDiff(fmt.Sprintf("%s[%d]", path, index), want[index], got[index], problems)
		}
	default:
		if !reflect.DeepEqual(preLifecycleNormalize(recorded), preLifecycleNormalize(current)) {
			*problems = append(*problems, fmt.Sprintf("%s: recorded %v, now %v", path, recorded, current))
		}
	}
}

// preLifecycleStates maps each task of an evidence report to its state.
func preLifecycleStates(t *testing.T, result map[string]any) map[string]string {
	t.Helper()
	states := map[string]string{}
	entries, _ := result["evidence"].([]any)
	for _, entry := range entries {
		fields, _ := entry.(map[string]any)
		id, _ := fields["task_id"].(string)
		state, _ := fields["state"].(string)
		states[id] = state
	}
	if len(states) == 0 {
		t.Fatalf("the report holds no evidence entry: %v", result)
	}
	return states
}

// preLifecycleVerdict keeps what a verdict says: whether it is releasable,
// each criterion of each feature with its blockers, the pending tasks, the
// blockers, the completion class, the exit status and the summary.
func preLifecycleVerdict(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	release, ok := result["release"].(map[string]any)
	if !ok {
		t.Fatalf("the result holds no verdict: %v", result)
	}
	features := []any{}
	list, _ := release["features"].([]any)
	for _, item := range list {
		feature, _ := item.(map[string]any)
		features = append(features, map[string]any{
			"feature": feature["feature"], "criteria": feature["criteria"], "pending": feature["pending"],
		})
	}
	return map[string]any{
		"releasable": release["releasable"], "features": features, "worktree": release["worktree"],
		"blockers": result["blockers"], "completion": result["completion"],
		"exit_code": result["exit_code"], "summary": preLifecycleNormalize(result["summary"]),
	}
}

// TestPreLifecycleRecording checks that the recording is what it claims to
// be: written by the named release, complete, and holding every state.
func TestPreLifecycleRecording(t *testing.T) {
	version, err := os.ReadFile(preLifecyclePath("recorded", "version.txt"))
	if err != nil {
		t.Fatalf("read the recorded version: %v — regenerate the recording with testdata/pre_lifecycle/record.sh", err)
	}
	if !strings.Contains(string(version), "walden "+preLifecycleVersion+" ") {
		t.Fatalf("the recording names %q, want walden %s", strings.TrimSpace(string(version)), preLifecycleVersion)
	}

	// Every record names the release in its profile, so the ledgers show
	// where they came from without trusting the file beside them.
	for _, phase := range []string{"complete", "scenario"} {
		for _, feature := range preLifecycleFeatures {
			records := preLifecycleLedger(t, phase, feature)
			if len(records) == 0 {
				t.Errorf("%s/%s: the recorded ledger holds no record", phase, feature)
			}
			for id, record := range records {
				profile, _ := record["profile"].(map[string]any)
				if profile["walden"] != preLifecycleVersion {
					t.Errorf("%s/%s task %s: recorded by %v, want %s", phase, feature, id, profile["walden"], preLifecycleVersion)
				}
			}
		}
	}

	// The complete run holds a record for every leaf of every plan.
	for _, feature := range preLifecycleFeatures {
		records := preLifecycleLedger(t, "complete", feature)
		for _, task := range preLifecycleLeaves(t, feature) {
			if _, recorded := records[task.ID]; !recorded {
				t.Errorf("complete/%s: leaf %s has no record", feature, task.ID)
			}
		}
	}

	// The scenario reaches every state the release derives.
	seen := map[string]bool{}
	for _, feature := range preLifecycleFeatures {
		for _, state := range preLifecycleStates(t, preLifecycleRecorded(t, "evidence-status-"+feature)) {
			seen[state] = true
		}
	}
	for _, state := range []string{"verified", "failed", "stale-spec", "stale-code", "unattested", "unrecorded", "pending"} {
		if !seen[state] {
			t.Errorf("the recorded scenario never reaches %s", state)
		}
	}

	for _, command := range preLifecycleCommands {
		preLifecycleRecorded(t, command.name)
	}
}

// preLifecycleLeaves parses the plan the complete run executed.
func preLifecycleLeaves(t *testing.T, feature string) []*spec.Task {
	t.Helper()
	path := preLifecyclePath("recorded", "complete", "walden", "specs", feature, "tasks.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v — regenerate the recording with testdata/pre_lifecycle/record.sh", path, err)
	}
	document, err := spec.ParseDocument(path, data)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	tree, err := spec.ParseTaskTree(document)
	if err != nil {
		t.Fatalf("parse the plan of %s: %v", feature, err)
	}
	return tree.LeafTasks()
}

// TestPreLifecycleCompatibility requires from the current code what the
// release before proof kinds derived for a portfolio without declarations.
func TestPreLifecycleCompatibility(t *testing.T) {
	t.Run("fingerprints", func(t *testing.T) {
		for _, feature := range preLifecycleFeatures {
			records := preLifecycleLedger(t, "complete", feature)
			leaves := preLifecycleLeaves(t, feature)
			if len(leaves) == 0 {
				t.Fatalf("%s: the plan holds no leaf", feature)
			}
			for _, task := range leaves {
				record := records[task.ID]
				if record["task_fingerprint_scheme"] != spec.TaskFingerprintScheme {
					t.Errorf("%s task %s: recorded scheme %v, now %s", feature, task.ID, record["task_fingerprint_scheme"], spec.TaskFingerprintScheme)
				}
				if got := spec.TaskDefinitionFingerprint(task); got != record["task_fingerprint"] {
					t.Errorf("%s task %s: recorded fingerprint %v, now %s", feature, task.ID, record["task_fingerprint"], got)
				}
			}
		}
	})

	t.Run("states", func(t *testing.T) {
		rebuildPreLifecycleScenario(t)
		for _, feature := range preLifecycleFeatures {
			recorded := preLifecycleStates(t, preLifecycleRecorded(t, "evidence-status-"+feature))
			current := preLifecycleStates(t, preLifecycleCurrent(t, "evidence", "status", feature))
			if !reflect.DeepEqual(recorded, current) {
				t.Errorf("%s: recorded states %v, now %v", feature, recorded, current)
			}
		}
	})

	t.Run("verdict", func(t *testing.T) {
		rebuildPreLifecycleScenario(t)
		for name, args := range map[string][]string{
			"release-check":        {"release", "check"},
			"release-check-steady": {"release", "check", "steady"},
		} {
			recorded := preLifecycleVerdict(t, preLifecycleRecorded(t, name))
			current := preLifecycleVerdict(t, preLifecycleCurrent(t, args...))
			if !reflect.DeepEqual(recorded, current) {
				want, _ := json.MarshalIndent(recorded, "", "  ")
				got, _ := json.MarshalIndent(current, "", "  ")
				t.Errorf("%s: recorded verdict\n%s\nnow\n%s", name, want, got)
			}
		}
		if releasable := preLifecycleRecorded(t, "release-check-steady")["release"].(map[string]any)["releasable"]; releasable != true {
			t.Errorf("the recording holds no releasable verdict: %v", releasable)
		}
		if releasable := preLifecycleRecorded(t, "release-check")["release"].(map[string]any)["releasable"]; releasable != false {
			t.Errorf("the recording holds no blocked verdict: %v", releasable)
		}
	})

	t.Run("json", func(t *testing.T) {
		rebuildPreLifecycleScenario(t)
		for _, command := range preLifecycleCommands {
			var problems []string
			preLifecycleDiff("result", preLifecycleRecorded(t, command.name), preLifecycleCurrent(t, command.args...), &problems)
			for _, problem := range problems {
				t.Errorf("%s: %s", command.name, problem)
			}
		}
	})
}
