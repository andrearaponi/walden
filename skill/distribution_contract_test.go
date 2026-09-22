package skill

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Distribution contract: the binary knows nothing about the skill (no embed,
// no agent paths, no channel or command literals) and no current document
// describes a Walden skill command. Every negative check is paired with a
// positive control so a broken matcher fails loudly instead of passing
// vacuously.
func TestDistributionContract(t *testing.T) {
	root := authoringRoot(t)

	// firstHit returns "path: literal" for the first forbidden literal found in
	// any file selected by include, or "" when none matches.
	firstHit := func(t *testing.T, include func(path string) bool, literals []string) string {
		t.Helper()
		var hit string
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			if entry.IsDir() {
				if rel == ".git" || rel == "temp" || rel == ".walden" || rel == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if hit != "" || !include(rel) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, literal := range literals {
				if bytes.Contains(data, []byte(literal)) {
					hit = rel + ": " + literal
					return nil
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return hit
	}
	// matchesAny is the same matcher applied to one in-memory fixture: the
	// positive control that proves the literal set can actually match.
	matchesAny := func(text string, literals []string) bool {
		for _, literal := range literals {
			if strings.Contains(text, literal) {
				return true
			}
		}
		return false
	}
	nonTestGo := func(rel string) bool {
		return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go")
	}
	goOrTemplate := func(rel string) bool {
		return nonTestGo(rel) || strings.HasPrefix(rel, "templates/")
	}

	t.Run("no-embed", func(t *testing.T) {
		// Every embed directive in the module, read line by line: none may name
		// the guide. Template embeds are legitimate and stay. The marker is
		// assembled at runtime so this file itself never carries the literal.
		marker := "//go:" + "embed"
		var directives []string
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			if entry.IsDir() {
				if rel == ".git" || rel == "temp" || rel == ".walden" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(rel, ".go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), marker) {
					directives = append(directives, rel+": "+strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		forbidden := []string{"skill/", "SKILL.md"}
		for _, directive := range directives {
			if matchesAny(directive, forbidden) {
				t.Fatalf("an embed directive names the guide: %s", directive)
			}
		}
		if len(directives) == 0 {
			t.Fatal("positive control: no embed directive found at all; the walk is broken")
		}
		if !matchesAny("skill/embed.go: "+marker+" walden/SKILL.md", forbidden) {
			t.Fatal("positive control: embed matcher does not match the old directive")
		}
	})

	t.Run("binary", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "walden")
		cmd := exec.Command("go", "build", "-o", out, "./cmd/walden")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=auto")
		if build, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go build: %v\n%s", err, build)
		}
		binary, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(binary, []byte("Walden drafts and maintains feature specs")) {
			t.Fatal("the binary carries the guide's description")
		}
		if !bytes.Contains(binary, []byte("Usage:")) {
			t.Fatal("positive control: the binary does not even carry its usage text")
		}
	})

	t.Run("no-agent-paths", func(t *testing.T) {
		literals := []string{".claude/skills", "walden-skill-version", "<!-- walden", `".codex"`, `".copilot"`, `"opencode"`, "copilot-instructions"}
		if hit := firstHit(t, nonTestGo, literals); hit != "" {
			t.Fatalf("non-test source still names an agent placement target: %s", hit)
		}
		if _, err := os.Stat(filepath.Join(root, "internal/skilldist")); !os.IsNotExist(err) {
			t.Fatal("internal/skilldist still exists")
		}
		oldRegistry := `filepath.Join(home, ".claude", "skills", "walden", "SKILL.md") ... Name: "opencode" ... ".codex" ... <!-- walden-skill-version: `
		if !matchesAny(oldRegistry, literals) {
			t.Fatal("positive control: agent-path matcher does not match the old registry literals")
		}
	})

	t.Run("no-channel-literals", func(t *testing.T) {
		literals := []string{"npx skills", "skills.sh", "Skills CLI", "skill install", "skill uninstall", "skill status", "skill show"}
		if hit := firstHit(t, goOrTemplate, literals); hit != "" {
			t.Fatalf("non-test source or template still names the channel or a removed command: %s", hit)
		}
		oldArgs := `Path: "skill install", Syntax: "skill install <agent>|--all [--project] [--json]"`
		if !matchesAny(oldArgs, literals) {
			t.Fatal("positive control: channel matcher does not match the old registry lines")
		}
	})

	t.Run("docs", func(t *testing.T) {
		literals := []string{"walden skill install", "walden skill uninstall", "walden skill status", "walden skill show", "--skill <agent", "one manager per skill copy", "re-syncs"}
		include := func(rel string) bool {
			switch {
			case rel == "README.md", rel == "CONTRIBUTING.md":
				return true
			case strings.HasPrefix(rel, "docs/") && strings.HasSuffix(rel, ".md"):
				return true
			case strings.HasPrefix(rel, "skill/walden/") && strings.HasSuffix(rel, ".md"):
				return true
			}
			return false
		}
		if hit := firstHit(t, include, literals); hit != "" {
			t.Fatalf("current document still describes a removed operation: %s", hit)
		}
		usage := exec.Command("go", "run", "./cmd/walden", "--help")
		usage.Dir = root
		usage.Env = append(os.Environ(), "GOTOOLCHAIN=auto")
		help, err := usage.CombinedOutput()
		if err != nil {
			t.Fatalf("walden --help: %v\n%s", err, help)
		}
		if strings.Contains(string(help), "skill") {
			t.Fatalf("usage text still mentions skill:\n%s", help)
		}
		before, err := os.ReadFile(filepath.Join(root, "skill/testdata/distribution/readme-before.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !matchesAny(string(before), literals) {
			t.Fatal("positive control: docs matcher does not match the pre-change README fixture")
		}
	})

	t.Run("setup-sh", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, "setup.sh"))
		if err != nil {
			t.Fatal(err)
		}
		for _, removed := range []string{"skill install", "skill status", "skill uninstall"} {
			if strings.Contains(string(data), removed) {
				t.Fatalf("setup.sh still delegates %q to the binary", removed)
			}
		}
		if !strings.Contains(string(data), "npx skills add andrearaponi/walden") {
			t.Fatal("setup.sh does not point at the Skills CLI")
		}
	})
}
