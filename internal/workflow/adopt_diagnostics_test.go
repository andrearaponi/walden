package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

func TestAdoptDiagnosticsObservation(t *testing.T) {
	t.Run("boundaries order zero and default compatibility", func(t *testing.T) {
		root := t.TempDir()
		writeVerifyFixture(t, root)
		baseIdentity := identityYielding("100644 blob aaa\tmain.go\n")
		overrideIdentityRunner(t, baseIdentity)
		completeBoth(t, root)
		clock := time.Now()
		gitCalls, probeCalls := 0, 0
		overrideIdentityRunner(t, integrityProofFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
			gitCalls++
			clock = clock.Add(700 * time.Millisecond)
			return baseIdentity.Run(ctx, name, args...)
		}))
		writeProbes(t, root, "- marker: [\"probe\"]\n")
		overrideProbeRunner(t, probeFunc(func(context.Context, string, ...string) (shell.Response, error) {
			probeCalls++
			clock = clock.Add(3 * time.Second)
			return shell.Response{Stdout: "probe-v1"}, nil
		}))
		events := []string{}
		calls := 0
		durations := []time.Duration{125 * time.Millisecond, 0}
		runner := integrityProofFunc(func(context.Context, string, ...string) (shell.Response, error) {
			id := fmt.Sprintf("1.%d", calls+1)
			if len(events) == 0 || events[len(events)-1] != "start "+id {
				t.Fatalf("runner preceded its start event: %v", events)
			}
			events = append(events, "run "+id)
			clock = clock.Add(durations[calls])
			calls++
			return shell.Response{Stdout: "ok"}, nil
		})
		result, err := VerifyWithOptions(context.Background(), root, "todo-app-demo", true, false, runner, VerifyOptions{
			Now: func() time.Time { return clock },
			TaskStarted: func(event VerifyTaskStart) {
				if event.Index != calls+1 || event.Total != 2 {
					t.Fatalf("wrong selected position: %+v", event)
				}
				events = append(events, "start "+event.TaskID)
				clock = clock.Add(time.Second) // Rendering is outside the proof timer.
			},
			TaskFinished: func(event VerifyTaskFinish) {
				if event.Elapsed != durations[calls-1] || !event.Passed || event.AssertionResult != "passed" || event.Integrity != "pure" || event.Index != calls || event.Total != 2 {
					t.Fatalf("completion observation = %+v", event)
				}
				events = append(events, "finish "+event.TaskID)
				clock = clock.Add(time.Second)
			},
		})
		if err != nil || calls != 2 || probeCalls != 1 || gitCalls != 8 {
			t.Fatalf("extra/lost execution or capture: err=%v proof=%d probe=%d git=%d", err, calls, probeCalls, gitCalls)
		}
		if !reflect.DeepEqual(events, []string{"start 1.1", "run 1.1", "finish 1.1", "start 1.2", "run 1.2", "finish 1.2"}) {
			t.Fatalf("wrong ordering: %v", events)
		}
		for i, outcome := range result.Outcomes {
			if outcome.Elapsed == nil || *outcome.Elapsed != durations[i] || outcome.State != evidence.StateVerified {
				t.Fatalf("measurement absent or includes surrounding work: %+v", outcome)
			}
		}
		observed, err := evidence.Load(root, "todo-app-demo")
		if err != nil {
			t.Fatal(err)
		}
		gitCalls = 0
		plain, err := Verify(context.Background(), root, "todo-app-demo", true, false, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"}))
		if err != nil || gitCalls != 8 || probeCalls != 1 {
			t.Fatalf("default path changed: %v git=%d probe=%d", err, gitCalls, probeCalls)
		}
		for _, outcome := range plain.Outcomes {
			if outcome.Elapsed != nil {
				t.Fatal("ordinary Verify acquired observation timing")
			}
		}
		baseline, err := evidence.Load(root, "todo-app-demo")
		if err != nil {
			t.Fatal(err)
		}
		for id, record := range observed.Tasks {
			other := baseline.Tasks[id]
			record.VerifiedAt, other.VerifiedAt = "", ""
			if !reflect.DeepEqual(record, other) {
				t.Fatalf("observation changed persisted proof/binding/policy facts for %s", id)
			}
		}
		data, err := os.ReadFile(evidence.DocumentPath(root, "todo-app-demo"))
		if err != nil || strings.Contains(string(data), "elapsed") {
			t.Fatalf("timing leaked into storage: %v\n%s", err, data)
		}
	})

	t.Run("no events for skips and preflight refusals", func(t *testing.T) {
		root := t.TempDir()
		writeVerifyFixture(t, root)
		overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
		completeBoth(t, root)
		starts, finishes, ticks := 0, 0, 0
		options := VerifyOptions{
			Now:          func() time.Time { ticks++; return time.Now() },
			TaskStarted:  func(VerifyTaskStart) { starts++ },
			TaskFinished: func(VerifyTaskFinish) { finishes++ },
		}
		before, _ := os.ReadFile(evidence.DocumentPath(root, "todo-app-demo"))
		result, err := VerifyWithOptions(context.Background(), root, "todo-app-demo", false, false, testutil.NewFakeRunner(), options)
		if err != nil || starts != 0 || finishes != 0 || ticks != 0 || len(result.Outcomes) != 0 || len(result.Skipped) != 2 || result.EvidencePersisted != nil {
			t.Fatalf("skipped work fabricated observations: %+v %v", result, err)
		}
		after, _ := os.ReadFile(evidence.DocumentPath(root, "todo-app-demo"))
		if string(before) != string(after) {
			t.Fatal("no-op rewrote its ledger")
		}
		ledger, _ := evidence.Load(root, "todo-app-demo")
		delete(ledger.Tasks, "1.2")
		if err := evidence.Save(root, ledger); err != nil {
			t.Fatal(err)
		}
		options.TaskStarted = func(event VerifyTaskStart) {
			starts++
			if event.TaskID != "1.2" || event.Index != 1 || event.Total != 1 {
				t.Fatalf("skipped task affected position: %+v", event)
			}
		}
		result, err = VerifyWithOptions(context.Background(), root, "todo-app-demo", false, false, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}), options)
		if err != nil || starts != 1 || finishes != 1 || ticks != 2 || len(result.Outcomes) != 1 {
			t.Fatalf("wrong selected observations: %+v %v", result, err)
		}
		starts, finishes, ticks = 0, 0, 0
		overrideIdentityRunner(t, &scriptedIdentityRunner{fail: true})
		if _, err := VerifyWithOptions(context.Background(), root, "todo-app-demo", true, false, testutil.NewFakeRunner(), options); err == nil || starts != 0 || finishes != 0 || ticks != 0 {
			t.Fatal("unavailable identity fabricated a proof attempt")
		}
		path := filepath.Join(root, ".walden/specs/todo-app-demo/tasks.md")
		data, _ := os.ReadFile(path)
		if err := os.WriteFile(path, append(data, []byte("\nDrift.\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyWithOptions(context.Background(), root, "todo-app-demo", true, false, testutil.NewFakeRunner(), options); err == nil || starts != 0 || finishes != 0 || ticks != 0 {
			t.Fatal("blocked chain fabricated a proof attempt")
		}
	})

	for _, kind := range []string{"start", "exit", "timeout", "mutation", "timeout-mutation"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			writeVerifyFixture(t, root)
			if strings.Contains(kind, "timeout") {
				feature, err := spec.LoadFeature(root, "todo-app-demo")
				if err != nil {
					t.Fatal(err)
				}
				body := strings.Replace(feature.Tasks.Body, `["go", "test", "./internal/spec"]`, "[\"go\", \"test\", \"./internal/spec\"]\n        timeout: 10ms", 1)
				writeFreshFeatureDoc(t, root, "todo-app-demo", "tasks.md", approvedTasksContent(body, feature.Tasks.ApprovedAt, feature.Design.ApprovedAt, feature.Design.ApprovedFingerprint))
			}
			overrideIdentityRunner(t, &dynamicIdentityRunner{root: root})
			completeBoth(t, root)
			clock := time.Now()
			calls := 0
			var finishes []VerifyTaskFinish
			runner := integrityProofFunc(func(ctx context.Context, _ string, _ ...string) (shell.Response, error) {
				calls++
				clock = clock.Add(100 * time.Millisecond)
				if calls != 1 {
					return shell.Response{Stdout: "ok"}, nil
				}
				if strings.Contains(kind, "mutation") {
					if err := os.WriteFile(filepath.Join(root, "generated.txt"), []byte("changed"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				if strings.Contains(kind, "timeout") {
					<-ctx.Done()
					clock = clock.Add(25 * time.Millisecond) // Includes runner cleanup.
				}
				if kind == "start" {
					return shell.Response{}, errors.New("fixture-start-error")
				}
				if kind == "exit" {
					return shell.Response{ExitCode: 1, Stderr: "fixture-exit-error"}, nil
				}
				return shell.Response{Stdout: "ok"}, nil
			})
			result, err := VerifyWithOptions(context.Background(), root, "todo-app-demo", true, false, runner, VerifyOptions{
				Now: func() time.Time { return clock },
				TaskFinished: func(event VerifyTaskFinish) {
					finishes = append(finishes, event)
					if len(event.ChangedPaths) > 0 {
						event.ChangedPaths[0] = "OBSERVER-MUTATION"
					}
				},
			})
			if err != nil || calls != 2 || len(finishes) != 2 || len(result.Outcomes) != 2 {
				t.Fatalf("attempts lost: %+v %v", result, err)
			}
			wantDuration := 100 * time.Millisecond
			if strings.Contains(kind, "timeout") {
				wantDuration += 25 * time.Millisecond
			}
			first := finishes[0]
			if first.Passed || first.Failure == "" || first.Elapsed != wantDuration || *result.Outcomes[0].Elapsed != wantDuration {
				t.Fatalf("failed attempt misreported: %+v", first)
			}
			if strings.Contains(kind, "mutation") {
				if first.Integrity != "mutated" || finishes[1].Integrity != "contaminated" || finishes[1].AssertionResult != "passed" || finishes[1].Passed {
					t.Fatalf("policy result lost: %+v", finishes)
				}
				ledger, _ := evidence.Load(root, "todo-app-demo")
				if !reflect.DeepEqual(ledger.Tasks["1.1"].Execution.ChangedPaths, []string{"generated.txt"}) || !reflect.DeepEqual(result.Outcomes[0].Assessment.Execution.Facts.ChangedPaths, []string{"generated.txt"}) {
					t.Fatal("observer mutated persisted or derived facts through an alias")
				}
			} else if first.AssertionResult != "failed" || first.Integrity != "pure" {
				t.Fatalf("assertion failure confused with policy: %+v", first)
			}
		})
	}
}
