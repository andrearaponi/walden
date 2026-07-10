package selfupdate

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/skilldist"
)

// defaultBaseURL is the production release host. It is intentionally not
// user-configurable: all release traffic stays on HTTPS to GitHub, and the
// injectable field below exists only as a test seam.
const defaultBaseURL = "https://github.com/andrearaponi/walden"

// Options configures an update run. Production callers start from
// DefaultOptions; tests fill the seams (BaseURL, HTTPClient, Runner,
// ExecutablePath) explicitly.
type Options struct {
	CurrentVersion string
	TargetTag      string // empty targets the latest release
	BaseURL        string
	OS             string
	Arch           string
	ExecutablePath string // empty resolves the running binary
	WorkDir        string
	Env            skilldist.Env
	HTTPClient     *http.Client
	Runner         shell.Runner
}

// DefaultOptions returns production defaults for the running binary.
func DefaultOptions(currentVersion string) (Options, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return Options{}, fmt.Errorf("resolve working directory: %w", err)
	}

	return Options{
		CurrentVersion: currentVersion,
		BaseURL:        defaultBaseURL,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		WorkDir:        workDir,
		Env:            skilldist.EnvFromOS(),
		HTTPClient:     &http.Client{Timeout: 30 * time.Second},
		Runner:         shell.NewExecRunner(),
	}, nil
}

// Status reports the outcome of a check: what runs now, what the target
// release is, and whether they differ.
type Status struct {
	CurrentVersion  string
	TargetVersion   string
	UpdateAvailable bool
}

// Check resolves the target release and compares it with the current
// version. It never touches the filesystem.
func Check(opts Options) (Status, error) {
	tag, err := resolveTarget(opts.HTTPClient, opts.BaseURL, opts.TargetTag)
	if err != nil {
		return Status{}, err
	}

	return Status{
		CurrentVersion:  opts.CurrentVersion,
		TargetVersion:   tag,
		UpdateAvailable: tag != opts.CurrentVersion,
	}, nil
}
