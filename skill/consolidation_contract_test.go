package skill

import (
	"strings"
	"testing"
)

func guideSection(t *testing.T, guide, start, end string) string {
	t.Helper()
	_, section, found := strings.Cut(guide, start)
	if !found {
		t.Fatalf("guide section %q is missing", start)
	}
	section, _, _ = strings.Cut(section, end)
	return section
}

// TestConsolidationGuidance pins the consolidation procedure in the guide.
// It checks text only and claims no agent compliance.
func TestConsolidationGuidance(t *testing.T) {
	guide := string(canonicalGuide(t))
	entry := guideSection(t, guide, "## Entry Decision", "## CLI Prerequisite")
	lookup := guideSection(t, guide, "## Command Lookup", "## Authoring Phases")
	procedure := guideSection(t, guide, "### Consolidation", "### Release judgment")

	t.Run("discovery starts from the approved view", func(t *testing.T) {
		requireAuthoringText(t, entry, "approved `.walden/contracts.md`", "start discovery from it",
			"read in full only the features it identifies as related")
	})
	t.Run("command lookup", func(t *testing.T) {
		requireAuthoringText(t, lookup, "`walden consolidate`", "`walden consolidate start`", "`walden consolidate open`", "`walden consolidate approve`")
	})
	t.Run("bounded scoped review", func(t *testing.T) {
		requireAuthoringText(t, procedure, "suggested or due", "`walden consolidate`",
			"read `.walden/contracts.md` and only the features in the reported scope", "widely cited files",
			"overlaps, contradictions, removals still cited elsewhere and obsolete obligations", "qualified identifiers")
	})
	t.Run("compare against the linked statements", func(t *testing.T) {
		requireAuthoringText(t, procedure, "compare every added, changed or removed statement", "the linked statements the report shows beside it",
			"contradiction, duplicated rule or dependency on a removed or changed statement", "both qualified identifiers",
			"no need to read unrelated specifications in full")
	})
	t.Run("coherence review in the view", func(t *testing.T) {
		requireAuthoringText(t, procedure, "`## Coherence Review`", "one `### <feature>` entry per pending feature", "the inconsistencies found, or none",
			"citing the linked statements you compared as `feature#ID`", "refuses to open or approve the view")
	})
	t.Run("command rows and procedure match the CLI", func(t *testing.T) {
		requireAuthoringText(t, lookup, "comparisons of each change with its linked statements",
			"or, with changes pending, its coherence review is incomplete", "refused while a covered feature has a revision in review")
		requireAuthoringText(t, procedure, "as `feature#ID` in inline code",
			"If `status` reports the view stale, review it and run `walden consolidate open` again.")
	})
	t.Run("fixes through the normal review path", func(t *testing.T) {
		requireAuthoringText(t, procedure, "revision of the affected specification through its normal review path",
			"never approve, retire or execute anything")
	})
	t.Run("view update and approval", func(t *testing.T) {
		requireAuthoringText(t, procedure, "`Purpose:`", "`Active:`", "`Reserved:`", "keep reserved identifiers", "`Sources:`", "`Related:`",
			"`walden consolidate open`", "`walden consolidate approve` only after the user's explicit approval",
			"never edit `.walden/consolidation.json` or the view frontmatter by hand")
	})
	t.Run("existing portfolios and older CLIs", func(t *testing.T) {
		requireAuthoringText(t, procedure, "`walden consolidate start`", "unconsolidated backlog", "small batches", "`walden consolidate --help`")
	})
	t.Run("examples stay generic", func(t *testing.T) {
		for _, term := range []string{"desk-booking", "coworking", "K9", "winston"} {
			if strings.Contains(strings.ToLower(procedure+entry), strings.ToLower(term)) {
				t.Errorf("consolidation guidance mentions evaluation or adopter term %q", term)
			}
		}
	})
}
