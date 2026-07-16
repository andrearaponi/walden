package adopt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/testutil"
)

func TestAdoptApplyPartition(t *testing.T) {
	root := t.TempDir()
	preFingerprintDocs(t, root, "old-era")
	sealedDocs(t, root, "needs-proofs", true)
	sealedDocs(t, root, "drifted", true)
	driftedPath := filepath.Join(root, ".walden", "specs", "drifted", "requirements.md")
	content, _ := os.ReadFile(driftedPath)
	edited := strings.Replace(string(content), "# Requirements Document", "# Requirements Document\n\nEdited after approval.", 1)
	if err := os.WriteFile(driftedPath, []byte(edited), 0o644); err != nil {
		t.Fatalf("drift the doc: %v", err)
	}

	// Sorted order: drifted (blocked, untouched), needs-proofs (fails),
	// old-era (seals, passes).
	runner := testutil.NewFakeRunner(
		testutil.Response{Stderr: "assertion blew up", ExitCode: 1},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)

	progressCalls := []string{}
	report, err := Apply(context.Background(), root, "", runner, func(name string, index, total int) {
		progressCalls = append(progressCalls, fmt.Sprintf("%s %d/%d", name, index, total))
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	byName := map[string]FeatureAdoption{}
	for _, feature := range report.Features {
		byName[feature.Name] = feature
	}

	if got := byName["old-era"]; len(got.SealedDocs) != 3 || strings.Join(got.Verified, ",") != "1.1" || len(got.Failed) != 0 {
		t.Fatalf("old-era = %+v, want 3 sealed docs and task 1.1 verified", got)
	}
	if got := byName["needs-proofs"]; strings.Join(got.Failed, ",") != "1.1" || len(got.Verified) != 0 {
		t.Fatalf("needs-proofs = %+v, want task 1.1 failed", got)
	}
	blocked := byName["drifted"]
	if blocked.Class != ClassBlocked || blocked.Error == "" || len(blocked.SealedDocs) != 0 {
		t.Fatalf("drifted = %+v, want blocked with reason and no writes", blocked)
	}
	after, _ := os.ReadFile(driftedPath)
	if string(after) != edited {
		t.Fatal("apply touched a blocked feature")
	}

	if strings.Join(progressCalls, "; ") != "drifted 1/3; needs-proofs 2/3; old-era 3/3" {
		t.Fatalf("progress = %v", progressCalls)
	}
	if report.Totals.Verified != 1 || report.Totals.Failed != 1 || report.Totals.Blocked != 1 || report.Totals.SealedDocs != 3 {
		t.Fatalf("totals = %+v", report.Totals)
	}
}

func TestAdoptApplyResume(t *testing.T) {
	root := t.TempDir()
	preFingerprintDocs(t, root, "old-era")

	// First run: the proof fails — the seal lands, the failure is recorded.
	failing := testutil.NewFakeRunner(testutil.Response{Stderr: "broken env", ExitCode: 1})
	first, err := Apply(context.Background(), root, "", failing, nil)
	if err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	if first.Totals.Failed != 1 || first.Totals.SealedDocs != 3 {
		t.Fatalf("first run = %+v, want the seal plus one failure", first.Totals)
	}

	// Second run: environment fixed — the failed record is re-proven, the
	// seal is already in place.
	passing := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	second, err := Apply(context.Background(), root, "", passing, nil)
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if second.Totals.Verified != 1 || second.Totals.Failed != 0 || second.Totals.SealedDocs != 0 {
		t.Fatalf("second run = %+v, want one verified and nothing sealed", second.Totals)
	}

	// Third run: nothing left — the feature classifies complete.
	third, err := Apply(context.Background(), root, "", testutil.NewFakeRunner(), nil)
	if err != nil {
		t.Fatalf("third Apply: %v", err)
	}
	if third.Features[0].Class != ClassComplete || third.Totals.Verified != 0 || third.Totals.SealedDocs != 0 {
		t.Fatalf("third run = %+v, want a complete no-op", third.Features[0])
	}
}
