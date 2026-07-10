package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

// scriptedIdentityRunner answers the git invocations behind the code
// identity with fixed responses.
type scriptedIdentityRunner struct {
	statusResponse shell.Response
	lsTreeResponse shell.Response
	fail           bool
}

func (r *scriptedIdentityRunner) Run(_ context.Context, _ string, args ...string) (shell.Response, error) {
	if r.fail {
		return shell.Response{}, errors.New("git unavailable")
	}
	if len(args) >= 3 && args[2] == "ls-tree" {
		return r.lsTreeResponse, nil
	}
	return r.statusResponse, nil
}

func overrideIdentityRunner(t *testing.T, runner shell.Runner) {
	t.Helper()
	previous := identityRunner
	identityRunner = runner
	t.Cleanup(func() { identityRunner = previous })
}

func writeEvidenceFixture(t *testing.T, root string) {
	t.Helper()
	writeFreshFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-21T14:00:00Z
last_modified: 2026-03-21T14:00:00Z
---

# Requirements Document
`)
	writeFreshFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-21T14:10:00Z
last_modified: 2026-03-21T14:10:00Z
source_requirements_approved_at: 2026-03-21T14:00:00Z
---

# Feature Design
`)
	writeFreshFeatureDoc(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-21T14:20:00Z
last_modified: 2026-03-21T14:20:00Z
source_design_approved_at: 2026-03-21T14:10:00Z
---

# Implementation Plan

- [ ] 1. Build parser
  - [ ] 1.1 Implement parser
    - Requirements: `+"`R1`"+`
    - Design: Task Store
    - Verification:
      - command: ["go", "test", "./internal/spec"]
`)
}

func TestCompleteTaskEvidenceRecordsChainAndIdentity(t *testing.T) {
	root := t.TempDir()
	writeEvidenceFixture(t, root)
	overrideIdentityRunner(t, &scriptedIdentityRunner{
		statusResponse: shell.Response{ExitCode: 0},
		lsTreeResponse: shell.Response{ExitCode: 0, Stdout: "100644 blob aaa\tmain.go\n"},
	})

	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	result, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.1", runner)
	if err != nil {
		t.Fatalf("CompleteTask returned error: %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", result.Warnings)
	}

	ledger, err := evidence.Load(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	record, exists := ledger.Tasks["1.1"]
	if !exists {
		t.Fatal("no evidence entry recorded for the completed task")
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load feature: %v", err)
	}
	if record.RequirementsFingerprint != feature.Requirements.Fields["approved_fingerprint"] || record.RequirementsFingerprint == "" {
		t.Fatalf("requirements fingerprint not bound: %q", record.RequirementsFingerprint)
	}
	if record.DesignFingerprint != feature.Design.Fields["approved_fingerprint"] || record.DesignFingerprint == "" {
		t.Fatalf("design fingerprint not bound: %q", record.DesignFingerprint)
	}
	if !strings.HasPrefix(record.CodeIdentity, "sha256:") {
		t.Fatalf("code identity not recorded: %q", record.CodeIdentity)
	}
	if !strings.HasPrefix(record.TaskFingerprint, "sha256:") {
		t.Fatalf("task fingerprint not recorded: %q", record.TaskFingerprint)
	}
	if record.Result != evidence.ResultPassed || len(record.Steps) != 1 {
		t.Fatalf("record outcome = %q with %d steps", record.Result, len(record.Steps))
	}
}

func TestCompleteTaskEvidenceSaveFailureLeavesCheckboxUntouched(t *testing.T) {
	root := t.TempDir()
	writeEvidenceFixture(t, root)
	overrideIdentityRunner(t, &scriptedIdentityRunner{fail: true})

	// A file where the evidence directory must go makes Save fail.
	if err := os.WriteFile(filepath.Join(root, ".walden", "evidence"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocking file: %v", err)
	}

	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	_, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.1", runner)
	if err == nil {
		t.Fatal("CompleteTask succeeded despite an unpersistable evidence record")
	}
	if !strings.Contains(err.Error(), "evidence") {
		t.Fatalf("error %q does not name the evidence failure", err)
	}

	tree, err := spec.LoadTaskTree(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load task tree: %v", err)
	}
	for _, task := range tree.LeafTasks() {
		if task.Completed {
			t.Fatalf("task %s checkbox mutated despite the evidence failure", task.ID)
		}
	}
}

func TestCompleteTaskEvidenceNoGitDegradesWithWarning(t *testing.T) {
	root := t.TempDir()
	writeEvidenceFixture(t, root)
	overrideIdentityRunner(t, &scriptedIdentityRunner{fail: true})

	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	result, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.1", runner)
	if err != nil {
		t.Fatalf("CompleteTask returned error: %v", err)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "identity") {
		t.Fatalf("expected an identity warning, got %v", result.Warnings)
	}

	ledger, err := evidence.Load(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	if ledger.Tasks["1.1"].CodeIdentity != "" {
		t.Fatalf("identity recorded despite git being unavailable: %q", ledger.Tasks["1.1"].CodeIdentity)
	}
	if ledger.Tasks["1.1"].Result != evidence.ResultPassed {
		t.Fatal("completion did not stay successful without git")
	}
}
