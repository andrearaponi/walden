//go:build darwin || linux

package installtest

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

const legacyHeading = "Cleaning up copies from before v0.11.0"

func repoFile(t *testing.T, name string) string {
	t.Helper()
	_, current, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(current), "../..", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// githubSlug is GitHub's heading anchor rule: lower case, punctuation other
// than hyphens and underscores dropped, spaces turned into hyphens.
func githubSlug(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}

// section returns the body under a "### <heading>" line, up to the next
// heading of the same or a higher level.
func section(t *testing.T, doc, heading string) string {
	t.Helper()
	start := strings.Index(doc, "\n### "+heading+"\n")
	if start < 0 {
		t.Fatalf("heading %q not found", heading)
	}
	body := doc[start+len("\n### "+heading+"\n"):]
	if end := regexp.MustCompile(`(?m)^#{2,3} `).FindStringIndex(body); end != nil {
		body = body[:end[0]]
	}
	return body
}

// The documents users already read point to the cleanup, and the README
// section carries everything needed to clean up by hand where the script
// cannot run.
func TestLegacyCopiesDocumentation(t *testing.T) {
	readme := repoFile(t, "README.md")
	legacy := section(t, readme, legacyHeading)
	anchor := "#" + githubSlug(legacyHeading)
	oneLiner := "curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh -s -- --remove-legacy-skill"

	t.Run("command-and-outcomes", func(t *testing.T) {
		contains(t, legacy, oneLiner,
			"removed", "kept", "reported",
			"symbolic link", "Skills CLI store", "walden-skill-version", "project")
	})

	t.Run("locations-for-manual-removal", func(t *testing.T) {
		contains(t, legacy,
			"~/.claude/skills/walden/SKILL.md",
			"${COPILOT_HOME:-~/.copilot}/skills/walden/SKILL.md",
			"$OPENCODE_HOME/skills/walden/SKILL.md",
			"${XDG_CONFIG_HOME:-~/.config}/opencode/skills/walden/SKILL.md",
			"${CODEX_HOME:-~/.codex}/AGENTS.md",
			"~/.claude/commands/walden.md",
			legacyBegin, legacyEnd,
			"setup.sh",
			".claude/skills/walden/SKILL.md", "AGENTS.md", "commit",
			"%USERPROFILE%")
	})

	t.Run("ai-skill-links-here", func(t *testing.T) {
		if anchor != "#cleaning-up-copies-from-before-v0110" {
			t.Fatalf("slug rule drifted: %s", anchor)
		}
		contains(t, section(t, readme, "AI skill"), "]("+anchor+")")
	})

	t.Run("install-page-points-without-commands", func(t *testing.T) {
		install := repoFile(t, "skill/walden/install.md")
		contains(t, install, "(https://github.com/andrearaponi/walden"+anchor+")")
		if strings.Contains(install, "../../README.md") {
			t.Fatal("install.md links to the README by a relative path, which does not exist in an installed copy")
		}
		if regexp.MustCompile(`\brm\s`).MatchString(install) || strings.Contains(install, "--remove-legacy-skill") {
			t.Fatal("install.md carries a removal command; it ships inside the audited skill directory")
		}
	})

	t.Run("changelog-unreleased", func(t *testing.T) {
		changelog := repoFile(t, "CHANGELOG.md")
		unreleased := strings.Index(changelog, "\n## [Unreleased]\n")
		released := strings.Index(changelog, "\n## [0.11.0]")
		if unreleased < 0 || released < 0 || unreleased > released {
			t.Fatalf("[Unreleased] must precede [0.11.0]: %d, %d", unreleased, released)
		}
		contains(t, changelog[unreleased:released], "--remove-legacy-skill", "PATH")
	})
}
