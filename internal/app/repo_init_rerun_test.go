package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const rerunWorkflowTarget = ".github/workflows/validate-walden.yml"

// rerunKeptTargets are the bootstrap files a run never writes once they exist.
var rerunKeptTargets = []string{
	".walden/constitution.md",
	".walden/lessons.md",
	".walden/.gitignore",
	".github/pull_request_template.md",
}

// rerunRoot returns a temporary repository that already holds git metadata
// and makes it the working directory, so no test of this file runs git init.
func rerunRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("create git metadata: %v", err)
	}
	t.Chdir(root)
	return root
}

// rerunSetVersion makes the binary under test run as the given version.
func rerunSetVersion(t *testing.T, version string) {
	t.Helper()
	previous := Version
	Version = version
	t.Cleanup(func() { Version = previous })
}

// rerunCommand runs one walden command, fails the test unless it exits with
// the wanted code, and returns what it wrote to stdout and to stderr.
func rerunCommand(t *testing.T, wantExit int, args ...string) (string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if exit := Run(args, &stdout, &stderr); exit != wantExit {
		t.Fatalf("walden %s exited %d, want %d\nstdout: %s\nstderr: %s",
			strings.Join(args, " "), exit, wantExit, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

func rerunWrite(t *testing.T, root, target, content string) {
	t.Helper()
	path := filepath.Join(root, target)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create the directory of %s: %v", target, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", target, err)
	}
}

func rerunRead(t *testing.T, root, target string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, target))
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	return string(content)
}

// rerunResult decodes the result object of a JSON envelope, key by key, and
// reports whether the command succeeded.
func rerunResult(t *testing.T, stdout string) (ok bool, result map[string]json.RawMessage) {
	t.Helper()
	var envelope struct {
		OK     bool                       `json:"ok"`
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode the JSON result: %v\n%s", err, stdout)
	}
	return envelope.OK, envelope.Result
}

// rerunList decodes one list of the result; an absent or null key is empty.
func rerunList(t *testing.T, result map[string]json.RawMessage, key string) []string {
	t.Helper()
	raw, present := result[key]
	if !present {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("decode %s: %v", key, err)
	}
	return values
}

func rerunContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestRunRepoInitKeepsRecordedLesson(t *testing.T) {
	const lessons = ".walden/lessons.md"
	root := rerunRoot(t)
	rerunCommand(t, 0, "repo", "init")
	rerunCommand(t, 0, "lesson", "log",
		"--feature", "demo",
		"--phase", "requirements",
		"--trigger", "a criterion had no oracle",
		"--lesson", "ask for the oracle first",
		"--guardrail", "name the oracle in the criterion")
	recorded := rerunRead(t, root, lessons)
	if !strings.Contains(recorded, "ask for the oracle first") {
		t.Fatalf("lesson log did not record the lesson:\n%s", recorded)
	}

	stdout, _ := rerunCommand(t, 0, "repo", "init", "--json")

	if got := rerunRead(t, root, lessons); got != recorded {
		t.Errorf("a second repo init rewrote the lessons file:\n got %q\nwant %q", got, recorded)
	}
	_, result := rerunResult(t, stdout)
	if !rerunContains(rerunList(t, result, "skipped_files"), lessons) {
		t.Errorf("the lessons file is not reported as skipped: %s", stdout)
	}
	if rerunContains(rerunList(t, result, "updated_files"), lessons) {
		t.Errorf("the lessons file is reported as updated: %s", stdout)
	}
}

func TestRunRepoInitJSONReportsKeptAndRefreshed(t *testing.T) {
	root := rerunRoot(t)
	rerunSetVersion(t, "v0.7.0")
	rerunCommand(t, 0, "repo", "init")
	for _, target := range rerunKeptTargets {
		rerunWrite(t, root, target, "content the user wrote in "+target+"\n")
	}

	rerunSetVersion(t, "v0.7.1")
	stdout, _ := rerunCommand(t, 0, "repo", "init", "--json")

	ok, result := rerunResult(t, stdout)
	if !ok {
		t.Fatalf("the run did not succeed: %s", stdout)
	}

	skipped := rerunList(t, result, "skipped_files")
	sort.Strings(skipped)
	wantSkipped := append([]string(nil), rerunKeptTargets...)
	sort.Strings(wantSkipped)
	if strings.Join(skipped, "\n") != strings.Join(wantSkipped, "\n") {
		t.Errorf("skipped_files = %v, want the four kept files %v", skipped, wantSkipped)
	}

	updated := rerunList(t, result, "updated_files")
	if len(updated) != 1 || updated[0] != rerunWorkflowTarget {
		t.Errorf("updated_files = %v, want only %s", updated, rerunWorkflowTarget)
	}
	if created := rerunList(t, result, "created_files"); len(created) != 0 {
		t.Errorf("created_files = %v, want none", created)
	}

	// The keys the released result of repo init holds, on a first and on a
	// second run. A run of this feature may hold fewer, never another.
	released := map[string]bool{
		"summary":                 true,
		"created_files":           true,
		"updated_files":           true,
		"changed_files":           true,
		"skipped_files":           true,
		"git_initialized":         true,
		"git_already_initialized": true,
		"next_action":             true,
		"warnings":                true,
		"exit_code":               true,
	}
	for key := range result {
		if !released[key] {
			t.Errorf("the result holds the key %q, which the released result does not", key)
		}
	}
}

// rerunPinlessWorkflow runs a walden binary the repository already holds, so
// it names no install target.
const rerunPinlessWorkflow = `name: Validate Walden

on:
  pull_request:

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Validate Walden specs
        run: ./bin/walden validate --all --json
`

func TestRunRepoInitWarnsWithoutInstallTarget(t *testing.T) {
	warningFragments := []string{rerunWorkflowTarget, "pin was not refreshed", "delete the file", "walden repo init", "regenerate"}

	t.Run("the JSON result carries one warning", func(t *testing.T) {
		root := rerunRoot(t)
		rerunCommand(t, 0, "repo", "init")
		rerunWrite(t, root, rerunWorkflowTarget, rerunPinlessWorkflow)

		stdout, _ := rerunCommand(t, 0, "repo", "init", "--json")

		ok, result := rerunResult(t, stdout)
		if !ok {
			t.Fatalf("the run did not succeed: %s", stdout)
		}
		warnings := rerunList(t, result, "warnings")
		if len(warnings) != 1 {
			t.Fatalf("warnings = %v, want exactly one", warnings)
		}
		for _, fragment := range warningFragments {
			if !strings.Contains(warnings[0], fragment) {
				t.Errorf("the warning does not mention %q: %s", fragment, warnings[0])
			}
		}
		if !rerunContains(rerunList(t, result, "skipped_files"), rerunWorkflowTarget) {
			t.Errorf("the workflow is not reported as skipped: %s", stdout)
		}
		if got := rerunRead(t, root, rerunWorkflowTarget); got != rerunPinlessWorkflow {
			t.Errorf("the workflow was rewritten:\n got %q\nwant %q", got, rerunPinlessWorkflow)
		}
	})

	t.Run("the text output prints the warning", func(t *testing.T) {
		root := rerunRoot(t)
		rerunCommand(t, 0, "repo", "init")
		rerunWrite(t, root, rerunWorkflowTarget, rerunPinlessWorkflow)

		stdout, _ := rerunCommand(t, 0, "repo", "init")

		if !strings.Contains(stdout, "Warnings:") {
			t.Errorf("the text output has no warnings section:\n%s", stdout)
		}
		for _, fragment := range warningFragments {
			if !strings.Contains(stdout, fragment) {
				t.Errorf("the text output does not mention %q:\n%s", fragment, stdout)
			}
		}
	})

	t.Run("a refreshing run carries no warning", func(t *testing.T) {
		rerunRoot(t)
		rerunSetVersion(t, "v0.7.0")
		rerunCommand(t, 0, "repo", "init")

		rerunSetVersion(t, "v0.7.1")
		stdout, _ := rerunCommand(t, 0, "repo", "init", "--json")

		_, result := rerunResult(t, stdout)
		if updated := rerunList(t, result, "updated_files"); len(updated) != 1 || updated[0] != rerunWorkflowTarget {
			t.Fatalf("the run did not refresh the workflow: %s", stdout)
		}
		if warnings := rerunList(t, result, "warnings"); len(warnings) != 0 {
			t.Errorf("a refreshing run carries warnings: %v", warnings)
		}
	})

	t.Run("a run that finds the pin current carries no warning", func(t *testing.T) {
		rerunRoot(t)
		rerunSetVersion(t, "v0.7.1")
		rerunCommand(t, 0, "repo", "init")

		stdout, _ := rerunCommand(t, 0, "repo", "init", "--json")

		_, result := rerunResult(t, stdout)
		if warnings := rerunList(t, result, "warnings"); len(warnings) != 0 {
			t.Errorf("a run that finds the pin current carries warnings: %v", warnings)
		}
	})

	t.Run("an unreadable workflow fails the run and is named", func(t *testing.T) {
		root := rerunRoot(t)
		rerunCommand(t, 0, "repo", "init")
		// A directory at the path of the workflow makes the read fail on
		// every platform, with no permission involved.
		path := filepath.Join(root, rerunWorkflowTarget)
		if err := os.Remove(path); err != nil {
			t.Fatalf("delete the workflow: %v", err)
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatalf("put a directory at the workflow path: %v", err)
		}

		stdout, _ := rerunCommand(t, 1, "repo", "init", "--json")
		ok, result := rerunResult(t, stdout)
		if ok {
			t.Errorf("the failed run reports ok: %s", stdout)
		}
		var summary string
		if err := json.Unmarshal(result["summary"], &summary); err != nil {
			t.Fatalf("decode the summary: %v", err)
		}
		if !strings.Contains(summary, rerunWorkflowTarget) {
			t.Errorf("the summary does not name the workflow: %s", summary)
		}

		_, stderr := rerunCommand(t, 1, "repo", "init")
		if !strings.Contains(stderr, rerunWorkflowTarget) {
			t.Errorf("the error on stderr does not name the workflow: %s", stderr)
		}
	})
}

func TestRepoInitHelp(t *testing.T) {
	stdout, _ := rerunCommand(t, 0, "repo", "init", "--help")

	// What a run does to existing files: the three statements.
	for _, statement := range []string{
		"keeps every file that exists",
		"The only change to an existing file is the version of the workflow's walden install target",
		"To regenerate a file from its template, delete it and run the command again",
	} {
		if !strings.Contains(stdout, statement) {
			t.Errorf("the help does not state %q:\n%s", statement, stdout)
		}
	}
	// What the help already carried stays.
	for _, kept := range []string{"repo init [--json]", "Initialize Walden in the current repository", "--json"} {
		if !strings.Contains(stdout, kept) {
			t.Errorf("the help lost %q:\n%s", kept, stdout)
		}
	}
}
