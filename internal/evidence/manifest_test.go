package evidence

import (
	"context"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
)

func TestCaptureManifestDigestMatchesIdentity(t *testing.T) {
	root := t.TempDir()
	fixture := func() *fakeGit {
		return &fakeGit{responses: map[string]shell.Response{
			"status": {ExitCode: 0},
			"ls-tree": {ExitCode: 0, Stdout: lsTreeListing(
				"100644 blob aaa\tmain.go",
				"100755 blob bbb\tscripts/run.sh",
			)},
		}}
	}

	identity, ok := Identity(context.Background(), fixture(), root)
	if !ok {
		t.Fatal("identity not computed")
	}
	manifest, ok := CaptureManifest(context.Background(), fixture(), root)
	if !ok {
		t.Fatal("manifest not captured")
	}

	if digest := manifest.Digest(); digest != identity {
		t.Fatalf("manifest digest diverged from identity: %s vs %s", digest, identity)
	}
	if len(manifest) != 2 {
		t.Fatalf("expected 2 manifest entries, got %d", len(manifest))
	}
	if blob := manifest["main.go"]; blob != "100644:aaa" {
		t.Fatalf("unexpected manifest entry for main.go: %q", blob)
	}
}

func TestCaptureManifestUnavailableGit(t *testing.T) {
	root := t.TempDir()
	failing := &fakeGit{responses: map[string]shell.Response{
		"status": {ExitCode: 128},
	}}

	if manifest, ok := CaptureManifest(context.Background(), failing, root); ok {
		t.Fatalf("expected capture to report unavailable git, got %v", manifest)
	}
}

func TestDiffPaths(t *testing.T) {
	before := Manifest{
		"main.go":   "100644:aaa",
		"go.mod":    "100644:mmm",
		"README.md": "100644:rrr",
	}
	after := Manifest{
		"main.go":   "100644:aaa",
		"go.mod":    "100644:changed",
		"build/out": "100755:nnn",
	}

	diff := DiffPaths(before, after)
	if got, want := strings.Join(diff, ","), "README.md,build/out,go.mod"; got != want {
		t.Fatalf("unexpected diff paths: %q, want %q", got, want)
	}

	if diff := DiffPaths(before, before); len(diff) != 0 {
		t.Fatalf("expected empty diff for identical manifests, got %v", diff)
	}
}
