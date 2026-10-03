package adopt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
	"github.com/andrearaponi/walden/internal/workflow"
)

type diagnosticsRunnerFunc func(context.Context, string, ...string) (shell.Response, error)

func (f diagnosticsRunnerFunc) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	return f(ctx, name, args...)
}

func diagnosticsRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.name", "Diagnostics test")
	gitIn(t, root, "config", "user.email", "diagnostics@example.test")
	gitIn(t, root, "commit", "--allow-empty", "-qm", "fixture")
	return root
}

func diagnosticsFeature(t *testing.T, root, name, body string) {
	t.Helper()
	sealedDocs(t, root, name, true)
	feature, err := spec.LoadFeature(root, name)
	if err != nil {
		t.Fatal(err)
	}
	feature.Tasks.Body = "# Implementation Plan\n\n" + body
	feature.Tasks.Fields["approved_fingerprint"] = spec.Fingerprint(feature.Tasks.Path, feature.Tasks.Body)
	if err := spec.SaveDocument(feature.Tasks); err != nil {
		t.Fatal(err)
	}
}

func diagnosticsLeaf(id string, complete bool, commands ...[]string) string {
	marker := " "
	if complete {
		marker = "x"
	}
	indent := ""
	if strings.Contains(id, ".") {
		indent = "  "
	}
	body := fmt.Sprintf("%s- [%s] %s. Proof %s\n%s  - Requirements: `R1`\n%s  - Design: Architecture\n%s  - Verification:\n", indent, marker, id, id, indent, indent, indent)
	for _, command := range commands {
		argv, _ := json.Marshal(command)
		body += fmt.Sprintf("%s    - command: %s\n", indent, argv)
	}
	return body
}

func diagnosticsSnapshot(t *testing.T, root string) string {
	t.Helper()
	digest := sha256.New()
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		fmt.Fprintf(digest, "%s\x00%s\x00", relative, data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func TestAdoptDiagnosticsWorkload(t *testing.T) {
	root := diagnosticsRepo(t)
	command := []string{"true"}
	diagnosticsFeature(t, root, "two", "- [x] 1. Container\n"+
		diagnosticsLeaf("1.1", true, command, command)+diagnosticsLeaf("1.2", true, command, command, command)+diagnosticsLeaf("2", false, command))
	diagnosticsFeature(t, root, "legacy", "- [x] 1. Legacy\n  - Requirements: `R1`\n  - Design: Architecture\n  - Verification: true\n")
	diagnosticsFeature(t, root, "opaque", diagnosticsLeaf("1", true, []string{"sh", "-c", "false; walden verify must-not-run"}))
	diagnosticsFeature(t, root, "pending", diagnosticsLeaf("1", false, command))
	diagnosticsFeature(t, root, "verified", diagnosticsLeaf("1", true, command))
	if _, err := workflow.Verify(context.Background(), root, "verified", false, false, testutil.NewFakeRunner(testutil.Response{})); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"absent", "draft", "drifted", "malformed", "corrupt", "unreadable"} {
		diagnosticsFeature(t, root, name, diagnosticsLeaf("1", true, command))
	}
	if err := os.Remove(filepath.Join(root, ".walden/specs/absent/tasks.md")); err != nil {
		t.Fatal(err)
	}
	feature, err := spec.LoadFeature(root, "draft")
	if err != nil {
		t.Fatal(err)
	}
	feature.Tasks.Fields["status"] = "draft"
	if err := spec.SaveDocument(feature.Tasks); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".walden/specs/drifted/requirements.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("\nChanged after approval.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	diagnosticsFeature(t, root, "malformed", strings.Replace(diagnosticsLeaf("1", true, command), `["true"]`, `[not-json]`, 1))
	if err := os.WriteFile(evidence.DocumentPath(root, "corrupt"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(root, ".walden/specs/unreadable/requirements.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name         string
		tasks, steps int
		class        string
	}{
		{"two", 2, 5, ClassReprove}, {"legacy", 1, 1, ClassReprove}, {"opaque", 1, 1, ClassReprove},
		{"pending", 0, 0, ClassComplete}, {"verified", 0, 0, ClassComplete},
		{"absent", 0, 0, ClassComplete}, {"draft", 0, 0, ClassComplete},
		{"drifted", 0, 0, ClassBlocked}, {"malformed", 0, 0, ClassBlocked},
		{"corrupt", 0, 0, ClassBlocked}, {"unreadable", 0, 0, ClassBlocked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := Plan(context.Background(), root, tc.name)
			if err != nil || len(report.Features) != 1 {
				t.Fatalf("plan: %+v, %v", report, err)
			}
			feature := report.Features[0]
			work := feature.Workload
			if feature.Class != tc.class || feature.ReproveCount != tc.tasks {
				t.Fatalf("existing classification changed: %+v", feature)
			}
			if tc.class == ClassBlocked {
				if work.Available || work.Reason == "" || !reflect.DeepEqual(report.Workload.UnassessedFeatures, []string{tc.name}) {
					t.Fatalf("blocked work presented as known: %+v", report)
				}
			} else if !work.Available || work.Tasks != tc.tasks || work.Steps != tc.steps || work.Reason != "" {
				t.Fatalf("workload = %+v, want tasks=%d steps=%d", work, tc.tasks, tc.steps)
			}
		})
	}
	report, err := Plan(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Workload.AssessedTasks != 4 || report.Workload.AssessedSteps != 7 ||
		!reflect.DeepEqual(report.Workload.UnassessedFeatures, []string{"corrupt", "drifted", "malformed", "unreadable"}) {
		t.Fatalf("portfolio workload = %+v", report.Workload)
	}
}

func TestAdoptDiagnosticsPlanReadOnly(t *testing.T) {
	root := diagnosticsRepo(t)
	proofSentinel := filepath.Join(root, "proof-ran")
	probeSentinel := filepath.Join(root, "probe-ran")
	diagnosticsFeature(t, root, "one", diagnosticsLeaf("1", true, []string{"sh", "-c", `printf proof > "$1"`, "sh", proofSentinel}))
	diagnosticsFeature(t, root, "two", diagnosticsLeaf("1", false, []string{"true"}))
	probe, _ := json.Marshal([]string{"sh", "-c", `printf probe > "$1"`, "sh", probeSentinel})
	if err := os.WriteFile(filepath.Join(root, ".walden/environment.md"), []byte("# Environment\n- sentinel: "+string(probe)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inner := gitRunner
	var calls [][]string
	gitRunner = diagnosticsRunnerFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		calls = append(calls, append([]string{name}, args...))
		return inner.Run(ctx, name, args...)
	})
	t.Cleanup(func() { gitRunner = inner })

	before := diagnosticsSnapshot(t, root)
	first, err := Plan(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	firstCalls := append([][]string{}, calls...)
	calls = nil
	second, err := Plan(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || before != diagnosticsSnapshot(t, root) {
		t.Fatal("planning changed its result or repository bytes")
	}
	// These unrecorded fixtures need only the existing single scope identity:
	// one status and one ls-tree. Step accounting must not rescan per task.
	if len(firstCalls) != 2 || !reflect.DeepEqual(calls, firstCalls) {
		t.Fatalf("additional Git/history work: %v / %v", firstCalls, calls)
	}
	for _, path := range []string{proofSentinel, probeSentinel} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("planning executed %s: %v", path, err)
		}
	}
}
