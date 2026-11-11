package skill

import (
	"regexp"
	"strings"
	"testing"
)

// TestProofLifecycleGuide pins what the guide tells an agent about delivery
// proofs. A delivery proof is not proven again once attested, so the guide
// has to say when the declaration is right and when it is a way around a
// failing proof. It checks text only and claims no agent compliance.
func TestProofLifecycleGuide(t *testing.T) {
	guide := string(canonicalGuide(t))
	authoring := guideSection(t, guide, "## Proof Authoring", "## Execute And Recover")
	recover := guideSection(t, guide, "## Execute And Recover", "## Lessons And Summaries")

	// The paragraph that holds the declaration: the five statements of the
	// criterion belong together, where a plan's proofs are written.
	var declaration string
	for _, paragraph := range strings.Split(authoring, "\n\n") {
		if strings.Contains(paragraph, "`- Proof: delivery`") {
			declaration = paragraph
		}
	}
	if declaration == "" {
		t.Fatal("the proof authoring section does not name the declaration `- Proof: delivery`")
	}

	t.Run("which proofs are delivery proofs", func(t *testing.T) {
		requireAuthoringText(t, declaration, "describes a delivery, not a behavior", "a fact about what shipped",
			"is not proven again")
	})
	t.Run("never to silence a failing regression proof", func(t *testing.T) {
		requireAuthoringText(t, declaration, "never a way to silence a failing regression proof")
	})
	t.Run("lasting checks get a regression leaf of their own", func(t *testing.T) {
		requireAuthoringText(t, declaration, "a check that must keep holding gets a regression leaf of its own")
	})
	t.Run("a plan change the user approves", func(t *testing.T) {
		requireAuthoringText(t, declaration, "a plan change that the user approves")
	})
	t.Run("adoption before any verify", func(t *testing.T) {
		requireAuthoringText(t, declaration, "after a declaration on a completed task",
			"goes through `walden adopt <feature>` before any `walden verify`")
	})
	t.Run("the capability is read from the help", func(t *testing.T) {
		// The guide supports older CLIs, which skip the line: the help of
		// the installed CLI says whether it knows delivery proofs. No version
		// number is named, because the release that ships them fixes it.
		requireAuthoringText(t, declaration, "`walden verify --help`")
		if version := regexp.MustCompile(`v\d+\.\d+`).FindString(declaration); version != "" {
			t.Errorf("the declaration paragraph names the version %s: the capability is recognized from the help", version)
		}
	})
	t.Run("the state and the ledger versions", func(t *testing.T) {
		requireAuthoringText(t, recover, "`verified`, `delivered`, `stale-spec`", "`v1alpha2`", "`v1alpha3`")
	})
}
