package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestProofLifecycleDocumentation holds the documents to what a delivery
// proof is and to what it does not guarantee. It checks that the facts are
// named, not that sentences are exact, so a reworded paragraph does not fail
// it and a dropped fact does.
func TestProofLifecycleDocumentation(t *testing.T) {
	root := authoringSourceRoot(t)
	read := func(relative string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(data)
	}
	require := func(t *testing.T, name, text string, fragments ...string) {
		t.Helper()
		for _, fragment := range fragments {
			if !strings.Contains(text, fragment) {
				t.Errorf("%s does not mention %q", name, fragment)
			}
		}
	}
	// section returns the text from a heading to the next heading of the
	// same or a higher level.
	section := func(t *testing.T, name, text, heading string) string {
		t.Helper()
		start := strings.Index(text, heading)
		if start < 0 {
			t.Fatalf("%s has no section %q", name, heading)
		}
		rest := text[start+len(heading):]
		level := strings.Repeat("#", len(heading)-len(strings.TrimLeft(heading, "#")))
		for _, marker := range []string{"\n" + level + " ", "\n## "} {
			if end := strings.Index(rest, marker); end >= 0 {
				rest = rest[:end]
			}
		}
		return rest
	}

	t.Run("reference", func(t *testing.T) {
		// The spec format reference: the declaration and the leaf rule, the
		// attestation and the two ledger versions.
		format := read("docs/reference/spec-format.md")
		tasks := section(t, "docs/reference/spec-format.md", format, "## `tasks.md`")
		require(t, "the tasks.md section", tasks, "`Proof: delivery`", "leaf task", "regression proof",
			"task-definition fingerprint", "approval")
		ledger := section(t, "docs/reference/spec-format.md", format, "## `evidence/<feature>.json`")
		require(t, "the evidence section", ledger, "`attestation`", "`v1alpha2`", "`v1alpha3`")

		// The lifecycle guide: the state, what keeps it and what loses it.
		lifecycle := read("docs/lifecycle.md")
		delivery := section(t, "docs/lifecycle.md", lifecycle, "### Delivery proofs")
		require(t, "docs/lifecycle.md", lifecycle, "| `delivered` |")
		require(t, "the delivery section", delivery, "`delivered`", "attested", "commit",
			"`stale-spec`", "statement", "declaration", "`walden verify`", "`walden adopt`")

		// The CLI reference: the behavior of the four commands and the order
		// after a declaration on a completed task.
		cli := read("docs/reference/cli.md")
		verify := section(t, "docs/reference/cli.md", cli, "### `walden verify")
		require(t, "the verify section", verify, "delivered", "kept", "awaits adoption", "`walden adopt`",
			"refuses", "without uncommitted")
		adopt := section(t, "docs/reference/cli.md", cli, "### `walden adopt")
		require(t, "the adopt section", adopt, "seals", "attestation", "without executing",
			"`no-provenance`", "`proof-changed`", "`statements-unknown`", "`no-commit`", "`walden verify <feature> --task <id>`")
		release := section(t, "docs/reference/cli.md", cli, "### `walden release check")
		require(t, "the release check section", release, "delivered", "commit", "only delivery proofs cover")
		status := section(t, "docs/reference/cli.md", cli, "### `walden evidence status")
		require(t, "the evidence status section", status, "delivered", "commit", "attestation")
		if !regexp.MustCompile(`(?s)walden adopt.{0,200}then.{0,120}walden verify <feature> --task`).MatchString(adopt + verify) {
			t.Error("docs/reference/cli.md does not give the order after a declaration on a completed task: walden adopt, then walden verify --task")
		}

		// The JSON reference: the fields.
		reference := read("docs/reference/json.md")
		require(t, "docs/reference/json.md", reference, "`delivered`", "`delivery`", "`attestation`", "`sealed`",
			"`awaits_adoption`", "`explanation.attestation`", "`no-tree`", "`uncommitted-paths`", "`undeclared-run`",
			"`kept`", "`left_for_adoption`", "`delivery_only`", "`sealable_passes`", "`sealed_passes`",
			"`unsealed_passes`", "`task.proof`", "`v1alpha3`")

		// The adoption guide: the sealing.
		adoption := read("docs/adoption.md")
		require(t, "docs/adoption.md", adoption, "delivery", "seals", "attestation", "`walden verify <feature> --task <id>`")
	})

	t.Run("limits", func(t *testing.T) {
		// The four limits, where the state is described.
		delivery := section(t, "docs/lifecycle.md", read("docs/lifecycle.md"), "### Delivery proofs")
		for limit, pattern := range map[string]string{
			"a delivered task is not proven again":                        `not proven again`,
			"the attested commit is not checked against the history":      `not checked against the history`,
			"a pass on a tree with uncommitted changes is no attestation": `uncommitted (changes|paths?)[^.]*(no|not an) attestation`,
			"an earlier CLI judges a delivery task as a regression task":  `(earlier|older) (CLI|release)[^.]*regression`,
		} {
			if !regexp.MustCompile(pattern).MatchString(delivery) {
				t.Errorf("docs/lifecycle.md does not name the limit: %s", limit)
			}
		}
	})

	t.Run("changelog", func(t *testing.T) {
		// The entry is read in the range above the heading of the release
		// before this feature, so the check survives the release that ships
		// the change.
		changelog := read("CHANGELOG.md")
		end := strings.Index(changelog, "\n## [0.14.5]")
		if end < 0 {
			t.Fatal("CHANGELOG.md has no heading for the previous release, 0.14.5")
		}
		entries := changelog[:end]
		entry := regexp.MustCompile(`(?m)^- \*\*Breaking \(gate criteria contract\): [^\n]*`).FindString(entries)
		if entry == "" {
			t.Fatal("CHANGELOG.md holds no entry labeled as a breaking change of the gate criteria above the heading of 0.14.5")
		}
		require(t, "the changelog entry", entry, "`walden release check`", "delivered")
		require(t, "CHANGELOG.md", entries, "`- Proof: delivery`", "`walden adopt --apply`", "`v1alpha3`")
	})
}
