package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestCommandFlagUniformity proves NFR1 by iterating the command registry:
// every leaf answers --help on stdout with exit 0 (R1.AC1, R1.AC2), rejects
// an undeclared flag with a non-zero exit naming the flag (R2.AC1, R2.AC2,
// R2.AC3), and honors the JSON envelope on rejection when --json is present
// (R2.AC4). New commands inherit this coverage by registration alone.
func TestCommandFlagUniformity(t *testing.T) {
	// Run in an empty directory: flag triage must resolve before any
	// repository state is touched, so no command should need a real repo.
	t.Chdir(t.TempDir())

	for _, spec := range commandRegistry {
		for _, leaf := range spec.leaves() {
			pathArgs := strings.Fields(leaf.Path)

			t.Run(leaf.Path+"/help", func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				exitCode := Run(append(append([]string{}, pathArgs...), "--help"), &stdout, &stderr)
				if exitCode != 0 {
					t.Fatalf("%s --help exited %d, stderr: %s", leaf.Path, exitCode, stderr.String())
				}
				if !strings.Contains(stdout.String(), leaf.Syntax) {
					t.Fatalf("%s --help stdout %q does not contain syntax %q", leaf.Path, stdout.String(), leaf.Syntax)
				}
			})

			t.Run(leaf.Path+"/unknown-flag", func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				exitCode := Run(append(append([]string{}, pathArgs...), "--bogus-flag-xyz"), &stdout, &stderr)
				if exitCode == 0 {
					t.Fatalf("%s accepted --bogus-flag-xyz", leaf.Path)
				}
				combined := stdout.String() + stderr.String()
				if !strings.Contains(combined, "--bogus-flag-xyz") {
					t.Fatalf("%s rejection %q does not name the flag", leaf.Path, combined)
				}
				if !strings.Contains(combined, "--help") {
					t.Fatalf("%s rejection %q does not point at --help", leaf.Path, combined)
				}
			})

			t.Run(leaf.Path+"/unknown-flag-json", func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				exitCode := Run(append(append([]string{}, pathArgs...), "--bogus-flag-xyz", "--json"), &stdout, &stderr)
				if exitCode == 0 {
					t.Fatalf("%s accepted --bogus-flag-xyz with --json", leaf.Path)
				}
				var envelope struct {
					SchemaVersion string `json:"schema_version"`
					Command       string `json:"command"`
					OK            bool   `json:"ok"`
				}
				if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
					t.Fatalf("%s JSON rejection is not a valid envelope: %v\nstdout: %s", leaf.Path, err, stdout.String())
				}
				if envelope.OK {
					t.Fatalf("%s JSON rejection reports ok=true", leaf.Path)
				}
				if envelope.SchemaVersion == "" || envelope.Command == "" {
					t.Fatalf("%s JSON rejection missing envelope fields: %s", leaf.Path, stdout.String())
				}
			})
		}
	}
}

// TestGroupDispatcherHelp proves group commands answer --help with their
// subcommand list (R1.AC1, R1.AC2 at the group level).
func TestGroupDispatcherHelp(t *testing.T) {
	t.Chdir(t.TempDir())

	for _, spec := range commandRegistry {
		if len(spec.Subcommands) == 0 {
			continue
		}
		t.Run(spec.Path, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exitCode := Run([]string{spec.Path, "--help"}, &stdout, &stderr)
			if exitCode != 0 {
				t.Fatalf("%s --help exited %d, stderr: %s", spec.Path, exitCode, stderr.String())
			}
			for _, sub := range spec.Subcommands {
				if !strings.Contains(stdout.String(), sub.Path) {
					t.Fatalf("%s --help does not list subcommand %q:\n%s", spec.Path, sub.Path, stdout.String())
				}
			}
		})
	}
}
