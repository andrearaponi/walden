package release

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/testutil"
	"github.com/andrearaponi/walden/internal/workflow"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

const draftReqHeader = `---
status: draft
approved_at:
last_modified: 2026-07-11T13:00:00Z
approved_fingerprint:
---

`

const draftDesignHeader = `---
status: draft
approved_at:
last_modified: 2026-07-11T13:00:00Z
approved_fingerprint:
source_requirements_approved_at:
source_requirements_fingerprint:
---

`

const draftTasksHeader = `---
status: draft
approved_at:
last_modified: 2026-07-11T13:00:00Z
approved_fingerprint:
source_design_approved_at:
source_design_fingerprint:
---

`

func certifiableRequirements() string {
	return draftReqHeader + "# Requirements Document\n\n## Introduction\n\nGate fixture.\n\n## Requirements\n\n### R1 Markers\n\n**User Story:** As a tester, I want markers, so that the gate has proofs to judge.\n\n#### Acceptance Criteria\n\n1. `R1.AC1` WHEN the first proof runs, the system SHALL succeed.\n2. `R1.AC2` WHEN the second proof runs, the system SHALL succeed.\n"
}

func certifiableDesign(extra string) string {
	return draftDesignHeader + "# Feature Design\n\n## Overview\n\nTwo trivially provable markers." + extra + "\n\n## Architecture\n\nOne shell proof per task.\n\n## Options Considered\n\n### Option A\n\n- Summary: shell proofs.\n- Why chosen: trivial.\n\n### Option B\n\n- Summary: none.\n- Why rejected: n/a.\n\n## Simplicity And Elegance Review\n\n- Simplest viable shape: two commands.\n- Coupling check: none.\n- Future-proofing: none.\n\n## Failure Modes And Tradeoffs\n\n- Failure mode: none.\n- Mitigation: none.\n- Tradeoff: none.\n\n## Testing Strategy\n\nShell exit codes.\n\n## Verification Plan\n\n- Requirement proof: shell exits.\n- Test evidence: exit codes.\n\n## Requirement Coverage\n\n| Requirement | Covered By |\n| --- | --- |\n| `R1` | markers |\n"
}

func certifiableTasks() string {
	return draftTasksHeader + "# Implementation Plan\n\n- [ ] 1. Markers\n  - [ ] 1.1 First marker\n    - Requirements: `R1.AC1`\n    - Design: Overview\n    - Verification:\n      - command: [\"sh\", \"-c\", \"true\"]\n        covers: [\"R1.AC1\"]\n  - [ ] 1.2 Second marker\n    - Requirements: `R1.AC2`\n    - Design: Overview\n    - Verification:\n      - command: [\"sh\", \"-c\", \"true\"]\n        covers: [\"R1.AC2\"]\n"
}

// addFeature writes and approves a certifiable feature.
func addFeature(t *testing.T, root, name, requirements, design, tasks string) {
	t.Helper()
	write(t, root, ".walden/specs/"+name+"/requirements.md", requirements)
	write(t, root, ".walden/specs/"+name+"/design.md", design)
	write(t, root, ".walden/specs/"+name+"/tasks.md", tasks)
	for _, phase := range []string{"requirements", "design", "tasks"} {
		parsed, err := workflow.ParsePhase(phase)
		if err != nil {
			t.Fatalf("parse phase: %v", err)
		}
		if _, err := workflow.OpenReview(root, name, parsed); err != nil {
			t.Fatalf("open %s: %v", phase, err)
		}
		if _, err := workflow.ApproveReview(root, name, parsed); err != nil {
			t.Fatalf("approve %s: %v", phase, err)
		}
	}
}

// gateRepo builds a real git repo with one completed, committed feature.
func gateRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "gate@walden.dev")
	gitIn(t, root, "config", "user.name", "Gate")

	addFeature(t, root, "gate-demo", certifiableRequirements(), certifiableDesign(""), certifiableTasks())
	write(t, root, "src.txt", "MARKER-A\nMARKER-B\n")

	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)
	if _, err := workflow.CompleteAllTasks(context.Background(), root, "gate-demo", runner); err != nil {
		t.Fatalf("complete fixture: %v", err)
	}

	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-qm", "gate-demo: everything, evidence included")
	return root
}

func criterion(t *testing.T, cert FeatureCertification, name string) CriterionResult {
	t.Helper()
	for _, c := range cert.Criteria {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("criterion %s missing from %v", name, cert.Criteria)
	return CriterionResult{}
}

func TestReleaseCheckAllGreen(t *testing.T) {
	root := gateRepo(t)

	ledgerBefore, err := os.ReadFile(filepath.Join(root, ".walden/evidence/gate-demo.json"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}

	report, err := ReleaseCheck(context.Background(), root, "", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if !report.Releasable() {
		t.Fatalf("green repository not releasable: %+v", report)
	}
	if len(report.Features) != 1 || len(report.Features[0].Criteria) != 4 {
		t.Fatalf("expected 1 feature with 4 criteria: %+v", report.Features)
	}
	for _, c := range report.Features[0].Criteria {
		if !c.Passed {
			t.Fatalf("criterion %s failed on a green repo: %v", c.Name, c.Blockers)
		}
	}

	ledgerAfter, _ := os.ReadFile(filepath.Join(root, ".walden/evidence/gate-demo.json"))
	if string(ledgerBefore) != string(ledgerAfter) {
		t.Fatal("release check mutated the ledger")
	}

	scoped, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
	if err != nil || len(scoped.Features) != 1 {
		t.Fatalf("feature-scoped run: %v %+v", err, scoped.Features)
	}
}

func TestReleaseCheckChainBlockerStillEvaluatesEverything(t *testing.T) {
	root := gateRepo(t)

	// Tamper with the approved requirements body: the chain goes stale.
	path := filepath.Join(root, ".walden/specs/gate-demo/requirements.md")
	content, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(content, []byte("\ntampered\n")...), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	report, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	cert := report.Features[0]
	if chain := criterion(t, cert, "chain"); chain.Passed || len(chain.Blockers) == 0 {
		t.Fatalf("stale chain not blocked: %+v", chain)
	}
	if len(cert.Criteria) != 4 {
		t.Fatalf("criteria short-circuited: %+v", cert.Criteria)
	}
	if report.Releasable() {
		t.Fatal("stale chain certified as releasable")
	}
}

func TestReleaseCheckValidationBlocker(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "g@g")
	gitIn(t, root, "config", "user.name", "G")

	// A design missing required sections approves fine (approval does not
	// validate) but fails full-spec validation.
	brokenDesign := draftDesignHeader + "# Feature Design\n\n## Overview\n\nBare.\n\n## Requirement Coverage\n\n| Requirement | Covered By |\n| --- | --- |\n| `R1` | markers |\n"
	addFeature(t, root, "gate-demo", certifiableRequirements(), brokenDesign, certifiableTasks())

	report, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	valid := criterion(t, report.Features[0], "validation")
	if valid.Passed || !strings.Contains(strings.Join(valid.Blockers, " "), "walden validate") {
		t.Fatalf("validation blocker missing or without remedy: %+v", valid)
	}
}

func TestReleaseCheckDecisionMarkerBlocksOnlyWhenApproved(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "g@g")
	gitIn(t, root, "config", "user.name", "G")

	// Approved design carrying an unresolved checkpoint.
	addFeature(t, root, "marked", certifiableRequirements(), certifiableDesign("\n\n[decision: which store backs this?]"), certifiableTasks())

	report, err := ReleaseCheck(context.Background(), root, "marked", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	decisions := criterion(t, report.Features[0], "decisions")
	if decisions.Passed || !strings.Contains(decisions.Blockers[0], "design.md") {
		t.Fatalf("approved marker not blocked: %+v", decisions)
	}

	// A draft carrying a checkpoint is legitimate in-flight work.
	write(t, root, ".walden/specs/inflight/requirements.md", certifiableRequirements())
	write(t, root, ".walden/specs/inflight/design.md", certifiableDesign("\n\n[decision: still open]"))
	write(t, root, ".walden/specs/inflight/tasks.md", certifiableTasks())

	report, err = ReleaseCheck(context.Background(), root, "inflight", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck inflight: %v", err)
	}
	if decisions := criterion(t, report.Features[0], "decisions"); !decisions.Passed {
		t.Fatalf("draft marker blocked certification: %+v", decisions)
	}
}

func TestReleaseCheckDecisionMarkerIgnoresHTMLComments(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "g@g")
	gitIn(t, root, "config", "user.name", "G")

	// The production-shaped assumed note: prose inside an HTML comment that
	// mentions the marker to deny its presence must never block.
	commented := "\n\n<!-- assumed: no [decision: checkpoints — every fork was resolved during requirements -->"
	addFeature(t, root, "commented", certifiableRequirements(), certifiableDesign(commented), certifiableTasks())

	report, err := ReleaseCheck(context.Background(), root, "commented", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if decisions := criterion(t, report.Features[0], "decisions"); !decisions.Passed {
		t.Fatalf("marker inside a comment blocked certification: %+v", decisions)
	}

	// An unterminated comment would swallow the rest of the document from
	// the scan — the malformed comment itself blocks certification.
	unterminated := "\n\n<!-- assumed: still drafting\n\n[decision: which store?]"
	addFeature(t, root, "unterminated", certifiableRequirements(), certifiableDesign(unterminated), certifiableTasks())
	report, err = ReleaseCheck(context.Background(), root, "unterminated", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck unterminated: %v", err)
	}
	decisions := criterion(t, report.Features[0], "decisions")
	if decisions.Passed {
		t.Fatalf("unterminated comment did not block: %+v", decisions)
	}
	joined := strings.Join(decisions.Blockers, " ")
	if !strings.Contains(joined, "unterminated HTML comment in design.md") || !strings.Contains(joined, "-->") {
		t.Fatalf("blocker missing document name or remedy: %+v", decisions.Blockers)
	}

	// A real marker beside a commented one still blocks.
	mixed := "\n\n<!-- assumed: none open -->\n\n[decision: pick a queue]"
	addFeature(t, root, "mixed", certifiableRequirements(), certifiableDesign(mixed), certifiableTasks())
	report, err = ReleaseCheck(context.Background(), root, "mixed", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck mixed: %v", err)
	}
	if decisions := criterion(t, report.Features[0], "decisions"); decisions.Passed {
		t.Fatal("marker outside the comment did not block")
	}
}

func TestReleaseCheckEvidenceBlockerNamesStateAndRemedy(t *testing.T) {
	root := gateRepo(t)

	// The code moves after certification-worthy completion.
	write(t, root, "src.txt", "MARKER-A\nMARKER-B\nchanged\n")

	report, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	ev := criterion(t, report.Features[0], "evidence")
	joined := strings.Join(ev.Blockers, " ")
	if ev.Passed || !strings.Contains(joined, "stale-code") || !strings.Contains(joined, "walden verify gate-demo") {
		t.Fatalf("evidence blocker missing state or remedy: %+v", ev)
	}
}

// halfDoneRepo builds the flip's canonical fixture: a certifiable feature
// with task 1.1 executed and 1.2 still pending, committed.
func halfDoneRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "g@g")
	gitIn(t, root, "config", "user.name", "G")

	addFeature(t, root, "gate-demo", certifiableRequirements(), certifiableDesign(""), certifiableTasks())
	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	if _, err := workflow.CompleteTask(context.Background(), root, "gate-demo", "1.1", runner); err != nil {
		t.Fatalf("complete 1.1: %v", err)
	}
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-qm", "half done")
	return root
}

// The v0.10.0 flip's witness: the plan is a promise the release must keep or
// visibly defer — pending work blocks the default verdict.
func TestReleaseCheckPendingBlocksByDefault(t *testing.T) {
	root := halfDoneRepo(t)

	report, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if report.Releasable() {
		t.Fatalf("pending work certified releasable under the flipped default: %+v", report)
	}
	ev := criterion(t, report.Features[0], "evidence")
	joined := strings.Join(ev.Blockers, " ")
	if !strings.Contains(joined, "task 1.2 is pending") {
		t.Fatalf("evidence criterion does not carry the pending blocker: %v", ev.Blockers)
	}
	if !strings.Contains(joined, "execute it") || !strings.Contains(joined, "--allow-pending --reason") {
		t.Fatalf("pending blocker does not name both remedies: %v", ev.Blockers)
	}
	if report.Completion() != CompletionWithPending {
		t.Fatalf("completion = %s, want with-pending", report.Completion())
	}
}

func TestReleaseCheckWaiverRestoresInformational(t *testing.T) {
	root := halfDoneRepo(t)

	report, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{AllowPending: true, WaiverReason: "deferred to release 1.3"})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if !report.Releasable() {
		t.Fatalf("waived pending work still blocked: %+v", report)
	}
	if pending := report.Features[0].Pending; len(pending) != 1 || pending[0] != "1.2" {
		t.Fatalf("pending fact list = %v, want [1.2]", pending)
	}
	if report.Completion() != CompletionWithWaivers {
		t.Fatalf("completion = %s, want with-waivers", report.Completion())
	}
	if report.WaiverReason != "deferred to release 1.3" {
		t.Fatalf("waiver reason not carried: %q", report.WaiverReason)
	}
}

func TestWaivedTasksDerivation(t *testing.T) {
	root := halfDoneRepo(t)

	waived, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{AllowPending: true, WaiverReason: "deferred"})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if got := strings.Join(waived.WaivedTasks(), ";"); got != "gate-demo: 1.2" {
		t.Fatalf("waived tasks = %q", got)
	}

	blocked, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if blocked.WaivedTasks() != nil {
		t.Fatalf("no waiver must derive no waived tasks, got %v", blocked.WaivedTasks())
	}

	// A waiver with nothing pending records nothing: the class is complete.
	green := gateRepo(t)
	idle, err := ReleaseCheck(context.Background(), green, "gate-demo", Options{AllowPending: true, WaiverReason: "unnecessary"})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if idle.Completion() != CompletionComplete || len(idle.WaivedTasks()) != 0 {
		t.Fatalf("idle waiver = %s %v, want complete with no waived tasks", idle.Completion(), idle.WaivedTasks())
	}
}

func TestReleaseCheckWalksFeaturesSorted(t *testing.T) {
	root := gateRepo(t)
	addFeature(t, root, "alpha-extra", certifiableRequirements(), certifiableDesign(""), certifiableTasks())
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-qm", "second feature specs")

	report, err := ReleaseCheck(context.Background(), root, "", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if len(report.Features) != 2 || report.Features[0].Feature != "alpha-extra" || report.Features[1].Feature != "gate-demo" {
		t.Fatalf("features not sorted: %+v", report.Features)
	}
}

func TestReleaseCheckWorktreePartitionsPaths(t *testing.T) {
	root := gateRepo(t)

	write(t, root, "untracked-note.txt", "wip")
	write(t, root, ".walden/scratch.txt", "local")
	gitIn(t, root, "mv", "src.txt", "renamed.txt")

	report, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	joined := strings.Join(report.WorktreeBlockers, " ")
	if !strings.Contains(joined, "untracked-note.txt") || !strings.Contains(joined, "renamed.txt") {
		t.Fatalf("worktree blockers incomplete: %v", report.WorktreeBlockers)
	}
	if strings.Contains(joined, "src.txt") {
		t.Fatalf("rename source reported as a live path: %v", report.WorktreeBlockers)
	}
	if len(report.WaldenDirty) != 1 || !strings.Contains(report.WaldenDirty[0], ".walden/scratch.txt") {
		t.Fatalf(".walden dirt not partitioned as warning: %v", report.WaldenDirty)
	}
	if report.Releasable() {
		t.Fatal("dirty worktree certified as releasable")
	}
}

func TestReleaseCheckBlocksWithoutGit(t *testing.T) {
	root := t.TempDir()
	addFeature(t, root, "gate-demo", certifiableRequirements(), certifiableDesign(""), certifiableTasks())
	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)
	if _, err := workflow.CompleteAllTasks(context.Background(), root, "gate-demo", runner); err != nil {
		t.Fatalf("complete: %v", err)
	}

	report, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if !report.GitSkipped {
		t.Fatal("no-git repository did not report the worktree criterion skipped")
	}
	// A verdict without a code identity names no tree: fail closed.
	if report.Releasable() {
		t.Fatalf("git-less repository certified releasable: %+v", report)
	}
	joined := strings.Join(report.WorktreeBlockers, " ")
	if !strings.Contains(joined, "no usable git repository") || !strings.Contains(joined, "initialize git") {
		t.Fatalf("identity blocker missing or unnamed remedy: %v", report.WorktreeBlockers)
	}
}

func TestReleaseCheckStrictBlocksDirtyWalden(t *testing.T) {
	root := gateRepo(t)
	write(t, root, ".walden/scratch.txt", "refreshed ledger\n")

	// Default mode: dirty .walden/ stays a warning partition, not a blocker.
	report, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
	if err != nil {
		t.Fatalf("ReleaseCheck: %v", err)
	}
	if !report.Releasable() {
		t.Fatalf("dirty .walden/ blocked a non-strict run: %+v", report)
	}
	if len(report.WaldenDirty) != 1 || !strings.Contains(report.WaldenDirty[0], ".walden/scratch.txt") {
		t.Fatalf(".walden dirt not partitioned: %v", report.WaldenDirty)
	}

	// Strict mode: the same path blocks, partition intact, remedy named.
	report, err = ReleaseCheck(context.Background(), root, "gate-demo", Options{Strict: true})
	if err != nil {
		t.Fatalf("ReleaseCheck strict: %v", err)
	}
	if report.Releasable() {
		t.Fatalf("dirty .walden/ certified releasable under strict: %+v", report)
	}
	if len(report.WaldenDirty) != 1 {
		t.Fatalf("strict promotion emptied the partition: %v", report.WaldenDirty)
	}
	joined := strings.Join(report.WorktreeBlockers, " ")
	if !strings.Contains(joined, "uncommitted under .walden/: .walden/scratch.txt") || !strings.Contains(joined, "--strict") {
		t.Fatalf("strict blocker missing path or remedy: %v", report.WorktreeBlockers)
	}
}
