package selfupdate

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/andrearaponi/walden/internal/shell"
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
	HTTPClient     *http.Client
	Runner         shell.Runner
}

// DefaultOptions returns production defaults for the running binary.
func DefaultOptions(currentVersion string) (Options, error) {
	return Options{
		CurrentVersion: currentVersion,
		BaseURL:        defaultBaseURL,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
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

// ErrUnsupportedOS reports a platform where in-place self-update is not
// implemented. Windows cannot rename a running executable, and no Windows
// swap exists yet, so the command refuses before any network use rather than
// leaving a staged file or a half-replaced binary.
type ErrUnsupportedOS struct{ OS string }

func (e ErrUnsupportedOS) Error() string {
	return fmt.Sprintf("walden update is not supported on %s: install with `go install github.com/andrearaponi/walden/cmd/walden@<tag>` or download the %s release asset (walden-<tag>-%s-<arch>.exe) from GitHub releases", e.OS, e.OS, e.OS)
}

// guardOS rejects platforms without an in-place update path. It runs before
// release resolution so an unsupported host never reaches the network.
func guardOS(opts Options) error {
	if opts.OS == "windows" {
		return ErrUnsupportedOS{OS: "Windows"}
	}
	return nil
}

// Check resolves the target release and compares it with the current
// version. It never touches the filesystem.
func Check(opts Options) (Status, error) {
	if err := guardOS(opts); err != nil {
		return Status{}, err
	}
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
