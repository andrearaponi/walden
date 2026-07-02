package skilldist

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/skill"
)

func TestInstallProjectClaudeWritesUnderWorkDir(t *testing.T) {
	opts, _ := testOptions(t)
	report, err := Install(mustLookup(t, "claude"), ScopeProject, opts)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := filepath.Join(opts.WorkDir, ".claude", "skills", "walden", "SKILL.md")
	if report.Path != want {
		t.Fatalf("expected path %s, got %s", want, report.Path)
	}
	installed, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if !bytes.Equal(installed, Stamp(skill.Content(), "v9.9.9")) {
		t.Fatal("project install must place the stamped embedded skill")
	}
}

func TestInstallProjectCodexWritesAgentsFileUnderWorkDir(t *testing.T) {
	opts, _ := testOptions(t)
	report, err := Install(mustLookup(t, "codex"), ScopeProject, opts)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := filepath.Join(opts.WorkDir, "AGENTS.md")
	if report.Path != want {
		t.Fatalf("expected path %s, got %s", want, report.Path)
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

func TestInstallProjectUnsupportedAgentsFail(t *testing.T) {
	opts, _ := testOptions(t)
	for _, name := range []string{"copilot", "opencode"} {
		_, err := Install(mustLookup(t, name), ScopeProject, opts)
		if err == nil {
			t.Fatalf("%s: expected project-scope install to fail", name)
		}
		if !strings.Contains(err.Error(), "only user scope") {
			t.Fatalf("%s: error must explain user-only support, got: %v", name, err)
		}
	}
}

func TestInstallProjectSkipsLegacyCleanup(t *testing.T) {
	opts, home := testOptions(t)
	legacy := filepath.Join(home, ".claude", "commands", "walden.md")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("old command"), 0o644); err != nil {
		t.Fatalf("seed legacy command: %v", err)
	}

	report, err := Install(mustLookup(t, "claude"), ScopeProject, opts)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if report.LegacyRemoved {
		t.Fatal("project-scope installs must not touch user-scope legacy files")
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatal("legacy user file must remain untouched by project installs")
	}
}

func TestInstallAllCoversEveryAgentInOrder(t *testing.T) {
	opts, home := testOptions(t)
	reports, err := InstallAll(opts)
	if err != nil {
		t.Fatalf("InstallAll: %v", err)
	}
	wantOrder := []string{"claude", "codex", "copilot", "opencode"}
	if len(reports) != len(wantOrder) {
		t.Fatalf("expected %d reports, got %d", len(wantOrder), len(reports))
	}
	for i, name := range wantOrder {
		if reports[i].Agent != name {
			t.Fatalf("report %d: expected agent %s, got %s", i, name, reports[i].Agent)
		}
		if reports[i].Scope != ScopeUser {
			t.Fatalf("report %d: expected user scope, got %s", i, reports[i].Scope)
		}
	}
	for _, suffix := range []string{
		".claude/skills/walden/SKILL.md",
		".codex/AGENTS.md",
		".copilot/skills/walden/SKILL.md",
		".config/opencode/skills/walden/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(home, suffix)); err != nil {
			t.Fatalf("expected %s to be installed: %v", suffix, err)
		}
	}
}

func TestInstallAllStopsOnFirstFailure(t *testing.T) {
	opts, home := testOptions(t)
	// Make the codex target unwritable by occupying CODEX_HOME with a file.
	blocker := filepath.Join(home, "codex-blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	opts.Env.CodexHome = filepath.Join(blocker, "nested")

	reports, err := InstallAll(opts)
	if err == nil {
		t.Fatal("expected InstallAll to fail on the codex step")
	}
	if !strings.Contains(err.Error(), "install codex") {
		t.Fatalf("error must name the failing agent, got: %v", err)
	}
	if len(reports) != 1 || reports[0].Agent != "claude" {
		t.Fatalf("expected exactly the claude report before the failure, got %+v", reports)
	}
	for _, suffix := range []string{".copilot/skills/walden/SKILL.md", ".config/opencode/skills/walden/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(home, suffix)); !os.IsNotExist(err) {
			t.Fatalf("agents after the failure must not be installed: %s", suffix)
		}
	}
}
