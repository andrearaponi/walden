package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapSkillContract(t *testing.T) {
	text := string(Content())
	requireAuthoringText(t, text, "## CLI Prerequisite And Installation", "v0.10.2", "command -v walden", "$HOME/.local/bin/walden",
		"explicit approval before any download", "--no-skill", "--version v0.10.2", "version --json", "do not downgrade",
		"macOS/Linux", "amd64/arm64", "npx skills update walden", "Do not use `walden update`", "overlapping", "persistent shell",
		"Bifurcation Test", "TDD describes development order", "Never hand-edit workflow frontmatter")
	if strings.Contains(text, "If missing, stop that operation and point to the install instructions") {
		t.Error("unconditional missing-CLI instruction still conflicts with authorized bootstrap")
	}
	entry := strings.Index(text, "## Entry Decision")
	bootstrap := strings.Index(text, "## CLI Prerequisite And Installation")
	if entry < 0 || bootstrap <= entry {
		t.Error("entry routing must still precede the prerequisite flow")
	}
	if !strings.Contains(text, "https://raw.githubusercontent.com/andrearaponi/walden/v0.10.2/install.sh") {
		t.Error("bootstrap script is not release-pinned")
	}
}

func TestBootstrapDocumentationContract(t *testing.T) {
	for _, name := range []string{"README.md", "docs/quickstart.md", "docs/reference/cli.md", "skill/walden/install-claude.md", "skill/walden/install-codex.md", "skill/walden/install-copilot.md", "skill/walden/install-opencode.md", "CHANGELOG.md", "RELEASE_NOTES.md"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(authoringRoot(t), name))
			if err != nil {
				t.Fatal(err)
			}
			requireAuthoringText(t, string(data), "--no-skill", "Skills CLI")
		})
	}
	readme, err := os.ReadFile(filepath.Join(authoringRoot(t), "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	requireAuthoringText(t, string(readme), "npx skills add andrearaponi/walden --skill walden", "v0.10.2", "walden update")
}
