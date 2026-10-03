package adopt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/testutil"
	"github.com/andrearaponi/walden/internal/workflow"
)

func TestAdoptDiagnosticsOutcomes(t *testing.T) {
	for _, name := range []string{"exit", "output", "timeout", "start", "drift", "integrity", "timeout-mutation"} {
		t.Run(name, func(t *testing.T) {
			root := diagnosticsRepo(t)
			body := diagnosticsLeaf("1", true, []string{"proof"})
			diagnostic := "OUTPUT-FAILURE-SENTINEL"
			switch name {
			case "output":
				body += "      expect_output: EXPECTED-MARKER\n"
				diagnostic = "EXPECTED-MARKER"
			case "timeout", "timeout-mutation":
				body += "      timeout: 10ms\n"
				diagnostic = "exceeded timeout 10ms"
			case "start":
				diagnostic = "START-FAILURE-SENTINEL"
			case "drift":
				diagnostic = "environment drift: platform:"
			case "integrity":
				body += diagnosticsLeaf("2", true, []string{"proof"}) + diagnosticsLeaf("3", true, []string{"proof"})
				diagnostic = "verification policy:"
			}
			if name == "timeout-mutation" {
				body += diagnosticsLeaf("2", true, []string{"proof"})
			}
			diagnosticsFeature(t, root, "sample", body)
			if name == "drift" {
				if _, err := workflow.Verify(context.Background(), root, "sample", false, false, testutil.NewFakeRunner(testutil.Response{})); err != nil {
					t.Fatal(err)
				}
				ledger, err := evidence.Load(root, "sample")
				if err != nil {
					t.Fatal(err)
				}
				record := ledger.Tasks["1"]
				record.Profile = evidence.Profile{"platform": "old-platform"}
				record.Result, record.Execution.AssertionResult = evidence.ResultFailed, evidence.ResultFailed
				ledger.Tasks["1"] = record
				if err := evidence.Save(root, ledger); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			runner := diagnosticsRunnerFunc(func(ctx context.Context, command string, _ ...string) (shell.Response, error) {
				calls++
				if command != "proof" {
					t.Fatalf("unexpected diagnostic execution: %s", command)
				}
				switch name {
				case "output":
					return shell.Response{Stdout: "not the marker"}, nil
				case "start":
					return shell.Response{}, errors.New(diagnostic)
				case "timeout":
					<-ctx.Done()
					return shell.Response{}, nil
				case "integrity", "timeout-mutation":
					if (name == "integrity" && calls == 2) || (name == "timeout-mutation" && calls == 1) {
						if err := os.WriteFile(filepath.Join(root, "changed.txt"), []byte("mutation"), 0o644); err != nil {
							t.Fatal(err)
						}
						if name == "timeout-mutation" {
							<-ctx.Done()
						}
					}
					return shell.Response{}, nil
				default:
					return shell.Response{ExitCode: 3, Stderr: "OUTPUT-FAILURE-SENTINEL"}, nil
				}
			})
			report, err := Apply(context.Background(), root, "sample", runner, nil)
			if err != nil {
				t.Fatal(err)
			}
			feature := report.Features[0]
			if len(feature.Outcomes) != calls || calls == 0 || feature.EvidencePersisted == nil || !*feature.EvidencePersisted {
				t.Fatalf("missing actual outcomes/persistence: %+v", feature)
			}
			failed := feature.Outcomes[0]
			if name == "integrity" {
				if calls != 3 || !reflect.DeepEqual(feature.Verified, []string{"1"}) || !reflect.DeepEqual(feature.Failed, []string{"2", "3"}) {
					t.Fatalf("partition changed: %+v", feature)
				}
				prefix := feature.Outcomes[0]
				if !prefix.Passed || prefix.State != evidence.StateStaleCode {
					t.Fatalf("prefix facts changed: %+v", prefix)
				}
				failed = feature.Outcomes[1]
				for i, integrity := range []string{"mutated", "contaminated"} {
					entry := feature.Outcomes[i+1]
					facts := entry.Assessment.Execution.Facts
					if entry.Passed || facts.AssertionResult != "passed" || facts.Integrity != integrity || facts.CauseTask != "2" {
						t.Fatalf("lost assertion/policy facts: %+v", entry)
					}
				}
				if len(feature.Warnings) == 0 || !strings.Contains(feature.Warnings[0], "changed.txt") {
					t.Fatalf("lost verifier warnings: %+v", feature)
				}
			} else if name == "timeout-mutation" {
				if calls != 2 || len(feature.Failed) != 2 || !strings.Contains(failed.Failure, "changed.txt") {
					t.Fatalf("timeout/policy continuation lost: %+v", feature)
				}
			} else if calls != 1 || !reflect.DeepEqual(feature.Failed, []string{"1"}) || len(feature.Verified) != 0 {
				t.Fatalf("wrong execution count/partition: %+v", feature)
			}
			if !strings.Contains(failed.Failure, diagnostic) || failed.Passed || failed.State != evidence.StateFailed {
				t.Fatalf("diagnosis %q lost: %+v", diagnostic, failed)
			}
		})
	}
}

func TestAdoptDiagnosticsFeatureErrors(t *testing.T) {
	for _, failure := range []string{"seal", "entry", "save"} {
		t.Run(failure, func(t *testing.T) {
			root := diagnosticsRepo(t)
			if failure == "seal" {
				preFingerprintDocs(t, root, "a-bad")
			} else {
				diagnosticsFeature(t, root, "a-bad", diagnosticsLeaf("1", true, []string{"proof", "bad"}))
			}
			diagnosticsFeature(t, root, "z-good", diagnosticsLeaf("1", true, []string{"proof", "good"}))
			progress := []string{}
			calls := []string{}
			runner := diagnosticsRunnerFunc(func(_ context.Context, _ string, args ...string) (shell.Response, error) {
				calls = append(calls, args[0])
				if failure == "save" && args[0] == "bad" {
					if err := os.MkdirAll(evidence.DocumentPath(root, "a-bad"), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				return shell.Response{}, nil
			})
			report, err := Apply(context.Background(), root, "", runner, func(name string, _, _ int) {
				progress = append(progress, name)
				if name != "a-bad" {
					return
				}
				if failure == "seal" {
					path := filepath.Join(root, ".walden/specs/a-bad/design.md")
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(path, 0o755); err != nil {
						t.Fatal(err)
					}
				} else if failure == "entry" {
					path := filepath.Join(root, ".walden/specs/a-bad/tasks.md")
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, append(data, []byte("\nDrift.\n")...), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			})
			if err != nil || len(report.Features) != 2 {
				t.Fatalf("lost feature partition: %+v %v", report, err)
			}
			bad, good := report.Features[0], report.Features[1]
			if bad.Error == "" || len(bad.Verified) != 0 || len(bad.Failed) != 0 || report.Totals.Errors != 1 || report.Totals.Verified != 1 || !reflect.DeepEqual(good.Verified, []string{"1"}) {
				t.Fatalf("feature error changed accounting/continuation: %+v", report)
			}
			if !reflect.DeepEqual(progress, []string{"a-bad", "z-good"}) {
				t.Fatalf("feature order changed: %v", progress)
			}
			if failure == "save" {
				if len(bad.Outcomes) != 1 || !bad.Outcomes[0].Passed || bad.EvidencePersisted == nil || *bad.EvidencePersisted || !reflect.DeepEqual(calls, []string{"bad", "good"}) {
					t.Fatalf("late save failure lost its observed, unpersisted execution: %+v, calls=%v", bad, calls)
				}
			} else if len(bad.Outcomes) != 0 || bad.EvidencePersisted != nil || !reflect.DeepEqual(calls, []string{"good"}) {
				t.Fatalf("entry failure invented an execution: %+v, calls=%v", bad, calls)
			}
		})
	}
}
