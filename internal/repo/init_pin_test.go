package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const workflowTarget = ".github/workflows/validate-walden.yml"

func readWorkflow(t *testing.T, root string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, workflowTarget))
	if err != nil {
		t.Fatalf("read generated workflow: %v", err)
	}
	return string(content)
}

func TestInitPinsWorkflowToReleaseVersion(t *testing.T) {
	root := t.TempDir()

	if _, err := Init(root, "v0.7.1"); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	workflow := readWorkflow(t, root)
	if !strings.Contains(workflow, "cmd/walden@v0.7.1") {
		t.Fatalf("workflow does not pin the generating version:\n%s", workflow)
	}
	if strings.Contains(workflow, "{{WALDEN_VERSION}}") {
		t.Fatal("workflow still carries the unsubstituted version token")
	}
	if strings.Contains(workflow, "@latest") {
		t.Fatal("workflow installs @latest despite a release version")
	}
}

func TestInitDevVersionFallsBackToLatest(t *testing.T) {
	root := t.TempDir()

	if _, err := Init(root, "dev"); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	workflow := readWorkflow(t, root)
	if !strings.Contains(workflow, "cmd/walden@latest") {
		t.Fatalf("dev build did not fall back to @latest:\n%s", workflow)
	}
	if strings.Contains(workflow, "{{WALDEN_VERSION}}") {
		t.Fatal("workflow still carries the unsubstituted version token")
	}
}

func TestInitRefreshesPinOnRerunWithNewerBinary(t *testing.T) {
	root := t.TempDir()

	if _, err := Init(root, "v0.7.0"); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	report, err := Init(root, "v0.7.1")
	if err != nil {
		t.Fatalf("second Init: %v", err)
	}

	updated := false
	for _, path := range report.UpdatedFiles {
		if path == workflowTarget {
			updated = true
		}
	}
	if !updated {
		t.Fatalf("re-init did not report the workflow as updated: %+v", report)
	}
	if workflow := readWorkflow(t, root); !strings.Contains(workflow, "cmd/walden@v0.7.1") {
		t.Fatalf("workflow pin not refreshed:\n%s", workflow)
	}
}

func TestInitNonReleaseVersionsFallBackToLatest(t *testing.T) {
	// go install resolves release tags and pseudo-versions, never
	// git-describe strings — pinning one breaks every generated CI run.
	cases := []string{
		"v0.7.1-2-gc360063",
		"v0.7.2-0.20260711060210-abcdef123456",
		"(devel)",
		"v0.7",
		"dev",
		"",
	}

	for _, version := range cases {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			if _, err := Init(root, version); err != nil {
				t.Fatalf("Init returned error: %v", err)
			}
			workflow := readWorkflow(t, root)
			if !strings.Contains(workflow, "cmd/walden@latest") {
				t.Fatalf("version %q did not fall back to @latest:\n%s", version, workflow)
			}
		})
	}
}
