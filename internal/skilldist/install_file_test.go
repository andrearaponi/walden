package skilldist

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/skill"
)

func testOptions(t *testing.T) (Options, string) {
	t.Helper()
	home := t.TempDir()
	return Options{Version: "v9.9.9", WorkDir: t.TempDir(), Env: Env{Home: home}}, home
}

func mustLookup(t *testing.T, name string) Agent {
	t.Helper()
	agent, err := Lookup(name)
	if err != nil {
		t.Fatalf("Lookup(%s): %v", name, err)
	}
	return agent
}

func TestInstallFileUserScopeTargets(t *testing.T) {
	cases := []struct {
		agent  string
		suffix string
	}{
		{"claude", ".claude/skills/walden/SKILL.md"},
		{"copilot", ".copilot/skills/walden/SKILL.md"},
		{"opencode", ".config/opencode/skills/walden/SKILL.md"},
	}
	for _, tc := range cases {
		t.Run(tc.agent, func(t *testing.T) {
			opts, home := testOptions(t)
			report, err := Install(mustLookup(t, tc.agent), ScopeUser, opts)
			if err != nil {
				t.Fatalf("Install: %v", err)
			}
			want := filepath.Join(home, tc.suffix)
			if report.Path != want {
				t.Fatalf("expected path %s, got %s", want, report.Path)
			}
			if report.Replaced {
				t.Fatal("first install must not report replaced")
			}
			installed, err := os.ReadFile(want)
			if err != nil {
				t.Fatalf("read installed skill: %v", err)
			}
			if !bytes.Equal(installed, Stamp(skill.Content(), "v9.9.9")) {
				t.Fatal("installed content must be the embedded skill plus the version stamp")
			}
		})
	}
}

func TestInstallFileStampsVersionAndPreservesBody(t *testing.T) {
	opts, home := testOptions(t)
	if _, err := Install(mustLookup(t, "claude"), ScopeUser, opts); err != nil {
		t.Fatalf("Install: %v", err)
	}
	installed, err := os.ReadFile(filepath.Join(home, ".claude/skills/walden/SKILL.md"))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	body, version := Strip(installed)
	if version != "v9.9.9" {
		t.Fatalf("expected stamped version v9.9.9, got %q", version)
	}
	if !bytes.Equal(body, skill.Content()) {
		t.Fatal("stripped body must equal the embedded skill")
	}
}

func TestInstallFileReplacesExisting(t *testing.T) {
	opts, home := testOptions(t)
	target := filepath.Join(home, ".claude/skills/walden/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("OLD-SENTINEL-CONTENT"), 0o644); err != nil {
		t.Fatalf("seed stale skill: %v", err)
	}

	report, err := Install(mustLookup(t, "claude"), ScopeUser, opts)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !report.Replaced {
		t.Fatal("reinstall over an existing file must report replaced")
	}
	installed, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if bytes.Contains(installed, []byte("OLD-SENTINEL-CONTENT")) {
		t.Fatal("stale content must be fully replaced")
	}
}

func TestInstallFileRemovesLegacyCommand(t *testing.T) {
	opts, home := testOptions(t)
	legacy := filepath.Join(home, ".claude", "commands", "walden.md")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("old command"), 0o644); err != nil {
		t.Fatalf("seed legacy command: %v", err)
	}

	report, err := Install(mustLookup(t, "claude"), ScopeUser, opts)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !report.LegacyRemoved {
		t.Fatal("expected legacy removal to be reported")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("legacy command file must be removed")
	}
}

func TestInstallFileWriteFailureLeavesNoPartialFile(t *testing.T) {
	opts, home := testOptions(t)
	dir := filepath.Join(home, ".claude", "skills", "walden")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := Install(mustLookup(t, "claude"), ScopeUser, opts)
	if err == nil {
		t.Fatal("expected install to fail on a read-only directory")
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatalf("read target dir: %v", readErr)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "walden-skill") || entry.Name() == "SKILL.md" {
			t.Fatalf("failed install must leave no partial file, found %s", entry.Name())
		}
	}
}
