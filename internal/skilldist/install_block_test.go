package skilldist

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/andrearaponi/walden/skill"
)

func TestInstallBlockMarkersMatchSetupScript(t *testing.T) {
	if blockBegin != "# --- BEGIN WALDEN SKILL ---" {
		t.Fatalf("BEGIN marker drifted from setup.sh: %q", blockBegin)
	}
	if blockEnd != "# --- END WALDEN SKILL ---" {
		t.Fatalf("END marker drifted from setup.sh: %q", blockEnd)
	}
}

func TestInstallBlockCreatesAgentsFile(t *testing.T) {
	opts, home := testOptions(t)
	report, err := Install(mustLookup(t, "codex"), ScopeUser, opts)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := filepath.Join(home, ".codex", "AGENTS.md")
	if report.Path != want {
		t.Fatalf("expected path %s, got %s", want, report.Path)
	}
	if report.Replaced {
		t.Fatal("first install must not report replaced")
	}

	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	interior, found, err := blockInterior(data, want)
	if err != nil || !found {
		t.Fatalf("expected a walden block (found=%t err=%v)", found, err)
	}
	if !bytes.Equal(interior, Stamp(skill.Content(), "v9.9.9")) {
		t.Fatal("block interior must be the stamped embedded skill")
	}
}

func TestInstallBlockAppendsPreservingExistingContent(t *testing.T) {
	opts, home := testOptions(t)
	target := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	prior := "# My agents\n\npersonal notes\n"
	if err := os.WriteFile(target, []byte(prior), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}

	if _, err := Install(mustLookup(t, "codex"), ScopeUser, opts); err != nil {
		t.Fatalf("Install: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !bytes.HasPrefix(data, []byte(prior)) {
		t.Fatal("existing content must be preserved ahead of the appended block")
	}
	if !bytes.Contains(data, []byte(blockBegin)) || !bytes.Contains(data, []byte(blockEnd)) {
		t.Fatal("appended block must carry both markers")
	}
}

func TestInstallBlockReplacesExistingBlockInPlace(t *testing.T) {
	opts, home := testOptions(t)
	target := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	prefix := "# My agents\n\npersonal notes\n\n"
	suffix := "trailing notes\n"
	seeded := prefix + blockBegin + "\nOLD-SENTINEL-CONTENT\n" + blockEnd + "\n" + suffix
	if err := os.WriteFile(target, []byte(seeded), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}

	report, err := Install(mustLookup(t, "codex"), ScopeUser, opts)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !report.Replaced {
		t.Fatal("installing over an existing block must report replaced")
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !bytes.HasPrefix(data, []byte(prefix)) {
		t.Fatal("content before the block must be preserved byte for byte")
	}
	if !bytes.HasSuffix(data, []byte(suffix)) {
		t.Fatal("content after the block must be preserved byte for byte")
	}
	if bytes.Contains(data, []byte("OLD-SENTINEL-CONTENT")) {
		t.Fatal("the old block interior must be fully replaced")
	}
	if bytes.Count(data, []byte(blockBegin)) != 1 {
		t.Fatal("replacement must not duplicate the block")
	}
	interior, found, err := blockInterior(data, target)
	if err != nil || !found {
		t.Fatalf("expected a walden block (found=%t err=%v)", found, err)
	}
	if !bytes.Equal(interior, Stamp(skill.Content(), "v9.9.9")) {
		t.Fatal("block interior must be the stamped embedded skill")
	}
}

func TestInstallBlockCorruptMarkersAbortWithoutModifying(t *testing.T) {
	opts, home := testOptions(t)
	target := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	corrupt := "notes\n" + blockBegin + "\nno end marker\n"
	if err := os.WriteFile(target, []byte(corrupt), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}

	_, err := Install(mustLookup(t, "codex"), ScopeUser, opts)
	if !errors.Is(err, ErrCorruptBlock) {
		t.Fatalf("expected ErrCorruptBlock, got %v", err)
	}
	data, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read AGENTS.md: %v", readErr)
	}
	if !bytes.Equal(data, []byte(corrupt)) {
		t.Fatal("a corrupt block must abort the operation without modifying the file")
	}
}
