package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

// chdirContract moves the test into an isolated directory, mirroring the
// e2e-test convention, so filesystem-dependent commands see a clean root.
func chdirContract(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	if err := os.Chdir(root); err != nil {
		t.Fatalf("enter temp root: %v", err)
	}
	return root
}

// TestJSONErrorContract asserts the envelope contract for one error-inducing
// invocation of every --json command: parseable envelope on stdout, ok=false,
// the command's canonical name, a non-empty summary, a non-zero exit code,
// and nothing on stderr. Two commands are absent by design: `version` has no
// error path at all, and `skill status` fails only when the working directory
// cannot be resolved, which is not portably inducible (its error path routes
// through the same unit-tested renderer as every other command).
func TestJSONErrorContract(t *testing.T) {
	cases := []struct {
		name          string
		args          []string
		wantCommand   string
		wantInSummary string
		setup         func(t *testing.T, root string)
	}{
		{
			name:        "repo init on unusable root",
			args:        []string{"repo", "init", "--json"},
			wantCommand: "repo-init",
			setup: func(t *testing.T, root string) {
				// A file where .walden must go makes bootstrap fail.
				if err := os.WriteFile(filepath.Join(root, ".walden"), []byte("x"), 0o644); err != nil {
					t.Fatalf("seed blocking file: %v", err)
				}
			},
		},
		{
			name:        "feature init without a name",
			args:        []string{"feature", "init", "--json"},
			wantCommand: "feature-init",
		},
		{
			name:        "status with invalid feature name",
			args:        []string{"status", "!!!", "--json"},
			wantCommand: "status",
		},
		{
			name:        "validate without a feature",
			args:        []string{"validate", "--json"},
			wantCommand: "validate",
		},
		{
			name:        "reconcile with invalid feature name",
			args:        []string{"reconcile", "!!!", "--json"},
			wantCommand: "reconcile",
		},
		{
			name:        "lesson log without required flags",
			args:        []string{"lesson", "log", "--json"},
			wantCommand: "lesson-log",
		},
		{
			name:        "task status with invalid feature name",
			args:        []string{"task", "status", "!!!", "--json"},
			wantCommand: "task-status",
		},
		{
			name:        "task start with invalid feature name",
			args:        []string{"task", "start", "!!!", "--json"},
			wantCommand: "task-start",
		},
		{
			name:        "task complete with invalid feature name",
			args:        []string{"task", "complete", "!!!", "1.1", "--json"},
			wantCommand: "task-complete",
		},
		{
			name:        "task complete-all with invalid feature name",
			args:        []string{"task", "complete-all", "!!!", "--json"},
			wantCommand: "task-complete-all",
		},
		{
			name:        "review open without --phase",
			args:        []string{"review", "open", "demo", "--json"},
			wantCommand: "review-open",
		},
		{
			name:          "review approve blocked by workflow state",
			args:          []string{"review", "approve", "demo", "--phase", "requirements", "--json"},
			wantCommand:   "review-approve",
			wantInSummary: "requirements",
			setup: func(t *testing.T, root string) {
				var stdout, stderr bytes.Buffer
				if code := Run([]string{"repo", "init"}, &stdout, &stderr); code != 0 {
					t.Fatalf("repo init failed: %s", stderr.String())
				}
				if code := Run([]string{"feature", "init", "demo"}, &stdout, &stderr); code != 0 {
					t.Fatalf("feature init failed: %s", stderr.String())
				}
			},
		},
		{
			name:        "update refused on a dev build",
			args:        []string{"update", "--json"},
			wantCommand: "update",
			setup: func(t *testing.T, _ string) {
				restoreVersion := Version
				restoreSeam := buildInfoVersion
				t.Cleanup(func() {
					Version = restoreVersion
					buildInfoVersion = restoreSeam
				})
				Version = "dev"
				buildInfoVersion = func() string { return "(devel)" }
			},
		},
		{
			name:        "skill install with unknown agent",
			args:        []string{"skill", "install", "nonexistent-agent", "--json"},
			wantCommand: "skill-install",
		},
		{
			name:        "skill uninstall with invalid flag combination",
			args:        []string{"skill", "uninstall", "--all", "--project", "--json"},
			wantCommand: "skill-uninstall",
		},
		{
			name:        "verify with invalid feature name",
			args:        []string{"verify", "!!!", "--json"},
			wantCommand: "verify",
		},
		{
			name:        "evidence status with invalid feature name",
			args:        []string{"evidence", "status", "!!!", "--json"},
			wantCommand: "evidence-status",
		},
		{
			name:        "release check with invalid feature name",
			args:        []string{"release", "check", "!!!", "--json"},
			wantCommand: "release-check",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := chdirContract(t)
			if tc.setup != nil {
				tc.setup(t, root)
			}

			var stdout, stderr bytes.Buffer
			exitCode := Run(tc.args, &stdout, &stderr)

			if exitCode == 0 {
				t.Fatalf("expected non-zero exit code, got 0 (stdout: %s)", stdout.String())
			}

			var envelope output.Envelope
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("stdout is not a JSON envelope: %v (stdout %q, stderr %q)", err, stdout.String(), stderr.String())
			}
			if envelope.SchemaVersion != "v0beta1" {
				t.Fatalf("schema_version = %q, want v0beta1", envelope.SchemaVersion)
			}
			if envelope.Command != tc.wantCommand {
				t.Fatalf("command = %q, want %q", envelope.Command, tc.wantCommand)
			}
			if envelope.OK {
				t.Fatal("ok = true for a failing invocation")
			}
			if strings.TrimSpace(envelope.Result.Summary) == "" {
				t.Fatal("summary is empty")
			}
			if tc.wantInSummary != "" && !strings.Contains(envelope.Result.Summary, tc.wantInSummary) {
				t.Fatalf("summary %q does not carry the blocking cause %q", envelope.Result.Summary, tc.wantInSummary)
			}
			if envelope.Result.ExitCode == 0 {
				t.Fatal("result.exit_code = 0 for a failing invocation")
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr not empty in JSON mode: %q", stderr.String())
			}
		})
	}
}

func TestJSONErrorContractTextMode(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantOnErr string
	}{
		{"invalid feature name", []string{"status", "!!!"}, "feature name cannot be empty"},
		{"missing review phase", []string{"review", "open", "demo"}, "--phase"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chdirContract(t)

			var stdout, stderr bytes.Buffer
			exitCode := Run(tc.args, &stdout, &stderr)

			if exitCode == 0 {
				t.Fatal("expected non-zero exit code")
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout not empty for a text-mode error: %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.wantOnErr) {
				t.Fatalf("stderr %q does not carry %q", stderr.String(), tc.wantOnErr)
			}
			if strings.Contains(stderr.String(), "\"schema_version\"") {
				t.Fatalf("text mode leaked a JSON envelope: %q", stderr.String())
			}
		})
	}
}

func TestJSONErrorContractUnknownCommandKeepsUsage(t *testing.T) {
	cases := [][]string{
		{"frobnicate", "--json"},
		{"repo", "frobnicate", "--json"},
	}

	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			chdirContract(t)

			var stdout, stderr bytes.Buffer
			exitCode := Run(args, &stdout, &stderr)

			if exitCode == 0 {
				t.Fatal("expected non-zero exit code")
			}
			if !strings.Contains(stderr.String(), "unknown command") || !strings.Contains(stderr.String(), "Usage:") {
				t.Fatalf("stderr %q does not carry usage guidance", stderr.String())
			}
		})
	}
}
