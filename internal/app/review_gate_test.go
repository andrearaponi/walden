package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

const (
	gateMarkerRemedy  = "resolve it, or quote it in a code span"
	gateCommentRemedy = "close it with -->, or quote it in a code span"
)

// gateRequirementsBody is a requirements body that passes validation.
const gateRequirementsBody = "# Requirements Document\n\n## Introduction\n\nGate fixture.\n\n## Requirements\n\n### R1 Gate\n\n**User Story:** As a tester, I want a document to judge, so that the review gate has one.\n\n#### Acceptance Criteria\n\n1. `R1.AC1` WHEN the proof runs, the system SHALL succeed.\n   - Acceptance check: the proof exits zero.\n"

// gateRequirementsRepo enters an isolated repository holding the feature demo,
// its requirements replaced by gateRequirementsBody.
func gateRequirementsRepo(t *testing.T) string {
	t.Helper()
	root := chdirContract(t)
	for _, args := range [][]string{{"repo", "init"}, {"feature", "init", "demo"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("walden %s failed: %s", strings.Join(args, " "), stderr.String())
		}
	}
	setRequirementsBody(t, root, gateRequirementsBody)
	return root
}

// setRequirementsBody replaces the body of demo's requirements and keeps
// their frontmatter.
func setRequirementsBody(t *testing.T, root, body string) {
	t.Helper()
	path := filepath.Join(root, ".walden", "specs", "demo", "requirements.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	text := string(data)
	end := strings.Index(text[len("---\n"):], "\n---\n")
	if !strings.HasPrefix(text, "---\n") || end < 0 {
		t.Fatalf("requirements without frontmatter: %q", text)
	}
	frontmatter := text[:len("---\n")+end+len("\n---\n")]
	if err := os.WriteFile(path, []byte(frontmatter+"\n"+body), 0o644); err != nil {
		t.Fatalf("write requirements: %v", err)
	}
}

func requirementsBodyLine(t *testing.T, root, fragment string) int {
	t.Helper()
	feature, err := spec.LoadFeature(root, "demo")
	if err != nil {
		t.Fatalf("load feature: %v", err)
	}
	for index, line := range strings.Split(strings.ReplaceAll(feature.Requirements.Body, "\r\n", "\n"), "\n") {
		if strings.Contains(line, fragment) {
			return index + 1
		}
	}
	t.Fatalf("no requirements body line holds %q", fragment)
	return 0
}

// runEnvelope runs a --json invocation and returns its exit code with the
// envelope and its result decoded as generic objects.
func runEnvelope(t *testing.T, args ...string) (int, map[string]json.RawMessage, map[string]json.RawMessage) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	if stderr.Len() != 0 {
		t.Fatalf("stderr not empty in JSON mode: %q", stderr.String())
	}
	var envelope, result map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v (%q)", err, stdout.String())
	}
	if err := json.Unmarshal(envelope["result"], &result); err != nil {
		t.Fatalf("result is not an object: %v", err)
	}
	return code, envelope, result
}

func TestValidateKeepsDraftUnresolved(t *testing.T) {
	root := gateRequirementsRepo(t)
	validate := func() (int, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := Run([]string{"validate", "demo", "--json"}, &stdout, &stderr)
		return code, stdout.String() + stderr.String()
	}

	cleanCode, clean := validate()
	if cleanCode != 0 {
		t.Fatalf("the clean draft does not validate: %s", clean)
	}
	setRequirementsBody(t, root, strings.Replace(gateRequirementsBody, "Gate fixture.", "Gate fixture: [decision: which store backs this?]", 1))
	if !strings.Contains(requirementsBodyLineText(t, root, "Gate fixture"), "[decision:") {
		t.Fatal("the draft does not hold the marker")
	}
	markedCode, marked := validate()
	if markedCode != cleanCode || marked != clean {
		t.Fatalf("a bare marker changed the validation of a draft:\nclean (%d): %s\nmarked (%d): %s", cleanCode, clean, markedCode, marked)
	}
}

func requirementsBodyLineText(t *testing.T, root, fragment string) string {
	t.Helper()
	feature, err := spec.LoadFeature(root, "demo")
	if err != nil {
		t.Fatalf("load feature: %v", err)
	}
	if feature.Requirements.Status != "draft" {
		t.Fatalf("requirements status = %q, want draft", feature.Requirements.Status)
	}
	return strings.Split(feature.Requirements.Body, "\n")[requirementsBodyLine(t, root, fragment)-1]
}

func TestReviewGateHelp(t *testing.T) {
	const want = "An open [decision: …] marker or an HTML comment that never closes refuses the command, naming its body line: resolve the marker or close the comment, or quote it in a code span, a fenced block or a proof line of tasks.md."

	for _, command := range []string{"open", "approve"} {
		t.Run(command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run([]string{"review", command, "--help"}, &stdout, &stderr); code != 0 {
				t.Fatalf("exit = %d, want 0: %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "\n  "+want+"\n") {
				t.Fatalf("help does not hold the statement among its details:\n%s", stdout.String())
			}
		})
	}
}

// TestReviewGateDocs pins the review gates and the quoted text to the
// documentation, checks that no page still presents the release gate as the
// only check, and pins the change to the changelog range above the previous
// release.
func TestReviewGateDocs(t *testing.T) {
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

	require("docs/reference/spec-format.md", read("docs/reference/spec-format.md"),
		"## Unresolved content",
		"`walden review open` and `walden review approve` refuse a document holding either one",
		"text between two equal runs of backticks on one line",
		"fenced code blocks",
		"the proof steps and attributes of `tasks.md`",
		"decision markers inside a terminated HTML comment",
		"hides nothing",
	)
	require("docs/reference/cli.md", read("docs/reference/cli.md"),
		"Refuses out-of-order phases and a document holding unresolved content",
		"Refuses a document holding unresolved content, as `review open` does",
	)
	require("docs/lifecycle.md", read("docs/lifecycle.md"),
		"Both steps refuse a document that still holds",
		"read as the review gates read it",
	)
	require("docs/workflow.md", read("docs/workflow.md"),
		"`review open` and `review approve` refuse a document that still holds one",
	)
	require("docs/agentic.md", read("docs/agentic.md"),
		"the review gates refuse a document that still holds one",
	)

	replaced := []string{
		"blocking at the release gate while unresolved in an approved document",
		"the release gate blocks on decision markers in approved documents",
		"the release gate will not let them ship unresolved",
	}
	err := filepath.WalkDir(filepath.Join(root, "docs"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, sentence := range replaced {
			if strings.Contains(string(data), sentence) {
				t.Errorf("%s still presents the release gate as the only check: %q", path, sentence)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs: %v", err)
	}

	changelog := read("CHANGELOG.md")
	start := strings.Index(changelog, "## [Unreleased]")
	if start < 0 {
		t.Fatal("CHANGELOG.md has no [Unreleased] section")
	}
	end := strings.Index(changelog[start:], "\n## [0.14.3]")
	if end < 0 {
		t.Fatal("CHANGELOG.md has no [0.14.3] section below [Unreleased]")
	}
	require("CHANGELOG.md above [0.14.3]", changelog[start:start+end+1],
		"**The review gates refuse unresolved content.**",
		"**A quoted marker no longer blocks.**",
	)
}

func TestReviewGateJSONEnvelope(t *testing.T) {
	root := gateRequirementsRepo(t)

	// The shape of a failure envelope from before the gate existed.
	_, baselineEnvelope, baselineResult := runEnvelope(t, "review", "open", "demo", "--json")

	unresolved := gateRequirementsBody + "\nA bare [decision: which store backs this?] stays open.\n\n<!-- assumed: a comment that never closes\n"
	requireRefusal := func(t *testing.T, command string, args ...string) {
		t.Helper()
		markerLine := requirementsBodyLine(t, root, "A bare")
		commentLine := requirementsBodyLine(t, root, "assumed: a comment")
		code, envelope, result := runEnvelope(t, args...)
		if code == 0 {
			t.Fatalf("%s accepted a document holding unresolved content", command)
		}
		var ok bool
		var schema, name, summary string
		for key, target := range map[string]any{"ok": &ok, "schema_version": &schema, "command": &name} {
			if err := json.Unmarshal(envelope[key], target); err != nil {
				t.Fatalf("decode %s: %v", key, err)
			}
		}
		if err := json.Unmarshal(result["summary"], &summary); err != nil {
			t.Fatalf("decode summary: %v", err)
		}
		if ok || schema != "v0beta1" || name != command {
			t.Fatalf("envelope = ok %v, schema %q, command %q", ok, schema, name)
		}
		for _, fragment := range []string{"requirements.md", fmt.Sprintf("body line %d (", markerLine), fmt.Sprintf("body line %d (", commentLine), gateMarkerRemedy, gateCommentRemedy} {
			if !strings.Contains(summary, fragment) {
				t.Errorf("summary does not carry %q: %s", fragment, summary)
			}
		}
		if got, want := sortedKeys(envelope), sortedKeys(baselineEnvelope); !slices.Equal(got, want) {
			t.Errorf("envelope keys = %v, want %v", got, want)
		}
		if got, want := sortedKeys(result), sortedKeys(baselineResult); !slices.Equal(got, want) {
			t.Errorf("result keys = %v, want %v", got, want)
		}
	}

	setRequirementsBody(t, root, unresolved)
	requireRefusal(t, "review-open", "review", "open", "demo", "--phase", "requirements", "--json")

	// Approval: a clean document is opened, then gains the content.
	setRequirementsBody(t, root, gateRequirementsBody)
	if code, _, _ := runEnvelope(t, "review", "open", "demo", "--phase", "requirements", "--json"); code != 0 {
		t.Fatal("review open refused clean requirements")
	}
	setRequirementsBody(t, root, unresolved)
	requireRefusal(t, "review-approve", "review", "approve", "demo", "--phase", "requirements", "--json")
}
