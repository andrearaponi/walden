package skilldist

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallRemovesFileKindInstallations(t *testing.T) {
	for _, name := range []string{"claude", "copilot", "opencode"} {
		t.Run(name, func(t *testing.T) {
			opts, _ := testOptions(t)
			agent := mustLookup(t, name)
			installReport, err := Install(agent, ScopeUser, opts)
			if err != nil {
				t.Fatalf("Install: %v", err)
			}

			report, err := Uninstall(agent, ScopeUser, opts)
			if err != nil {
				t.Fatalf("Uninstall: %v", err)
			}
			if report.NotInstalled {
				t.Fatal("expected an installed skill to be removed")
			}
			if _, err := os.Stat(filepath.Dir(installReport.Path)); !os.IsNotExist(err) {
				t.Fatal("the skills/walden directory must be removed")
			}
		})
	}
}

func TestUninstallBlockPreservesUnrelatedContent(t *testing.T) {
	opts, home := testOptions(t)
	target := filepath.Join(home, ".codex", "AGENTS.md")
	prior := "# My agents\n\npersonal notes\n"
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte(prior), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}

	agent := mustLookup(t, "codex")
	if _, err := Install(agent, ScopeUser, opts); err != nil {
		t.Fatalf("Install: %v", err)
	}
	report, err := Uninstall(agent, ScopeUser, opts)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if report.NotInstalled {
		t.Fatal("expected the block to be removed")
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !bytes.Equal(data, []byte(prior)) {
		t.Fatalf("install+uninstall must restore the original file\nwant: %q\ngot:  %q", prior, data)
	}
}

func TestUninstallBlockRemovesEmptiedFile(t *testing.T) {
	opts, home := testOptions(t)
	agent := mustLookup(t, "codex")
	if _, err := Install(agent, ScopeUser, opts); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if _, err := Uninstall(agent, ScopeUser, opts); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("an AGENTS.md holding only the walden block must be removed entirely")
	}
}

func TestUninstallBlockCorruptMarkersAbort(t *testing.T) {
	opts, home := testOptions(t)
	target := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	corrupt := "notes\n" + blockBegin + "\nno end marker\n"
	if err := os.WriteFile(target, []byte(corrupt), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}

	_, err := Uninstall(mustLookup(t, "codex"), ScopeUser, opts)
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

func TestUninstallProjectScope(t *testing.T) {
	opts, _ := testOptions(t)
	agent := mustLookup(t, "claude")
	if _, err := Install(agent, ScopeProject, opts); err != nil {
		t.Fatalf("Install: %v", err)
	}

	report, err := Uninstall(agent, ScopeProject, opts)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if report.NotInstalled {
		t.Fatal("expected the project installation to be removed")
	}
	if _, err := os.Stat(filepath.Join(opts.WorkDir, ".claude", "skills", "walden")); !os.IsNotExist(err) {
		t.Fatal("project skill directory must be removed")
	}
}

func TestUninstallMissingInstallationIsNoOp(t *testing.T) {
	for _, name := range AgentNames() {
		t.Run(name, func(t *testing.T) {
			opts, _ := testOptions(t)
			report, err := Uninstall(mustLookup(t, name), ScopeUser, opts)
			if err != nil {
				t.Fatalf("Uninstall of absent skill must not error: %v", err)
			}
			if !report.NotInstalled {
				t.Fatal("expected NotInstalled to be reported")
			}
		})
	}
}

func TestUninstallRemovesLegacyCommand(t *testing.T) {
	opts, home := testOptions(t)
	legacy := filepath.Join(home, ".claude", "commands", "walden.md")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("old command"), 0o644); err != nil {
		t.Fatalf("seed legacy command: %v", err)
	}

	report, err := Uninstall(mustLookup(t, "claude"), ScopeUser, opts)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if !report.LegacyRemoved {
		t.Fatal("expected legacy removal to be reported")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("legacy command file must be removed")
	}
}

func TestUninstallAllCoversEveryAgent(t *testing.T) {
	opts, home := testOptions(t)
	if _, err := InstallAll(opts); err != nil {
		t.Fatalf("InstallAll: %v", err)
	}

	reports, err := UninstallAll(opts)
	if err != nil {
		t.Fatalf("UninstallAll: %v", err)
	}
	if len(reports) != len(AgentNames()) {
		t.Fatalf("expected %d reports, got %d", len(AgentNames()), len(reports))
	}
	for _, suffix := range []string{
		".claude/skills/walden",
		".codex/AGENTS.md",
		".copilot/skills/walden",
		".config/opencode/skills/walden",
	} {
		if _, err := os.Stat(filepath.Join(home, suffix)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed", suffix)
		}
	}
}
