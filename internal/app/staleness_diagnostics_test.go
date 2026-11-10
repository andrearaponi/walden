package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/testutil"
)

// diagnosticsRepo is a committed repository with three code files, whose
// leaves 1.1 and 1.2 are completed at HEAD. Git reads no global or system
// configuration here, so the machine's commit signing, hooks, file-system
// monitor and line-ending conversion cannot reach the test; the git started
// by the commands under test inherits the same environment. Tests that use
// this helper set process environment and are not parallel.
func diagnosticsRepo(t *testing.T) (root, head string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root = setupReleasableFeature(t, verifyTaskFeature)
	diagnosticsFile(t, root, "main.go", "package main\n")
	diagnosticsFile(t, root, "old.go", "package main // old\n")
	diagnosticsFile(t, root, ".gitignore", "*.log\n")
	integrityGit(t, root, "add", "-A")
	integrityGit(t, root, "commit", "-qm", "code")
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"}))
	integrityMustCLI(t, "task", "complete-all", verifyTaskFeature)
	return root, integrityGit(t, root, "rev-parse", "HEAD")
}

func diagnosticsFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// statusText runs `evidence status` in text mode.
func statusText(t *testing.T) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"evidence", "status", verifyTaskFeature}, &stdout, &stderr); code != 0 {
		t.Fatalf("evidence status exited %d: %s%s", code, stdout.String(), stderr.String())
	}
	return stdout.String()
}

// statusEntries runs `evidence status --json` and returns its entries by task.
func statusEntries(t *testing.T) map[string]output.EvidenceStatus {
	t.Helper()
	result := integrityMustCLI(t, "evidence", "status", verifyTaskFeature)
	entries := map[string]output.EvidenceStatus{}
	for _, entry := range result.Evidence {
		entries[entry.TaskID] = entry
	}
	return entries
}

// rewriteLedger edits the recorded tasks of the ledger as a hand would.
func rewriteLedger(t *testing.T, root string, change func(tasks map[string]any)) {
	t.Helper()
	path := evidence.DocumentPath(root, verifyTaskFeature)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse ledger: %v", err)
	}
	change(document["tasks"].(map[string]any))
	data, err = json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write ledger: %v", err)
	}
}

func codeExplanation(t *testing.T, entries map[string]output.EvidenceStatus, id string) *evidence.CodeExplanation {
	t.Helper()
	entry := entries[id]
	if entry.Explanation == nil || entry.Explanation.Code == nil {
		t.Fatalf("task %s (%s) carries no code explanation", id, entry.State)
	}
	return entry.Explanation.Code
}

// linesOf returns the text lines that follow a task's entry line, up to the
// next entry.
func linesOf(text, id string) []string {
	var lines []string
	inside := false
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "- "+id+": "):
			inside = true
		case strings.HasPrefix(line, "- "):
			inside = false
		case inside:
			lines = append(lines, line)
		}
	}
	return lines
}

func TestEvidenceStatusExplainsStaleCode(t *testing.T) {
	t.Run("paths", func(t *testing.T) {
		root, head := diagnosticsRepo(t)

		// One file changed in a new commit, one untracked file added, one
		// tracked file deleted.
		diagnosticsFile(t, root, "main.go", "package main // edited\n")
		integrityGit(t, root, "commit", "-qam", "edit")
		diagnosticsFile(t, root, "notes.txt", "notes\n")
		if err := os.Remove(filepath.Join(root, "old.go")); err != nil {
			t.Fatal(err)
		}

		want := "  since commit " + head + ": changed main.go, added notes.txt, removed old.go"
		text := statusText(t)
		for _, id := range []string{"1.1", "1.2"} {
			if !strings.Contains(strings.Join(linesOf(text, id), "\n"), want) {
				t.Fatalf("task %s: want the line %q in:\n%s", id, want, text)
			}
		}
		if count := strings.Count(text, "since commit"); count != 2 {
			t.Fatalf("want one explanation for each stale task, got %d:\n%s", count, text)
		}
	})

	t.Run("identity-mismatch", func(t *testing.T) {
		root, _ := diagnosticsRepo(t)
		rewriteLedger(t, root, func(tasks map[string]any) {
			tree := tasks["1.1"].(map[string]any)["tree"].(map[string]any)
			tree["paths"] = map[string]any{"ghost.go": "100644:" + strings.Repeat("0", 40)}
		})
		staleTree(t, root)

		entries := statusEntries(t)
		code := codeExplanation(t, entries, "1.1")
		if code.Unavailable != evidence.ReasonIdentityMismatch || len(code.Paths) != 0 {
			t.Fatalf("explanation = %+v, want %s and no path", code, evidence.ReasonIdentityMismatch)
		}
		// The untouched record still explains itself.
		if other := codeExplanation(t, entries, "1.2"); other.Unavailable != "" || len(other.Paths) != 1 {
			t.Fatalf("task 1.2 explanation = %+v, want its one changed path", other)
		}

		text := statusText(t)
		lines := strings.Join(linesOf(text, "1.1"), "\n")
		if !strings.Contains(lines, "do not reproduce the recorded code identity") || strings.Contains(lines, "since commit") {
			t.Fatalf("task 1.1 must say why, and name no path:\n%s", text)
		}
		if strings.Contains(text, "ghost.go") {
			t.Fatalf("a path from facts that do not check was named:\n%s", text)
		}
	})

	t.Run("commit-missing", func(t *testing.T) {
		root, _ := diagnosticsRepo(t)
		absent := strings.Repeat("1", 40)
		rewriteLedger(t, root, func(tasks map[string]any) {
			tasks["1.1"].(map[string]any)["tree"].(map[string]any)["commit"] = absent
		})
		staleTree(t, root)

		code := codeExplanation(t, statusEntries(t), "1.1")
		if code.Unavailable != evidence.ReasonCommitMissing || len(code.Paths) != 0 || code.RecordedCommit != absent {
			t.Fatalf("explanation = %+v, want %s naming %s and no path", code, evidence.ReasonCommitMissing, absent)
		}
		lines := strings.Join(linesOf(statusText(t), "1.1"), "\n")
		if !strings.Contains(lines, "recorded commit "+absent+" is not in this repository") || strings.Contains(lines, "since commit") {
			t.Fatalf("task 1.1 must name the commit and no path:\n%s", lines)
		}
	})

	t.Run("missing-facts", func(t *testing.T) {
		root, head := diagnosticsRepo(t)
		rewriteLedger(t, root, func(tasks map[string]any) {
			// A record written before the facts were kept.
			delete(tasks["1.1"].(map[string]any), "tree")
			// A record written with more differing paths than the limit.
			tasks["1.2"].(map[string]any)["tree"] = map[string]any{"commit": head, "paths_omitted": 137}
		})
		staleTree(t, root)

		entries := statusEntries(t)
		legacy, overLimit := codeExplanation(t, entries, "1.1"), codeExplanation(t, entries, "1.2")
		if legacy.Unavailable != evidence.ReasonNoCommit || overLimit.Unavailable != evidence.ReasonPathsNotKept {
			t.Fatalf("reasons = %q and %q, want %q and %q", legacy.Unavailable, overLimit.Unavailable, evidence.ReasonNoCommit, evidence.ReasonPathsNotKept)
		}
		if len(legacy.Paths) != 0 || len(overLimit.Paths) != 0 {
			t.Fatalf("neither record can name a path: %+v, %+v", legacy, overLimit)
		}

		text := statusText(t)
		first, second := strings.Join(linesOf(text, "1.1"), "\n"), strings.Join(linesOf(text, "1.2"), "\n")
		if !strings.Contains(first, "keeps no commit") || !strings.Contains(first, "walden verify "+verifyTaskFeature+" records it") {
			t.Fatalf("task 1.1 must name the missing fact and what records it:\n%s", first)
		}
		if !strings.Contains(second, "(137)") || !strings.Contains(second, "walden verify "+verifyTaskFeature+" records them") {
			t.Fatalf("task 1.2 must name the missing fact and what records it:\n%s", second)
		}
		if strings.Contains(text, "since commit") {
			t.Fatalf("no path can be named:\n%s", text)
		}
	})

	t.Run("display-limit", func(t *testing.T) {
		root, head := diagnosticsRepo(t)
		names := []string{}
		for i := 0; i < 10; i++ {
			name := fmt.Sprintf("p%02d.txt", i)
			diagnosticsFile(t, root, name, name+"\n")
			names = append(names, "added "+name)
		}

		atLimit := "  since commit " + head + ": " + strings.Join(names, ", ")
		text := statusText(t)
		if !strings.Contains(text, atLimit+"\n") || strings.Contains(text, " more") {
			t.Fatalf("at the limit every path is shown and nothing is omitted, want %q in:\n%s", atLimit, text)
		}

		diagnosticsFile(t, root, "p10.txt", "p10\n")
		text = statusText(t)
		if !strings.Contains(text, atLimit+", +1 more\n") || strings.Contains(text, "p10.txt") {
			t.Fatalf("over the limit the first ten in path order and the number omitted, want %q in:\n%s", atLimit+", +1 more", text)
		}
		if paths := codeExplanation(t, statusEntries(t), "1.1").Paths; len(paths) != 11 || paths[10].Path != "p10.txt" {
			t.Fatalf("the JSON result carries every path, got %d", len(paths))
		}
	})

	t.Run("json", func(t *testing.T) {
		root, head := diagnosticsRepo(t)

		// A verified entry has nothing to explain and gains no field.
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"evidence", "status", verifyTaskFeature, "--json"}, &stdout, &stderr); code != 0 {
			t.Fatalf("evidence status exited %d: %s", code, stderr.String())
		}
		if strings.Contains(stdout.String(), `"explanation"`) {
			t.Fatalf("a verified entry carries an explanation:\n%s", stdout.String())
		}

		staleTree(t, root)
		entries := statusEntries(t)
		for _, id := range []string{"1.1", "1.2"} {
			entry := entries[id]
			code := codeExplanation(t, entries, id)
			if code.RecordedCommit != head || code.Unavailable != "" {
				t.Fatalf("task %s: explanation = %+v, want the paths since %s", id, code, head)
			}
			if want := []evidence.PathChange{{Path: "new-code", Kind: evidence.KindAdded}}; !reflect.DeepEqual(code.Paths, want) {
				t.Fatalf("task %s: paths = %v, want %v", id, code.Paths, want)
			}
			// The fields the entry had before keep their names and values.
			if entry.State != evidence.StateStaleCode || entry.CodeFreshness != "stale" || entry.Passed == nil || !*entry.Passed {
				t.Fatalf("task %s: earlier fields changed: %+v", id, entry)
			}
			if entry.RecordedIdentity == "" || entry.CurrentIdentity == "" || entry.RecordedIdentity == entry.CurrentIdentity {
				t.Fatalf("task %s: identities changed: %q, %q", id, entry.RecordedIdentity, entry.CurrentIdentity)
			}
			if len(entry.Gaps) != 1 || entry.Gaps[0].Kind != "code-stale" {
				t.Fatalf("task %s: gaps changed: %+v", id, entry.Gaps)
			}
		}
	})
}

const specDiagnosticsRequirements = `---
status: approved
approved_at: 2026-03-22T07:00:00Z
last_modified: 2026-03-22T07:00:00Z
---

# Requirements Document

### R1 Parser

#### Acceptance Criteria

1. ` + "`R1.AC1`" + ` WHEN input arrives, the system SHALL parse it.
   - Acceptance check: a parsed tree is returned.

### R2 Errors

#### Acceptance Criteria

1. ` + "`R2.AC1`" + ` IF input is empty, THEN the system SHALL reject it.
   - Acceptance check: an error names the empty input.

## Non-Functional Requirements

- ` + "`NFR1`" + ` Parsing stays deterministic.
`

const specDiagnosticsDesign = `---
status: approved
approved_at: 2026-03-22T07:10:00Z
last_modified: 2026-03-22T07:10:00Z
source_requirements_approved_at: 2026-03-22T07:00:00Z
---

# Feature Design
`

const specDiagnosticsTasks = `---
status: approved
approved_at: 2026-03-22T07:20:00Z
last_modified: 2026-03-22T07:20:00Z
source_design_approved_at: 2026-03-22T07:10:00Z
---

# Implementation Plan

- [ ] 1. Build parser
  - [ ] 1.1 Implement parser
    - Requirements: ` + "`R1.AC1`, `R2.AC1`, `NFR1`" + `
    - Design: Task Store
    - Verification:
      - command: ["go", "test", "./internal/spec"]
  - [ ] 1.2 Keep it deterministic
    - Requirements: ` + "`NFR1`" + `
    - Design: Task Store
    - Verification:
      - command: ["go", "test", "./internal/workflow"]
`

// specDiagnosticsRepo is a committed repository whose task 1.1 names three
// statements and whose task 1.2 names one, both completed.
func specDiagnosticsRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := chdirContract(t)
	writeStatusFeatureFile(t, root, verifyTaskFeature, "requirements.md", specDiagnosticsRequirements)
	writeStatusFeatureFile(t, root, verifyTaskFeature, "design.md", specDiagnosticsDesign)
	writeStatusFeatureFile(t, root, verifyTaskFeature, "tasks.md", specDiagnosticsTasks)
	gitify(t, root)
	overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"}))
	integrityMustCLI(t, "task", "complete-all", verifyTaskFeature)
	return root
}

// reapprove edits a document and seals the result, as an approval of the
// changed content does.
func reapprove(t *testing.T, root, name string, edit func(content string) string) {
	t.Helper()
	path := filepath.Join(root, ".walden", "specs", verifyTaskFeature, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	edited := edit(string(data))
	if edited == string(data) {
		t.Fatalf("the edit left %s unchanged", name)
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	stampSpecFingerprint(t, root, verifyTaskFeature, name)
}

// rewordRequirements rewords R1.AC1, removes R2.AC1 and adds R3.AC1.
func rewordRequirements(content string) string {
	content = strings.Replace(content, "the system SHALL parse it.", "the system SHALL parse it at once.", 1)
	start := strings.Index(content, "### R2 Errors")
	end := strings.Index(content, "## Non-Functional Requirements")
	added := "### R3 Speed\n\n#### Acceptance Criteria\n\n1. `R3.AC1` The system SHALL answer within a second.\n   - Acceptance check: a timed call returns in time.\n\n"
	return content[:start] + added + content[end:]
}

func specExplanation(t *testing.T, entries map[string]output.EvidenceStatus, id string) *evidence.SpecExplanation {
	t.Helper()
	entry := entries[id]
	if entry.Explanation == nil || entry.Explanation.Spec == nil {
		t.Fatalf("task %s (%s) carries no specification explanation", id, entry.State)
	}
	return entry.Explanation.Spec
}

func TestEvidenceStatusExplainsStaleSpec(t *testing.T) {
	t.Run("approvals", func(t *testing.T) {
		// A changed design, approved again, concerns every task.
		root := specDiagnosticsRepo(t)
		reapprove(t, root, "design.md", func(content string) string { return content + "\nA later decision.\n" })

		entries := statusEntries(t)
		text := statusText(t)
		for _, id := range []string{"1.1", "1.2"} {
			if entries[id].State != evidence.StateStaleSpec {
				t.Fatalf("task %s is %s, want stale-spec", id, entries[id].State)
			}
			spec := specExplanation(t, entries, id)
			if !reflect.DeepEqual(spec.Approvals, []string{evidence.ApprovalDesign}) {
				t.Fatalf("task %s: approvals = %v, want the design", id, spec.Approvals)
			}
			lines := strings.Join(linesOf(text, id), "\n")
			if !strings.Contains(lines, "  changed approvals: design") || strings.Contains(lines, "changed statements") {
				t.Fatalf("task %s must name the design and no statement:\n%s", id, lines)
			}
		}

		// A changed task definition concerns that task alone.
		root = specDiagnosticsRepo(t)
		reapprove(t, root, "tasks.md", func(content string) string {
			return strings.Replace(content, `["go", "test", "./internal/workflow"]`, `["go", "test", "-count=1", "./internal/workflow"]`, 1)
		})

		entries = statusEntries(t)
		if spec := specExplanation(t, entries, "1.2"); !reflect.DeepEqual(spec.Approvals, []string{evidence.ApprovalTask}) {
			t.Fatalf("task 1.2: approvals = %v, want its own definition", spec.Approvals)
		}
		if sibling := entries["1.1"]; sibling.State != evidence.StateVerified || sibling.Explanation != nil {
			t.Fatalf("task 1.1 must stay as it was, got %s with %+v", sibling.State, sibling.Explanation)
		}
		if lines := strings.Join(linesOf(statusText(t), "1.2"), "\n"); !strings.Contains(lines, "  changed approvals: task definition") {
			t.Fatalf("task 1.2 must name its definition:\n%s", lines)
		}
	})

	t.Run("statements", func(t *testing.T) {
		root := specDiagnosticsRepo(t)
		reapprove(t, root, "requirements.md", rewordRequirements)

		entries := statusEntries(t)
		named := specExplanation(t, entries, "1.1")
		want := []evidence.StatementChange{{ID: "R1.AC1", Kind: evidence.KindChanged}, {ID: "R2.AC1", Kind: evidence.KindRemoved}}
		if !reflect.DeepEqual(named.Approvals, []string{evidence.ApprovalRequirements}) || !reflect.DeepEqual(named.Statements, want) || named.StatementsUnchanged {
			t.Fatalf("task 1.1: explanation = %+v, want %v", named, want)
		}
		untouched := specExplanation(t, entries, "1.2")
		if !untouched.StatementsUnchanged || len(untouched.Statements) != 0 {
			t.Fatalf("task 1.2 named only NFR1, which did not change: %+v", untouched)
		}

		text := statusText(t)
		first, second := strings.Join(linesOf(text, "1.1"), "\n"), strings.Join(linesOf(text, "1.2"), "\n")
		for _, line := range []string{"  changed approvals: requirements", "  changed statements: R1.AC1 changed, R2.AC1 no longer defined"} {
			if !strings.Contains(first, line) {
				t.Fatalf("task 1.1: want the line %q in:\n%s", line, first)
			}
		}
		if !strings.Contains(second, "  changed statements: none of the statements this task names") {
			t.Fatalf("task 1.2 must say that none of its statements changed:\n%s", second)
		}
	})

	t.Run("no-fingerprints", func(t *testing.T) {
		root := specDiagnosticsRepo(t)
		rewriteLedger(t, root, func(tasks map[string]any) {
			delete(tasks["1.1"].(map[string]any), "statements")
		})
		reapprove(t, root, "requirements.md", rewordRequirements)

		spec := specExplanation(t, statusEntries(t), "1.1")
		if spec.Unavailable != evidence.ReasonNoStatementFingerprints || len(spec.Statements) != 0 || spec.StatementsUnchanged {
			t.Fatalf("explanation = %+v, want %s and no statement", spec, evidence.ReasonNoStatementFingerprints)
		}
		lines := strings.Join(linesOf(statusText(t), "1.1"), "\n")
		if !strings.Contains(lines, "keeps no statement fingerprints") || !strings.Contains(lines, "walden verify "+verifyTaskFeature+" records them") {
			t.Fatalf("task 1.1 must name the missing fact and what records it:\n%s", lines)
		}
		if strings.Contains(lines, "R1.AC1") || strings.Contains(lines, "R2.AC1") {
			t.Fatalf("no statement can be named:\n%s", lines)
		}
	})

	t.Run("json", func(t *testing.T) {
		root := specDiagnosticsRepo(t)
		reapprove(t, root, "requirements.md", rewordRequirements)

		var stdout, stderr bytes.Buffer
		if code := Run([]string{"evidence", "status", verifyTaskFeature, "--json"}, &stdout, &stderr); code != 0 {
			t.Fatalf("evidence status exited %d: %s", code, stderr.String())
		}
		var wire struct {
			Result struct {
				Evidence []map[string]any `json:"evidence"`
			} `json:"result"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &wire); err != nil {
			t.Fatalf("unmarshal envelope: %v", err)
		}
		first := wire.Result.Evidence[0]
		spec := first["explanation"].(map[string]any)["spec"].(map[string]any)
		if !reflect.DeepEqual(spec["approvals"], []any{"requirements"}) {
			t.Fatalf("approvals on the wire = %v", spec["approvals"])
		}
		wantStatements := []any{
			map[string]any{"id": "R1.AC1", "kind": "changed"},
			map[string]any{"id": "R2.AC1", "kind": "removed"},
		}
		if !reflect.DeepEqual(spec["statements"], wantStatements) {
			t.Fatalf("statements on the wire = %v, want %v", spec["statements"], wantStatements)
		}
		second := wire.Result.Evidence[1]["explanation"].(map[string]any)["spec"].(map[string]any)
		if second["statements_unchanged"] != true {
			t.Fatalf("task 1.2 on the wire = %v, want statements_unchanged", second)
		}

		// The fields the entry had before keep their names and values.
		if first["task_id"] != "1.1" || first["state"] != "stale-spec" || first["passed"] != true {
			t.Fatalf("earlier fields changed: %v", first)
		}
		gaps := first["gaps"].([]any)
		if len(gaps) == 0 || gaps[0].(map[string]any)["kind"] != "chain-changed" {
			t.Fatalf("gaps changed: %v", gaps)
		}
	})
}

// diagnosticsTreeBytes digests every file under root with its mode, the git index and
// the ignore files included, leaving out only the object store and git's
// other internals.
func diagnosticsTreeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if strings.HasPrefix(rel, ".git/") && rel != ".git/info" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(rel, ".git/") && rel != ".git/index" && rel != ".git/info/exclude" {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = fmt.Sprintf("%v:%x", info.Mode(), sha256.Sum256(data))
		return nil
	})
	if err != nil {
		t.Fatalf("read the tree: %v", err)
	}
	return files
}

func TestEvidenceStatusCurrentIdentity(t *testing.T) {
	t.Run("inputs", func(t *testing.T) {
		root, head := diagnosticsRepo(t)

		clean := statusText(t)
		if want := "Current identity: commit " + head + ", no uncommitted path\n"; !strings.Contains(clean, want) {
			t.Fatalf("a clean tree names its commit and no path, want %q in:\n%s", want, clean)
		}
		if strings.Index(clean, "Current identity:") > strings.Index(clean, "Evidence:") {
			t.Fatalf("the identity comes before the evidence it explains:\n%s", clean)
		}

		// A forgotten edit, a deleted file, a staged file and an untracked
		// copy are inputs of the identity; an ignored file and a file under
		// .walden/ are not.
		diagnosticsFile(t, root, "main.go", "package main // edited\n")
		if err := os.Remove(filepath.Join(root, "old.go")); err != nil {
			t.Fatal(err)
		}
		diagnosticsFile(t, root, "staged.txt", "staged\n")
		integrityGit(t, root, "add", "staged.txt")
		diagnosticsFile(t, root, "vendor/copy/SKILL.md", "a copy\n")
		diagnosticsFile(t, root, "vendor/copy/notes.md", "notes\n")
		diagnosticsFile(t, root, "debug.log", "ignored\n")
		diagnosticsFile(t, root, ".walden/scratch.txt", "outside the identity\n")

		text := statusText(t)
		want := strings.Join([]string{
			"Current identity: commit " + head + " plus 5 uncommitted path(s)",
			"- changed main.go",
			"- removed old.go",
			"- added staged.txt",
			"- added (untracked) vendor/copy/SKILL.md",
			"- added (untracked) vendor/copy/notes.md",
			"Evidence:",
		}, "\n")
		if !strings.Contains(text, want) {
			t.Fatalf("want the section\n%s\nin:\n%s", want, text)
		}
		for _, outside := range []string{"debug.log", "scratch.txt"} {
			if strings.Contains(text, outside) {
				t.Fatalf("%s is outside the identity and must not be named:\n%s", outside, text)
			}
		}
	})

	t.Run("no-commit", func(t *testing.T) {
		t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
		t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
		root := chdirContract(t)
		writeStatusFeatureFile(t, root, verifyTaskFeature, "requirements.md", specDiagnosticsRequirements)
		writeStatusFeatureFile(t, root, verifyTaskFeature, "design.md", specDiagnosticsDesign)
		writeStatusFeatureFile(t, root, verifyTaskFeature, "tasks.md", specDiagnosticsTasks)
		diagnosticsFile(t, root, "main.go", "package main\n")
		integrityGit(t, root, "init", "-q", "-b", "main")
		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"}))
		integrityMustCLI(t, "task", "complete-all", verifyTaskFeature)

		text := statusText(t)
		want := "Current identity: no commit yet, 1 uncommitted path(s)\n- added (untracked) main.go\nEvidence:"
		if !strings.Contains(text, want) {
			t.Fatalf("want the section\n%s\nin:\n%s", want, text)
		}
		current := integrityMustCLI(t, "evidence", "status", verifyTaskFeature).CurrentTree
		if current == nil || current.Commit != "" || len(current.Paths) != 1 || !current.Paths[0].Untracked {
			t.Fatalf("current tree = %+v, want no commit and the one untracked path", current)
		}
	})

	t.Run("untouched", func(t *testing.T) {
		root, _ := diagnosticsRepo(t)
		diagnosticsFile(t, root, "main.go", "package main // edited\n")
		diagnosticsFile(t, root, "notes.txt", "notes\n")
		if err := os.Remove(filepath.Join(root, "old.go")); err != nil {
			t.Fatal(err)
		}
		diagnosticsFile(t, root, "staged.txt", "staged\n")
		integrityGit(t, root, "add", "staged.txt")

		before := diagnosticsTreeBytes(t, root)
		if _, tracked := before[".git/index"]; !tracked {
			t.Fatal("the snapshot must include the index")
		}
		text := statusText(t)
		statusEntries(t)
		if !strings.Contains(text, "- changed main.go") || !strings.Contains(text, "since commit") {
			t.Fatalf("the command must have named paths for the check to mean something:\n%s", text)
		}
		if after := diagnosticsTreeBytes(t, root); !reflect.DeepEqual(before, after) {
			t.Fatalf("naming paths changed the tree, the index or an ignore file:\nbefore %v\nafter  %v", before, after)
		}
	})

	t.Run("json", func(t *testing.T) {
		root, head := diagnosticsRepo(t)
		clean := integrityMustCLI(t, "evidence", "status", verifyTaskFeature).CurrentTree
		if clean == nil || clean.Commit != head || len(clean.Paths) != 0 {
			t.Fatalf("clean tree = %+v, want commit %s and no path", clean, head)
		}

		diagnosticsFile(t, root, "main.go", "package main // edited\n")
		diagnosticsFile(t, root, "notes.txt", "notes\n")
		if err := os.Remove(filepath.Join(root, "old.go")); err != nil {
			t.Fatal(err)
		}
		current := integrityMustCLI(t, "evidence", "status", verifyTaskFeature).CurrentTree
		want := []evidence.TreePath{
			{Path: "main.go", Kind: evidence.KindChanged},
			{Path: "notes.txt", Kind: evidence.KindAdded, Untracked: true},
			{Path: "old.go", Kind: evidence.KindRemoved},
		}
		if current == nil || current.Commit != head || !reflect.DeepEqual(current.Paths, want) {
			t.Fatalf("current tree = %+v, want commit %s and %v", current, head, want)
		}
	})
}

// durationDiagnosticsRepo is specDiagnosticsRepo with a two-step proof in
// task 1.1, whose recorded steps are then given 200 and 300 milliseconds
// while task 1.2 loses its duration, as a record written before durations
// were kept.
func durationDiagnosticsRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := chdirContract(t)
	tasks := strings.Replace(specDiagnosticsTasks,
		`      - command: ["go", "test", "./internal/spec"]`,
		`      - command: ["go", "test", "./internal/spec"]`+"\n"+`      - command: ["go", "vet", "./internal/spec"]`, 1)
	writeStatusFeatureFile(t, root, verifyTaskFeature, "requirements.md", specDiagnosticsRequirements)
	writeStatusFeatureFile(t, root, verifyTaskFeature, "design.md", specDiagnosticsDesign)
	writeStatusFeatureFile(t, root, verifyTaskFeature, "tasks.md", tasks)
	gitify(t, root)
	overrideCommandRunner(t, testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"},
	))
	integrityMustCLI(t, "task", "complete-all", verifyTaskFeature)

	rewriteLedger(t, root, func(tasks map[string]any) {
		steps := tasks["1.1"].(map[string]any)["steps"].([]any)
		if len(steps) != 2 {
			t.Fatalf("task 1.1 recorded %d step(s), want 2", len(steps))
		}
		steps[0].(map[string]any)["duration_ms"] = 200
		steps[1].(map[string]any)["duration_ms"] = 300
		for _, step := range tasks["1.2"].(map[string]any)["steps"].([]any) {
			delete(step.(map[string]any), "duration_ms")
		}
	})
	return root
}

func TestEvidenceStatusRecordedDuration(t *testing.T) {
	t.Run("sum", func(t *testing.T) {
		durationDiagnosticsRepo(t)
		lines := strings.Join(linesOf(statusText(t), "1.1"), "\n")
		if !strings.Contains(lines, "  recorded duration: 500 ms") {
			t.Fatalf("steps of 200 and 300 milliseconds are a recorded execution of 500:\n%s", lines)
		}
	})

	t.Run("absent", func(t *testing.T) {
		durationDiagnosticsRepo(t)
		text := statusText(t)
		if lines := strings.Join(linesOf(text, "1.2"), "\n"); strings.Contains(lines, "recorded duration") {
			t.Fatalf("a record without durations shows none, never zero:\n%s", text)
		}
		if strings.Count(text, "recorded duration") != 1 {
			t.Fatalf("want the duration of task 1.1 alone:\n%s", text)
		}
	})

	t.Run("json", func(t *testing.T) {
		durationDiagnosticsRepo(t)
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"evidence", "status", verifyTaskFeature, "--json"}, &stdout, &stderr); code != 0 {
			t.Fatalf("evidence status exited %d: %s", code, stderr.String())
		}
		var wire struct {
			Result struct {
				Evidence []map[string]any `json:"evidence"`
			} `json:"result"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &wire); err != nil {
			t.Fatalf("unmarshal envelope: %v", err)
		}
		first, second := wire.Result.Evidence[0], wire.Result.Evidence[1]
		if first["task_id"] != "1.1" || first["recorded_duration_ms"] != float64(500) {
			t.Fatalf("task 1.1 on the wire = %v, want recorded_duration_ms 500", first)
		}
		if _, present := second["recorded_duration_ms"]; present {
			t.Fatalf("task 1.2 has no measurement and must carry no field: %v", second)
		}
		// elapsed_ms keeps its meaning: the duration of a current attempt,
		// which a read view does not have.
		if _, present := first["elapsed_ms"]; present {
			t.Fatalf("a read view gained elapsed_ms: %v", first)
		}
	})
}

// stripFacts removes from the ledger everything this feature records, which
// leaves the ledger an earlier CLI would have written.
func stripFacts(t *testing.T, root string) {
	t.Helper()
	rewriteLedger(t, root, func(tasks map[string]any) {
		for _, task := range tasks {
			record := task.(map[string]any)
			delete(record, "tree")
			delete(record, "statements")
			for _, step := range record["steps"].([]any) {
				delete(step.(map[string]any), "duration_ms")
			}
		}
	})
}

// judgments runs the three commands that read or judge evidence and returns
// each result as JSON, with the explanations left out: what remains is the
// states, the exit statuses and the release verdict.
func judgments(t *testing.T, proofs int) map[string]string {
	t.Helper()
	responses := make([]testutil.Response, proofs)
	for i := range responses {
		responses[i] = testutil.Response{Stdout: "ok"}
	}

	results := map[string]string{}
	for name, args := range map[string][]string{
		"evidence status": {"evidence", "status", verifyTaskFeature},
		"verify --check":  {"verify", verifyTaskFeature, "--check"},
		"release check":   {"release", "check", verifyTaskFeature},
	} {
		overrideCommandRunner(t, testutil.NewFakeRunner(responses...))
		result, code := integrityCLI(t, args...)
		result.CurrentTree = nil
		for i := range result.Evidence {
			result.Evidence[i].Explanation = nil
			result.Evidence[i].RecordedDurationMS = nil
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		results[name] = fmt.Sprintf("exit %d: %s", code, data)
	}
	return results
}

// The facts explain; they decide nothing. Every state, exit status and
// verdict is the same for a ledger that carries them and for the same ledger
// without them, which is the ledger an earlier CLI wrote.
func TestDiagnosticFactsDecideNothing(t *testing.T) {
	scenarios := map[string]func(t *testing.T) string{
		"verified": func(t *testing.T) string {
			root, _ := diagnosticsRepo(t)
			return root
		},
		"stale-code": func(t *testing.T) string {
			root, _ := diagnosticsRepo(t)
			staleTree(t, root)
			return root
		},
		"stale-spec": func(t *testing.T) string {
			root := specDiagnosticsRepo(t)
			reapprove(t, root, "tasks.md", func(content string) string {
				return strings.Replace(content, `["go", "test", "./internal/workflow"]`, `["go", "test", "-count=1", "./internal/workflow"]`, 1)
			})
			return root
		},
		"failed": func(t *testing.T) string {
			root, _ := diagnosticsRepo(t)
			overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{ExitCode: 1}, testutil.Response{ExitCode: 1}))
			if _, code := integrityCLI(t, "verify", verifyTaskFeature, "--all"); code == 0 {
				t.Fatal("failing proofs passed")
			}
			return root
		},
		"pending": func(t *testing.T) string {
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			root := setupReleasableFeature(t, verifyTaskFeature)
			overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}))
			integrityMustCLI(t, "task", "complete", verifyTaskFeature, "1.1")
			return root
		},
	}

	for name, build := range scenarios {
		t.Run(name, func(t *testing.T) {
			root := build(t)
			ledger := string(readLedger(t, root))
			for _, fact := range []string{`"tree"`, `"statements"`, `"duration_ms"`} {
				if !strings.Contains(ledger, fact) {
					t.Fatalf("the ledger must carry %s for the comparison to mean something:\n%s", fact, ledger)
				}
			}

			with := judgments(t, 2)
			if !strings.Contains(with["evidence status"], `"state":"`+name+`"`) {
				t.Fatalf("the scenario does not reach %s: %s", name, with["evidence status"])
			}
			stripFacts(t, root)
			without := judgments(t, 2)

			for command, result := range with {
				if result != without[command] {
					t.Fatalf("%s judged differently with the facts:\nwith    %s\nwithout %s", command, result, without[command])
				}
			}
		})
	}
}

// A ledger is written only to record evidence. Reading one that lacks the
// facts does not add them, and reading one that has them changes nothing.
func TestReadCommandsLeaveLedgerUntouched(t *testing.T) {
	read := func(t *testing.T) {
		t.Helper()
		for _, args := range [][]string{
			{"evidence", "status", verifyTaskFeature},
			{"evidence", "status", verifyTaskFeature, "--json"},
			{"release", "check", verifyTaskFeature},
			{"adopt", verifyTaskFeature},
			{"status", verifyTaskFeature},
			{"task", "status", verifyTaskFeature},
		} {
			var stdout, stderr bytes.Buffer
			Run(args, &stdout, &stderr)
		}
	}

	for _, name := range []string{"without-facts", "with-facts"} {
		t.Run(name, func(t *testing.T) {
			root, _ := diagnosticsRepo(t)
			staleTree(t, root)
			if name == "without-facts" {
				stripFacts(t, root)
			}
			path := evidence.DocumentPath(root, verifyTaskFeature)
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			bytesBefore := readLedger(t, root)

			read(t)

			after, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(bytesBefore, readLedger(t, root)) || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("a read command wrote the ledger")
			}
		})
	}
}

// Everything `evidence status --json` carried before keeps its name and its
// value; the feature only adds.
func TestEvidenceStatusJSONKeepsEarlierFields(t *testing.T) {
	root, _ := diagnosticsRepo(t)
	staleTree(t, root)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"evidence", "status", verifyTaskFeature, "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("evidence status exited %d: %s", code, stderr.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope["schema_version"] != "v0beta1" || envelope["command"] != "evidence-status" || envelope["ok"] != true {
		t.Fatalf("the envelope changed: %v", envelope)
	}

	result := envelope["result"].(map[string]any)
	earlierResult := []string{"summary", "changed_files", "skipped_files", "next_action", "warnings", "evidence", "exit_code"}
	assertKeys(t, "result", result, earlierResult, []string{"current_tree"})
	if result["summary"] != "evidence status for "+verifyTaskFeature+": 2 stale-code" || result["exit_code"] != float64(0) {
		t.Fatalf("the result changed: %v", result)
	}
	if result["next_action"] != "Run walden verify "+verifyTaskFeature+" to re-prove the current code" {
		t.Fatalf("the next action changed: %v", result["next_action"])
	}

	entry := result["evidence"].([]any)[0].(map[string]any)
	earlierEntry := []string{"task_id", "state", "passed", "recorded_identity", "current_identity", "profile", "binding", "code_freshness", "execution", "gaps"}
	assertKeys(t, "entry", entry, earlierEntry, []string{"explanation", "recorded_duration_ms"})
	if entry["task_id"] != "1.1" || entry["state"] != "stale-code" || entry["passed"] != true || entry["code_freshness"] != "stale" {
		t.Fatalf("an earlier field changed its value: %v", entry)
	}
	if entry["binding"].(map[string]any)["state"] != "current" || entry["execution"].(map[string]any)["state"] != "supported" {
		t.Fatalf("an earlier assessment changed: %v", entry)
	}
	gap := entry["gaps"].([]any)[0].(map[string]any)
	if gap["kind"] != "code-stale" || gap["message"] != "recorded and current code identities differ" {
		t.Fatalf("an earlier gap changed: %v", gap)
	}
}

// assertKeys requires every earlier key and allows only the named additions.
func assertKeys(t *testing.T, what string, object map[string]any, earlier, added []string) {
	t.Helper()
	allowed := map[string]bool{}
	for _, key := range earlier {
		allowed[key] = true
		if _, present := object[key]; !present {
			t.Fatalf("%s lost the field %q: %v", what, key, object)
		}
	}
	for _, key := range added {
		allowed[key] = true
	}
	for key := range object {
		if !allowed[key] {
			t.Fatalf("%s gained an unexpected field %q", what, key)
		}
	}
}
