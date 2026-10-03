package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
)

type adoptDiagnosticRunnerFunc func(context.Context, string, ...string) (shell.Response, error)

func (f adoptDiagnosticRunnerFunc) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	return f(ctx, name, args...)
}

func adoptDiagnosticOverrideRunner(t *testing.T, runner shell.Runner) {
	t.Helper()
	previous := commandRunner
	commandRunner = runner
	t.Cleanup(func() { commandRunner = previous })
}

func adoptDiagnosticTasks(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, ".walden/specs", name, "tasks.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	end := strings.Index(text, "---\n\n")
	if end < 0 {
		t.Fatal("fixture frontmatter terminator missing")
	}
	writeRawFeatureFile(t, root, name, "tasks.md", text[:end+5]+"# Implementation Plan\n\n"+body)
}

func adoptDiagnosticTask(id string, attributes string) string {
	return fmt.Sprintf("- [x] %s. Proof %s\n  - Requirements: `R1`\n  - Design: Architecture\n  - Verification:\n    - command: [\"true\"]\n%s", id, id, attributes)
}

func adoptDiagnosticEvidence(t *testing.T, feature map[string]any) []map[string]any {
	t.Helper()
	values, ok := feature["evidence"].([]any)
	if !ok || len(values) == 0 {
		t.Fatalf("executed outcomes missing: %+v", feature)
	}
	entries := make([]map[string]any, 0, len(values))
	for _, value := range values {
		entries = append(entries, adoptDiagnosticObject(t, value))
	}
	return entries
}

func TestAdoptDiagnosticsOutcomeOutput(t *testing.T) {
	cases := []struct {
		name, attributes, diagnostic string
	}{
		{"exit", "", "DIAGNOSTIC-EXIT-SENTINEL"},
		{"output", "      expect_output: required-marker\n", "required-marker"},
		{"timeout", "      timeout: 10ms\n", "exceeded timeout 10ms"},
		{"start", "", "DIAGNOSTIC-START-SENTINEL"},
		{"integrity", "", "verification policy"},
		{"timeout-mutation", "      timeout: 10ms\n", "exceeded timeout 10ms"},
		{"save", "", "DIAGNOSTIC-EXIT-SENTINEL"},
		{"entry", "", "malformed probe"},
	}
	for _, tc := range cases {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", tc.name, jsonMode), func(t *testing.T) {
				root := adoptFixture(t)
				body := adoptDiagnosticTask("1", tc.attributes)
				if tc.name == "integrity" {
					body += adoptDiagnosticTask("2", "") + adoptDiagnosticTask("3", "")
				}
				if tc.name == "timeout-mutation" {
					body += adoptDiagnosticTask("2", "")
				}
				adoptDiagnosticTasks(t, root, "old-era", body)
				if tc.name == "entry" {
					if err := os.WriteFile(filepath.Join(root, ".walden/environment.md"), []byte("- malformed\n"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				calls := 0
				adoptDiagnosticOverrideRunner(t, adoptDiagnosticRunnerFunc(func(ctx context.Context, _ string, _ ...string) (shell.Response, error) {
					calls++
					switch tc.name {
					case "integrity", "timeout-mutation":
						if (tc.name == "integrity" && calls == 2) || (tc.name == "timeout-mutation" && calls == 1) {
							if err := os.WriteFile(filepath.Join(root, "mutation.txt"), []byte("changed"), 0o644); err != nil {
								t.Fatal(err)
							}
							if tc.name == "timeout-mutation" {
								<-ctx.Done()
							}
						}
						return shell.Response{}, nil
					case "save":
						if err := os.MkdirAll(filepath.Join(root, ".walden/evidence/old-era.json"), 0o755); err != nil {
							t.Fatal(err)
						}
						fallthrough
					case "exit":
						return shell.Response{ExitCode: 7, Stderr: "DIAGNOSTIC-EXIT-SENTINEL"}, nil
					case "start":
						return shell.Response{}, errors.New("DIAGNOSTIC-START-SENTINEL")
					case "timeout":
						<-ctx.Done()
						return shell.Response{}, nil
					default:
						return shell.Response{Stdout: "not the required output"}, nil
					}
				}))
				args := []string{"adopt", "old-era", "--apply"}
				if jsonMode {
					args = append(args, "--json")
				}
				stdout, stderr, code := adoptDiagnosticRun(args...)
				if code != 1 {
					t.Fatalf("failure exit=%d: %s %s", code, stdout, stderr)
				}
				wantCalls := 1
				if tc.name == "entry" {
					wantCalls = 0
				} else if tc.name == "integrity" {
					wantCalls = 3
				} else if tc.name == "timeout-mutation" {
					wantCalls = 2
				}
				if calls != wantCalls {
					t.Fatalf("diagnostic replay or lost execution: calls=%d want=%d", calls, wantCalls)
				}
				if !jsonMode {
					if !strings.Contains(stderr, tc.diagnostic) || !strings.Contains(stderr, "old-era") {
						t.Fatalf("lost feature diagnosis %q:\n%s", tc.diagnostic, stderr)
					}
					if tc.name == "integrity" {
						for _, fragment := range []string{"stale-code", "passed=true", "assertion=passed", "integrity=mutated", "integrity=contaminated", "mutation.txt"} {
							if !strings.Contains(stderr, fragment) {
								t.Errorf("missing %q:\n%s", fragment, stderr)
							}
						}
					}
					if tc.name == "save" && !strings.Contains(stderr, "evidence not persisted") {
						t.Fatalf("unpersisted execution presented as durable: %s", stderr)
					}
					return
				}
				if stderr != "" {
					t.Fatalf("unexpected JSON stderr: %s", stderr)
				}
				result := adoptDiagnosticObject(t, adoptDiagnosticJSON(t, stdout)["result"])
				adoption := adoptDiagnosticObject(t, result["adoption"])
				feature := adoptDiagnosticFeatures(t, adoption)["old-era"]
				if tc.name == "entry" {
					if !strings.Contains(fmt.Sprint(feature["reason"]), tc.diagnostic) || feature["evidence"] != nil || feature["failed"] != nil || feature["evidence_persisted"] != nil {
						t.Fatalf("entry error invented outcomes or lost reason: %+v", feature)
					}
					return
				}
				entries := adoptDiagnosticEvidence(t, feature)
				if len(entries) != wantCalls {
					t.Fatalf("outcomes=%d want=%d", len(entries), wantCalls)
				}
				failed := entries[0]
				if tc.name == "integrity" {
					if entries[0]["passed"] != true || entries[0]["state"] != "stale-code" {
						t.Fatalf("prefix acceptance conflated with freshness: %+v", entries[0])
					}
					failed = entries[1]
					for i, integrity := range []string{"mutated", "contaminated"} {
						entry := entries[i+1]
						facts := adoptDiagnosticObject(t, adoptDiagnosticObject(t, entry["execution"])["facts"])
						if entry["passed"] != false || facts["assertion_result"] != "passed" || facts["integrity"] != integrity || facts["cause_task"] != "2" {
							t.Fatalf("policy facts lost: %+v", entry)
						}
					}
					if !strings.Contains(fmt.Sprint(result["warnings"]), "old-era:") {
						t.Fatalf("feature-qualified warnings missing: %+v", result)
					}
				}
				if !strings.Contains(fmt.Sprint(failed["failure"]), tc.diagnostic) {
					t.Fatalf("missing failure diagnosis: %+v", failed)
				}
				if feature["evidence_persisted"] != (tc.name != "save") {
					t.Fatalf("incorrect persistence observation: %+v", feature)
				}
				if tc.name == "save" && (!strings.Contains(fmt.Sprint(feature["reason"]), "persist refreshed evidence") || feature["failed"] != nil || feature["verified"] != nil) {
					t.Fatalf("late write error changed legacy accounting: %+v", feature)
				}
			})
		}
	}
}
