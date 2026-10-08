//go:build darwin || linux

package installtest

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Formats written before v0.11.0 by setup.sh (v0.1.0–v0.4.0) and by the
// binary's former skill subcommands (v0.5.0–v0.10.5).
const (
	legacyBegin = "# --- BEGIN WALDEN SKILL ---"
	legacyEnd   = "# --- END WALDEN SKILL ---"
)

func stampedGuide() string {
	return "---\nname: walden\n---\n\n# Walden\n\nOLD-GUIDE-SENTINEL-4c1f\n<!-- walden-skill-version: v0.10.5 -->\n"
}

func unstampedGuide() string {
	return "---\nname: walden\n---\n\n# Walden\n\nUNSTAMPED-SENTINEL-9b2e\n"
}

func legacyBlock() string { return legacyBegin + "\n" + stampedGuide() + legacyEnd + "\n" }

const (
	codexBefore = "USER-BEFORE-SENTINEL-1a7d\n\n"
	codexAfter  = "USER-AFTER-SENTINEL-5e3c\n"
)

// legacyPaths are the user-scope locations with no override in effect.
type legacyPaths struct{ claude, copilot, opencode, codex, command string }

func (f *fixture) legacy() legacyPaths {
	return legacyPaths{
		claude:   filepath.Join(f.home, ".claude/skills/walden"),
		copilot:  filepath.Join(f.home, ".copilot/skills/walden"),
		opencode: filepath.Join(f.home, ".config/opencode/skills/walden"),
		codex:    filepath.Join(f.home, ".codex/AGENTS.md"),
		command:  filepath.Join(f.home, ".claude/commands/walden.md"),
	}
}

// seedLegacy writes every legacy kind at its default user-scope location.
func (f *fixture) seedLegacy() legacyPaths {
	p := f.legacy()
	for _, dir := range []string{p.claude, p.copilot, p.opencode} {
		put(f.t, filepath.Join(dir, "SKILL.md"), stampedGuide(), 0o644)
	}
	put(f.t, p.codex, codexBefore+legacyBlock()+codexAfter, 0o600)
	put(f.t, p.command, "LEGACY-COMMAND-SENTINEL-7d20\n", 0o644)
	return p
}

// linkClaudeToStore replaces the Claude location with a Skills CLI symlink.
func (f *fixture) linkClaudeToStore() string {
	f.t.Helper()
	link := f.legacy().claude
	if err := os.RemoveAll(link); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Symlink("../../.agents/skills/walden", link); err != nil {
		f.t.Fatal(err)
	}
	return link
}

func gone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s still exists (err=%v)", path, err)
	}
}

func present(t *testing.T, path, want string) {
	t.Helper()
	if got := read(t, path); got != want {
		t.Fatalf("%s changed:\n%s", path, got)
	}
}

func contains(t *testing.T, output string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in:\n%s", want, output)
		}
	}
}

// treeSnapshot records every entry under root, symlinks by target, without
// following links.
func treeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			out[rel] = "link:" + target
		case info.IsDir():
			out[rel] = fmt.Sprintf("dir:%o", info.Mode().Perm())
		default:
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[rel] = fmt.Sprintf("file:%o:%x", info.Mode().Perm(), sha256.Sum256(data))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sameTree(t *testing.T, before, after map[string]string) {
	t.Helper()
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("tree changed:\nbefore %v\nafter  %v", before, after)
	}
}

// The cleanup mode removes what Walden provably wrote before v0.11.0, reports
// everything else, and never operates through a link or on the Skills CLI
// store.
func TestBootstrapInstallerRemoveLegacySkill(t *testing.T) {
	const flag = "--remove-legacy-skill"

	t.Run("mode-only", func(t *testing.T) {
		f := newFixture(t, "curl")
		f.seedLegacy()
		put(t, f.binary(), read(t, f.payload), 0o755)
		output, code := f.run("/bin/sh", flag)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, output)
		}
		if f.eventText() != "" {
			t.Fatalf("cleanup probed the platform, fetched or ran the binary: %s", f.eventText())
		}
		present(t, f.binary(), read(t, f.payload))
	})

	t.Run("conflicts", func(t *testing.T) {
		for _, args := range [][]string{{flag, "--version", "v0.10.2"}, {"--no-verify", flag}, {flag, "--uninstall"}, {"--uninstall", flag}} {
			t.Run(strings.Join(args, "_"), func(t *testing.T) {
				f := newFixture(t, "curl")
				f.seedLegacy()
				put(t, f.binary(), read(t, f.payload), 0o755)
				before := treeSnapshot(t, f.home)
				output, code := f.run("/bin/sh", args...)
				conflict := args[0]
				if conflict == flag {
					conflict = args[1]
				}
				if code == 0 || !strings.Contains(output, "cannot be combined with") || !strings.Contains(output, conflict) {
					t.Fatalf("conflict not rejected: exit %d\n%s", code, output)
				}
				if f.eventText() != "" {
					t.Fatalf("rejected invocation reached platform/network/payload: %s", f.eventText())
				}
				sameTree(t, before, treeSnapshot(t, f.home))
			})
		}
	})

	t.Run("help", func(t *testing.T) {
		f := newFixture(t, "curl")
		output, code := f.run("/bin/sh", "--help")
		if code != 0 {
			t.Fatalf("help: %s", output)
		}
		contains(t, output, flag, "not touched")
	})

	t.Run("stamped-copies", func(t *testing.T) {
		t.Run("defaults", func(t *testing.T) {
			f := newFixture(t, "curl")
			p := f.seedLegacy()
			userFile := filepath.Join(p.copilot, "USER-NOTES-SENTINEL.md")
			put(t, userFile, "mine\n", 0o644)
			output, code := f.run("/bin/sh", flag)
			if code != 0 {
				t.Fatalf("exit %d:\n%s", code, output)
			}
			gone(t, p.claude)
			gone(t, p.opencode)
			gone(t, filepath.Join(p.copilot, "SKILL.md"))
			present(t, userFile, "mine\n")
			contains(t, output, "removed: "+filepath.Join(p.claude, "SKILL.md"), "removed: "+filepath.Join(p.copilot, "SKILL.md"), "removed: "+filepath.Join(p.opencode, "SKILL.md"))
		})
		t.Run("overrides", func(t *testing.T) {
			f := newFixture(t, "curl")
			p := f.seedLegacy()
			copilotHome, opencodeHome := filepath.Join(f.root, "copilot-home"), filepath.Join(f.root, "opencode-home")
			f.env["COPILOT_HOME"], f.env["OPENCODE_HOME"] = copilotHome, opencodeHome
			f.env["XDG_CONFIG_HOME"] = filepath.Join(f.root, "xdg-ignored-when-opencode-home-is-set")
			overridden := []string{filepath.Join(copilotHome, "skills/walden"), filepath.Join(opencodeHome, "skills/walden")}
			for _, dir := range overridden {
				put(t, filepath.Join(dir, "SKILL.md"), stampedGuide(), 0o644)
			}
			output, code := f.run("/bin/sh", flag)
			if code != 0 {
				t.Fatalf("exit %d:\n%s", code, output)
			}
			for _, dir := range overridden {
				gone(t, dir)
			}
			// With an override in effect the former writer used only the
			// override, so the default locations are not legacy locations.
			present(t, filepath.Join(p.copilot, "SKILL.md"), stampedGuide())
			present(t, filepath.Join(p.opencode, "SKILL.md"), stampedGuide())
		})
		t.Run("xdg", func(t *testing.T) {
			f := newFixture(t, "curl")
			p := f.seedLegacy()
			xdg := filepath.Join(f.root, "xdg")
			f.env["XDG_CONFIG_HOME"] = xdg
			dir := filepath.Join(xdg, "opencode/skills/walden")
			put(t, filepath.Join(dir, "SKILL.md"), stampedGuide(), 0o644)
			output, code := f.run("/bin/sh", flag)
			if code != 0 {
				t.Fatalf("exit %d:\n%s", code, output)
			}
			gone(t, dir)
			present(t, filepath.Join(p.opencode, "SKILL.md"), stampedGuide())
		})
	})

	t.Run("symlink-kept", func(t *testing.T) {
		f := newFixture(t, "curl")
		p := f.seedLegacy()
		link := f.linkClaudeToStore()
		store := read(t, filepath.Join(f.home, ".agents/skills/walden/SKILL.md"))
		// A real directory whose SKILL.md is a link is kept as well.
		if err := os.Remove(filepath.Join(p.copilot, "SKILL.md")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(f.home, ".agents/skills/walden/SKILL.md"), filepath.Join(p.copilot, "SKILL.md")); err != nil {
			t.Fatal(err)
		}
		output, code := f.run("/bin/sh", flag)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, output)
		}
		if target, err := os.Readlink(link); err != nil || target != "../../.agents/skills/walden" {
			t.Fatalf("Claude link changed: %q %v", target, err)
		}
		if target, err := os.Readlink(filepath.Join(p.copilot, "SKILL.md")); err != nil || target == "" {
			t.Fatalf("Copilot SKILL.md link changed: %q %v", target, err)
		}
		present(t, filepath.Join(f.home, ".agents/skills/walden/SKILL.md"), store)
		contains(t, output, "kept (symbolic link): "+link, "kept (symbolic link): "+p.copilot)
	})

	t.Run("unstamped-reported", func(t *testing.T) {
		f := newFixture(t, "curl")
		p := f.seedLegacy()
		put(t, filepath.Join(p.copilot, "SKILL.md"), unstampedGuide(), 0o644)
		output, code := f.run("/bin/sh", flag)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, output)
		}
		present(t, filepath.Join(p.copilot, "SKILL.md"), unstampedGuide())
		contains(t, output, "check by hand (no Walden stamp): "+filepath.Join(p.copilot, "SKILL.md"))
	})

	t.Run("codex-block", func(t *testing.T) {
		for _, override := range []bool{false, true} {
			t.Run(fmt.Sprintf("CODEX_HOME-set-%v", override), func(t *testing.T) {
				f := newFixture(t, "curl")
				p := f.seedLegacy()
				target := p.codex
				if override {
					home := filepath.Join(f.root, "codex-home")
					f.env["CODEX_HOME"] = home
					target = filepath.Join(home, "AGENTS.md")
					put(t, target, codexBefore+legacyBlock()+codexAfter, 0o600)
				}
				output, code := f.run("/bin/sh", flag)
				if code != 0 {
					t.Fatalf("exit %d:\n%s", code, output)
				}
				present(t, target, codexBefore+codexAfter)
				if info, err := os.Stat(target); err != nil || info.Mode().Perm() != 0o600 {
					t.Fatalf("rewritten AGENTS.md lost its mode: %v %v", info.Mode(), err)
				}
				contains(t, output, "removed Walden block: "+target)
				if override {
					present(t, p.codex, codexBefore+legacyBlock()+codexAfter)
				}
				if leftovers, _ := filepath.Glob(target + ".walden.*"); len(leftovers) != 0 {
					t.Fatalf("temporary files left behind: %v", leftovers)
				}
			})
		}
	})

	t.Run("codex-only-block", func(t *testing.T) {
		f := newFixture(t, "curl")
		p := f.seedLegacy()
		put(t, p.codex, "\n"+legacyBlock()+"\n", 0o600)
		output, code := f.run("/bin/sh", flag)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, output)
		}
		gone(t, p.codex)
		contains(t, output, "removed (it held only a Walden block): "+p.codex)
	})

	t.Run("codex-unterminated", func(t *testing.T) {
		f := newFixture(t, "curl")
		p := f.seedLegacy()
		broken := "USER-TEXT-SENTINEL-2b8f\n" + legacyBegin + "\n" + stampedGuide()
		put(t, p.codex, broken, 0o600)
		output, code := f.run("/bin/sh", flag)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, output)
		}
		present(t, p.codex, broken)
		contains(t, output, "left unchanged (unterminated Walden block): "+p.codex)
	})

	t.Run("legacy-command", func(t *testing.T) {
		f := newFixture(t, "curl")
		p := f.seedLegacy()
		output, code := f.run("/bin/sh", flag)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, output)
		}
		gone(t, p.command)
		contains(t, output, "removed: "+p.command)
	})

	t.Run("skills-cli-store-untouched", func(t *testing.T) {
		f := newFixture(t, "curl")
		f.seedLegacy()
		f.linkClaudeToStore()
		put(t, filepath.Join(f.home, ".agents/skills/other/SKILL.md"), "OTHER-SKILL-SENTINEL\n", 0o644)
		before := treeSnapshot(t, filepath.Join(f.home, ".agents"))
		output, code := f.run("/bin/sh", flag)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, output)
		}
		sameTree(t, before, treeSnapshot(t, filepath.Join(f.home, ".agents")))
	})

	t.Run("failure-continues", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("permission failures cannot be induced as root")
		}
		f := newFixture(t, "curl")
		p := f.seedLegacy()
		if err := os.Chmod(p.copilot, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p.copilot, 0o755) })
		output, code := f.run("/bin/sh", flag)
		if code == 0 {
			t.Fatalf("a failed removal exited zero:\n%s", output)
		}
		present(t, filepath.Join(p.copilot, "SKILL.md"), stampedGuide())
		contains(t, output, "failed to remove: "+filepath.Join(p.copilot, "SKILL.md"))
		gone(t, p.claude)
		gone(t, p.opencode)
		gone(t, p.command)
		present(t, p.codex, codexBefore+codexAfter)
	})

	t.Run("report", func(t *testing.T) {
		f := newFixture(t, "curl")
		p := f.seedLegacy()
		link := f.linkClaudeToStore()
		put(t, filepath.Join(p.opencode, "SKILL.md"), unstampedGuide(), 0o644)
		output, code := f.run("/bin/sh", flag)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, output)
		}
		contains(t, output,
			"kept (symbolic link): "+link,
			"removed: "+filepath.Join(p.copilot, "SKILL.md"),
			"check by hand (no Walden stamp): "+filepath.Join(p.opencode, "SKILL.md"),
			"removed Walden block: "+p.codex,
			"removed: "+p.command)
		if strings.Contains(output, "nothing to remove") {
			t.Fatalf("claims nothing to remove after removing files:\n%s", output)
		}
	})

	t.Run("nothing-to-remove", func(t *testing.T) {
		f := newFixture(t, "curl")
		// Only a Skills CLI store and unrelated files: drop the fixture's
		// unrelated file at the Claude legacy location.
		if err := os.RemoveAll(f.legacy().claude); err != nil {
			t.Fatal(err)
		}
		before := treeSnapshot(t, f.home)
		output, code := f.run("/bin/sh", flag)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, output)
		}
		sameTree(t, before, treeSnapshot(t, f.home))
		contains(t, output, "nothing to remove")
	})

	t.Run("project-report-only", func(t *testing.T) {
		f := newFixture(t, "curl")
		project := filepath.Join(f.root, ".claude/skills/walden/SKILL.md")
		agents := filepath.Join(f.root, "AGENTS.md")
		put(t, project, stampedGuide(), 0o644)
		put(t, agents, "TEAM-TEXT-SENTINEL\n\n"+legacyBlock(), 0o644)
		output, code := f.run("/bin/sh", flag)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, output)
		}
		present(t, project, stampedGuide())
		present(t, agents, "TEAM-TEXT-SENTINEL\n\n"+legacyBlock())
		note := "project copy (remove it in the repository and commit): "
		contains(t, output, note+"./.claude/skills/walden/SKILL.md", note+"./AGENTS.md")
	})

	t.Run("ends-with-pointer", func(t *testing.T) {
		f := newFixture(t, "curl")
		f.seedLegacy()
		output, code := f.run("/bin/sh", flag)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, output)
		}
		lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
		if last := strings.TrimSpace(lines[len(lines)-1]); last != "npx skills add andrearaponi/walden" {
			t.Fatalf("output does not end with the Skills CLI line: %q", last)
		}
	})

	t.Run("shadow-in-cleanup", func(t *testing.T) {
		f := newFixture(t, "curl")
		f.seedLegacy()
		put(t, f.binary(), read(t, f.payload), 0o755)
		dir := filepath.Join(f.root, "shadow-bin")
		shadow := filepath.Join(dir, "walden")
		put(t, shadow, "#!/bin/sh\nprintf 'walden v0.9.9 (shadow)\\n'\n", 0o755)
		f.env["PATH"] = dir + string(os.PathListSeparator) + f.bin
		output, code := f.run("/bin/sh", flag)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, output)
		}
		contains(t, output, "walden on your PATH is "+shadow+" (walden v0.9.9), not "+f.binary()+".", "walden@<tag>")
	})

	t.Run("dash", func(t *testing.T) {
		shell, err := exec.LookPath("dash")
		if err != nil {
			t.Skip("dash unavailable; the /bin/sh cases remain required")
		}
		f := newFixture(t, "curl")
		p := f.seedLegacy()
		output, code := f.run(shell, flag)
		if code != 0 {
			t.Fatalf("dash: exit %d\n%s", code, output)
		}
		gone(t, p.claude)
		gone(t, p.copilot)
		gone(t, p.opencode)
		gone(t, p.command)
		present(t, p.codex, codexBefore+codexAfter)
	})
}

// Without the flag the installer leaves every legacy location alone: the
// cleanup is opt-in.
func TestBootstrapInstallerLegacyOptIn(t *testing.T) {
	for _, args := range [][]string{{"--version", "v0.10.2"}, {"--no-skill", "--version", "v0.10.2"}, {"--uninstall"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			f := newFixture(t, "curl")
			f.seedLegacy()
			put(t, filepath.Join(f.root, ".claude/skills/walden/SKILL.md"), stampedGuide(), 0o644)
			put(t, filepath.Join(f.root, "AGENTS.md"), legacyBlock(), 0o644)
			if args[0] == "--uninstall" {
				put(t, f.binary(), read(t, f.payload), 0o755)
			}
			before := f.homeSnapshot()
			output, code := f.run("/bin/sh", args...)
			if code != 0 {
				t.Fatalf("exit %d:\n%s", code, output)
			}
			f.assertHomeUnchanged(before)
			present(t, filepath.Join(f.root, ".claude/skills/walden/SKILL.md"), stampedGuide())
			present(t, filepath.Join(f.root, "AGENTS.md"), legacyBlock())
		})
	}
}
