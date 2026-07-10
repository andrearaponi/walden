package selfupdate

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func checkServer(t *testing.T, latestTag string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/"+latestTag, http.StatusFound)
	}))
}

func TestCheckReportsUpdateAvailable(t *testing.T) {
	server := checkServer(t, "v0.7.0")
	defer server.Close()

	status, err := Check(Options{
		CurrentVersion: "v0.5.0",
		BaseURL:        server.URL,
		HTTPClient:     server.Client(),
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	want := Status{CurrentVersion: "v0.5.0", TargetVersion: "v0.7.0", UpdateAvailable: true}
	if status != want {
		t.Fatalf("Check = %+v, want %+v", status, want)
	}
}

func TestCheckReportsAlreadyCurrent(t *testing.T) {
	server := checkServer(t, "v0.5.0")
	defer server.Close()

	status, err := Check(Options{
		CurrentVersion: "v0.5.0",
		BaseURL:        server.URL,
		HTTPClient:     server.Client(),
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if status.UpdateAvailable {
		t.Fatalf("Check reported an update for an identical version: %+v", status)
	}
}

func TestCheckHonorsPinnedTarget(t *testing.T) {
	server := checkServer(t, "v0.7.0")
	defer server.Close()

	status, err := Check(Options{
		CurrentVersion: "v0.7.0",
		TargetTag:      "v0.6.0",
		BaseURL:        server.URL,
		HTTPClient:     server.Client(),
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	want := Status{CurrentVersion: "v0.7.0", TargetVersion: "v0.6.0", UpdateAvailable: true}
	if status != want {
		t.Fatalf("Check = %+v, want %+v (pinned downgrade)", status, want)
	}
}

func TestCheckDefaultOptionsAreProductionSafe(t *testing.T) {
	opts, err := DefaultOptions("v0.5.0")
	if err != nil {
		t.Fatalf("DefaultOptions returned error: %v", err)
	}

	if !strings.HasPrefix(opts.BaseURL, "https://github.com/") {
		t.Fatalf("default BaseURL = %q, want an https://github.com/ URL", opts.BaseURL)
	}
	if opts.HTTPClient == nil || opts.HTTPClient.Timeout != 30*time.Second {
		t.Fatalf("default HTTP client must carry a 30s timeout, got %+v", opts.HTTPClient)
	}
	if opts.OS == "" || opts.Arch == "" {
		t.Fatalf("default platform not populated: os=%q arch=%q", opts.OS, opts.Arch)
	}
	if opts.Runner == nil {
		t.Fatal("default Runner is nil")
	}
	if opts.CurrentVersion != "v0.5.0" {
		t.Fatalf("CurrentVersion = %q, want v0.5.0", opts.CurrentVersion)
	}
}
