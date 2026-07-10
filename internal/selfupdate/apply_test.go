package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/skilldist"
)

// releaseServer serves a complete fake release: latest redirect, one platform
// asset containing binaryContent, and a checksums.txt with its real digest.
func releaseServer(t *testing.T, tag string, binaryContent []byte) *httptest.Server {
	t.Helper()
	asset := assetName(tag, runtime.GOOS, runtime.GOARCH)
	digest := sha256.Sum256(binaryContent)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(digest[:]), asset)

	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/"+tag, http.StatusFound)
	})
	mux.HandleFunc("/releases/download/"+tag+"/"+asset, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(binaryContent)
	})
	mux.HandleFunc("/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})
	return httptest.NewServer(mux)
}

// applyFixture assembles an isolated install: a fake current executable, a
// home with the claude skill installed, and Options wired to the test server.
func applyFixture(t *testing.T, server *httptest.Server) (Options, string, string) {
	t.Helper()

	installDir := t.TempDir()
	executable := filepath.Join(installDir, "walden")
	if err := os.WriteFile(executable, []byte("OLD-BINARY"), 0o755); err != nil {
		t.Fatalf("seed current executable: %v", err)
	}

	home := t.TempDir()
	skillPath := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("skill body\n"), 0o644); err != nil {
		t.Fatalf("seed skill file: %v", err)
	}

	opts := Options{
		CurrentVersion: "v0.5.0",
		BaseURL:        server.URL,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		ExecutablePath: executable,
		WorkDir:        t.TempDir(),
		Env:            skilldist.Env{Home: home},
		HTTPClient:     server.Client(),
		Runner:         shell.NewExecRunner(),
	}
	return opts, executable, installDir
}

func assertNoUpdateArtifacts(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read install dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".walden-update-") || strings.HasPrefix(entry.Name(), ".walden-backup-") {
			t.Fatalf("leftover update artifact %s", entry.Name())
		}
	}
}

func TestApplyEndToEndInstallsAndSyncs(t *testing.T) {
	newBinary := []byte("#!/bin/sh\necho \"walden v0.7.0 (schema v0beta1)\"\n")
	server := releaseServer(t, "v0.7.0", newBinary)
	defer server.Close()

	opts, executable, installDir := applyFixture(t, server)

	report, err := Apply(context.Background(), opts)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	installed, readErr := os.ReadFile(executable)
	if readErr != nil {
		t.Fatalf("read installed binary: %v", readErr)
	}
	if string(installed) != string(newBinary) {
		t.Fatalf("installed binary does not match the release asset")
	}
	if info, _ := os.Stat(executable); info.Mode().Perm() != 0o755 {
		t.Fatalf("installed binary mode = %v, want 0755", info.Mode().Perm())
	}

	if report.PreviousVersion != "v0.5.0" || report.InstalledVersion != "v0.7.0" {
		t.Fatalf("report versions = %q -> %q, want v0.5.0 -> v0.7.0", report.PreviousVersion, report.InstalledVersion)
	}
	if !strings.HasSuffix(report.ReleaseNotesURL, "/releases/tag/v0.7.0") {
		t.Fatalf("release notes URL = %q, want .../releases/tag/v0.7.0", report.ReleaseNotesURL)
	}
	if len(report.SyncedSkills) != 1 || report.SyncedSkills[0].Agent != "claude" {
		t.Fatalf("synced skills = %+v, want the claude slot", report.SyncedSkills)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", report.Warnings)
	}
	if report.AlreadyUpToDate {
		t.Fatal("report claims already up to date after an install")
	}

	assertNoUpdateArtifacts(t, installDir)
}

func TestApplyRollsBackWhenSmokeTestFails(t *testing.T) {
	brokenBinary := []byte("#!/bin/sh\nexit 1\n")
	server := releaseServer(t, "v0.7.0", brokenBinary)
	defer server.Close()

	opts, executable, installDir := applyFixture(t, server)

	_, err := Apply(context.Background(), opts)
	if err == nil {
		t.Fatal("Apply accepted a binary that fails its smoke test")
	}
	if !strings.Contains(err.Error(), "restored") {
		t.Fatalf("error %q does not state the previous binary was restored", err)
	}

	content, readErr := os.ReadFile(executable)
	if readErr != nil {
		t.Fatalf("read executable: %v", readErr)
	}
	if string(content) != "OLD-BINARY" {
		t.Fatalf("executable = %q, want restored OLD-BINARY", content)
	}

	assertNoUpdateArtifacts(t, installDir)
}

func TestApplyAlreadyUpToDateMakesNoChanges(t *testing.T) {
	server := releaseServer(t, "v0.5.0", []byte("irrelevant"))
	defer server.Close()

	opts, executable, installDir := applyFixture(t, server)

	report, err := Apply(context.Background(), opts)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !report.AlreadyUpToDate {
		t.Fatalf("report = %+v, want AlreadyUpToDate", report)
	}

	content, _ := os.ReadFile(executable)
	if string(content) != "OLD-BINARY" {
		t.Fatalf("executable modified on an up-to-date install: %q", content)
	}
	assertNoUpdateArtifacts(t, installDir)
}
