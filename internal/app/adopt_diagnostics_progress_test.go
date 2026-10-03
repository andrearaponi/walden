package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andrearaponi/walden/internal/adopt"
	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/workflow"
)

func TestAdoptDiagnosticsProgressOutput(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		t.Run(fmt.Sprintf("success/json=%t", jsonMode), func(t *testing.T) {
			root := adoptFixture(t)
			adoptDiagnosticTasks(t, root, "old-era", adoptDiagnosticTask("1", "")+adoptDiagnosticTask("2", ""))
			var stdout, stderr bytes.Buffer
			calls := 0
			adoptDiagnosticOverrideRunner(t, adoptDiagnosticRunnerFunc(func(_ context.Context, _ string, _ ...string) (shell.Response, error) {
				calls++
				if jsonMode {
					if stdout.Len() != 0 {
						t.Error("JSON mode streamed progress before its envelope")
					}
				} else {
					start := fmt.Sprintf("old-era task %d (%d/2): starting proof", calls, calls)
					if !strings.Contains(stdout.String(), start) {
						t.Errorf("runner started before progress %q:\n%s", start, stdout.String())
					}
					if calls == 2 && !strings.Contains(stdout.String(), "old-era task 1: execution accepted") {
						t.Error("second proof started before first completion was reported")
					}
				}
				return shell.Response{}, nil
			}))
			args := []string{"adopt", "old-era", "--apply"}
			if jsonMode {
				args = append(args, "--json")
			}
			if code := Run(args, &stdout, &stderr); code != 0 || calls != 2 || stderr.Len() != 0 {
				t.Fatalf("apply exit=%d calls=%d stderr=%s", code, calls, stderr.String())
			}
			if !jsonMode {
				for _, fragment := range []string{"adopting old-era (1/1)", "old-era task 2: execution accepted", "Adoption elapsed:", "elapsed=", " ms"} {
					if !strings.Contains(stdout.String(), fragment) {
						t.Errorf("missing %q:\n%s", fragment, stdout.String())
					}
				}
				return
			}
			result := adoptDiagnosticObject(t, adoptDiagnosticJSON(t, stdout.String())["result"])
			adoption := adoptDiagnosticObject(t, result["adoption"])
			feature := adoptDiagnosticFeatures(t, adoption)["old-era"]
			total, ok := adoption["elapsed_ms"].(float64)
			if !ok || total < 0 {
				t.Fatalf("actual invocation duration missing: %+v", adoption)
			}
			featureTime, ok := feature["elapsed_ms"].(float64)
			if !ok || featureTime < 0 || featureTime > total {
				t.Fatalf("actual feature duration missing/incoherent: %+v", feature)
			}
			for _, entry := range adoptDiagnosticEvidence(t, feature) {
				duration, ok := entry["elapsed_ms"].(float64)
				if !ok || duration < 0 || duration > featureTime {
					t.Fatalf("measured task duration missing/incoherent: %+v", entry)
				}
			}
		})
	}
	t.Run("fatal planning error retains only measured invocation", func(t *testing.T) {
		root := adoptFixture(t)
		if err := os.RemoveAll(filepath.Join(root, ".walden/specs")); err != nil {
			t.Fatal(err)
		}
		stdout, _, code := adoptDiagnosticRun("adopt", "--apply", "--json")
		if code != 1 {
			t.Fatalf("planning error exit=%d", code)
		}
		result := adoptDiagnosticObject(t, adoptDiagnosticJSON(t, stdout)["result"])
		adoption := adoptDiagnosticObject(t, result["adoption"])
		if _, ok := adoption["elapsed_ms"].(float64); !ok || adoption["scope"] != nil || adoption["workload"] != nil || adoption["features"] != nil {
			t.Fatalf("fatal plan error lost timing or fabricated scope: %+v", adoption)
		}
	})
	t.Run("live and final projections use supplied measurements", func(t *testing.T) {
		invocation, featureTime := 300*time.Millisecond, 200*time.Millisecond
		attempt, zero := 125*time.Millisecond+999*time.Microsecond, time.Duration(0)
		report := adopt.ApplyReport{
			Scope: evidence.NewScope(true, "sample"), Elapsed: &invocation,
			Features: []adopt.FeatureAdoption{{
				Name: "sample", Class: adopt.ClassReprove, Elapsed: &featureTime,
				Outcomes: []workflow.VerifyOutcome{
					{TaskID: "1", State: "verified", Passed: true, Elapsed: &attempt},
					{TaskID: "2", State: "verified", Passed: true, Elapsed: &zero},
				},
			}},
		}
		result := adoptApplyResult(report)
		var text, machine, live bytes.Buffer
		output.PrintText(&text, result)
		if err := output.PrintJSON(&machine, "adopt", result); err != nil {
			t.Fatal(err)
		}
		printAdoptTaskFinish(&live, "sample", workflow.VerifyTaskFinish{TaskID: "1", Passed: true, AssertionResult: "passed", Integrity: "pure", Elapsed: attempt})
		for _, fragment := range []string{"elapsed=125 ms", "elapsed=0 ms", "elapsed=200 ms", "Adoption elapsed: 300 ms"} {
			if !strings.Contains(text.String(), fragment) {
				t.Errorf("final text lost measured values %q: %s", fragment, text.String())
			}
		}
		if !strings.Contains(live.String(), "execution accepted") || !strings.Contains(live.String(), "elapsed=125 ms") || strings.Contains(live.String(), "verified") {
			t.Fatalf("live result misrepresented measurement/freshness: %s", live.String())
		}
		adoption := adoptDiagnosticObject(t, adoptDiagnosticObject(t, adoptDiagnosticJSON(t, machine.String())["result"])["adoption"])
		feature := adoptDiagnosticFeatures(t, adoption)["sample"]
		entries := adoptDiagnosticEvidence(t, feature)
		if adoption["elapsed_ms"] != float64(300) || feature["elapsed_ms"] != float64(200) || entries[0]["elapsed_ms"] != float64(125) || entries[1]["elapsed_ms"] != float64(0) || elapsedMilliseconds(nil) != nil {
			t.Fatalf("JSON measurement or absence differs: %+v", adoption)
		}
	})
}
