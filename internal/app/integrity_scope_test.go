package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

func integrityScope(t *testing.T, value any, kind string, features ...string) {
	t.Helper()
	data, _ := json.Marshal(value)
	var wire struct {
		Scope struct {
			Kind      string   `json:"kind"`
			Features  []string `json:"features"`
			Guarantee string   `json:"guarantee"`
		} `json:"scope"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Scope.Kind != kind || !reflect.DeepEqual(wire.Scope.Features, features) || wire.Scope.Guarantee != evidence.Guarantee {
		t.Fatalf("incorrect/absent scope: %s", data)
	}
}

func integrityLegacy(t *testing.T, root, name string, ids ...string) {
	t.Helper()
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
	for _, id := range ids {
		task, ok := tree.FindTask(id)
		if !ok {
			t.Fatal("fixture task not found")
		}
		record := ledger.Tasks[id]
		record.TaskFingerprint, record.TaskFingerprintScheme, record.Execution = spec.LegacyTaskDefinitionFingerprint(task), "", nil
		ledger.Tasks[id] = record
	}
	data, _ := json.Marshal(ledger)
	if err := os.WriteFile(evidence.DocumentPath(root, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func integrityComplete(t *testing.T, name string) {
	t.Helper()
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "PASS"}, testutil.Response{Stdout: "PASS"}))
	integrityMustCLI(t, "task", "complete-all", name)
}

func TestEvidenceIntegrityScopedExecution(t *testing.T) {
	root := integrityRepo(t, "selected")
	integrityFeature(t, root, "outside")
	integrityEditTaskBody(t, root, "outside", `["printf", "PASS"]`, `["sh", "-c", "touch OUTSIDE-RAN; printf PASS"]`)
	integrityComplete(t, "selected")
	integrityComplete(t, "outside")
	integrityLegacy(t, root, "selected", "1.1")
	outsideBefore, _ := os.ReadFile(evidence.DocumentPath(root, "outside"))
	before, err := evidence.Load(root, "selected")
	if err != nil {
		t.Fatal(err)
	}
	feature, err := spec.LoadFeature(root, "selected")
	if err != nil {
		t.Fatal(err)
	}
	runner := overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "PASS"}))
	result := integrityMustCLI(t, "verify", "selected")
	if len(runner.Calls()) != 1 || len(result.Evidence) != 1 || result.Evidence[0].TaskID != "1.1" || result.Evidence[0].State != "verified" {
		t.Fatalf("selective scope expanded or did not refresh real evidence: %+v calls=%v", result, runner.Calls())
	}
	integrityScope(t, result, "feature", "selected")
	after, err := evidence.Load(root, "selected")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Tasks["1.2"], after.Tasks["1.2"]) || after.Tasks["1.1"].Execution == nil || after.Tasks["1.1"].Execution.Origin != "verify" {
		t.Fatal("unselected record changed or refresh lacked provenance")
	}
	unchanged, err := spec.LoadFeature(root, "selected")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(feature, unchanged) {
		t.Fatal("verification re-approved or changed documents")
	}
	ledgerBefore, _ := os.ReadFile(evidence.DocumentPath(root, "selected"))
	runner = overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "PASS"}, testutil.Response{Stdout: "PASS"}))
	integrityMustCLI(t, "verify", "selected", "--all", "--check")
	ledgerAfter, _ := os.ReadFile(evidence.DocumentPath(root, "selected"))
	outsideAfter, _ := os.ReadFile(evidence.DocumentPath(root, "outside"))
	if len(runner.Calls()) != 2 || !bytes.Equal(ledgerBefore, ledgerAfter) || !bytes.Equal(outsideBefore, outsideAfter) {
		t.Fatal("forced/check scope wrote or executed outside selection")
	}
	if _, err := os.Stat(filepath.Join(root, "OUTSIDE-RAN")); !os.IsNotExist(err) {
		t.Fatal("unrelated historical command executed")
	}
}

func TestEvidenceIntegrityScopedCertification(t *testing.T) {
	root := integrityRepo(t, "active")
	integrityFeature(t, root, "historical")
	integrityComplete(t, "active")
	integrityComplete(t, "historical")
	integrityLegacy(t, root, "historical", "1.1", "1.2")
	integrityGit(t, root, "add", ".walden")
	integrityGit(t, root, "commit", "-qm", "mixed assurance portfolio")
	selected := integrityMustCLI(t, "release", "check", "active", "--strict")
	integrityScope(t, selected.Release, "feature", "active")
	if !strings.Contains(selected.Summary, "active") || selected.Release.Worktree.InputBinding != "matched" {
		t.Fatalf("partial success did not identify its actual scope: %+v", selected)
	}
	all, code := integrityCLI(t, "release", "check")
	if code != 1 || !strings.Contains(strings.Join(all.Blockers, " "), "producer") {
		t.Fatalf("portfolio legacy gap was hidden: %+v", all)
	}
	integrityScope(t, all.Release, "portfolio", "active", "historical")
	if waived, code := integrityCLI(t, "release", "check", "--allow-pending", "--reason", "does not waive assurance"); code != 1 || waived.Release.Releasable {
		t.Fatal("pending waiver bypassed execution assurance")
	}
	blocked, code := integrityCLI(t, "release", "check", "historical", "--strict")
	if code != 1 || !strings.Contains(blocked.NextAction, "walden release check historical --strict") {
		t.Fatalf("retry broadened or changed selected scope: %+v", blocked)
	}
	plan := integrityMustCLI(t, "adopt", "historical")
	integrityScope(t, plan.Adoption, "feature", "historical")
	if !strings.Contains(plan.NextAction, "walden adopt historical --apply") {
		t.Fatalf("apply suggestion broadened scope: %s", plan.NextAction)
	}
	plain := integrityMustCLI(t, "release", "check", "active")
	if plain.Release.Worktree.InputBinding != "not-requested" {
		t.Fatal("non-strict claimed committed metadata binding")
	}
}

func integrityAssertGapView(t *testing.T, view output.EvidenceStatus, state string) {
	t.Helper()
	if view.State != state || view.Passed == nil || !*view.Passed || view.Binding == nil || view.Binding.State != "reconstructed-equivalent" || view.Execution == nil || view.Execution.State != "legacy-unattested" || len(view.Gaps) == 0 {
		t.Fatalf("stored pass/unknown guarantee conflated: %+v", view)
	}
}

func TestEvidenceIntegrityAssuranceSurfaces(t *testing.T) {
	root := integrityRepo(t, "surfaces")
	integrityComplete(t, "surfaces")
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "PASS"}, testutil.Response{Stdout: "PASS"}))
	verified := integrityMustCLI(t, "verify", "surfaces", "--all")
	for _, entry := range verified.Evidence {
		if entry.State != "verified" || entry.Execution == nil || entry.Execution.Facts == nil || entry.Execution.Facts.Origin != "verify" || entry.Execution.Facts.Policy != evidence.VerifyPolicy {
			t.Fatalf("current producer metadata not exposed: %+v", entry)
		}
	}
	integrityLegacy(t, root, "surfaces", "1.1")
	status := integrityMustCLI(t, "evidence", "status", "surfaces")
	integrityAssertGapView(t, status.Evidence[0], "unattested")
	if !strings.Contains(status.Summary, "1 unattested") {
		t.Fatalf("unknown guarantee missing from counts: %+v", status)
	}
	if err := os.WriteFile(filepath.Join(root, "new-code"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	status = integrityMustCLI(t, "evidence", "status", "surfaces")
	integrityAssertGapView(t, status.Evidence[0], "stale-code")
	readiness := integrityMustCLI(t, "task", "status", "surfaces")
	if !strings.Contains(strings.Join(readiness.Warnings, " "), "producer") {
		t.Fatalf("readiness hid the specific assurance gap: %+v", readiness)
	}
	plan := integrityMustCLI(t, "adopt", "surfaces")
	integrityAssertGapView(t, plan.Adoption.Features[0].Evidence[0], "stale-code")
	release, code := integrityCLI(t, "release", "check", "surfaces")
	if code != 1 {
		t.Fatal("stale/legacy evidence certified")
	}
	data, _ := json.Marshal(release.Release.Features[0])
	var wire struct {
		Evidence []output.EvidenceStatus `json:"evidence"`
	}
	if err := json.Unmarshal(data, &wire); err != nil || len(wire.Evidence) != 2 {
		t.Fatalf("release omitted its assurance assessment: %s %v", data, err)
	}
	integrityAssertGapView(t, wire.Evidence[0], "stale-code")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"adopt", "surfaces"}, &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	for _, text := range []string{"binding=reconstructed-equivalent", "code=stale", "execution=legacy-unattested"} {
		if !strings.Contains(stdout.String(), text) {
			t.Fatalf("human report lacks %s: %s", text, stdout.String())
		}
	}
	path := evidence.DocumentPath(root, "surfaces")
	original, _ := os.ReadFile(path)
	bad := bytes.Replace(original, []byte(`"schema_version":"v1alpha2"`), []byte(`"schema_version":"v9alpha9"`), 1)
	if bytes.Equal(original, bad) {
		t.Fatal("fixture schema replacement was vacuous")
	}
	if err := os.WriteFile(path, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"evidence", "status", "surfaces"}, {"adopt", "surfaces"}, {"release", "check", "surfaces"}, {"task", "status", "surfaces"}} {
		result, _ := integrityCLI(t, args...)
		data, _ := json.Marshal(result)
		if strings.Contains(string(data), "remove the file") || strings.Contains(string(data), "delete the ledger") || !strings.Contains(string(data), "retain") {
			t.Fatalf("destructive or missing migration remedy for %v: %s", args, data)
		}
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, bad) {
		t.Fatal("diagnosis changed the unsupported ledger")
	}
}
