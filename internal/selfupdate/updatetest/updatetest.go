// Package updatetest provides an in-process fake release host and a seeded
// install fixture for exercising the update flow without network access. It
// lives under internal/selfupdate so every net/http import in the module —
// production or test support — stays confined to the selfupdate subtree.
package updatetest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"

	"github.com/andrearaponi/walden/internal/selfupdate"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/skilldist"
)

// TB is the subset of testing.TB the fixture needs, kept local so importing
// this package never drags the testing package into a build.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
	Cleanup(func())
}

// Fixture wires a complete offline update environment: a fake release host
// serving one tag for the current platform (with real checksums), a fake
// installed executable, and a home carrying one installed claude skill.
type Fixture struct {
	Options    selfupdate.Options
	Executable string
}

// New builds the fixture for tag, serving binaryContent as the release
// asset. The returned Options carry currentVersion and every seam pointed at
// local doubles; all resources tear down via t.Cleanup.
func New(t TB, tag, currentVersion string, binaryContent []byte) Fixture {
	t.Helper()

	server := newReleaseServer(tag, binaryContent)
	t.Cleanup(server.Close)

	installDir := tempDir(t)
	executable := filepath.Join(installDir, "walden")
	if err := os.WriteFile(executable, []byte("OLD-BINARY"), 0o755); err != nil {
		t.Fatalf("seed current executable: %v", err)
	}

	home := tempDir(t)
	skillPath := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("skill body\n"), 0o644); err != nil {
		t.Fatalf("seed skill file: %v", err)
	}

	return Fixture{
		Options: selfupdate.Options{
			CurrentVersion: currentVersion,
			BaseURL:        server.URL,
			OS:             runtime.GOOS,
			Arch:           runtime.GOARCH,
			ExecutablePath: executable,
			WorkDir:        tempDir(t),
			Env:            skilldist.Env{Home: home},
			HTTPClient:     server.Client(),
			Runner:         shell.NewExecRunner(),
		},
		Executable: executable,
	}
}

// newReleaseServer serves the release layout the updater consumes: the
// latest redirect, one platform asset, and its checksums.txt entry.
func newReleaseServer(tag string, binaryContent []byte) *httptest.Server {
	asset := fmt.Sprintf("walden-%s-%s-%s", tag, runtime.GOOS, runtime.GOARCH)
	digest := sha256.Sum256(binaryContent)

	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/"+tag, http.StatusFound)
	})
	mux.HandleFunc("/releases/download/"+tag+"/"+asset, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(binaryContent)
	})
	mux.HandleFunc("/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(digest[:]), asset)
	})
	return httptest.NewServer(mux)
}

func tempDir(t TB) string {
	dir, err := os.MkdirTemp("", "walden-updatetest-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
