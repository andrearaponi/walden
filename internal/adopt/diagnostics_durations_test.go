package adopt

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/testutil"
	"github.com/andrearaponi/walden/internal/workflow"
)

func TestAdoptDiagnosticsDurations(t *testing.T) {
	t.Run("direct enclosing intervals include planning not nested sums", func(t *testing.T) {
		root := diagnosticsRepo(t)
		diagnosticsFeature(t, root, "sample", diagnosticsLeaf("1", true, []string{"proof"})+diagnosticsLeaf("2", true, []string{"proof"}))
		clock := time.Now()
		inner := gitRunner
		planCalls := 0
		gitRunner = diagnosticsRunnerFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
			planCalls++
			clock = clock.Add(10 * time.Millisecond)
			return inner.Run(ctx, name, args...)
		})
		t.Cleanup(func() { gitRunner = inner })
		starts, finishes := []string{}, []string{}
		calls := 0
		runner := diagnosticsRunnerFunc(func(context.Context, string, ...string) (shell.Response, error) {
			calls++
			clock = clock.Add(5 * time.Millisecond)
			return shell.Response{}, nil
		})
		report, err := ApplyWithOptions(context.Background(), root, "sample", runner, ApplyOptions{
			Now: func() time.Time { return clock },
			FeatureStarted: func(name string, index, total int) {
				if name != "sample" || index != 1 || total != 1 {
					t.Fatalf("feature progress: %s %d/%d", name, index, total)
				}
				clock = clock.Add(100 * time.Millisecond)
			},
			TaskStarted: func(feature string, event workflow.VerifyTaskStart) {
				starts = append(starts, feature+":"+event.TaskID)
				clock = clock.Add(30 * time.Millisecond)
			},
			TaskFinished: func(feature string, event workflow.VerifyTaskFinish) {
				finishes = append(finishes, feature+":"+event.TaskID)
				if event.Elapsed != 5*time.Millisecond {
					t.Fatalf("progress included non-proof time: %+v", event)
				}
				clock = clock.Add(40 * time.Millisecond)
			},
		})
		if err != nil || calls != 2 || planCalls != 2 {
			t.Fatalf("run failed or additional work: %v proof=%d plan=%d", err, calls, planCalls)
		}
		feature := report.Features[0]
		if report.Elapsed == nil || *report.Elapsed != 270*time.Millisecond || feature.Elapsed == nil || *feature.Elapsed != 250*time.Millisecond {
			t.Fatalf("intervals not measured directly: invocation=%v feature=%v", report.Elapsed, feature.Elapsed)
		}
		for _, outcome := range feature.Outcomes {
			if outcome.Elapsed == nil || *outcome.Elapsed != 5*time.Millisecond {
				t.Fatalf("wrong proof duration: %+v", outcome)
			}
		}
		if !reflect.DeepEqual(starts, []string{"sample:1", "sample:2"}) || !reflect.DeepEqual(starts, finishes) {
			t.Fatalf("wrong feature/task association: %v %v", starts, finishes)
		}
	})

	for _, kind := range []string{"zero-and-noop", "blocked", "seal", "entry", "save"} {
		t.Run(kind, func(t *testing.T) {
			root := diagnosticsRepo(t)
			if kind == "seal" {
				preFingerprintDocs(t, root, "sample")
			} else {
				diagnosticsFeature(t, root, "sample", diagnosticsLeaf("1", true, []string{"proof"}))
			}
			if kind == "blocked" {
				path := filepath.Join(root, ".walden/specs/sample/requirements.md")
				data, _ := os.ReadFile(path)
				if err := os.WriteFile(path, append(data, []byte("\nDrift.\n")...), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			clock := time.Now()
			events, calls := 0, 0
			options := ApplyOptions{
				Now: func() time.Time { return clock },
				FeatureStarted: func(_ string, _, _ int) {
					if kind == "zero-and-noop" {
						return
					}
					clock = clock.Add(100 * time.Millisecond)
					if kind == "seal" || kind == "entry" {
						name := "design.md"
						if kind == "entry" {
							name = "tasks.md"
						}
						path := filepath.Join(root, ".walden/specs/sample", name)
						if err := os.Remove(path); err != nil {
							t.Fatal(err)
						}
						if err := os.Mkdir(path, 0o755); err != nil {
							t.Fatal(err)
						}
					}
				},
				TaskStarted:  func(string, workflow.VerifyTaskStart) { events++ },
				TaskFinished: func(string, workflow.VerifyTaskFinish) { events++ },
			}
			runner := diagnosticsRunnerFunc(func(context.Context, string, ...string) (shell.Response, error) {
				calls++
				if kind == "save" {
					clock = clock.Add(5 * time.Millisecond)
					if err := os.MkdirAll(evidence.DocumentPath(root, "sample"), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				return shell.Response{}, nil
			})
			report, err := ApplyWithOptions(context.Background(), root, "sample", runner, options)
			if err != nil {
				t.Fatal(err)
			}
			feature := report.Features[0]
			if report.Elapsed == nil || feature.Elapsed == nil {
				t.Fatal("processing durations absent")
			}
			if kind == "zero-and-noop" {
				if calls != 1 || events != 2 || *report.Elapsed != 0 || *feature.Elapsed != 0 || feature.Outcomes[0].Elapsed == nil || *feature.Outcomes[0].Elapsed != 0 {
					t.Fatalf("measured zero treated as absent: %+v", report)
				}
				before, _ := os.ReadFile(evidence.DocumentPath(root, "sample"))
				events = 0
				second, err := ApplyWithOptions(context.Background(), root, "sample", testutil.NewFakeRunner(), options)
				after, _ := os.ReadFile(evidence.DocumentPath(root, "sample"))
				if err != nil || events != 0 || len(second.Features[0].Outcomes) != 0 || second.Features[0].EvidencePersisted != nil || string(before) != string(after) || second.Elapsed == nil || second.Features[0].Elapsed == nil {
					t.Fatalf("no-op invented execution or lost processing duration: %+v %v", second, err)
				}
			} else if kind == "save" {
				if calls != 1 || events != 2 || *report.Elapsed != 105*time.Millisecond || *feature.Elapsed != 105*time.Millisecond || *feature.Outcomes[0].Elapsed != 5*time.Millisecond || feature.Error == "" || feature.EvidencePersisted == nil || *feature.EvidencePersisted {
					t.Fatalf("late error lost measured, unpersisted outcomes: %+v", feature)
				}
			} else if calls != 0 || events != 0 || len(feature.Outcomes) != 0 || *report.Elapsed != 100*time.Millisecond || *feature.Elapsed != 100*time.Millisecond || feature.Error == "" {
				t.Fatalf("refused feature lost timing or invented attempts: %+v", report)
			}
		})
	}

	t.Run("fatal plan failure", func(t *testing.T) {
		clock := time.Now()
		report, err := ApplyWithOptions(context.Background(), t.TempDir(), "", testutil.NewFakeRunner(), ApplyOptions{
			Now: func() time.Time { clock = clock.Add(5 * time.Millisecond); return clock },
		})
		if err == nil || report.Elapsed == nil || *report.Elapsed != 5*time.Millisecond || report.Workload != nil || report.Scope.Kind != "" || len(report.Features) != 0 {
			t.Fatalf("fatal plan failure: %+v %v", report, err)
		}
	})
}
