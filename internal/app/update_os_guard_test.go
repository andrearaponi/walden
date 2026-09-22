package app

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/selfupdate"
)

type recordingTransport struct{ calls int }

func (r *recordingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.calls++
	return nil, http.ErrNotSupported
}

// On Windows the binary cannot replace itself in place and no Windows asset
// swap is implemented: update must refuse before any network use and name
// the two supported paths.
func TestUpdateRefusesOnWindows(t *testing.T) {
	restoreVersion, restoreOptions := Version, updateOptions
	t.Cleanup(func() { Version, updateOptions = restoreVersion, restoreOptions })
	Version = "v0.10.4"
	transport := &recordingTransport{}
	staging := t.TempDir()
	updateOptions = func(current string) (selfupdate.Options, error) {
		return selfupdate.Options{CurrentVersion: current, BaseURL: "http://release.invalid", OS: "windows", Arch: "amd64",
			ExecutablePath: staging + "/walden.exe", HTTPClient: &http.Client{Transport: transport}}, nil
	}
	for _, args := range [][]string{{"update"}, {"update", "--check"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(args, &stdout, &stderr)
			text := stdout.String() + stderr.String()
			if code == 0 {
				t.Fatalf("update on windows must exit non-zero, got 0:\n%s", text)
			}
			if transport.calls != 0 {
				t.Fatalf("update on windows made %d HTTP request(s) before refusing", transport.calls)
			}
			for _, want := range []string{"go install github.com/andrearaponi/walden/cmd/walden@", ".exe", "Windows"} {
				if !strings.Contains(text, want) {
					t.Errorf("refusal must name %q, got:\n%s", want, text)
				}
			}
		})
	}
}
