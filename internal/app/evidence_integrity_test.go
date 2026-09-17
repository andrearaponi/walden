package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

func integrityCLI(t *testing.T, args ...string) (output.Result, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append(args, "--json"), &stdout, &stderr)
	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("walden %v returned invalid JSON: %v (%s; %s)", args, err, stdout.String(), stderr.String())
	}
	if envelope.OK != (code == 0) || envelope.Result.ExitCode != code {
		t.Fatalf("inconsistent exit/envelope for %v: %+v, exit %d", args, envelope, code)
	}
	return envelope.Result, code
}

func integrityMustCLI(t *testing.T, args ...string) output.Result {
	t.Helper()
	result, code := integrityCLI(t, args...)
	if code != 0 {
		t.Fatalf("walden %v failed: %+v", args, result)
	}
	return result
}

func integrityGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func integrityFeature(t *testing.T, root, name string) {
	t.Helper()
	tasks := strings.ReplaceAll(releasableTasks, `command: ["sh", "-c", "true"]`, "command: [\"printf\", \"PASS\"]\n        expect_exit: 0\n        expect_output: \"PASS\"\n        timeout: 60s")
	writeRawFeatureFile(t, root, name, "requirements.md", releasableRequirements)
	writeRawFeatureFile(t, root, name, "design.md", releasableDesign)
	writeRawFeatureFile(t, root, name, "tasks.md", tasks)
	for _, phase := range []string{"requirements", "design", "tasks"} {
		integrityMustCLI(t, "review", "open", name, "--phase", phase)
		integrityMustCLI(t, "review", "approve", name, "--phase", phase)
	}
}

func integrityRepo(t *testing.T, name string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("Git is required by the integrity acceptance fixture")
	}
	root := chdirContract(t)
	integrityFeature(t, root, name)
	gitify(t, root)
	return root
}

func integrityEditTaskBody(t *testing.T, root, name, old, replacement string) {
	t.Helper()
	path := filepath.Join(root, ".walden", "specs", name, "tasks.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("fixture task edit did not find %q", old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, replacement, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	integrityMustCLI(t, "reconcile", name)
	integrityMustCLI(t, "review", "open", name, "--phase", "tasks")
	integrityMustCLI(t, "review", "approve", name, "--phase", "tasks")
}

type integrityRunnerFunc func(context.Context, string, ...string) (shell.Response, error)

func (f integrityRunnerFunc) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	return f(ctx, name, args...)
}

func integrityWaldenFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(filepath.Join(root, ".walden"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestEvidenceIntegrityStrictReleaseOutput(t *testing.T) {
	const name = "strict-inputs"
	root := integrityRepo(t, name)
	integrityGit(t, root, "rm", "-r", "--cached", ".walden")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".walden/\nscratch/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	integrityGit(t, root, "add", ".gitignore")
	integrityGit(t, root, "commit", "-qm", "ignored local metadata")
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "PASS"}, testutil.Response{Stdout: "PASS"}))
	integrityMustCLI(t, "task", "complete-all", name)
	if err := os.WriteFile(filepath.Join(root, ".walden", "environment.md"), []byte("- broken: not-an-argv\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := overrideCommandRunner(t, testutil.NewFakeRunner())
	before := integrityWaldenFiles(t, root)
	integrityMustCLI(t, "release", "check", name)
	result, code := integrityCLI(t, "release", "check", name, "--strict", "--allow-pending", "--reason", "does not waive inputs")
	if code != 1 || result.Release == nil || result.Release.Releasable || !strings.Contains(strings.Join(result.Blockers, " "), ".walden/specs/strict-inputs/requirements.md") {
		t.Fatalf("CLI accepted ignored local inputs: exit=%d result=%+v", code, result)
	}
	if len(runner.Calls()) != 0 || !reflect.DeepEqual(before, integrityWaldenFiles(t, root)) {
		t.Fatal("release executed proofs/probes or wrote inputs")
	}
	integrityGit(t, root, "add", "-f", ".walden")
	integrityGit(t, root, "commit", "-qm", "committed metadata")
	if err := os.MkdirAll(filepath.Join(root, "scratch"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scratch", "cache"), []byte("irrelevant"), 0o644); err != nil {
		t.Fatal(err)
	}
	result = integrityMustCLI(t, "release", "check", name, "--strict")
	if result.CertifiedCommit != integrityGit(t, root, "rev-parse", "HEAD") {
		t.Fatal("verdict did not name the compared commit")
	}
}

func TestEvidenceIntegrityAdoptionAssessment(t *testing.T) {
	const name = "legacy-assessment"
	root := integrityRepo(t, name)
	integrityEditTaskBody(t, root, name, `["printf", "PASS"]`, `["sh", "-c", "touch PROOF-RAN; printf PASS"]`)
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "PASS"}, testutil.Response{Stdout: "PASS"}))
	integrityMustCLI(t, "task", "complete-all", name)
	feature, err := spec.LoadFeature(root, name)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := evidence.Load(root, name)
	if err != nil {
		t.Fatal(err)
	}
	ledger.SchemaVersion = "v1alpha1"
	for _, task := range tree.LeafTasks() {
		record := ledger.Tasks[task.ID]
		record.TaskFingerprint = spec.LegacyTaskDefinitionFingerprint(task)
		record.TaskFingerprintScheme, record.Execution = "", nil
		ledger.Tasks[task.ID] = record
	}
	data, _ := json.Marshal(ledger)
	if err := os.WriteFile(evidence.DocumentPath(root, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("new code"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".walden", "environment.md"), []byte("- smoke: [\"sh\", \"-c\", \"touch PROBE-RAN\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := integrityWaldenFiles(t, root)
	runner := overrideCommandRunner(t, testutil.NewFakeRunner())
	plan := integrityMustCLI(t, "adopt", name)
	data, _ = json.Marshal(plan.Adoption.Features[0])
	var wire struct {
		Evidence []struct {
			State         string `json:"state"`
			CodeFreshness string `json:"code_freshness"`
			Binding       struct {
				State string `json:"state"`
			} `json:"binding"`
			Execution struct {
				State string `json:"state"`
			} `json:"execution"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Evidence) != 2 {
		t.Fatalf("adoption lacks per-task independent assessments: %s", data)
	}
	for _, entry := range wire.Evidence {
		if entry.Binding.State != "reconstructed-equivalent" || entry.CodeFreshness != "stale" || entry.Execution.State != "legacy-unattested" || entry.State != "stale-code" {
			t.Fatalf("migration conflated its three dimensions: %+v", entry)
		}
	}
	if len(runner.Calls()) != 0 || !reflect.DeepEqual(before, integrityWaldenFiles(t, root)) {
		t.Fatal("read-only migration executed proofs or changed facts")
	}
	for _, marker := range []string{"PROBE-RAN", "PROOF-RAN"} {
		if _, err := os.Stat(filepath.Join(root, marker)); !os.IsNotExist(err) {
			t.Fatalf("migration executed %s", marker)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".walden", "environment.md"), []byte("- broken: not-an-argv\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	integrityMustCLI(t, "adopt", name)
}

func TestEvidenceIntegrityExecutionOutcomes(t *testing.T) {
	t.Run("policy failures retain assertion facts", func(t *testing.T) {
		const name = "integrity-execution"
		root := integrityRepo(t, name)
		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "PASS"}, testutil.Response{Stdout: "PASS"}))
		integrityMustCLI(t, "task", "complete-all", name)
		previous := commandRunner
		t.Cleanup(func() { commandRunner = previous })
		calls := 0
		commandRunner = integrityRunnerFunc(func(_ context.Context, _ string, _ ...string) (shell.Response, error) {
			calls++
			if calls == 1 {
				if err := os.WriteFile(filepath.Join(root, "mutated.txt"), []byte("mutation"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			return shell.Response{Stdout: "PASS"}, nil
		})
		result, code := integrityCLI(t, "verify", name, "--all")
		if code != 1 || calls != 2 || len(result.Evidence) != 2 || result.Evidence[1].State != "failed" || result.Evidence[1].Passed == nil || *result.Evidence[1].Passed {
			t.Fatalf("CLI presented contaminated success: exit=%d calls=%d %+v", code, calls, result)
		}
		if !strings.Contains(result.Evidence[1].Failure, "contaminat") {
			t.Fatalf("missing policy attribution: %+v", result.Evidence[1])
		}
	})
	t.Run("adoption propagates unavailable identity", func(t *testing.T) {
		root := chdirContract(t)
		const name = "no-git"
		integrityFeature(t, root, name)
		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "PASS"}, testutil.Response{Stdout: "PASS"}))
		integrityMustCLI(t, "task", "complete-all", name)
		runner := overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "PASS"}, testutil.Response{Stdout: "PASS"}))
		result, code := integrityCLI(t, "adopt", name, "--apply")
		if code != 1 || len(runner.Calls()) != 0 || result.Adoption == nil || result.Adoption.Features[0].Reason == "" {
			t.Fatalf("adoption converted a preflight error to success: exit=%d calls=%v result=%+v", code, runner.Calls(), result)
		}
	})
}

func TestEvidenceIntegrityContractBindingLifecycle(t *testing.T) {
	const name = "integrity-demo"
	root := integrityRepo(t, name)
	overrideCommandRunner(t, testutil.NewFakeRunner(
		testutil.Response{Stdout: "PASS", ExitCode: 0},
		testutil.Response{Stdout: "PASS", ExitCode: 0},
	))
	integrityMustCLI(t, "task", "complete-all", name)

	feature, err := spec.LoadFeature(root, name)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := evidence.Load(root, name)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tree.LeafTasks() {
		if ledger.Tasks[task.ID].TaskFingerprint != spec.TaskDefinitionFingerprint(task) {
			t.Fatalf("completion and spec consumer disagree for %s", task.ID)
		}
	}
	status := integrityMustCLI(t, "evidence", "status", name)
	if len(status.Evidence) != 2 || status.Evidence[0].State != "verified" || status.Evidence[1].State != "verified" {
		t.Fatalf("current complete records not verified: %+v", status)
	}
	if result := integrityMustCLI(t, "task", "status", name); len(result.Warnings) != 0 {
		t.Fatalf("readiness disagrees with evidence status: %+v", result)
	}
	plan := integrityMustCLI(t, "adopt", name)
	if plan.Adoption == nil || len(plan.Adoption.Features) != 1 || plan.Adoption.Features[0].Class != "complete" {
		t.Fatalf("adoption disagrees on current records: %+v", plan)
	}
	integrityMustCLI(t, "release", "check", name)
	runner := overrideCommandRunner(t, testutil.NewFakeRunner())
	integrityMustCLI(t, "verify", name)
	if len(runner.Calls()) != 0 {
		t.Fatalf("unchanged contracts were re-executed: %+v", runner.Calls())
	}

	integrityEditTaskBody(t, root, name, `expect_output: "PASS"`, `expect_output: "MISSING"`)
	status = integrityMustCLI(t, "evidence", "status", name)
	if status.Evidence[0].State != "stale-spec" || status.Evidence[1].State != "verified" {
		t.Fatalf("changed assertion reused evidence or invalidated a sibling: %+v", status.Evidence)
	}
	if result, code := integrityCLI(t, "release", "check", name); code == 0 {
		t.Fatalf("changed assertion certified before re-execution: %+v", result)
	}
	runner = overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "PASS", ExitCode: 0}))
	result, code := integrityCLI(t, "verify", name)
	if code != 1 || len(runner.Calls()) != 1 || len(result.Evidence) != 1 || result.Evidence[0].TaskID != "1.1" || result.Evidence[0].State != "failed" {
		t.Fatalf("selective verify did not execute/fail the changed assertion: exit=%d calls=%+v result=%+v", code, runner.Calls(), result)
	}
	if result, code := integrityCLI(t, "release", "check", name); code == 0 {
		t.Fatalf("failed new assertion became releasable: %+v", result)
	}
}
