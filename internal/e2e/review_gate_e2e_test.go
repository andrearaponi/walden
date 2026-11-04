package e2e

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const gateRequirementsBody = `# Requirements Document

## Introduction

Review gate fixture.

## Requirements

### R1 Gate

**User Story:** As a tester, I want a document to judge, so that the review gate has one.

#### Acceptance Criteria

1. ` + "`R1.AC1`" + ` WHEN the proof runs, the system SHALL succeed.
   - Acceptance check: the proof exits zero.
`

// specDigests hashes every document of the feature, frontmatter included.
func specDigests(t *testing.T, dir, feature string) map[string][sha256.Size]byte {
	t.Helper()
	digests := map[string][sha256.Size]byte{}
	for _, name := range []string{"requirements.md", "design.md", "tasks.md"} {
		data, err := os.ReadFile(filepath.Join(dir, ".walden", "specs", feature, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		digests[name] = sha256.Sum256(data)
	}
	return digests
}

func requireSpecUnchanged(t *testing.T, dir, feature string, before map[string][sha256.Size]byte) {
	t.Helper()
	for name, digest := range specDigests(t, dir, feature) {
		if digest != before[name] {
			t.Fatalf("%s changed although the review command was refused", name)
		}
	}
}

// TestE2EReviewGateRefusesUnresolvedContent drives both review gates through
// the CLI entry point: unresolved content refuses each command without
// touching the specification, and its removal lets the same command through.
func TestE2EReviewGateRefusesUnresolvedContent(t *testing.T) {
	dir := t.TempDir()
	mustCLI(t, dir, "repo", "init")
	mustCLI(t, dir, "feature", "init", "gate-demo")

	const document = ".walden/specs/gate-demo/requirements.md"
	scaffold, err := os.ReadFile(filepath.Join(dir, document))
	if err != nil {
		t.Fatalf("read scaffold: %v", err)
	}
	end := strings.Index(string(scaffold)[len("---\n"):], "\n---\n")
	if end < 0 {
		t.Fatalf("scaffold without frontmatter: %s", scaffold)
	}
	clean := string(scaffold)[:len("---\n")+end+len("\n---\n")] + "\n" + gateRequirementsBody

	writeFile(t, dir, document, clean+"\nA bare [decision: which store backs this?] stays open.\n")
	before := specDigests(t, dir, "gate-demo")
	out, code := cli(t, dir, "review", "open", "gate-demo", "--phase", "requirements")
	if code == 0 {
		t.Fatalf("review open accepted unresolved content: %s", out)
	}
	requireSpecUnchanged(t, dir, "gate-demo", before)
	for _, fragment := range []string{"requirements.md", "body line", "resolve it, or quote it in a code span"} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("refusal does not carry %q: %s", fragment, out)
		}
	}

	writeFile(t, dir, document, clean)
	mustCLI(t, dir, "review", "open", "gate-demo", "--phase", "requirements")

	inReview, err := os.ReadFile(filepath.Join(dir, document))
	if err != nil {
		t.Fatalf("read requirements in review: %v", err)
	}
	writeFile(t, dir, document, string(inReview)+"\n<!-- assumed: a comment that never closes\n")
	before = specDigests(t, dir, "gate-demo")
	out, code = cli(t, dir, "review", "approve", "gate-demo", "--phase", "requirements")
	if code == 0 {
		t.Fatalf("review approve accepted unresolved content: %s", out)
	}
	requireSpecUnchanged(t, dir, "gate-demo", before)
	if !strings.Contains(out, "close it with -->, or quote it in a code span") {
		t.Fatalf("refusal does not carry the comment's remedy: %s", out)
	}

	writeFile(t, dir, document, string(inReview))
	mustCLI(t, dir, "review", "approve", "gate-demo", "--phase", "requirements")
	if status := mustCLI(t, dir, "status", "gate-demo", "--json"); !strings.Contains(status, `"name": "requirements.md"`) || !strings.Contains(status, `"status": "approved"`) {
		t.Fatalf("requirements not approved after the fix: %s", status)
	}
}
