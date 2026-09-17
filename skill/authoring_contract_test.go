package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func requireAuthoringText(t *testing.T, text string, fragments ...string) {
	t.Helper()
	text = strings.ToLower(strings.Join(strings.Fields(text), " "))
	for _, fragment := range fragments {
		if !strings.Contains(text, strings.ToLower(fragment)) {
			t.Errorf("missing operational contract fragment %q", fragment)
		}
	}
}

func TestAuthoringDocumentationContract(t *testing.T) {
	checks := map[string][]string{
		"README.md":                     {"Preserve or restore", "Introduce or change", "Unclear", "project-specific"},
		"docs/README.md":                {"review"},
		"docs/agentic.md":               {"contract", "optional", "Acceptance check", "human review"},
		"docs/workflow.md":              {"six", "optional", "Acceptance check:"},
		"docs/quickstart.md":            {"six", "optional", "Acceptance check:"},
		"docs/lifecycle.md":             {"Acceptance check", "read-only"},
		"docs/reference/spec-format.md": {"six", "optional", "Acceptance check:", "covers:"},
	}
	for path, fragments := range checks {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(authoringRoot(t), path))
			if err != nil {
				t.Fatal(err)
			}
			requireAuthoringText(t, string(data), fragments...)
			for _, obsolete := range []string{"the ceremony costs the agent, not you", "The ceremony costs the agent, not the human"} {
				if strings.Contains(string(data), obsolete) {
					t.Errorf("overstated review-cost claim remains: %s", obsolete)
				}
			}
		})
	}
}

func TestAuthoringSkillContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(authoringRoot(t), "skill/walden/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, duplicate := range []string{
		"## Product Boundary", "## Approval And Staleness Model",
		"### `requirements.md` Template", "### `design.md` Template", "### `tasks.md` Template",
		"Update `last_modified` on every edit", "Set `status: approved`", "Lesson Decision: none",
	} {
		if strings.Contains(text, duplicate) {
			t.Errorf("obsolete instruction/template remains: %s", duplicate)
		}
	}
	entry, phases := strings.Index(text, "## Entry Decision"), strings.Index(text, "## Authoring Phases")
	if entry < 0 || phases <= entry {
		t.Error("contract-impact routing must precede authoring phases")
	}
	requireAuthoringText(t, text,
		"CLI-generated scaffolds", "Never hand-edit workflow frontmatter",
		"Preserve or restore", "Introduce or change", "Unclear",
		"project rules", "existing tests", "evidence", "Never treat silence as approval",
		"explicit approval", "explicit execution", "--allow-pending", "retirement",
		"Bifurcation Test", "five", "[decision:", "in draft", "source:",
		"Acceptance check:", "one observable response", "background", "NFR",
		"covers:", "expect_output", "zero intended tests", "read-only",
		"-count=1", "-v", "10-minute", "--check", "--all",
		"newly logged lesson", "guardrail", "walden lesson log", "observed trigger", "scope narrowed", "already-green suite", "preventive lesson", "Before closing maintenance", "restoring original AC wording",
	)
	for _, heading := range []string{"Architecture", "Options Considered", "Simplicity And Elegance Review", "Failure Modes And Tradeoffs", "Verification Plan", "Requirement Coverage"} {
		requireAuthoringText(t, text, heading)
	}
}
