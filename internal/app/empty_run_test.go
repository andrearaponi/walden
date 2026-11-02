package app

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/testutil"
)

const (
	emptyPackageOutput  = "ok  \tlab/a\t0.243s [no tests to run]\n"
	testedPackageOutput = "ok  \tlab/b\t0.243s\n"

	// emptyRunPhrase is the part of the warning common to every proof.
	emptyRunPhrase = "passed without running any test"
)

// markerWarning spells out the warning of a leaf of the markers feature.
func markerWarning(id string) string {
	return "task " + id + `: command ["sh","-c","true"] passed without running any test: check its selector and assert the test by name with -v and expect_output`
}

// wireResult runs a command with --json and returns its exit code and result
// fields as the wire holds them.
func wireResult(t *testing.T, args ...string) (int, map[string]json.RawMessage) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append(args, "--json"), &stdout, &stderr)
	var envelope struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %v: %v\n%s%s", args, err, stdout.String(), stderr.String())
	}
	return code, envelope.Result
}

func wireWarnings(t *testing.T, result map[string]json.RawMessage) []string {
	t.Helper()
	var warnings []string
	if err := json.Unmarshal(result["warnings"], &warnings); err != nil {
		t.Fatalf("decode warnings %s: %v", result["warnings"], err)
	}
	return warnings
}

func TestEmptyRunTaskCompleteSurfaces(t *testing.T) {
	const want = `task 1: command ["go","test","./internal/spec"] passed without running any test: check its selector and assert the test by name with -v and expect_output`

	t.Run("json", func(t *testing.T) {
		setupPendingProof(t)
		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: emptyPackageOutput}))

		code, result := wireResult(t, "task", "complete", "todo-app-demo", "1")
		if code != 0 {
			t.Fatalf("exit = %d, want 0: %v", code, result)
		}
		if warnings := wireWarnings(t, result); !slices.Equal(warnings, []string{want}) {
			t.Fatalf("warnings = %q, want %q", warnings, want)
		}
	})

	t.Run("text", func(t *testing.T) {
		setupPendingProof(t)
		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: emptyPackageOutput}))

		var stdout, stderr bytes.Buffer
		if code := Run([]string{"task", "complete", "todo-app-demo", "1"}, &stdout, &stderr); code != 0 {
			t.Fatalf("exit = %d, want 0: %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Warnings:\n- "+want+"\n") {
			t.Fatalf("stdout does not hold the warning:\n%s", stdout.String())
		}
	})

	t.Run("a proof that ran tests keeps an empty array", func(t *testing.T) {
		setupPendingProof(t)
		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: testedPackageOutput}))

		code, result := wireResult(t, "task", "complete", "todo-app-demo", "1")
		if code != 0 {
			t.Fatalf("exit = %d, want 0: %v", code, result)
		}
		if got := string(result["warnings"]); got != "[]" {
			t.Fatalf("warnings = %s, want []", got)
		}
	})
}

func TestEmptyRunTaskCompleteAllSurfaces(t *testing.T) {
	empty := testutil.Response{Stdout: emptyPackageOutput}
	failing := testutil.Response{ExitCode: 1}
	tested := testutil.Response{Stdout: testedPackageOutput}

	setup := func(t *testing.T, responses ...testutil.Response) {
		t.Helper()
		setupReleasableFeature(t, verifyTaskFeature)
		overrideCommandRunner(t, testutil.NewFakeRunner(responses...))
	}

	t.Run("json", func(t *testing.T) {
		setup(t, empty, empty)

		code, result := wireResult(t, "task", "complete-all", verifyTaskFeature)
		if code != 0 {
			t.Fatalf("exit = %d, want 0: %v", code, result)
		}
		if want := []string{markerWarning("1.1"), markerWarning("1.2")}; !slices.Equal(wireWarnings(t, result), want) {
			t.Fatalf("warnings = %s, want %q", result["warnings"], want)
		}
	})

	t.Run("text", func(t *testing.T) {
		setup(t, empty, empty)

		var stdout, stderr bytes.Buffer
		if code := Run([]string{"task", "complete-all", verifyTaskFeature}, &stdout, &stderr); code != 0 {
			t.Fatalf("exit = %d, want 0: %s", code, stderr.String())
		}
		want := "Warnings:\n- " + markerWarning("1.1") + "\n- " + markerWarning("1.2") + "\n"
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout does not hold both warnings:\n%s", stdout.String())
		}
	})

	t.Run("a later failure keeps them in json", func(t *testing.T) {
		setup(t, empty, failing)

		code, result := wireResult(t, "task", "complete-all", verifyTaskFeature)
		if code == 0 {
			t.Fatalf("exit = 0, want a failure: %v", result)
		}
		if want := []string{markerWarning("1.1")}; !slices.Equal(wireWarnings(t, result), want) {
			t.Fatalf("warnings = %s, want %q", result["warnings"], want)
		}
	})

	t.Run("a later failure keeps them in text", func(t *testing.T) {
		setup(t, empty, failing)

		var stdout, stderr bytes.Buffer
		if code := Run([]string{"task", "complete-all", verifyTaskFeature}, &stdout, &stderr); code == 0 {
			t.Fatalf("exit = 0, want a failure: %s", stdout.String())
		}
		if !strings.Contains(stderr.String(), "Warnings:\n- "+markerWarning("1.1")+"\n") {
			t.Fatalf("stderr does not hold the warning:\n%s", stderr.String())
		}
	})

	t.Run("nothing to report stays null", func(t *testing.T) {
		setup(t, tested, tested)

		code, result := wireResult(t, "task", "complete-all", verifyTaskFeature)
		if code != 0 {
			t.Fatalf("exit = %d, want 0: %v", code, result)
		}
		if got := string(result["warnings"]); got != "null" {
			t.Fatalf("warnings = %s, want null", got)
		}
	})

	t.Run("text stays silent with nothing to report", func(t *testing.T) {
		setup(t, tested, tested)

		var stdout, stderr bytes.Buffer
		if code := Run([]string{"task", "complete-all", verifyTaskFeature}, &stdout, &stderr); code != 0 {
			t.Fatalf("exit = %d, want 0: %s", code, stderr.String())
		}
		if strings.Contains(stdout.String(), "Warnings:") {
			t.Fatalf("stdout holds a warnings section:\n%s", stdout.String())
		}
	})
}

func TestEmptyRunVerifySurfaces(t *testing.T) {
	empty := testutil.Response{Stdout: emptyPackageOutput}
	bothMarkers := []string{markerWarning("1.1"), markerWarning("1.2")}

	selections := []struct {
		name  string
		args  []string
		stale bool
		want  []string
	}{
		{name: "plain", args: nil, stale: true, want: bothMarkers},
		{name: "all", args: []string{"--all"}, want: bothMarkers},
		{name: "task", args: []string{"--task", "1.2"}, want: []string{markerWarning("1.2")}},
		{name: "check", args: []string{"--check"}, stale: true, want: bothMarkers},
	}
	for _, selection := range selections {
		args := append([]string{"verify", verifyTaskFeature}, selection.args...)
		setup := func(t *testing.T) {
			t.Helper()
			root := setupCompletedMarkers(t)
			if selection.stale {
				staleTree(t, root)
			}
			overrideCommandRunner(t, testutil.NewFakeRunner(empty, empty))
		}

		t.Run(selection.name+"/json", func(t *testing.T) {
			setup(t)
			code, result := wireResult(t, args...)
			if code != 0 {
				t.Fatalf("exit = %d, want 0: %v", code, result)
			}
			if warnings := wireWarnings(t, result); !slices.Equal(warnings, selection.want) {
				t.Fatalf("warnings = %q, want %q", warnings, selection.want)
			}
		})

		t.Run(selection.name+"/text", func(t *testing.T) {
			setup(t)
			var stdout, stderr bytes.Buffer
			if code := Run(args, &stdout, &stderr); code != 0 {
				t.Fatalf("exit = %d, want 0: %s", code, stderr.String())
			}
			want := "Warnings:\n- " + strings.Join(selection.want, "\n- ") + "\n"
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("stdout does not hold the warnings:\n%s", stdout.String())
			}
		})
	}
}

func TestEmptyRunFailingVerifyText(t *testing.T) {
	failing := testutil.Response{ExitCode: 1}
	setup := func(t *testing.T, first testutil.Response) {
		t.Helper()
		root := setupCompletedMarkers(t)
		staleTree(t, root)
		overrideCommandRunner(t, testutil.NewFakeRunner(first, failing))
	}

	t.Run("the warnings of the run reach stderr", func(t *testing.T) {
		setup(t, testutil.Response{Stdout: emptyPackageOutput})

		var stdout, stderr bytes.Buffer
		if code := Run([]string{"verify", verifyTaskFeature}, &stdout, &stderr); code == 0 {
			t.Fatalf("exit = 0, want a failure: %s", stdout.String())
		}
		if !strings.Contains(stderr.String(), "Warnings:\n- "+markerWarning("1.1")+"\n") {
			t.Fatalf("stderr does not hold the warning:\n%s", stderr.String())
		}
	})

	t.Run("without warnings the two lines stay", func(t *testing.T) {
		setup(t, testutil.Response{Stdout: testedPackageOutput})

		var stdout, stderr bytes.Buffer
		if code := Run([]string{"verify", verifyTaskFeature}, &stdout, &stderr); code == 0 {
			t.Fatalf("exit = 0, want a failure: %s", stdout.String())
		}
		lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
		if len(lines) != 2 || !strings.HasPrefix(lines[0], "verification failed for markers") || !strings.HasPrefix(lines[1], "Next action: ") {
			t.Fatalf("stderr = %q, want the summary and the next action alone", stderr.String())
		}
	})

	t.Run("json keeps one envelope on stdout", func(t *testing.T) {
		setup(t, testutil.Response{Stdout: emptyPackageOutput})

		var stdout, stderr bytes.Buffer
		if code := Run([]string{"verify", verifyTaskFeature, "--json"}, &stdout, &stderr); code == 0 {
			t.Fatalf("exit = 0, want a failure: %s", stdout.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want nothing", stderr.String())
		}
		if !json.Valid(stdout.Bytes()) || !strings.Contains(stdout.String(), emptyRunPhrase) {
			t.Fatalf("stdout does not hold one envelope with the warning:\n%s", stdout.String())
		}
	})
}

func TestEmptyRunAdoptApply(t *testing.T) {
	const want = `old-era: task 1: command ["true"] passed without running any test: check its selector and assert the test by name with -v and expect_output`
	empty := testutil.Response{Stdout: emptyPackageOutput}
	apply := []string{"adopt", "old-era", "--apply"}

	setup := func(t *testing.T, second testutil.Response) {
		t.Helper()
		root := adoptFixture(t)
		adoptDiagnosticTasks(t, root, "old-era", adoptDiagnosticTask("1", "")+adoptDiagnosticTask("2", ""))
		overrideCommandRunner(t, testutil.NewFakeRunner(empty, second))
	}

	for _, tc := range []struct {
		name     string
		second   testutil.Response
		exitCode int
	}{
		{"passing apply", testutil.Response{Stdout: testedPackageOutput}, 0},
		{"failing apply", testutil.Response{ExitCode: 1}, 1},
	} {
		t.Run(tc.name+"/json", func(t *testing.T) {
			setup(t, tc.second)
			code, result := wireResult(t, apply...)
			if code != tc.exitCode {
				t.Fatalf("exit = %d, want %d: %v", code, tc.exitCode, result)
			}
			if warnings := wireWarnings(t, result); !slices.Equal(warnings, []string{want}) {
				t.Fatalf("warnings = %q, want %q", warnings, want)
			}
		})

		t.Run(tc.name+"/text", func(t *testing.T) {
			setup(t, tc.second)
			var stdout, stderr bytes.Buffer
			code := Run(apply, &stdout, &stderr)
			if code != tc.exitCode {
				t.Fatalf("exit = %d, want %d: %s%s", code, tc.exitCode, stdout.String(), stderr.String())
			}
			stream := stdout.String()
			if code != 0 {
				stream = stderr.String()
			}
			if !strings.Contains(stream, "Warnings:\n- "+want+"\n") {
				t.Fatalf("output does not hold the warning:\n%s", stream)
			}
		})
	}
}

// wirePaths collects every key of a JSON value by its path, so two results
// compare by shape whatever their values say.
func wirePaths(value any, prefix string, paths map[string]bool) {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			path := prefix + "." + key
			paths[path] = true
			wirePaths(child, path, paths)
		}
	case []any:
		for _, child := range node {
			wirePaths(child, prefix+"[]", paths)
		}
	}
}

func TestEmptyRunAddsNoJSONField(t *testing.T) {
	empty := testutil.Response{Stdout: emptyPackageOutput}
	tested := testutil.Response{Stdout: testedPackageOutput}

	// shape runs one command under one proof output and returns the paths of
	// its whole envelope, after checking which kind of result it got.
	shape := func(t *testing.T, warned bool, args []string) []string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := Run(append(args, "--json"), &stdout, &stderr); code != 0 {
			t.Fatalf("%v exited %d: %s%s", args, code, stdout.String(), stderr.String())
		}
		if got := strings.Contains(stdout.String(), emptyRunPhrase); got != warned {
			t.Fatalf("%v warned = %v, want %v:\n%s", args, got, warned, stdout.String())
		}
		var document any
		if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
			t.Fatalf("decode %v: %v", args, err)
		}
		paths := map[string]bool{}
		wirePaths(document, "", paths)
		return sortedKeys(paths)
	}

	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, response testutil.Response) []string
	}{
		{"task complete", func(t *testing.T, response testutil.Response) []string {
			setupPendingProof(t)
			overrideCommandRunner(t, testutil.NewFakeRunner(response))
			return []string{"task", "complete", "todo-app-demo", "1"}
		}},
		{"task complete-all", func(t *testing.T, response testutil.Response) []string {
			setupReleasableFeature(t, verifyTaskFeature)
			overrideCommandRunner(t, testutil.NewFakeRunner(response, response))
			return []string{"task", "complete-all", verifyTaskFeature}
		}},
		{"verify", func(t *testing.T, response testutil.Response) []string {
			staleTree(t, setupCompletedMarkers(t))
			overrideCommandRunner(t, testutil.NewFakeRunner(response, response))
			return []string{"verify", verifyTaskFeature}
		}},
		{"adopt --apply", func(t *testing.T, response testutil.Response) []string {
			adoptFixture(t)
			overrideCommandRunner(t, testutil.NewFakeRunner(response))
			return []string{"adopt", "old-era", "--apply"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warned := shape(t, true, tc.setup(t, empty))
			silent := shape(t, false, tc.setup(t, tested))
			if !slices.Equal(warned, silent) {
				t.Fatalf("shapes differ:\nwarned: %v\nsilent: %v", warned, silent)
			}
		})
	}
}

func TestEmptyRunHelp(t *testing.T) {
	const want = "A proof that passes while go test ran no test warns and keeps its exit code: assert the test by name with -v and expect_output."

	for _, command := range [][]string{
		{"task", "complete"},
		{"task", "complete-all"},
		{"verify"},
	} {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(append(slices.Clone(command), "--help"), &stdout, &stderr); code != 0 {
				t.Fatalf("exit = %d, want 0: %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "\n  "+want+"\n") {
				t.Fatalf("help does not hold the statement among its details:\n%s", stdout.String())
			}
		})
	}
}
