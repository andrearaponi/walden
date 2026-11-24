package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/testutil"
)

const subdirectoryFeature = "subdirectory-demo"

// rootLayouts are the two places a Walden root can have in its repository.
// Every test of this file runs the same steps in both: the root at the top
// level is the control of the root below it.
var rootLayouts = []struct{ name, prefix string }{
	{"top level", ""},
	{"below the top level", "pkg/app"},
}

// subdirectoryTasks is a plan with two proofs that read the tree for real: a
// delivery proof that reads notes.txt and a regression proof that reads
// status.txt.
const subdirectoryTasks = `---
status: draft
approved_at:
last_modified: 2026-07-11T13:00:00Z
approved_fingerprint:
source_design_approved_at:
source_design_fingerprint:
---

# Implementation Plan

- [ ] 1. Markers
  - [ ] 1.1 Publish the notes
    - Requirements: ` + "`R1.AC1`" + `
    - Design: Overview
    - Proof: delivery
    - Verification:
      - command: ["grep", "-q", "released", "notes.txt"]
        covers: ["R1.AC1"]
  - [ ] 1.2 Check the status
    - Requirements: ` + "`R1.AC2`" + `
    - Design: Overview
    - Verification:
      - command: ["grep", "-q", "green", "status.txt"]
        covers: ["R1.AC2"]
`

// layoutRepo enters a Walden root that holds a certifiable feature with the
// plan of the caller, in a committed git repository that reads no global or
// system configuration. With an empty prefix the root is the top level;
// otherwise it lies at prefix below it and the repository also holds
// other/file.txt. files are written under the root before the commit.
//
// The fixtures of this package enter one temporary directory and make it
// both the root and the repository, so they cannot put the root below the
// top level; this one is built from the same helpers, each of which takes
// its directory.
func layoutRepo(t *testing.T, prefix, tasks string, files map[string]string) (top, root string) {
	t.Helper()
	for _, tool := range []string{"git", "grep"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not available in PATH")
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	top = chdirContract(t)
	root = filepath.Join(top, filepath.FromSlash(prefix))
	enterRoot(t, root)
	writeStatusFeatureFile(t, root, subdirectoryFeature, "requirements.md", releasableRequirements)
	writeStatusFeatureFile(t, root, subdirectoryFeature, "design.md", releasableDesign)
	writeStatusFeatureFile(t, root, subdirectoryFeature, "tasks.md", tasks)
	for _, phase := range []string{"requirements", "design", "tasks"} {
		integrityMustCLI(t, "review", "open", subdirectoryFeature, "--phase", phase)
		integrityMustCLI(t, "review", "approve", subdirectoryFeature, "--phase", phase)
	}
	for path, content := range files {
		diagnosticsFile(t, root, path, content)
	}
	if prefix != "" {
		diagnosticsFile(t, top, "other/file.txt", "other\n")
	}
	gitify(t, top)
	return top, root
}

// enterRoot makes root the working directory. chdirContract, called first,
// puts the previous one back when the test ends.
func enterRoot(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
}

func commitEverything(t *testing.T, top, message string) string {
	t.Helper()
	integrityGit(t, top, "add", "-A")
	integrityGit(t, top, "commit", "-qm", message)
	return integrityGit(t, top, "rev-parse", "HEAD")
}

func subdirectoryEntries(t *testing.T) (map[string]output.EvidenceStatus, output.Result) {
	t.Helper()
	result := integrityMustCLI(t, "evidence", "status", subdirectoryFeature)
	entries := map[string]output.EvidenceStatus{}
	for _, entry := range result.Evidence {
		entries[entry.TaskID] = entry
	}
	return entries, result
}

func subdirectoryStates(t *testing.T) map[string]string {
	t.Helper()
	entries, _ := subdirectoryEntries(t)
	states := map[string]string{}
	for id, entry := range entries {
		states[id] = entry.State
	}
	return states
}

func subdirectoryRecord(t *testing.T, root, id string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(evidence.DocumentPath(root, subdirectoryFeature))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var ledger struct {
		Tasks map[string]map[string]any `json:"tasks"`
	}
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatalf("parse ledger: %v", err)
	}
	record, exists := ledger.Tasks[id]
	if !exists {
		t.Fatalf("the ledger holds no record for task %s", id)
	}
	return record
}

// releaseVerdict runs release check and returns its status and exit code.
func releaseVerdict(t *testing.T, args ...string) (*output.ReleaseStatus, int) {
	t.Helper()
	result, code := integrityCLI(t, append([]string{"release", "check"}, args...)...)
	if result.Release == nil {
		t.Fatalf("release check %v returned no verdict: %+v", args, result)
	}
	return result.Release, code
}

func releaseStates(t *testing.T, status *output.ReleaseStatus) map[string]string {
	t.Helper()
	states := map[string]string{}
	for _, feature := range status.Features {
		if feature.Feature != subdirectoryFeature {
			continue
		}
		for _, entry := range feature.Evidence {
			states[entry.TaskID] = entry.State
		}
	}
	return states
}

var passingFiles = map[string]string{
	"notes.txt":  "released\n",
	"status.txt": "green\n",
	"gone.txt":   "soon removed\n",
}

// Evidence reacts to an uncommitted change under the root wherever the root
// lies: the regression task reads stale-code in every command that reports
// it, verify starts its proof, and the report and the record name the paths
// that differ from the commit, relative to the root. A file outside the root
// and a file under .walden/ are not among them.
func TestEvidenceFollowsWorkingTree(t *testing.T) {
	for _, layout := range rootLayouts {
		t.Run(layout.name, func(t *testing.T) {
			top, root := layoutRepo(t, layout.prefix, subdirectoryTasks, passingFiles)
			integrityMustCLI(t, "task", "complete", subdirectoryFeature, "1.1")
			integrityMustCLI(t, "task", "complete", subdirectoryFeature, "1.2")
			head := commitEverything(t, top, "evidence")

			entries, committed := subdirectoryEntries(t)
			if entries["1.1"].State != evidence.StateDelivered || entries["1.2"].State != evidence.StateVerified {
				t.Fatalf("states on the committed tree: 1.1 %s, 1.2 %s", entries["1.1"].State, entries["1.2"].State)
			}
			if committed.CurrentTree == nil || committed.CurrentTree.Commit != head || len(committed.CurrentTree.Paths) != 0 {
				t.Fatalf("on a committed tree the report names %+v, want the commit %s and no path", committed.CurrentTree, head)
			}

			// One modified, one untracked and one deleted path under the
			// root. The proof of 1.2 still passes on the modified file.
			diagnosticsFile(t, root, "status.txt", "green\nand more\n")
			diagnosticsFile(t, root, "new.txt", "new\n")
			if err := os.Remove(filepath.Join(root, "gone.txt")); err != nil {
				t.Fatal(err)
			}
			// What must stay out: a change under .walden/ and, below the
			// top level, a change in another directory of the repository.
			diagnosticsFile(t, root, ".walden/lessons.md", "# Lessons\n")
			if layout.prefix != "" {
				diagnosticsFile(t, top, "other/file.txt", "changed\n")
			}

			entries, report := subdirectoryEntries(t)
			if entries["1.2"].State != evidence.StateStaleCode {
				t.Errorf("evidence status: task 1.2 reads %s, want stale-code", entries["1.2"].State)
			}
			if entries["1.1"].State != evidence.StateDelivered {
				t.Errorf("evidence status: the delivered task reads %s", entries["1.1"].State)
			}
			wantPaths := []evidence.TreePath{
				{Path: "gone.txt", Kind: evidence.KindRemoved},
				{Path: "new.txt", Kind: evidence.KindAdded, Untracked: true},
				{Path: "status.txt", Kind: evidence.KindChanged},
			}
			if report.CurrentTree == nil || report.CurrentTree.Commit != head || !reflect.DeepEqual(report.CurrentTree.Paths, wantPaths) {
				t.Errorf("current_tree = %+v, want the commit %s and %+v", report.CurrentTree, head, wantPaths)
			}
			text := subdirectoryText(t, "evidence", "status", subdirectoryFeature)
			for _, line := range []string{
				"Current identity: commit " + head + " plus 3 uncommitted path(s)",
				"- removed gone.txt",
				"- added (untracked) new.txt",
				"- changed status.txt",
			} {
				if !strings.Contains(text, line+"\n") {
					t.Errorf("the text of evidence status lacks %q:\n%s", line, text)
				}
			}
			for _, absent := range []string{"other/file.txt", "lessons.md", "pkg/app/"} {
				if strings.Contains(text, absent) {
					t.Errorf("the text of evidence status names %q:\n%s", absent, text)
				}
			}

			traced := integrityMustCLI(t, "trace", subdirectoryFeature)
			if traced.Trace == nil {
				t.Fatal("trace returned no report")
			}
			for _, leaf := range traced.Trace.Leaves {
				if want := map[string]string{"1.1": evidence.StateDelivered, "1.2": evidence.StateStaleCode}[leaf.ID]; leaf.State != want {
					t.Errorf("trace: task %s reads %s, want %s", leaf.ID, leaf.State, want)
				}
			}
			verdict, code := releaseVerdict(t, subdirectoryFeature)
			if code == 0 || verdict.Releasable || releaseStates(t, verdict)["1.2"] != evidence.StateStaleCode {
				t.Errorf("release check: exit %d, releasable %t, states %v", code, verdict.Releasable, releaseStates(t, verdict))
			}
			status := integrityMustCLI(t, "task", "status", subdirectoryFeature)
			if warnings := strings.Join(status.Warnings, "\n"); !strings.Contains(warnings, "1.2 (stale-code)") {
				t.Errorf("task status does not warn about the stale task: %q", warnings)
			}

			// verify starts the proof of the stale task, and of no other.
			proofs := overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}))
			integrityMustCLI(t, "verify", subdirectoryFeature, "--check")
			calls := proofs.Calls()
			if len(calls) != 1 || calls[0].Name != "grep" || !reflect.DeepEqual(calls[0].Args, []string{"-q", "green", "status.txt"}) {
				t.Errorf("verify started %+v, want the proof of task 1.2 alone", calls)
			}

			// The record verify writes on this tree keeps the three paths,
			// by their path from the root: two with the identity of their
			// content, which git itself gives here, and one as absent.
			commandRunner = shell.NewExecRunner()
			integrityMustCLI(t, "verify", subdirectoryFeature)
			tree, _ := subdirectoryRecord(t, root, "1.2")["tree"].(map[string]any)
			wantRecorded := map[string]any{
				"gone.txt":   "",
				"new.txt":    "100644:" + integrityGit(t, root, "hash-object", "--", "new.txt"),
				"status.txt": "100644:" + integrityGit(t, root, "hash-object", "--", "status.txt"),
			}
			if tree["commit"] != head || !reflect.DeepEqual(tree["paths"], wantRecorded) {
				t.Errorf("the record keeps commit %v and paths %v, want %s and %v", tree["commit"], tree["paths"], head, wantRecorded)
			}
			if states := subdirectoryStates(t); states["1.2"] != evidence.StateVerified {
				t.Errorf("after verify on this tree task 1.2 reads %s", states["1.2"])
			}
		})
	}
}

// subdirectoryText runs a command in text mode and returns what it printed.
func subdirectoryText(t *testing.T, args ...string) string {
	t.Helper()
	var stdout, stderr strings.Builder
	Run(args, &stdout, &stderr)
	return stdout.String() + stderr.String()
}

// A proof that passes only because of uncommitted content is recorded with
// the identity of the tree it ran on, and a delivery proof is not attested on
// such a tree. When the content is discarded the evidence is stale, verify
// fails and the gate refuses, with and without --strict: before this was
// fixed, a root below the top level kept reading verified and delivered and
// the gate certified a commit on which both proofs fail.
func TestProofOnUncommittedContent(t *testing.T) {
	for _, layout := range rootLayouts {
		t.Run(layout.name, func(t *testing.T) {
			top, root := layoutRepo(t, layout.prefix, subdirectoryTasks, map[string]string{
				"notes.txt":  "draft\n",
				"status.txt": "red\n",
			})
			committedIdentity := func() string {
				entries, _ := subdirectoryEntries(t)
				return entries["1.2"].CurrentIdentity
			}()

			// The proofs fail on the commit and pass on the working tree.
			diagnosticsFile(t, root, "notes.txt", "released\n")
			diagnosticsFile(t, root, "status.txt", "green\n")
			integrityMustCLI(t, "task", "complete", subdirectoryFeature, "1.1")
			integrityMustCLI(t, "task", "complete", subdirectoryFeature, "1.2")

			entries, _ := subdirectoryEntries(t)
			delivery := entries["1.1"]
			if delivery.State != evidence.StateVerified || !delivery.Delivery || delivery.Attestation != nil {
				t.Errorf("the delivery task reads %s with attestation %+v, want verified and none", delivery.State, delivery.Attestation)
			}
			if delivery.Explanation == nil || delivery.Explanation.Attestation == nil || delivery.Explanation.Attestation.Unavailable != evidence.ReasonUncommittedPaths {
				t.Errorf("evidence status does not say that the proof ran on uncommitted paths: %+v", delivery.Explanation)
			}
			for _, id := range []string{"1.1", "1.2"} {
				record := subdirectoryRecord(t, root, id)
				if _, attested := record["attestation"]; attested {
					t.Errorf("the record of task %s holds an attestation", id)
				}
				if identity, _ := record["code_identity"].(string); identity == "" || identity == committedIdentity || identity != entries[id].CurrentIdentity {
					t.Errorf("the record of task %s carries %q: the commit gives %q and the tree the proof ran on %q", id, identity, committedIdentity, entries[id].CurrentIdentity)
				}
			}

			// The content is discarded and what walden wrote is committed.
			integrityGit(t, root, "checkout", "--", "notes.txt", "status.txt")
			commitEverything(t, top, "evidence")
			if states := subdirectoryStates(t); states["1.1"] != evidence.StateStaleCode || states["1.2"] != evidence.StateStaleCode {
				t.Errorf("after the content is gone the tasks read %v, want stale-code for both", states)
			}
			verified, code := integrityCLI(t, "verify", subdirectoryFeature, "--check")
			if code == 0 {
				t.Errorf("verify passed on a commit that lacks the content: %s", verified.Summary)
			}
			for _, entry := range verified.Evidence {
				if entry.Passed == nil || *entry.Passed {
					t.Errorf("verify: the proof of task %s is not reported as failed", entry.TaskID)
				}
			}
			for _, args := range [][]string{{subdirectoryFeature}, {subdirectoryFeature, "--strict"}} {
				verdict, code := releaseVerdict(t, args...)
				if code == 0 || verdict.Releasable {
					t.Errorf("release check %v certified a commit on which the proofs fail", args)
				}
			}

			// The same content, committed: the delivery proof is attested.
			diagnosticsFile(t, root, "notes.txt", "released\n")
			diagnosticsFile(t, root, "status.txt", "green\n")
			head := commitEverything(t, top, "the content")
			integrityMustCLI(t, "verify", subdirectoryFeature)
			entries, _ = subdirectoryEntries(t)
			if entries["1.1"].State != evidence.StateDelivered || entries["1.1"].Attestation == nil || entries["1.1"].Attestation.Commit != head {
				t.Errorf("on a committed tree the delivery task reads %s with %+v, want delivered at %s", entries["1.1"].State, entries["1.1"].Attestation, head)
			}
			if entries["1.2"].State != evidence.StateVerified {
				t.Errorf("on a committed tree the regression task reads %s", entries["1.2"].State)
			}
		})
	}
}

// appendingRunner stands for a proof that changes the tree it should only
// read: it appends to one file and reports success.
type appendingRunner struct{ path string }

func (r appendingRunner) Run(context.Context, string, ...string) (shell.Response, error) {
	file, err := os.OpenFile(r.path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return shell.Response{}, err
	}
	defer file.Close()
	if _, err := file.WriteString("appended by a proof\n"); err != nil {
		return shell.Response{}, err
	}
	return shell.Response{Stdout: "ok"}, nil
}

// A proof that changes a file during verify fails its task, and the run names
// the file relative to the root.
func TestMutatingProofIsCaught(t *testing.T) {
	for _, layout := range rootLayouts {
		t.Run(layout.name, func(t *testing.T) {
			top, root := layoutRepo(t, layout.prefix, subdirectoryTasks, passingFiles)
			integrityMustCLI(t, "task", "complete", subdirectoryFeature, "1.1")
			integrityMustCLI(t, "task", "complete", subdirectoryFeature, "1.2")
			commitEverything(t, top, "evidence")

			previous := commandRunner
			t.Cleanup(func() { commandRunner = previous })
			commandRunner = appendingRunner{path: filepath.Join(root, "status.txt")}

			result, code := integrityCLI(t, "verify", subdirectoryFeature, "--all")
			if code == 0 {
				t.Fatalf("verify passed with a proof that changed the tree: %s", result.Summary)
			}
			var regression *output.EvidenceStatus
			for index := range result.Evidence {
				if result.Evidence[index].TaskID == "1.2" {
					regression = &result.Evidence[index]
				}
			}
			if regression == nil {
				t.Fatalf("verify reports no outcome for task 1.2: %+v", result.Evidence)
			}
			if regression.Passed == nil || *regression.Passed || regression.State != evidence.StateFailed {
				t.Errorf("task 1.2 reads %s, want a failure", regression.State)
			}
			if !strings.Contains(regression.Failure, "working tree changed while task 1.2 proof ran") || !strings.Contains(regression.Failure, "status.txt") {
				t.Errorf("the failure does not name the side effect: %q", regression.Failure)
			}
			warnings := strings.Join(result.Warnings, "\n")
			if !strings.Contains(warnings, "proof side effects modified the repository: status.txt;") {
				t.Errorf("the run does not name the file from the root: %q", warnings)
			}
			if strings.Contains(warnings+regression.Failure, "pkg/app/") {
				t.Errorf("a path is named from the top level: %q / %q", warnings, regression.Failure)
			}
		})
	}
}

// A file outside the root is no part of the root's identity: a verified task
// stays verified after a file of another directory is modified, and after
// that change is committed.
func TestIdentityIgnoresPathsOutsideRoot(t *testing.T) {
	top, _ := layoutRepo(t, "pkg/app", subdirectoryTasks, passingFiles)
	integrityMustCLI(t, "task", "complete", subdirectoryFeature, "1.1")
	integrityMustCLI(t, "task", "complete", subdirectoryFeature, "1.2")
	head := commitEverything(t, top, "evidence")
	entries, _ := subdirectoryEntries(t)
	identity := entries["1.2"].CurrentIdentity
	if entries["1.2"].State != evidence.StateVerified || identity == "" {
		t.Fatalf("task 1.2 reads %s with identity %q on the committed tree", entries["1.2"].State, identity)
	}

	diagnosticsFile(t, top, "other/file.txt", "changed\n")
	diagnosticsFile(t, top, "other/new.txt", "new\n")
	entries, report := subdirectoryEntries(t)
	if entries["1.2"].State != evidence.StateVerified || entries["1.2"].CurrentIdentity != identity {
		t.Errorf("with a change in another directory task 1.2 reads %s with identity %q, want verified with %q", entries["1.2"].State, entries["1.2"].CurrentIdentity, identity)
	}
	if report.CurrentTree == nil || report.CurrentTree.Commit != head || len(report.CurrentTree.Paths) != 0 {
		t.Errorf("the report names %+v, want the commit %s and no path", report.CurrentTree, head)
	}

	moved := commitEverything(t, top, "another directory")
	entries, report = subdirectoryEntries(t)
	if entries["1.2"].State != evidence.StateVerified || entries["1.2"].CurrentIdentity != identity {
		t.Errorf("after the commit of that change task 1.2 reads %s with identity %q, want verified with %q", entries["1.2"].State, entries["1.2"].CurrentIdentity, identity)
	}
	if moved == head || report.CurrentTree == nil || report.CurrentTree.Commit != moved {
		t.Errorf("the report names %+v, want the new commit %s", report.CurrentTree, moved)
	}
}

// copyRecorded copies one file of a recorded fixture, byte for byte.
func copyRecorded(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The upgrade costs no proof: a ledger recorded by the released v0.16.0 on a
// committed tree reads as it did, with the root at the top level and below
// it. Each fixture under testdata/recorded-by-v0.16.0 holds what that release
// wrote when a delivery task and a regression task were completed on a
// committed tree: the three documents, the ledger and the two code files. The
// repository ignores every directory named .walden, so the fixture keeps the
// documents under specs/ and the ledger under evidence/ and the test puts
// them in place.
func TestRecordedLedgersKeepTheirStates(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
	fixtures, err := filepath.Abs(filepath.Join("testdata", "recorded-by-v0.16.0"))
	if err != nil {
		t.Fatal(err)
	}
	for _, layout := range []struct{ name, fixture, prefix string }{
		{"top level", "top-level", ""},
		{"below the top level", "below-top-level", "pkg/app"},
	} {
		t.Run(layout.name, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			fixture := filepath.Join(fixtures, layout.fixture)
			top := chdirContract(t)
			root := filepath.Join(top, filepath.FromSlash(layout.prefix))
			enterRoot(t, root)
			for from, to := range map[string]string{
				"notes.txt":                      "notes.txt",
				"status.txt":                     "status.txt",
				"specs/recorded/requirements.md": ".walden/specs/recorded/requirements.md",
				"specs/recorded/design.md":       ".walden/specs/recorded/design.md",
				"specs/recorded/tasks.md":        ".walden/specs/recorded/tasks.md",
				"evidence/recorded.json":         ".walden/evidence/recorded.json",
			} {
				copyRecorded(t, filepath.Join(fixture, filepath.FromSlash(from)), filepath.Join(root, filepath.FromSlash(to)))
			}
			if layout.prefix != "" {
				diagnosticsFile(t, top, "other/file.txt", "other\n")
			}
			gitify(t, top)

			ledger, err := os.ReadFile(filepath.Join(root, ".walden", "evidence", "recorded.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(ledger), `"walden": "v0.16.0"`) {
				t.Fatal("the fixture was not recorded by v0.16.0")
			}

			result := integrityMustCLI(t, "evidence", "status", "recorded")
			states := map[string]string{}
			for _, entry := range result.Evidence {
				states[entry.TaskID] = entry.State
			}
			if want := map[string]string{"1.1": evidence.StateDelivered, "1.2": evidence.StateVerified}; !reflect.DeepEqual(states, want) {
				t.Errorf("states = %v, want %v as v0.16.0 reported them", states, want)
			}
			if verdict, code := integrityCLI(t, "release", "check", "recorded"); code != 0 || verdict.Release == nil || !verdict.Release.Releasable {
				t.Errorf("the gate no longer certifies the recorded evidence: exit %d, %s", code, verdict.Summary)
			}
			after, err := os.ReadFile(filepath.Join(root, ".walden", "evidence", "recorded.json"))
			if err != nil || string(after) != string(ledger) {
				t.Errorf("reading the ledger changed it (%v)", err)
			}
		})
	}
}

// releaseBlockers joins the worktree blockers of a verdict, one to a line.
func releaseBlockers(status *output.ReleaseStatus) string {
	if status == nil {
		return ""
	}
	return strings.Join(status.Worktree.Blockers, "\n")
}
