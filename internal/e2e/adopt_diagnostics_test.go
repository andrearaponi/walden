package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/spec"
)

func diagnosticsNativeCLI(t *testing.T) string {
	t.Helper()
	requireGit(t)
	if _, err := exec.LookPath("sh"); err != nil {
		t.Fatal("the declared Git/shell acceptance fixtures require sh")
	}
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	binary := filepath.Join(t.TempDir(), "walden")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/walden")
	command.Dir = root
	if data, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build temporary CLI: %v\n%s", err, data)
	}
	return binary
}

func diagnosticsNativeRun(t *testing.T, binary, root string, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = root
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatalf("native CLI timed out: %v", args)
	}
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("native CLI could not run: %v", err)
	}
	return stdout.String(), stderr.String(), exit.ExitCode()
}

func diagnosticsNativeJSON(t *testing.T, binary, root string, wantCode int, args ...string) output.Result {
	t.Helper()
	stdout, stderr, code := diagnosticsNativeRun(t, binary, root, append(args, "--json")...)
	if code != wantCode || stderr != "" {
		t.Fatalf("%v exit=%d want=%d\nstdout=%s\nstderr=%s", args, code, wantCode, stdout, stderr)
	}
	var envelope output.Envelope
	decoder := json.NewDecoder(strings.NewReader(stdout))
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF || envelope.OK != (code == 0) || envelope.Result.ExitCode != code || envelope.SchemaVersion != "v0beta1" {
		t.Fatalf("invalid envelope boundary/verdict: %v %+v", err, envelope)
	}
	return envelope.Result
}

func diagnosticsCopyFeature(t *testing.T, root, name string) {
	t.Helper()
	for _, file := range []string{"requirements.md", "design.md", "tasks.md"} {
		data, err := os.ReadFile(filepath.Join(root, ".walden/specs/ledger-demo", file))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, filepath.Join(".walden/specs", name, file), string(data))
	}
}

func diagnosticsNativeProofs(t *testing.T, binary, root, name string, commands ...[]string) {
	t.Helper()
	feature, err := spec.LoadFeature(root, name)
	if err != nil {
		t.Fatal(err)
	}
	var body strings.Builder
	body.WriteString("# Implementation Plan\n\n- [x] 1. Markers\n")
	for i, command := range commands {
		id := "1." + strconv.Itoa(i+1)
		criterion := "R1.AC1"
		if i > 0 {
			criterion = "R1.AC2"
		}
		argv, _ := json.Marshal(command)
		body.WriteString("  - [x] " + id + ". Assert marker\n    - Requirements: `" + criterion + "`\n    - Design: Overview\n    - Verification:\n      - command: " + string(argv) + "\n        covers: [\"" + criterion + "\"]\n")
	}
	feature.Tasks.Body = body.String()
	if err := spec.SaveDocument(feature.Tasks); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"reconcile", name}, {"review", "open", name, "--phase", "tasks"}, {"review", "approve", name, "--phase", "tasks"},
	} {
		diagnosticsNativeJSON(t, binary, root, 0, args...)
	}
}

func diagnosticsFeatureResult(t *testing.T, result output.Result, name string) output.AdoptionFeature {
	t.Helper()
	if result.Adoption != nil {
		for _, feature := range result.Adoption.Features {
			if feature.Feature == name {
				return feature
			}
		}
	}
	t.Fatalf("missing adoption feature %s: %+v", name, result)
	return output.AdoptionFeature{}
}

func TestAdoptDiagnosticsEndToEnd(t *testing.T) {
	binary := diagnosticsNativeCLI(t)
	t.Run("scope plan actual failure retry and noop", func(t *testing.T) {
		root := fixtureRepo(t)
		for _, name := range []string{"target", "untouched", "a-blocked"} {
			diagnosticsCopyFeature(t, root, name)
		}
		path := filepath.Join(root, ".walden/specs/a-blocked/requirements.md")
		data, _ := os.ReadFile(path)
		writeFile(t, root, ".walden/specs/a-blocked/requirements.md", string(data)+"\nUnreviewed edit.\n")
		outside := t.TempDir()
		probe, trace := filepath.Join(outside, "probe"), filepath.Join(outside, "trace")
		argv, _ := json.Marshal([]string{"sh", "-c", `printf probe > "$1"`, "sh", probe})
		writeFile(t, root, ".walden/environment.md", "# Probes\n- marker: "+string(argv)+"\n")
		diagnosticsNativeProofs(t, binary, root, "target",
			[]string{"sh", "-c", `printf 'A\n' >> "$1"; grep -q MARKER-A src.txt`, "sh", trace},
			[]string{"sh", "-c", `printf 'B\n' >> "$1"; grep -q MARKER-B src.txt`, "sh", trace})
		writeFile(t, root, "src.txt", "MARKER-A\n")
		unrelatedBefore, _ := os.ReadFile(filepath.Join(root, ".walden/evidence/ledger-demo.json"))

		plan := diagnosticsNativeJSON(t, binary, root, 0, "adopt")
		if plan.Adoption == nil || plan.Adoption.Workload.AssessedTasks != 6 || plan.Adoption.Workload.AssessedSteps != 6 || strings.Join(plan.Adoption.Workload.UnassessedFeatures, ",") != "a-blocked" || plan.Adoption.ElapsedMS != nil {
			t.Fatalf("wrong portfolio assessment: %+v", plan.Adoption)
		}
		named := diagnosticsNativeJSON(t, binary, root, 0, "adopt", "target")
		if named.Adoption.Scope.Kind != "feature" || len(named.Adoption.Features) != 1 || named.Adoption.Workload.AssessedTasks != 2 {
			t.Fatalf("named assessment broadened: %+v", named.Adoption)
		}
		for _, path := range []string{probe, trace} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("planning executed probe/proof: %s %v", path, err)
			}
		}

		failed := diagnosticsNativeJSON(t, binary, root, 1, "adopt", "target", "--apply")
		feature := diagnosticsFeatureResult(t, failed, "target")
		if len(feature.Evidence) != 2 || strings.Join(feature.Verified, ",") != "1.1" || strings.Join(feature.Failed, ",") != "1.2" || feature.Evidence[1].Failure == "" || feature.EvidencePersisted == nil || !*feature.EvidencePersisted || feature.ElapsedMS == nil || failed.Adoption.ElapsedMS == nil {
			t.Fatalf("real failure diagnosis/timing lost: %+v", feature)
		}
		if _, err := os.Stat(probe); err != nil {
			t.Fatalf("fixture probe was not genuinely executable: %v", err)
		}
		traceData, _ := os.ReadFile(trace)
		if string(traceData) != "A\nB\n" {
			t.Fatalf("unexpected proof replay/order: %q", traceData)
		}
		unrelatedAfter, _ := os.ReadFile(filepath.Join(root, ".walden/evidence/ledger-demo.json"))
		if string(unrelatedBefore) != string(unrelatedAfter) {
			t.Fatal("named adoption changed unrelated evidence")
		}
		if _, err := os.Stat(filepath.Join(root, ".walden/evidence/untouched.json")); !os.IsNotExist(err) {
			t.Fatal("named adoption executed an unrelated feature")
		}

		writeFile(t, root, "src.txt", "MARKER-A\nMARKER-B\n")
		stdout, stderr, code := diagnosticsNativeRun(t, binary, root, "adopt", "target", "--apply")
		if code != 0 || stderr != "" {
			t.Fatalf("retry: %d %s %s", code, stdout, stderr)
		}
		start := strings.Index(stdout, "target task 1.1 (1/2): starting proof")
		finish := strings.Index(stdout, "target task 1.1: execution accepted")
		next := strings.Index(stdout, "target task 1.2 (2/2): starting proof")
		if start < 0 || finish <= start || next <= finish {
			t.Fatalf("native progress ordering: %s", stdout)
		}
		ledgerPath := filepath.Join(root, ".walden/evidence/target.json")
		before, _ := os.ReadFile(ledgerPath)
		traceBefore, _ := os.ReadFile(trace)
		noop := diagnosticsNativeJSON(t, binary, root, 0, "adopt", "target", "--apply")
		after, _ := os.ReadFile(ledgerPath)
		traceAfter, _ := os.ReadFile(trace)
		feature = diagnosticsFeatureResult(t, noop, "target")
		if feature.Class != "complete" || len(feature.Evidence) != 0 || feature.EvidencePersisted != nil || feature.ElapsedMS == nil || string(before) != string(after) || string(traceBefore) != string(traceAfter) {
			t.Fatalf("no-op invented execution or rewrote evidence: %+v", feature)
		}
	})

	t.Run("real mutator contaminated successor and stale prefix", func(t *testing.T) {
		root := fixtureRepo(t)
		diagnosticsNativeProofs(t, binary, root, "ledger-demo",
			[]string{"sh", "-c", "grep -q MARKER-A src.txt"},
			[]string{"sh", "-c", "printf 'MUTATED\\n' >> src.txt"},
			[]string{"sh", "-c", "grep -q MUTATED src.txt"})
		result := diagnosticsNativeJSON(t, binary, root, 1, "adopt", "ledger-demo", "--apply")
		feature := diagnosticsFeatureResult(t, result, "ledger-demo")
		if len(feature.Evidence) != 3 || feature.Evidence[0].Passed == nil || !*feature.Evidence[0].Passed || feature.Evidence[0].State != "stale-code" {
			t.Fatalf("pure prefix was reassigned: %+v", feature)
		}
		for i, integrity := range []string{"mutated", "contaminated"} {
			entry := feature.Evidence[i+1]
			if entry.Passed == nil || *entry.Passed || entry.Execution == nil || entry.Execution.Facts == nil || entry.Execution.Facts.AssertionResult != "passed" || entry.Execution.Facts.Integrity != integrity || entry.Execution.Facts.CauseTask != "1.2" {
				t.Fatalf("real policy distinction lost: %+v", entry)
			}
		}
		if len(result.Warnings) == 0 || !strings.Contains(strings.Join(result.Warnings, " "), "ledger-demo:") || !strings.Contains(feature.Evidence[1].Failure, "src.txt") {
			t.Fatal("native policy diagnostics lost their feature/path")
		}
	})

	t.Run("late write failure preserves results and continues portfolio", func(t *testing.T) {
		root := fixtureRepo(t)
		diagnosticsCopyFeature(t, root, "a-save-fails")
		diagnosticsCopyFeature(t, root, "z-good")
		diagnosticsNativeProofs(t, binary, root, "a-save-fails",
			[]string{"sh", "-c", "mkdir -p .walden/evidence/a-save-fails.json; printf NATIVE-FAILURE >&2; exit 4"},
			[]string{"sh", "-c", "grep -q MARKER-B src.txt"})
		result := diagnosticsNativeJSON(t, binary, root, 1, "adopt", "--apply")
		bad, good := diagnosticsFeatureResult(t, result, "a-save-fails"), diagnosticsFeatureResult(t, result, "z-good")
		if bad.EvidencePersisted == nil || *bad.EvidencePersisted || len(bad.Evidence) != 2 || !strings.Contains(bad.Evidence[0].Failure, "NATIVE-FAILURE") || !strings.Contains(bad.Reason, "persist refreshed evidence") || len(bad.Verified) != 0 || len(bad.Failed) != 0 {
			t.Fatalf("unpersisted results lost or promoted: %+v", bad)
		}
		if len(good.Verified) != 2 || good.Reason != "" || good.EvidencePersisted == nil || !*good.EvidencePersisted {
			t.Fatalf("feature error stopped remaining work: %+v", good)
		}
	})
}
