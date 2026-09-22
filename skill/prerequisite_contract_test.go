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
	text := string(canonicalGuide(t))
	requireAuthoringText(t, text, "## CLI Prerequisite", "v0.10.4", "command -v walden", "$HOME/.local/bin/walden", "version --json",
		"do not downgrade", "persistent shell", "releases page", "README", "do not run an installer",
		"Determine the platform", "macOS/Linux", "Windows", "go install github.com/andrearaponi/walden/cmd/walden@v0.10.4", ".exe",
		"command -v go", "Get-Command go", "https://github.com/andrearaponi/walden", "never the other branch", "never a fetched script",
		"exactly one", "Do not repeat the refusal",
		"npx skills update walden", "neither installs, inspects or updates the other", "none exists",
		"Bifurcation Test", "TDD describes development order", "Never hand-edit workflow frontmatter")
	for _, forbidden := range []string{"curl", "wget", "install.sh", "raw.githubusercontent.com", "--version v", "sudo", "--no-verify"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("guide must not contain %q: it would download or execute remote code", forbidden)
		}
	}
	// One channel: the guide names no Walden skill command, no installer skill
	// flag and no ownership protocol between managers.
	for _, removed := range []string{"walden skill", "--no-skill", "Do not use `walden update`", "overlapping", "`version` field", "which manager", "one manager"} {
		if strings.Contains(text, removed) {
			t.Errorf("guide still carries dual-channel text %q", removed)
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
	const pointer = "npx skills add andrearaponi/walden"

	// Exactly one installation page, covering every supported agent and both
	// Skills CLI scopes; the per-agent native pages are gone.
	pages, err := filepath.Glob(filepath.Join(root, "skill/walden/install*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || filepath.Base(pages[0]) != "install.md" {
		t.Fatalf("want exactly skill/walden/install.md, got %v", pages)
	}
	install, err := os.ReadFile(pages[0])
	if err != nil {
		t.Fatal(err)
	}
	requireAuthoringText(t, string(install), pointer, "npx skills update walden", "--global", "--copy",
		"claude", "codex", "copilot", "opencode", "v0.10.4", "go install github.com/andrearaponi/walden/cmd/walden@v0.10.4", "never installs the CLI", "manual fallback")
	for _, removed := range []string{"walden skill", "--no-skill", "embedded", "native Walden"} {
		if strings.Contains(string(install), removed) {
			t.Errorf("install page still carries %q", removed)
		}
	}

	// README: Skills CLI is the only skill route, with update and a Node-free fallback.
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	requireAuthoringText(t, string(readme), pointer, "npx skills update", "skill/walden/", "manual fallback", "not a second channel",
		"v0.10.4", "walden update", "does not install the CLI", "Windows", "go install github.com/andrearaponi/walden/cmd/walden@", ".exe")
	for _, removed := range []string{"walden skill", "--no-skill", "--skill <agent", "embedded skill", "re-sync", "one manager per skill copy"} {
		if strings.Contains(string(readme), removed) {
			t.Errorf("README still carries %q", removed)
		}
	}

	for _, name := range []string{"docs/quickstart.md", "docs/reference/cli.md", "docs/agentic.md"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			requireAuthoringText(t, string(data), "Skills CLI")
			for _, removed := range []string{"walden skill", "--no-skill", "embedded skill", "re-sync", "pinned binary-only bootstrap", "#cli-prerequisite-and-installation"} {
				if strings.Contains(string(data), removed) {
					t.Errorf("%s still carries %q", name, removed)
				}
			}
		})
	}
	quickstart, err := os.ReadFile(filepath.Join(root, "docs/quickstart.md"))
	if err != nil {
		t.Fatal(err)
	}
	requireAuthoringText(t, string(quickstart), "Windows", "go install github.com/andrearaponi/walden/cmd/walden@", ".exe")
	notes, err := os.ReadFile(filepath.Join(root, "RELEASE_NOTES.md"))
	if err != nil {
		t.Fatal(err)
	}
	requireAuthoringText(t, string(notes), "## Walden v0.10.4", "Windows", "effectiveVersion")
}
