package selfupdate

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveLatestFollowsRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/latest" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		http.Redirect(w, r, "/releases/tag/v0.7.0", http.StatusFound)
	}))
	defer server.Close()

	tag, err := resolveTarget(server.Client(), server.URL, "")
	if err != nil {
		t.Fatalf("resolveTarget returned error: %v", err)
	}
	if tag != "v0.7.0" {
		t.Fatalf("resolveTarget = %q, want v0.7.0", tag)
	}
}

func TestResolveLatestMissingLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := resolveTarget(server.Client(), server.URL, "")
	if err == nil {
		t.Fatal("resolveTarget accepted a response without a redirect")
	}
	if !strings.Contains(err.Error(), "--version") {
		t.Fatalf("error %q does not suggest --version pinning", err)
	}
}

func TestResolveLatestNonTagLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases", http.StatusFound)
	}))
	defer server.Close()

	_, err := resolveTarget(server.Client(), server.URL, "")
	if err == nil {
		t.Fatal("resolveTarget accepted a non-tag redirect target")
	}
	if !strings.Contains(err.Error(), "--version") {
		t.Fatalf("error %q does not suggest --version pinning", err)
	}
}

func TestResolveLatestTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 50 * time.Millisecond}
	_, err := resolveTarget(client, server.URL, "")
	if err == nil {
		t.Fatal("resolveTarget did not surface the client timeout")
	}
}

func TestResolvePinnedTagSkipsNetwork(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer server.Close()

	tag, err := resolveTarget(server.Client(), server.URL, "v0.6.0")
	if err != nil {
		t.Fatalf("resolveTarget returned error: %v", err)
	}
	if tag != "v0.6.0" {
		t.Fatalf("resolveTarget = %q, want v0.6.0", tag)
	}
	if hits.Load() != 0 {
		t.Fatalf("pinned resolution performed %d network requests, want 0", hits.Load())
	}
}

func TestResolvePinnedTagRejectsMalformed(t *testing.T) {
	_, err := resolveTarget(&http.Client{}, "http://unused.invalid", "0.6.0")
	if err == nil {
		t.Fatal("resolveTarget accepted a malformed pinned tag")
	}
	if !strings.Contains(err.Error(), "vMAJOR.MINOR.PATCH") {
		t.Fatalf("error %q does not name the expected format", err)
	}
}
