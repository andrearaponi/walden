package selfupdate

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testAsset = "walden-v0.7.0-linux-amd64"

func checksumServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/download/v0.7.0/checksums.txt" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func stagedFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "staging")
	if err := os.WriteFile(path, []byte("staged"), 0o600); err != nil {
		t.Fatalf("seed staging file: %v", err)
	}
	return path
}

func TestChecksumVerifiesMatchingEntry(t *testing.T) {
	digest := strings.Repeat("ab", 32)
	body := fmt.Sprintf("%s  walden-v0.7.0-darwin-arm64\n%s  %s\n", strings.Repeat("cd", 32), digest, testAsset)
	server := checksumServer(t, body, http.StatusOK)
	defer server.Close()

	staged := stagedFile(t)
	if err := verifyChecksum(server.Client(), server.URL, "v0.7.0", testAsset, staged, digest); err != nil {
		t.Fatalf("verifyChecksum rejected a matching digest: %v", err)
	}
	if _, err := os.Stat(staged); err != nil {
		t.Fatalf("staging file removed despite successful verification: %v", err)
	}
}

func TestChecksumMissingFileFailsClosed(t *testing.T) {
	server := checksumServer(t, "not found", http.StatusNotFound)
	defer server.Close()

	staged := stagedFile(t)
	err := verifyChecksum(server.Client(), server.URL, "v0.7.0", testAsset, staged, strings.Repeat("ab", 32))
	if err == nil {
		t.Fatal("verifyChecksum accepted a release without checksums.txt")
	}
	if !strings.Contains(err.Error(), "checksums.txt") {
		t.Fatalf("error %q does not name checksums.txt", err)
	}
	if _, statErr := os.Stat(staged); !os.IsNotExist(statErr) {
		t.Fatal("staging file survived a fail-closed abort")
	}
}

func TestChecksumMissingEntryFailsClosed(t *testing.T) {
	body := fmt.Sprintf("%s  walden-v0.7.0-darwin-arm64\n", strings.Repeat("cd", 32))
	server := checksumServer(t, body, http.StatusOK)
	defer server.Close()

	staged := stagedFile(t)
	err := verifyChecksum(server.Client(), server.URL, "v0.7.0", testAsset, staged, strings.Repeat("ab", 32))
	if err == nil {
		t.Fatal("verifyChecksum accepted a checksums.txt without the asset entry")
	}
	if !strings.Contains(err.Error(), testAsset) {
		t.Fatalf("error %q does not name the asset", err)
	}
	if _, statErr := os.Stat(staged); !os.IsNotExist(statErr) {
		t.Fatal("staging file survived a fail-closed abort")
	}
}

func TestChecksumMismatchReportsBothDigests(t *testing.T) {
	expected := strings.Repeat("ab", 32)
	actual := strings.Repeat("ef", 32)
	body := fmt.Sprintf("%s  %s\n", expected, testAsset)
	server := checksumServer(t, body, http.StatusOK)
	defer server.Close()

	staged := stagedFile(t)
	err := verifyChecksum(server.Client(), server.URL, "v0.7.0", testAsset, staged, actual)
	if err == nil {
		t.Fatal("verifyChecksum accepted a digest mismatch")
	}
	if !strings.Contains(err.Error(), expected) || !strings.Contains(err.Error(), actual) {
		t.Fatalf("error %q does not report both digests", err)
	}
	if _, statErr := os.Stat(staged); !os.IsNotExist(statErr) {
		t.Fatal("staging file survived a fail-closed abort")
	}
}
