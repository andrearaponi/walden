package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadAssetNameMatchesReleaseLayout(t *testing.T) {
	if got := assetName("v0.7.0", "darwin", "arm64"); got != "walden-v0.7.0-darwin-arm64" {
		t.Fatalf("assetName = %q, want walden-v0.7.0-darwin-arm64", got)
	}
}

func TestDownloadStreamsAssetAndHashes(t *testing.T) {
	payload := []byte("fake-release-binary-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/download/v0.7.0/walden-v0.7.0-linux-amd64" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "staging")
	digest, err := downloadAsset(server.Client(), server.URL, "v0.7.0", "walden-v0.7.0-linux-amd64", dest)
	if err != nil {
		t.Fatalf("downloadAsset returned error: %v", err)
	}

	wantDigest := sha256.Sum256(payload)
	if digest != hex.EncodeToString(wantDigest[:]) {
		t.Fatalf("digest = %q, want %q", digest, hex.EncodeToString(wantDigest[:]))
	}

	written, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read staged file: %v", err)
	}
	if string(written) != string(payload) {
		t.Fatalf("staged content = %q, want %q", written, payload)
	}
}

func TestDownloadFailureNamesAssetAndRelease(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "staging")
	_, err := downloadAsset(server.Client(), server.URL, "v0.7.0", "walden-v0.7.0-linux-amd64", dest)
	if err == nil {
		t.Fatal("downloadAsset accepted a missing asset")
	}
	if !strings.Contains(err.Error(), "walden-v0.7.0-linux-amd64") || !strings.Contains(err.Error(), "v0.7.0") {
		t.Fatalf("error %q does not name the asset and the release", err)
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("staging file created despite failed download")
	}
}

func TestDownloadSurfacesTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 50 * time.Millisecond}
	dest := filepath.Join(t.TempDir(), "staging")
	_, err := downloadAsset(client, server.URL, "v0.7.0", "walden-v0.7.0-linux-amd64", dest)
	if err == nil {
		t.Fatal("downloadAsset did not surface the client timeout")
	}
	if !strings.Contains(err.Error(), "walden-v0.7.0-linux-amd64") {
		t.Fatalf("error %q does not name the asset", err)
	}
}
