package e2e

import (
	"strings"
	"testing"
)

// TestE2EVerifyTaskCycle walks the repair loop with the compiled CLI in a real
// repository: a selected run proves one task, and the feature stays
// unreleasable until a feature-level run proves the other.
func TestE2EVerifyTaskCycle(t *testing.T) {
	requireGit(t)
	dir := releasableRepo(t)
	binary := diagnosticsNativeCLI(t)
	run := func(args ...string) (string, int) {
		t.Helper()
		stdout, stderr, code := diagnosticsNativeRun(t, binary, dir, args...)
		return stdout + stderr, code
	}
	assertHolds := func(out string, wants ...string) {
		t.Helper()
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Fatalf("output does not hold %q:\n%s", want, out)
			}
		}
	}

	if out, code := run("release", "check", "gate-demo"); code != 0 {
		t.Fatalf("the fixture does not start releasable: %s", out)
	}

	writeFile(t, dir, "src.txt", "MARKER-A\nMARKER-B\nrepaired\n")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "a repair stales both tasks")

	out, code := run("verify", "gate-demo", "--task", "1.1")
	if code != 0 {
		t.Fatalf("selected run failed (%d): %s", code, out)
	}
	assertHolds(out,
		"verification passed for task 1.1 of gate-demo",
		"1 other completed task(s) not verified: 1.2",
		"Next action: Run walden verify gate-demo to re-prove the other tasks",
	)

	out, code = run("release", "check", "gate-demo")
	if code == 0 {
		t.Fatalf("a selected run left the feature releasable: %s", out)
	}
	assertHolds(out, "NOT RELEASABLE — 1 blocker(s)", "1.1: verified", "gate-demo: task 1.2 is stale-code")

	out, code = run("verify", "gate-demo")
	if code != 0 {
		t.Fatalf("feature run failed (%d): %s", code, out)
	}
	assertHolds(out, "1 task(s) re-proven, 1 skipped as verified")

	if out, code := run("release", "check", "gate-demo"); code != 0 {
		t.Fatalf("the feature does not end releasable: %s", out)
	}
}
