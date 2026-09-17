package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/spec"
)

func integrityResult(t *testing.T, root string, args ...string) (output.Result, int) {
	t.Helper()
	text, code := cli(t, root, append(args, "--json")...)
	var envelope output.Envelope
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatalf("invalid CLI envelope for %v: %v (%s)", args, err, text)
	}
	if envelope.OK != (code == 0) || envelope.Result.ExitCode != code {
		t.Fatalf("exit/envelope disagree: %d %+v", code, envelope)
	}
	return envelope.Result, code
}

func integrityReapproveTasks(t *testing.T, root string, changes ...[2]string) {
	t.Helper()
	path := filepath.Join(root, ".walden/specs/gate-demo/tasks.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, change := range changes {
		if !strings.Contains(text, change[0]) {
			t.Fatalf("fixture edit missing %q", change[0])
		}
		text = strings.Replace(text, change[0], change[1], 1)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	mustCLI(t, root, "reconcile", "gate-demo")
	mustCLI(t, root, "review", "open", "gate-demo", "--phase", "tasks")
	mustCLI(t, root, "review", "approve", "gate-demo", "--phase", "tasks")
}

// These are integrated witnesses for behaviors introduced/tested with the
// corresponding kernel slices, not a substitute for their earlier red tests.
// Every step uses the actual CLI entry and process runner in owned fixtures.
func TestEvidenceIntegrityEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("Git is required for the integrity end-to-end acceptance group")
	}
	t.Run("approved assertion cannot reuse its old pass", func(t *testing.T) {
		root := releasableRepo(t)
		mustCLI(t, root, "release", "check", "gate-demo", "--strict")
		old := `command: ["sh", "-c", "grep -q MARKER-A src.txt"]`
		integrityReapproveTasks(t, root, [2]string{old, old + "\n        expect_output: \"NEW-ASSERTION\""})
		status, code := integrityResult(t, root, "evidence", "status", "gate-demo")
		if code != 0 || len(status.Evidence) != 2 || status.Evidence[0].State != "stale-spec" || status.Evidence[1].State != "verified" {
			t.Fatalf("assertion change kept its pass or moved a sibling: %+v", status)
		}
		verified, code := integrityResult(t, root, "verify", "gate-demo")
		if code != 1 || len(verified.Evidence) != 1 || verified.Evidence[0].TaskID != "1.1" || verified.Evidence[0].State != "failed" || !strings.Contains(verified.Evidence[0].Failure, "NEW-ASSERTION") {
			t.Fatalf("selective verify skipped the actual new assertion: %+v", verified)
		}
		if result, code := integrityResult(t, root, "release", "check", "gate-demo"); code == 0 || result.Release.Releasable {
			t.Fatal("failed current assertion was certified")
		}
	})
	t.Run("contaminated successor is retried after restoration", func(t *testing.T) {
		root := releasableRepo(t)
		mutator, _ := json.Marshal([]string{"sh", "-c", "if test -f .walden/mutate; then printf CONTAMINATED > src.txt; fi"})
		successor, _ := json.Marshal([]string{"sh", "-c", "if test -f .walden/reverify; then grep -q CONTAMINATED src.txt; else grep -q MARKER-B src.txt; fi"})
		integrityReapproveTasks(t, root,
			[2]string{`["sh", "-c", "grep -q MARKER-A src.txt"]`, string(mutator)},
			[2]string{`["sh", "-c", "grep -q MARKER-B src.txt"]`, string(successor)},
		)
		mustCLI(t, root, "verify", "gate-demo", "--all")
		original, err := os.ReadFile(filepath.Join(root, "src.txt"))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, ".walden/mutate", "fixture trigger")
		writeFile(t, root, ".walden/reverify", "fixture assertion branch")
		result, code := integrityResult(t, root, "verify", "gate-demo", "--all")
		if code != 1 || len(result.Evidence) != 2 {
			t.Fatalf("contaminating run did not continue and fail: %+v", result)
		}
		for _, entry := range result.Evidence {
			if entry.State != "failed" || entry.Passed == nil || *entry.Passed {
				t.Fatalf("contaminated run accepted a task: %+v", entry)
			}
		}
		later := result.Evidence[1]
		if later.Execution == nil || later.Execution.Facts == nil || later.Execution.Facts.AssertionResult != "passed" || later.Execution.Facts.CauseTask != "1.1" {
			t.Fatalf("passing contaminated assertion lost its policy distinction: %+v", later)
		}
		// Only this test restores its own source; the CLI must never do so.
		if data, _ := os.ReadFile(filepath.Join(root, "src.txt")); string(data) != "CONTAMINATED" {
			t.Fatal("CLI rolled back its source side effect")
		}
		if err := os.WriteFile(filepath.Join(root, "src.txt"), original, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(root, ".walden/mutate")); err != nil {
			t.Fatal(err)
		}
		status, code := integrityResult(t, root, "evidence", "status", "gate-demo")
		if code != 0 || status.Evidence[1].State != "failed" {
			t.Fatal("restored bytes revived the successor's unearned pass")
		}
		retried, code := integrityResult(t, root, "verify", "gate-demo")
		if code != 1 || len(retried.Evidence) != 2 || retried.Evidence[0].State != "verified" || retried.Evidence[1].State != "failed" || retried.Evidence[1].Execution.Facts.AssertionResult != "failed" {
			t.Fatalf("successor was skipped instead of failing on actual restored code: %+v", retried)
		}
		if _, code := integrityResult(t, root, "release", "check", "gate-demo"); code == 0 {
			t.Fatal("failed successor certified after restoration")
		}
	})
	t.Run("strict inputs and legacy assurance stay separate", func(t *testing.T) {
		root := releasableRepo(t)
		gitIn(t, root, "rm", "-r", "--cached", ".walden")
		writeFile(t, root, ".gitignore", ".walden/\nignored-scratch/\n")
		gitIn(t, root, "add", ".gitignore")
		gitIn(t, root, "commit", "-qm", "ignored local metadata")
		mustCLI(t, root, "verify", "gate-demo", "--all")
		mustCLI(t, root, "release", "check", "gate-demo")
		blocked, code := integrityResult(t, root, "release", "check", "gate-demo", "--strict")
		if code != 1 || blocked.Release.Worktree.InputBinding != "blocked" || !strings.Contains(strings.Join(blocked.Blockers, " "), ".walden/specs/gate-demo/requirements.md") {
			t.Fatalf("ignored inputs passed strict judgment: %+v", blocked)
		}
		gitIn(t, root, "add", "-f", ".walden")
		gitIn(t, root, "commit", "-qm", "same committed inputs")
		writeFile(t, root, "ignored-scratch/cache", "irrelevant")
		passed, code := integrityResult(t, root, "release", "check", "gate-demo", "--strict")
		if code != 0 || !passed.Release.Releasable || passed.Release.Worktree.InputBinding != "matched" || passed.CertifiedCommit == "" {
			t.Fatalf("committed positive control failed: %+v", passed)
		}
		feature, err := spec.LoadFeature(root, "gate-demo")
		if err != nil {
			t.Fatal(err)
		}
		tree, err := spec.ParseTaskTree(feature.Tasks)
		if err != nil {
			t.Fatal(err)
		}
		ledger, err := evidence.Load(root, "gate-demo")
		if err != nil {
			t.Fatal(err)
		}
		// Synthetic loss of old provenance: retain observed outcomes/bindings,
		// but do not pretend metadata translation attests historical purity.
		ledger.SchemaVersion = "v1alpha1"
		for _, task := range tree.LeafTasks() {
			record := ledger.Tasks[task.ID]
			record.Execution, record.TaskFingerprintScheme = nil, ""
			record.TaskFingerprint = spec.LegacyTaskDefinitionFingerprint(task)
			ledger.Tasks[task.ID] = record
		}
		data, _ := json.Marshal(ledger)
		if err := os.WriteFile(evidence.DocumentPath(root, "gate-demo"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		assessment, code := integrityResult(t, root, "adopt", "gate-demo")
		if code != 0 || assessment.Adoption == nil || len(assessment.Adoption.Features) != 1 || len(assessment.Adoption.Features[0].Evidence) != 2 {
			t.Fatalf("legacy assessment unavailable: %+v", assessment)
		}
		entry := assessment.Adoption.Features[0].Evidence[0]
		if code != 0 || entry.State != "unattested" || entry.Binding.State != "reconstructed-equivalent" || entry.CodeFreshness != "current" || entry.Execution.State != "legacy-unattested" {
			t.Fatalf("legacy binding recovery fabricated execution assurance: %+v", entry)
		}
		if _, code := integrityResult(t, root, "release", "check", "gate-demo"); code == 0 {
			t.Fatal("legacy unknowns certified even without requesting strict inputs")
		}
	})
}
