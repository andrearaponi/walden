package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

// Checkbox mutation by task completion is the one legitimate writer of an
// approved tasks.md: under the scoped normalization it must stay exempt, or
// the workflow would stale itself on every completed task.
func TestScopedNormalizationKeepsCompletedTasksFresh(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))

	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	if _, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.1", runner); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	feature, err := spec.LoadFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("LoadFeature: %v", err)
	}
	if !strings.Contains(feature.Tasks.Body, "- [x] 1.1") {
		t.Fatal("completion did not mutate the checkbox — the exemption is not exercised")
	}

	state := ResolveFeatureState(feature)
	if !state.Tasks.Fresh {
		t.Fatal("checkbox mutation by task completion staled the approved tasks.md")
	}
}
