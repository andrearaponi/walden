package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The guide must point to the official installation instructions and never
// carry a download-and-execute path itself (skills.sh audit: remote code).
func TestPrerequisiteSkillContract(t *testing.T) {
	text := string(Content())
	requireAuthoringText(t, text, "## CLI Prerequisite", "v0.10.3", "command -v walden", "$HOME/.local/bin/walden", "version --json",
		"do not downgrade", "persistent shell", "GitHub releases", "README", "do not run an installer",
		"npx skills update walden", "Do not use `walden update`", "overlapping", "walden skill status --json", "`version` field",
		"Bifurcation Test", "TDD describes development order", "Never hand-edit workflow frontmatter")
	for _, forbidden := range []string{"curl", "wget", "install.sh", "raw.githubusercontent.com", "--version v", "sudo", "--no-verify"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("guide must not contain %q: it would download or execute remote code", forbidden)
		}
	}
	if strings.Contains(text, "## CLI Prerequisite And Installation") {
		t.Error("installation section still present")
	}
	entry := strings.Index(text, "## Entry Decision")
	prerequisite := strings.Index(text, "## CLI Prerequisite")
	if entry < 0 || prerequisite <= entry {
		t.Error("entry routing must still precede the prerequisite section")
	}
}

func TestPrerequisiteDocumentationContract(t *testing.T) {
	root := authoringRoot(t)
	for _, name := range []string{"README.md", "docs/quickstart.md", "docs/reference/cli.md", "skill/walden/install-claude.md", "skill/walden/install-codex.md", "skill/walden/install-copilot.md", "skill/walden/install-opencode.md", "CHANGELOG.md", "RELEASE_NOTES.md"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			requireAuthoringText(t, text, "--no-skill", "Skills CLI")
			if strings.Contains(text, "pinned binary-only bootstrap") || strings.Contains(text, "#cli-prerequisite-and-installation") {
				t.Error("still documents the removed skill-driven bootstrap")
			}
		})
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	requireAuthoringText(t, string(readme), "npx skills add andrearaponi/walden --skill walden", "v0.10.3", "walden update", "does not install the CLI")
	notes, err := os.ReadFile(filepath.Join(root, "RELEASE_NOTES.md"))
	if err != nil {
		t.Fatal(err)
	}
	requireAuthoringText(t, string(notes), "## Walden v0.10.3", "audit")
}
