package evidence

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
)

const historyPlan = `# Implementation Plan

- [x] 1. Proof
  - Requirements: ` + "`R1.AC1`" + `
  - Design: A
  - Verification:
    - command: ["printf", "PASS"]
      expect_output: "PASS"
      covers: ["R1.AC1"]
- [ ] 2. Old sibling
  - Requirements: ` + "`R1.AC1`" + `
  - Design: A
  - Verification:
    - command: ["true"]
`

func historyGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, data)
	}
	return strings.TrimSpace(string(data))
}

type historyCountingRunner struct{ calls []string }

func (r *historyCountingRunner) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	return shell.NewExecRunner().Run(ctx, name, args...)
}

func historyFixture(t *testing.T, body string) (string, spec.Feature, spec.TaskTree, Document) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("Git is required by this acceptance fixture")
	}
	root := t.TempDir()
	path := filepath.Join(root, ".walden", "specs", "f", "tasks.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	// Historical frontmatter is context, not a new approval. The body alone
	// is the recorded fingerprint witness, including old extension names.
	if err := os.WriteFile(path, []byte("---\nstatus: done\ncompleted_at: historical\n---\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	historyGit(t, root, "init", "-q", "-b", "main")
	historyGit(t, root, "config", "user.email", "fixture@walden.test")
	historyGit(t, root, "config", "user.name", "Fixture")
	historyGit(t, root, "add", ".")
	historyGit(t, root, "commit", "-qm", "historical plan")
	currentBody := strings.Replace(historyPlan, "Old sibling", "Current sibling", 1)
	feature := spec.Feature{Name: "f", Tasks: spec.Document{Exists: true, Path: path, Body: currentBody},
		Requirements: spec.Document{ApprovedFingerprint: "sha256:req"}, Design: spec.Document{ApprovedFingerprint: "sha256:des"}}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err != nil {
		t.Fatal(err)
	}
	oldTree, err := spec.ParseTaskTree(spec.Document{Exists: true, Path: path, Body: historyPlan})
	if err != nil {
		t.Fatal(err)
	}
	record := Record{
		TaskFingerprint:         spec.LegacyTaskDefinitionFingerprint(oldTree.LeafTasks()[0]),
		RequirementsFingerprint: "sha256:req", DesignFingerprint: "sha256:des",
		TasksFingerprint: spec.Fingerprint(path, body), CodeIdentity: "code", Result: ResultPassed,
		Steps: []StepResult{{Command: []string{"printf", "PASS"}, ExpectedExit: 0, ActualExit: 0}},
	}
	return root, feature, tree, Document{Feature: "f", Tasks: map[string]Record{"1": record}}
}

func TestEvidenceIntegrityLegacyBindingRecovery(t *testing.T) {
	t.Run("historical sibling plan and per-invocation reuse", func(t *testing.T) {
		root, feature, tree, ledger := historyFixture(t, historyPlan)
		current, leafs := FeatureInputs(feature, tree)
		runner := &historyCountingRunner{}
		ResolvePlans(context.Background(), runner, root, feature, ledger, &current, "")
		entry := Derive(ledger, current, "other-code", true, leafs)[0]
		if entry.Binding.State != "reconstructed-equivalent" || entry.Binding.Commit == "" || entry.CodeFreshness != "stale" || entry.Execution.State != "legacy-unattested" || entry.State != "stale-code" {
			t.Fatalf("independent historical binding/freshness/provenance facts were lost: %+v", entry)
		}
		calls := len(runner.calls)
		ResolvePlans(context.Background(), runner, root, feature, ledger, &current, "")
		if len(runner.calls) != calls {
			t.Fatal("resolved witness was read again")
		}
		if entry := Derive(ledger, current, "code", true, leafs)[0]; entry.State != "unattested" {
			t.Fatal("binding recovery fabricated purity")
		}
	})
	t.Run("changed omitted assertion is not equivalent", func(t *testing.T) {
		root, feature, _, ledger := historyFixture(t, historyPlan)
		feature.Tasks.Body = strings.Replace(feature.Tasks.Body, `expect_output: "PASS"`, `expect_output: "MISSING"`, 1)
		tree, err := spec.ParseTaskTree(feature.Tasks)
		if err != nil {
			t.Fatal(err)
		}
		current, leafs := FeatureInputs(feature, tree)
		ResolvePlans(context.Background(), shell.NewExecRunner(), root, feature, ledger, &current, "")
		if entry := Derive(ledger, current, "code", true, leafs)[0]; entry.State != "stale-spec" || entry.Binding.State != "changed" {
			t.Fatalf("changed assertion reused a legacy binding: %+v", entry)
		}
	})
	t.Run("current witness needs no Git and detects contradictions", func(t *testing.T) {
		_, feature, _, ledger := historyFixture(t, historyPlan)
		feature.Tasks.Body = historyPlan
		tree, err := spec.ParseTaskTree(feature.Tasks)
		if err != nil {
			t.Fatal(err)
		}
		current, leafs := FeatureInputs(feature, tree)
		runner := &historyCountingRunner{}
		ResolvePlans(context.Background(), runner, t.TempDir(), feature, ledger, &current, "")
		if len(runner.calls) != 0 || Derive(ledger, current, "code", true, leafs)[0].Binding.State != "reconstructed-equivalent" {
			t.Fatal("current-body recovery executed Git or failed")
		}
		record := ledger.Tasks["1"]
		record.Steps[0].Command = []string{"echo", "fabricated"}
		ledger.Tasks["1"] = record
		if Derive(ledger, current, "code", true, leafs)[0].Binding.State != "contradictory" {
			t.Fatal("contradictory recorded argv accepted")
		}
		record.Result, record.Steps = ResultFailed, nil
		ledger.Tasks["1"] = record
		entry := Derive(ledger, current, "code", true, leafs)[0]
		if entry.Binding.State != "reconstructed-equivalent" || entry.State != "failed" {
			t.Fatalf("failed prefix was lost or upgraded: %+v", entry)
		}
	})
	t.Run("shallow history cannot supply an older plan", func(t *testing.T) {
		root, feature, tree, ledger := historyFixture(t, historyPlan)
		if err := os.WriteFile(feature.Tasks.Path, []byte("---\nstatus: approved\n---\n"+feature.Tasks.Body), 0o644); err != nil {
			t.Fatal(err)
		}
		historyGit(t, root, "add", ".")
		historyGit(t, root, "commit", "-qm", "current plan")
		clone := filepath.Join(t.TempDir(), "shallow")
		historyGit(t, root, "clone", "-q", "--depth=1", "file://"+filepath.ToSlash(root), clone)
		feature.Tasks.Path = filepath.Join(clone, ".walden", "specs", "f", "tasks.md")
		current, leafs := FeatureInputs(feature, tree)
		ResolvePlans(context.Background(), shell.NewExecRunner(), clone, feature, ledger, &current, "")
		entry := Derive(ledger, current, "code", true, leafs)[0]
		if entry.Binding.State != "unknown" || entry.State != "unattested" {
			t.Fatalf("shallow history fabricated a witness: %+v", entry)
		}
	})
	t.Run("absent hashes and cancellation remain explicit", func(t *testing.T) {
		root, feature, tree, ledger := historyFixture(t, historyPlan)
		current, leafs := FeatureInputs(feature, tree)
		record := ledger.Tasks["1"]
		record.TasksFingerprint = ""
		ledger.Tasks["1"] = record
		runner := &historyCountingRunner{}
		ResolvePlans(context.Background(), runner, root, feature, ledger, &current, "")
		entry := Derive(ledger, current, "code", true, leafs)[0]
		if len(runner.calls) != 0 || entry.Binding.State != "unknown" || !strings.Contains(entry.Binding.Detail, "missing") {
			t.Fatalf("absent hash was guessed: %+v", entry)
		}
		current, leafs = FeatureInputs(feature, tree)
		record.TasksFingerprint = spec.Fingerprint("tasks.md", historyPlan)
		ledger.Tasks["1"] = record
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ResolvePlans(ctx, runner, root, feature, ledger, &current, "")
		entry = Derive(ledger, current, "code", true, leafs)[0]
		if entry.State != "unattested" || entry.Binding.State != "unknown" {
			t.Fatalf("canceled lookup supplied a witness: %+v", entry)
		}
	})
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing snapshot", true: "unsupported historical syntax"}[invalid], func(t *testing.T) {
			body := historyPlan
			if invalid {
				body = strings.Replace(body, `["printf", "PASS"]`, `["bad\|escape"]`, 1)
			}
			root, feature, tree, ledger := historyFixture(t, body)
			if !invalid {
				record := ledger.Tasks["1"]
				record.TasksFingerprint = "sha256:" + strings.Repeat("a", 64)
				ledger.Tasks["1"] = record
			}
			current, leafs := FeatureInputs(feature, tree)
			runner := &historyCountingRunner{}
			ResolvePlans(context.Background(), runner, root, feature, ledger, &current, "")
			entry := Derive(ledger, current, "code", true, leafs)[0]
			if entry.Binding.State != "unknown" || entry.State != "unattested" {
				t.Fatalf("missing/unsupported witness granted a guarantee: %+v", entry)
			}
			if invalid && !strings.Contains(entry.Binding.Detail, "parse") {
				t.Fatalf("syntax gap not identified: %+v", entry.Binding)
			}
			calls := len(runner.calls)
			ResolvePlans(context.Background(), runner, root, feature, ledger, &current, "")
			if calls != len(runner.calls) {
				t.Fatal("missing witness lookup was repeated")
			}
		})
	}
}
