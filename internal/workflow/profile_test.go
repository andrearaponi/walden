package workflow

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/andrearaponi/walden/internal/shell"
)

// overrideProbeRunner swaps the probe seam and always resets the
// once-per-process cache: profile tests must observe fresh captures.
func overrideProbeRunner(t *testing.T, runner shell.Runner) {
	t.Helper()
	previous := probeRunner
	probeRunner = runner
	resetProfileCaptureForTests()
	t.Cleanup(func() {
		probeRunner = previous
		resetProfileCaptureForTests()
	})
}

func overrideRecordingVersion(t *testing.T, version string) {
	t.Helper()
	previous := RecordingVersion
	RecordingVersion = version
	t.Cleanup(func() { RecordingVersion = previous })
}

func writeProbes(t *testing.T, root, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".walden"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".walden", "environment.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write environment.md: %v", err)
	}
}

type probeFunc func(ctx context.Context, name string, args ...string) (shell.Response, error)

func (f probeFunc) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	return f(ctx, name, args...)
}

func TestRunProfileCapture(t *testing.T) {
	t.Run("platform and walden keys always present, declared keys only", func(t *testing.T) {
		root := t.TempDir()
		writeProbes(t, root, "- go: [\"go\", \"version\"]\n")
		overrideRecordingVersion(t, "v9.9.9-test")
		overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
			return shell.Response{Stdout: "  go version go1.25.0 darwin/arm64\n", ExitCode: 0}, nil
		}))

		profile, err := runProfile(context.Background(), root)
		if err != nil {
			t.Fatalf("runProfile: %v", err)
		}
		if profile["platform"] != runtime.GOOS+"/"+runtime.GOARCH {
			t.Fatalf("platform = %q", profile["platform"])
		}
		if profile["walden"] != "v9.9.9-test" {
			t.Fatalf("walden = %q", profile["walden"])
		}
		if profile["go"] != "go version go1.25.0 darwin/arm64" {
			t.Fatalf("probe output not trimmed-verbatim: %q", profile["go"])
		}
		if len(profile) != 3 {
			t.Fatalf("profile must contain only declared keys plus the two reserved ones: %v", profile)
		}
	})

	t.Run("probe failure and non-zero exit become marker values", func(t *testing.T) {
		root := t.TempDir()
		writeProbes(t, root, "- broken: [\"nope\"]\n- failing: [\"false\"]\n")
		overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
			if name == "nope" {
				return shell.Response{ExitCode: 1, Err: os.ErrNotExist}, os.ErrNotExist
			}
			return shell.Response{ExitCode: 3}, nil
		}))

		profile, err := runProfile(context.Background(), root)
		if err != nil {
			t.Fatalf("runProfile: %v", err)
		}
		if !strings.HasPrefix(profile["broken"], "probe failed:") {
			t.Fatalf("execution error not marked: %q", profile["broken"])
		}
		if profile["failing"] != "probe failed: exit 3" {
			t.Fatalf("non-zero exit not marked: %q", profile["failing"])
		}
	})

	t.Run("hung probe becomes a timeout marker", func(t *testing.T) {
		root := t.TempDir()
		writeProbes(t, root, "- hung: [\"sleep\", \"forever\"]\n")
		overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
			<-ctx.Done()
			return shell.Response{ExitCode: -1}, nil
		}))

		profile, err := runProfile(contextWithShortProbeBudget(t), root)
		if err != nil {
			t.Fatalf("runProfile: %v", err)
		}
		if !strings.HasPrefix(profile["hung"], "probe timed out") {
			t.Fatalf("timeout not marked: %q", profile["hung"])
		}
	})

	t.Run("malformed declaration fails loudly", func(t *testing.T) {
		root := t.TempDir()
		writeProbes(t, root, "- broken here\n")
		overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
			return shell.Response{ExitCode: 0}, nil
		}))

		if _, err := runProfile(context.Background(), root); err == nil || !strings.Contains(err.Error(), "malformed probe line") {
			t.Fatalf("expected a loud declaration error, got %v", err)
		}
	})
}

// contextWithShortProbeBudget returns a context whose deadline expires well
// before the production probe budget, so the timeout branch is exercised
// without waiting 30 seconds: the per-probe WithTimeout inherits the sooner
// parent deadline.
func contextWithShortProbeBudget(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	t.Cleanup(cancel)
	return ctx
}
