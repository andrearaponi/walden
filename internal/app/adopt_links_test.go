package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

// The real documents of internal/adopt/testdata/legacy-link and the times
// they hold: the design records requirements approved one second before the
// approval the requirements carry.
const (
	linkDocsApprovedByOldRelease = "approved-by-v0.1.0"
	linkDocsSealedByBackfill     = "sealed-by-v0.15.2"
	linkDesignRecordsAt          = "2026-11-23T11:50:38Z"
	linkRequirementsApprovedAt   = "2026-11-23T11:50:39Z"

	// The third set was approved under fingerprints by v0.4.0, and its
	// requirements were then approved again, unchanged: the design records
	// their first approval.
	linkDocsApprovedAgain       = "reapproved-by-v0.4.0"
	linkApprovedAgainRecordsAt  = "2026-11-23T12:41:06Z"
	linkApprovedAgainApprovedAt = "2026-11-23T12:41:09Z"
)

var linkDocumentNames = []string{"requirements.md", "design.md", "tasks.md"}

// linkDocsRepo enters an isolated directory that holds one set of the real
// documents as the feature greeting, and returns what it wrote by file name.
func linkDocsRepo(t *testing.T, set string) (string, map[string]string) {
	t.Helper()
	source := filepath.Join(authoringSourceRoot(t), "internal", "adopt", "testdata", "legacy-link", set)
	root := chdirContract(t)
	written := map[string]string{}
	for _, name := range linkDocumentNames {
		content, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			t.Fatalf("read test data: %v", err)
		}
		writeRawFeatureFile(t, root, "greeting", name, string(content))
		written[name] = string(content)
	}
	return root, written
}

// adoptEnvelope runs adopt on the feature greeting and decodes its result.
func adoptEnvelope(t *testing.T, wantExit int, flags ...string) (output.Result, string) {
	t.Helper()
	args := append([]string{"adopt", "greeting", "--json"}, flags...)
	stdout, _ := rerunCommand(t, wantExit, args...)
	var envelope output.Envelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode the adopt result: %v\n%s", err, stdout)
	}
	if envelope.Result.Adoption == nil || len(envelope.Result.Adoption.Features) != 1 {
		t.Fatalf("the adopt result holds no single feature: %s", stdout)
	}
	return envelope.Result, stdout
}

// adoptKeys lists the keys of the result and of its one feature.
func adoptKeys(t *testing.T, stdout string) (result, feature map[string]bool) {
	t.Helper()
	var envelope struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode the adopt result: %v", err)
	}
	var adoption struct {
		Features []map[string]json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(envelope.Result["adoption"], &adoption); err != nil || len(adoption.Features) != 1 {
		t.Fatalf("decode the adoption block: %v\n%s", err, stdout)
	}
	result, feature = map[string]bool{}, map[string]bool{}
	for key := range envelope.Result {
		result[key] = true
	}
	for key := range adoption.Features[0] {
		feature[key] = true
	}
	return result, feature
}

// linkStatusFresh fails unless status reads the three documents of greeting
// as approved and fresh.
func linkStatusFresh(t *testing.T) {
	t.Helper()
	stdout, _ := rerunCommand(t, 0, "status", "greeting", "--json")
	var envelope output.Envelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode the status result: %v\n%s", err, stdout)
	}
	if len(envelope.Result.Documents) != 3 {
		t.Fatalf("status reports %d document(s), want three: %s", len(envelope.Result.Documents), stdout)
	}
	for _, document := range envelope.Result.Documents {
		if document.Status != "approved" || !document.Fresh {
			t.Errorf("%s is %s, fresh=%v, want approved and fresh", document.Name, document.Status, document.Fresh)
		}
	}
}

func linkRequire(t *testing.T, what, text string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			t.Errorf("%s does not mention %q: %s", what, fragment, text)
		}
	}
}

func TestRunAdoptLegacyLink(t *testing.T) {
	t.Run("the plan blocks a legacy link whose times disagree", func(t *testing.T) {
		// What a feature blocked for its fingerprint reports today: the
		// result of a feature blocked for its link may hold no other key.
		root, _ := linkDocsRepo(t, linkDocsSealedByBackfill)
		design := filepath.Join(root, ".walden", "specs", "greeting", "design.md")
		content, err := os.ReadFile(design)
		if err != nil {
			t.Fatalf("read the design: %v", err)
		}
		if err := os.WriteFile(design, append(content, []byte("\nAn edit after the approval.\n")...), 0o644); err != nil {
			t.Fatalf("edit the design: %v", err)
		}
		known, knownOut := adoptEnvelope(t, 0)
		if known.Adoption.Features[0].Class != "blocked" {
			t.Fatalf("the reference feature is not blocked for its fingerprint: %s", knownOut)
		}
		knownResult, knownFeature := adoptKeys(t, knownOut)

		linkDocsRepo(t, linkDocsApprovedByOldRelease)
		result, stdout := adoptEnvelope(t, 0)

		feature := result.Adoption.Features[0]
		if feature.Class != "blocked" {
			t.Fatalf("class = %q, want blocked: %s", feature.Class, stdout)
		}
		linkRequire(t, "the reason", feature.Reason,
			"design.md", "requirements.md", linkDesignRecordsAt, linkRequirementsApprovedAt, "walden reconcile greeting")
		if len(feature.SealableDocs) != 0 {
			t.Errorf("a blocked feature promises documents to seal: %v", feature.SealableDocs)
		}
		gotResult, gotFeature := adoptKeys(t, stdout)
		for key := range gotResult {
			if !knownResult[key] {
				t.Errorf("the result holds the key %q, which a feature blocked for its fingerprint lacks", key)
			}
		}
		for key := range gotFeature {
			if !knownFeature[key] {
				t.Errorf("the feature holds the key %q, which a feature blocked for its fingerprint lacks", key)
			}
		}
	})

	t.Run("apply leaves the documents of the blocked feature", func(t *testing.T) {
		root, written := linkDocsRepo(t, linkDocsApprovedByOldRelease)

		result, stdout := adoptEnvelope(t, 1, "--apply")

		if result.Adoption.Features[0].Class != "blocked" {
			t.Errorf("the run does not report the feature as blocked: %s", stdout)
		}
		linkRequire(t, "the reason of the run", result.Adoption.Features[0].Reason, "design.md", "walden reconcile greeting")
		for _, name := range linkDocumentNames {
			content, err := os.ReadFile(filepath.Join(root, ".walden", "specs", "greeting", name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			if string(content) != written[name] {
				t.Errorf("apply changed %s:\n got %q\nwant %q", name, content, written[name])
			}
		}
	})

	t.Run("the plan warns about a sealed link whose times disagree", func(t *testing.T) {
		linkDocsRepo(t, linkDocsSealedByBackfill)

		result, stdout := adoptEnvelope(t, 0)

		if class := result.Adoption.Features[0].Class; class != "complete" {
			t.Errorf("class = %q, want complete, the class the previous release gives: %s", class, stdout)
		}
		if len(result.Warnings) != 1 {
			t.Fatalf("warnings = %v, want exactly one", result.Warnings)
		}
		if !strings.HasPrefix(result.Warnings[0], "greeting: ") {
			t.Errorf("the warning does not begin with the feature name: %s", result.Warnings[0])
		}
		linkRequire(t, "the warning", result.Warnings[0],
			"design.md", "requirements.md", linkDesignRecordsAt, linkRequirementsApprovedAt,
			"without a review", "approve them again")
		linkRequire(t, "the next action", result.NextAction, "warnings")

		text, _ := rerunCommand(t, 0, "adopt", "greeting")
		linkRequire(t, "the text output", text, "Warnings:", "greeting: design.md", linkDesignRecordsAt, linkRequirementsApprovedAt)

		// An apply run has nothing to do for this feature and names the link too.
		applied, appliedOut := adoptEnvelope(t, 0, "--apply")
		if len(applied.Warnings) == 0 || applied.Warnings[0] != result.Warnings[0] {
			t.Errorf("the apply run does not carry the warning of the plan first: %s", appliedOut)
		}
	})
}

func TestRunAdoptLegacyLinkRemedies(t *testing.T) {
	t.Run("a blocked feature is reconciled and approved again", func(t *testing.T) {
		linkDocsRepo(t, linkDocsApprovedByOldRelease)
		if result, stdout := adoptEnvelope(t, 0); result.Adoption.Features[0].Class != "blocked" {
			t.Fatalf("the feature is not blocked before the remedy: %s", stdout)
		}

		rerunCommand(t, 0, "reconcile", "greeting")
		for _, phase := range []string{"requirements", "design", "tasks"} {
			rerunCommand(t, 0, "review", "open", "greeting", "--phase", phase)
			rerunCommand(t, 0, "review", "approve", "greeting", "--phase", phase)
		}

		result, stdout := adoptEnvelope(t, 0)
		if result.Adoption.Features[0].Class == "blocked" {
			t.Errorf("the feature is still blocked after the remedy: %s", stdout)
		}
		if len(result.Warnings) != 0 {
			t.Errorf("the plan warns after the remedy: %v", result.Warnings)
		}
		linkStatusFresh(t)
	})

	t.Run("a reported chain is approved again", func(t *testing.T) {
		linkDocsRepo(t, linkDocsSealedByBackfill)
		if result, stdout := adoptEnvelope(t, 0); len(result.Warnings) != 1 {
			t.Fatalf("the plan does not report the chain before the remedy: %s", stdout)
		}

		// The approved documents below the requirements: the last one is
		// opened first, the first one is approved first.
		rerunCommand(t, 0, "review", "open", "greeting", "--phase", "tasks")
		rerunCommand(t, 0, "review", "open", "greeting", "--phase", "design")
		rerunCommand(t, 0, "review", "approve", "greeting", "--phase", "design")
		rerunCommand(t, 0, "review", "approve", "greeting", "--phase", "tasks")

		result, stdout := adoptEnvelope(t, 0)
		if len(result.Warnings) != 0 {
			t.Errorf("the plan still warns after the new approvals: %s", stdout)
		}
		linkStatusFresh(t)
	})
}

func TestAdoptHelpNamesLinks(t *testing.T) {
	stdout, _ := rerunCommand(t, 0, "adopt", "--help")

	// The block with its remedy, and the report.
	linkRequire(t, "the help", stdout,
		"blocks its feature",
		"upstream approval time",
		"nothing in that feature is sealed",
		"walden reconcile <feature>",
		"The plan warns",
		"without a review")
	// What the help already carried stays.
	linkRequire(t, "the help", stdout, "adopt [<feature>] [--apply] [--json]", "--apply", "--json")
}

func TestRunAdoptReportsChainApprovedAgain(t *testing.T) {
	linkDocsRepo(t, linkDocsApprovedAgain)

	result, stdout := adoptEnvelope(t, 0)

	// v0.15.2 plans this chain as complete and says nothing about it.
	if class := result.Adoption.Features[0].Class; class != "complete" {
		t.Errorf("class = %q, want complete, the class the previous release gives: %s", class, stdout)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", result.Warnings)
	}
	if !strings.HasPrefix(result.Warnings[0], "greeting: design.md") {
		t.Errorf("the warning does not begin with the feature and the downstream document: %s", result.Warnings[0])
	}
	linkRequire(t, "the warning", result.Warnings[0],
		"requirements.md", linkApprovedAgainRecordsAt, linkApprovedAgainApprovedAt,
		"requirements.md was approved again without a change",
		"a backfill by v0.10.0 to v0.15.2",
		"approve them again")
	linkRequire(t, "the next action", result.NextAction, "adoption guide", "warnings")

	// The approved documents below the requirements: the last one is opened
	// first, the first one is approved first.
	rerunCommand(t, 0, "review", "open", "greeting", "--phase", "tasks")
	rerunCommand(t, 0, "review", "open", "greeting", "--phase", "design")
	rerunCommand(t, 0, "review", "approve", "greeting", "--phase", "design")
	rerunCommand(t, 0, "review", "approve", "greeting", "--phase", "tasks")

	after, afterOut := adoptEnvelope(t, 0)
	if len(after.Warnings) != 0 {
		t.Errorf("the plan still warns after the new approvals: %s", afterOut)
	}
	linkStatusFresh(t)
}

func TestAdoptHelpNamesBothHistories(t *testing.T) {
	stdout, _ := rerunCommand(t, 0, "adopt", "--help")

	// The two histories that leave a sealed link whose times differ.
	linkRequire(t, "the help", stdout,
		"the upstream was approved again without a change",
		"a backfill by v0.10.0 to v0.15.2")
	// The block and its remedy stay.
	linkRequire(t, "the help", stdout, "blocks its feature", "walden reconcile <feature>")
}
