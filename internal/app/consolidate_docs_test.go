package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConsolidationDocs pins the reference and explanatory documentation of
// the consolidation cycle to the command registry and the JSON fields.
func TestConsolidationDocs(t *testing.T) {
	root := authoringSourceRoot(t)
	read := func(relative string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(data)
	}
	require := func(name, text string, fragments ...string) {
		t.Helper()
		for _, fragment := range fragments {
			if !strings.Contains(text, fragment) {
				t.Errorf("%s does not mention %q", name, fragment)
			}
		}
	}

	cli := read("docs/reference/cli.md")
	for _, command := range commandRegistry {
		if command.Path != "consolidate" {
			continue
		}
		for _, sub := range command.Subcommands {
			require("docs/reference/cli.md", cli, "### `walden "+sub.Syntax+"`")
		}
	}
	require("docs/reference/cli.md", cli, "read-only", "never blocks", "repository_warnings", "coherence review")

	require("docs/reference/json.md", read("docs/reference/json.md"),
		"## Consolidation (`consolidate --json`)", "`result.consolidation`", "`tracking`", "`pending`", "`backlog`", "`unconsolidated`",
		"`threshold`", "`view`", "`problem`", "`remedy`", "`scope`", "`hubs`", "`findings`", "`identifiers`",
		"`missing-file`", "`dangling-reference`", "`reserved-reuse`", "`view-mismatch`", "`release.repository_warnings`",
		"`comparisons`", "`added`", "`changed`", "`unchanged`", "`unverified`", "`removed`", "`previous`", "`coherence-review`")

	require("docs/consolidation.md", read("docs/consolidation.md"),
		"two", "three", "warning", "`.walden/contracts.md`", "`.walden/consolidation.json`", "backlog", "`Reserved:`",
		"`walden consolidate start`", "`walden consolidate open`", "`walden consolidate approve`", "widely cited",
		"acceptance-check lines", "never blocks", "fingerprint", "## Comparisons", "statement texts", "linked statements",
		"previous text", "compare each pending change with the statements linked to it",
		"## Coherence review", "`## Coherence Review`", "never its correctness")

	require("docs/README.md", read("docs/README.md"), "(consolidation.md)")
}
