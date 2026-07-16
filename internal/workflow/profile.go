package workflow

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
)

// RecordingVersion is the CLI version recording evidence, stamped by the app
// layer at dispatch: the import direction (app → workflow) forbids reading
// it from here.
var RecordingVersion = "dev"

// probeRunner executes declared environment probes. A seam, like
// identityRunner, so tests script probe outcomes without touching the proof
// runner's responses.
var probeRunner shell.Runner = shell.NewExecRunner()

// probeTimeout bounds each probe: capture is diagnostic and must never hang
// an evidence-producing command — a hung toolchain shim becomes a marker.
const probeTimeout = 30 * time.Second

type capturedProfile struct {
	profile evidence.Profile
	err     error
}

var (
	profileMu    sync.Mutex
	profileCache = map[string]capturedProfile{}
)

// runProfile captures the execution profile once per process and root — a
// CLI process is one command run against one root, so every record that run
// produces shares one capture. Probe failures degrade to marker values; the
// only error is a malformed declaration, which must fail the command loudly.
func runProfile(ctx context.Context, root string) (evidence.Profile, error) {
	profileMu.Lock()
	defer profileMu.Unlock()
	if cached, exists := profileCache[root]; exists {
		return cached.profile, cached.err
	}
	profile, err := captureProfile(ctx, root)
	profileCache[root] = capturedProfile{profile: profile, err: err}
	return profile, err
}

func captureProfile(ctx context.Context, root string) (evidence.Profile, error) {
	probes, err := spec.LoadEnvironmentProbes(root)
	if err != nil {
		return nil, err
	}

	profile := evidence.Profile{
		"platform": runtime.GOOS + "/" + runtime.GOARCH,
		"walden":   RecordingVersion,
	}
	for _, probe := range probes {
		probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
		response, err := probeRunner.Run(probeCtx, probe.Argv[0], probe.Argv[1:]...)
		timedOut := probeCtx.Err() == context.DeadlineExceeded
		cancel()

		switch {
		case timedOut:
			profile[probe.Name] = fmt.Sprintf("probe timed out after %s", probeTimeout)
		case err != nil:
			profile[probe.Name] = fmt.Sprintf("probe failed: %v", err)
		case response.ExitCode != 0:
			profile[probe.Name] = fmt.Sprintf("probe failed: exit %d", response.ExitCode)
		default:
			profile[probe.Name] = strings.TrimSpace(response.Stdout + response.Stderr)
		}
	}
	return profile, nil
}

// resetProfileCaptureForTests clears the per-root capture cache so tests
// with reused roots or scripted probe runners observe fresh captures.
func resetProfileCaptureForTests() {
	profileMu.Lock()
	defer profileMu.Unlock()
	profileCache = map[string]capturedProfile{}
}
