package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

// traceFixture builds the isolated repository with deliveryStatusFixture,
// which holds a feature of three statements, and then writes the three
// documents of the test over them. Git reads no global or system
// configuration there. Nothing is committed and no task is completed: each
// test decides both.
func traceFixture(t *testing.T, requirements, design, tasks string) string {
	t.Helper()
	root := deliveryStatusFixture(t, tasks)
	writeStatusFeatureFile(t, root, verifyTaskFeature, "requirements.md", requirements)
	writeStatusFeatureFile(t, root, verifyTaskFeature, "design.md", design)
	writeStatusFeatureFile(t, root, verifyTaskFeature, "tasks.md", tasks)
	return root
}

// completeTraceTask completes a leaf whose proof has the given number of
// steps, with a runner that answers each of them.
func completeTraceTask(t *testing.T, id string, steps int) {
	t.Helper()
	responses := make([]testutil.Response, steps)
	for index := range responses {
		responses[index] = testutil.Response{Stdout: "ok"}
	}
	overrideCommandRunner(t, testutil.NewFakeRunner(responses...))
	integrityMustCLI(t, "task", "complete", verifyTaskFeature, id)
}

// runTraceCLI runs `walden trace` with the given arguments and returns its
// two streams and its exit code.
func runTraceCLI(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = Run(append([]string{"trace"}, args...), &out, &errOut)
	return out.String(), errOut.String(), code
}

// traceBlock runs `walden trace <feature> --json` and returns the result and
// its trace block as the wire holds them, decoded into untyped maps.
func traceBlock(t *testing.T) (result, block map[string]any) {
	t.Helper()
	stdout, stderr, code := runTraceCLI(t, verifyTaskFeature, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("trace --json exited %d: %s%s", code, stdout, stderr)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("the output of trace --json is not JSON: %v\n%s", err, stdout)
	}
	if envelope["schema_version"] != "v0beta1" || envelope["command"] != "trace" || envelope["ok"] != true {
		t.Fatalf("the envelope = %v, want v0beta1, trace and ok", envelope)
	}
	result, _ = envelope["result"].(map[string]any)
	block, _ = result["trace"].(map[string]any)
	if block == nil {
		t.Fatalf("the result holds no trace block: %v", result)
	}
	return result, block
}

// traceRequirements declares seven criteria out of numeric order, two
// non-functional requirements and two constraints. With tracePlan a trace
// lists nine statements: no leaf names or covers NFR2 and C2.
const traceRequirements = `---
status: approved
approved_at: 2026-03-22T07:00:00Z
last_modified: 2026-03-22T07:00:00Z
---

# Requirements Document

## Requirements

### R2 Errors

#### Acceptance Criteria

1. ` + "`R2.AC1`" + ` IF input is empty, THEN the system SHALL reject it.
   - Acceptance check: an error names the empty input.

### R1 Parser

#### Acceptance Criteria

1. ` + "`R1.AC1`" + ` WHEN input arrives, the system SHALL parse it.
2. ` + "`R1.AC2`" + ` WHEN the parse ends, the system SHALL return a tree.
10. ` + "`R1.AC10`" + ` WHEN the tree is large, the system SHALL stream it.
3. ` + "`R1.AC3`" + ` WHEN a token is unknown, the system SHALL keep it.

### R3 Report

#### Acceptance Criteria

2. ` + "`R3.AC2`" + ` WHEN the report is asked, the system SHALL print it.
1. ` + "`R3.AC1`" + ` WHEN nothing was parsed, the system SHALL say so.

## Non-Functional Requirements

- ` + "`NFR1`" + ` Parsing stays deterministic.
- ` + "`NFR2`" + ` Nothing names or covers this one.

## Constraints And Dependencies

- ` + "`C1`" + ` Standard library only.
- ` + "`C2`" + ` Nothing names or covers this one either.
`

// The commands of tracePlan, as the plan declares them.
const (
	tracePipeCommand = `printf 'a|b' | grep -q 'a\|b'`
	traceTickCommand = "test \"`echo ok`\" = ok"
)

// traceLongCommand is shown as a command of 400 characters, sh -c and the
// quotes included: 392 inside the quotes. It holds && and >.
var traceLongCommand = "true" + strings.Repeat(" && true", 48) + " >&2"

// tracePlan holds four leaves: the delivery leaf 1.1, 1.2 with two steps,
// 1.3 whose first command is longer than the limit and whose second step
// covers nothing, and 1.4. No leaf names R3.AC1, 1.2 names the heading R2,
// and the Design line of 1.3 is free text.
func tracePlan() string {
	return `---
status: approved
approved_at: 2026-03-22T07:20:00Z
last_modified: 2026-03-22T07:20:00Z
source_design_approved_at: 2026-03-22T07:10:00Z
---

# Implementation Plan

- [ ] 1. Build parser
  - [ ] 1.1 Publish the notes
    - Requirements: ` + "`R1.AC1`, `R1.AC3`" + `
    - Design: Architecture
    - Proof: delivery
    - Verification:
      - command: ["grep", "-q", "released", "notes.txt"]
        covers: ["R1.AC1"]
  - [ ] 1.2 Parse and reject
    - Requirements: ` + "`R2.AC1`, `R1.AC2`, `R1.AC3`, `NFR1`, `R2`" + `
    - Design: Architecture, Verification Plan
    - Verification:
      - command: ["sh", "-c", "true"]
        covers: ["R2.AC1", "NFR1"]
      - command: ["sh", "-c", "printf 'a|b' | grep -q 'a\\|b'"]
        covers: ["R1.AC2", "R1.AC3", "C1"]
  - [ ] 1.3 Stream
    - Requirements: ` + "`R1.AC10`, `R3.AC2`" + `
    - Design: Task Store; a free note
    - Verification:
      - command: ["sh", "-c", "` + traceLongCommand + `"]
        covers: ["R1.AC10"]
      - command: ["go", "vet", "./..."]
  - [ ] 1.4 Report
    - Requirements: ` + "`R3.AC2`, `R1.AC2`" + `
    - Design: Architecture
    - Verification:
      - command: ["sh", "-c", "test \"` + "`echo ok`" + `\" = ok"]
        covers: ["R3.AC2", "R1.AC2"]
`
}

// traceStatesRepo is a committed repository that holds four states at once:
// leaf 1.1 is delivered, 1.2 was verified and then made stale by a committed
// change, 1.3 was verified after that change, and 1.4 is pending.
func traceStatesRepo(t *testing.T) string {
	t.Helper()
	root := traceFixture(t, traceRequirements, specDiagnosticsDesign, tracePlan())
	gitify(t, root)
	completeTraceTask(t, "1.1", 1)
	completeTraceTask(t, "1.2", 2)
	diagnosticsFile(t, root, "main.go", "package main // moved on\n")
	integrityGit(t, root, "add", "-A")
	integrityGit(t, root, "commit", "-qm", "the code moves on")
	completeTraceTask(t, "1.3", 2)
	return root
}

// The commands of tracePlan as a reader sees them, and as the cells of a
// Markdown table hold them.
const (
	tracePipeShown = `sh -c "printf 'a|b' | grep -q 'a\\|b'"`
	traceTickShown = "sh -c \"test \\\"`echo ok`\\\" = ok\""
	tracePipeCell  = "`sh -c \"printf 'a\\|b' \\| grep -q 'a\\\\\\|b'\"`"
	traceTickCell  = "`` sh -c \"test \\\"`echo ok`\\\" = ok\" ``"
)

// traceCutShown is the long command as a reader sees it: its first 119
// characters, sh -c and the opening quote among them, and the mark.
var traceCutShown = `sh -c "` + strings.Repeat("true && ", 14) + "…"

// traceStatesText is the whole text of the trace of traceStatesRepo, written
// by hand. {PIPE}, {TICK} and {CUT} stand for the three commands above.
const traceStatesText = `Summary: trace for markers: 9 statement(s), 8 hole(s): 1 unnamed, 0 uncovered, 7 not-standing
Trace:
- R2.AC1: named by 1.2
  - 1.2 step 1, stale-code: sh -c true
- R1.AC1: named by 1.1
  - 1.1 step 1, delivered (delivery proof): grep -q released notes.txt
- R1.AC2: named by 1.2, 1.4
  - 1.2 step 2, stale-code: {PIPE}
  - 1.4 step 1, pending: {TICK}
- R1.AC10: named by 1.3
  - 1.3 step 1, verified: {CUT}
- R1.AC3: named by 1.1, 1.2
  - 1.2 step 2, stale-code: {PIPE}
- R3.AC2: named by 1.3, 1.4
  - 1.4 step 1, pending: {TICK}
- R3.AC1: named by no leaf
- NFR1: named by 1.2
  - 1.2 step 1, stale-code: sh -c true
- C1: named by no leaf
  - 1.2 step 2, stale-code: {PIPE}
Holes:
- R3.AC1: no leaf names it
- R2.AC1: 1.2 step 1 is stale-code
- R1.AC2: 1.2 step 2 is stale-code
- R1.AC2: 1.4 step 1 is pending
- R1.AC3: 1.2 step 2 is stale-code
- R3.AC2: 1.4 step 1 is pending
- NFR1: 1.2 step 1 is stale-code
- C1: 1.2 step 2 is stale-code
Exit code: 0
`

// traceStatesMarkdown is the whole Markdown of the same trace.
const traceStatesMarkdown = `| Statement | Named by | Proof step | Evidence | Command |
| --- | --- | --- | --- | --- |
| ` + "`R2.AC1`" + ` | 1.2 | 1.2 step 1 | stale-code | ` + "`sh -c true`" + ` |
| ` + "`R1.AC1`" + ` | 1.1 | 1.1 step 1 (delivery proof) | delivered | ` + "`grep -q released notes.txt`" + ` |
| ` + "`R1.AC2`" + ` | 1.2, 1.4 | 1.2 step 2 | stale-code | {PIPE} |
| | | 1.4 step 1 | pending | {TICK} |
| ` + "`R1.AC10`" + ` | 1.3 | 1.3 step 1 | verified | {CUT} |
| ` + "`R1.AC3`" + ` | 1.1, 1.2 | 1.2 step 2 | stale-code | {PIPE} |
| ` + "`R3.AC2`" + ` | 1.3, 1.4 | 1.4 step 1 | pending | {TICK} |
| ` + "`R3.AC1`" + ` | no leaf | | | |
| ` + "`NFR1`" + ` | 1.2 | 1.2 step 1 | stale-code | ` + "`sh -c true`" + ` |
| ` + "`C1`" + ` | no leaf | 1.2 step 2 | stale-code | {PIPE} |

Holes: 8 (1 unnamed, 0 uncovered, 7 not-standing)

- ` + "`R3.AC1`" + `: no leaf names it
- ` + "`R2.AC1`" + `: 1.2 step 1 is stale-code
- ` + "`R1.AC2`" + `: 1.2 step 2 is stale-code
- ` + "`R1.AC2`" + `: 1.4 step 1 is pending
- ` + "`R1.AC3`" + `: 1.2 step 2 is stale-code
- ` + "`R3.AC2`" + `: 1.4 step 1 is pending
- ` + "`NFR1`" + `: 1.2 step 1 is stale-code
- ` + "`C1`" + `: 1.2 step 2 is stale-code
`

// TestTraceFormats: on a feature of nine listed statements declared out of
// numeric order and four leaves, the text and the Markdown are the ones
// written here by hand, and a second run of each format gives the same bytes.
func TestTraceFormats(t *testing.T) {
	traceStatesRepo(t)

	wantText := strings.NewReplacer("{PIPE}", tracePipeShown, "{TICK}", traceTickShown, "{CUT}", traceCutShown).Replace(traceStatesText)
	wantMarkdown := strings.NewReplacer("{PIPE}", tracePipeCell, "{TICK}", traceTickCell, "{CUT}", "`"+traceCutShown+"`").Replace(traceStatesMarkdown)

	for _, format := range []struct {
		name string
		args []string
		want string
	}{
		{"text", []string{verifyTaskFeature}, wantText},
		{"markdown", []string{verifyTaskFeature, "--markdown"}, wantMarkdown},
	} {
		t.Run(format.name, func(t *testing.T) {
			stdout, stderr, code := runTraceCLI(t, format.args...)
			if code != 0 || stderr != "" {
				t.Fatalf("trace exited %d: %s", code, stderr)
			}
			if stdout != format.want {
				t.Fatalf("the output differs from the one written by hand, %s", firstDifference(stdout, format.want))
			}
			if again, _, _ := runTraceCLI(t, format.args...); again != stdout {
				t.Errorf("a second run gives other bytes, %s", firstDifference(again, stdout))
			}
		})
	}

	t.Run("json", func(t *testing.T) {
		first, _, code := runTraceCLI(t, verifyTaskFeature, "--json")
		if code != 0 {
			t.Fatalf("trace --json exited %d", code)
		}
		if again, _, _ := runTraceCLI(t, verifyTaskFeature, "--json"); again != first {
			t.Errorf("a second run gives other bytes, %s", firstDifference(again, first))
		}
		// The envelope holds what the text output holds: printed as text, the
		// decoded result is the text output.
		var envelope output.Envelope
		if err := json.Unmarshal([]byte(first), &envelope); err != nil {
			t.Fatalf("unmarshal the envelope: %v", err)
		}
		var printed bytes.Buffer
		output.PrintText(&printed, envelope.Result)
		if printed.String() != wantText {
			t.Errorf("the envelope does not hold the facts of the text output, %s", firstDifference(printed.String(), wantText))
		}
	})
}

func anyList(values ...string) []any {
	list := make([]any, len(values))
	for index, value := range values {
		list[index] = value
	}
	return list
}

func traceProof(task string, step int, state string, command ...string) map[string]any {
	return map[string]any{"task_id": task, "step": float64(step), "command": anyList(command...), "state": state}
}

func traceNotStanding(statement, task string, step int, state string) map[string]any {
	return map[string]any{"kind": "not-standing", "statement": statement, "task_id": task, "step": float64(step), "state": state}
}

// TestTraceJSON: the block of the trace, read from the wire as untyped maps,
// is the one written here by hand, key by key: every statement with its kind,
// its leaves and its proof steps, every step with its whole command, its
// delivery mark and its state, every leaf with its design references as the
// plan writes them, and every hole.
func TestTraceJSON(t *testing.T) {
	traceStatesRepo(t)

	// The long command is one of 400 characters as a reader would see it.
	if shown := `sh -c "` + traceLongCommand + `"`; len(shown) != 400 || !strings.Contains(traceLongCommand, "&&") || !strings.Contains(traceLongCommand, ">") {
		t.Fatalf("the long command of the fixture is %d characters as shown, want 400 with && and >", len(shown))
	}

	result, block := traceBlock(t)
	assertKeys(t, "result", result, []string{"summary", "changed_files", "skipped_files", "warnings", "trace", "exit_code"}, nil)
	if result["summary"] != "trace for markers: 9 statement(s), 8 hole(s): 1 unnamed, 0 uncovered, 7 not-standing" {
		t.Errorf("summary = %v", result["summary"])
	}

	shTrue := traceProof("1.2", 1, "stale-code", "sh", "-c", "true")
	pipe := traceProof("1.2", 2, "stale-code", "sh", "-c", tracePipeCommand)
	tick := traceProof("1.4", 1, "pending", "sh", "-c", traceTickCommand)
	delivery := traceProof("1.1", 1, "delivered", "grep", "-q", "released", "notes.txt")
	delivery["delivery"] = true
	statement := func(id, kind string, namedBy []any, proofs ...any) map[string]any {
		if proofs == nil {
			proofs = []any{}
		}
		return map[string]any{"id": id, "kind": kind, "named_by": namedBy, "proofs": proofs}
	}
	want := map[string]any{
		"feature": "markers",
		"statements": []any{
			statement("R2.AC1", "criterion", anyList("1.2"), shTrue),
			statement("R1.AC1", "criterion", anyList("1.1"), delivery),
			statement("R1.AC2", "criterion", anyList("1.2", "1.4"), pipe, tick),
			// The command of four hundred characters is whole.
			statement("R1.AC10", "criterion", anyList("1.3"), traceProof("1.3", 1, "verified", "sh", "-c", traceLongCommand)),
			statement("R1.AC3", "criterion", anyList("1.1", "1.2"), pipe),
			statement("R3.AC2", "criterion", anyList("1.3", "1.4"), tick),
			statement("R3.AC1", "criterion", anyList()),
			statement("NFR1", "nfr", anyList("1.2"), shTrue),
			statement("C1", "constraint", anyList(), pipe),
		},
		"leaves": []any{
			map[string]any{"id": "1.1", "design_refs": anyList("Architecture"), "delivery": true, "state": "delivered"},
			map[string]any{"id": "1.2", "design_refs": anyList("Architecture", "Verification Plan"), "state": "stale-code"},
			// A reference that is no heading of the design is carried as
			// written, and is no hole.
			map[string]any{"id": "1.3", "design_refs": anyList("Task Store; a free note"), "state": "verified"},
			map[string]any{"id": "1.4", "design_refs": anyList("Architecture"), "state": "pending"},
		},
		"holes": []any{
			map[string]any{"kind": "unnamed", "statement": "R3.AC1"},
			traceNotStanding("R2.AC1", "1.2", 1, "stale-code"),
			traceNotStanding("R1.AC2", "1.2", 2, "stale-code"),
			traceNotStanding("R1.AC2", "1.4", 1, "pending"),
			traceNotStanding("R1.AC3", "1.2", 2, "stale-code"),
			traceNotStanding("R3.AC2", "1.4", 1, "pending"),
			traceNotStanding("NFR1", "1.2", 1, "stale-code"),
			traceNotStanding("C1", "1.2", 2, "stale-code"),
		},
	}
	if !reflect.DeepEqual(block, want) {
		for _, key := range []string{"feature", "statements", "leaves", "holes"} {
			if !reflect.DeepEqual(block[key], want[key]) {
				got, _ := json.MarshalIndent(block[key], "", " ")
				expected, _ := json.MarshalIndent(want[key], "", " ")
				t.Errorf("%s differ, %s", key, firstDifference(string(got), string(expected)))
			}
		}
		assertKeys(t, "trace", block, []string{"feature", "statements", "leaves", "holes"}, nil)
	}
}

// sixteenCriteria returns requirements that define sixteen criteria and a
// plan whose two leaves cover eight each.
func sixteenCriteria() (requirements, tasks string) {
	var defined, first, second, firstCovers, secondCovers []string
	for index := 1; index <= 16; index++ {
		id := fmt.Sprintf("R1.AC%d", index)
		defined = append(defined, fmt.Sprintf("%d. `%s` WHEN step %d runs, the system SHALL pass.", index, id, index))
		if index <= 8 {
			first, firstCovers = append(first, "`"+id+"`"), append(firstCovers, `"`+id+`"`)
		} else {
			second, secondCovers = append(second, "`"+id+"`"), append(secondCovers, `"`+id+`"`)
		}
	}
	requirements = "---\nstatus: approved\napproved_at: 2026-03-22T07:00:00Z\nlast_modified: 2026-03-22T07:00:00Z\n---\n\n# Requirements Document\n\n### R1 Steps\n\n#### Acceptance Criteria\n\n" +
		strings.Join(defined, "\n") + "\n"
	tasks = "---\nstatus: approved\napproved_at: 2026-03-22T07:20:00Z\nlast_modified: 2026-03-22T07:20:00Z\nsource_design_approved_at: 2026-03-22T07:10:00Z\n---\n\n# Implementation Plan\n\n- [ ] 1. Steps\n" +
		"  - [ ] 1.1 Publish the notes\n    - Requirements: " + strings.Join(first, ", ") + "\n    - Design: Architecture\n    - Proof: delivery\n    - Verification:\n" +
		"      - command: [\"grep\", \"-q\", \"released\", \"notes.txt\"]\n        covers: [" + strings.Join(firstCovers, ", ") + "]\n" +
		"  - [ ] 1.2 Keep the rest\n    - Requirements: " + strings.Join(second, ", ") + "\n    - Design: Architecture\n    - Verification:\n" +
		"      - command: [\"sh\", \"-c\", \"true\"]\n        covers: [" + strings.Join(secondCovers, ", ") + "]\n"
	return requirements, tasks
}

// TestTraceExitCode: a completed report exits 0 with holes and without, in
// each format, and its summary states the statements and the holes it lists.
func TestTraceExitCode(t *testing.T) {
	formats := [][]string{{verifyTaskFeature}, {verifyTaskFeature, "--markdown"}, {verifyTaskFeature, "--json"}}

	t.Run("with holes", func(t *testing.T) {
		traceStatesRepo(t)
		for _, args := range formats {
			if _, stderr, code := runTraceCLI(t, args...); code != 0 || stderr != "" {
				t.Errorf("trace %v exited %d with holes: %s", args[1:], code, stderr)
			}
		}
		// A guard: the report this subtest exits 0 on does list holes.
		result, block := traceBlock(t)
		holes := block["holes"].([]any)
		if len(holes) == 0 {
			t.Fatal("the fixture has no hole: the subtest proves nothing")
		}
		wantSummary := fmt.Sprintf("trace for markers: %d statement(s), %d hole(s): 1 unnamed, 0 uncovered, 7 not-standing", len(block["statements"].([]any)), len(holes))
		if result["summary"] != wantSummary {
			t.Errorf("summary = %q, want the counts of the lists: %q", result["summary"], wantSummary)
		}
		text, _, _ := runTraceCLI(t, verifyTaskFeature)
		listed := strings.Count(text[strings.Index(text, "\nHoles:\n"):], "\n- ")
		if listed != len(holes) {
			t.Errorf("the text lists %d hole(s) and the result holds %d", listed, len(holes))
		}
	})

	t.Run("sixteen statements and no hole", func(t *testing.T) {
		requirements, tasks := sixteenCriteria()
		root := traceFixture(t, requirements, specDiagnosticsDesign, tasks)
		gitify(t, root)
		completeTraceTask(t, "1.1", 1)
		completeTraceTask(t, "1.2", 1)

		for _, args := range formats {
			if _, stderr, code := runTraceCLI(t, args...); code != 0 || stderr != "" {
				t.Errorf("trace %v exited %d without holes: %s", args[1:], code, stderr)
			}
		}
		result, block := traceBlock(t)
		if got := len(block["statements"].([]any)); got != 16 {
			t.Fatalf("the trace lists %d statement(s), want 16", got)
		}
		if got := len(block["holes"].([]any)); got != 0 {
			t.Fatalf("the trace lists %d hole(s), want none: %v", got, block["holes"])
		}
		if result["summary"] != "trace for markers: 16 statement(s), 0 hole(s)" {
			t.Errorf("summary = %q, want sixteen statements and no hole", result["summary"])
		}
		text, _, _ := runTraceCLI(t, verifyTaskFeature)
		if !strings.HasPrefix(text, "Summary: trace for markers: 16 statement(s), 0 hole(s)\n") || !strings.Contains(text, "\nHoles: none\nExit code: 0\n") {
			t.Errorf("the text does not state sixteen statements and no hole:\n%s", text)
		}
		markdown, _, _ := runTraceCLI(t, verifyTaskFeature, "--markdown")
		if !strings.HasSuffix(markdown, "\n\nHoles: none\n") {
			t.Errorf("the Markdown does not end with the line of no holes:\n%s", markdown)
		}
	})
}

// TestTraceRefusesBothFormats: --markdown with --json is refused, naming the
// two flags, and no trace is printed.
func TestTraceRefusesBothFormats(t *testing.T) {
	traceStatesRepo(t)
	for _, args := range [][]string{
		{verifyTaskFeature, "--markdown", "--json"},
		{"--json", verifyTaskFeature, "--markdown"},
	} {
		stdout, stderr, code := runTraceCLI(t, args...)
		if code == 0 {
			t.Fatalf("trace %v exited 0", args)
		}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
			t.Fatalf("the refusal is not an envelope: %v\n%s", err, stdout)
		}
		result, _ := envelope["result"].(map[string]any)
		summary, _ := result["summary"].(string)
		if envelope["ok"] != false || !strings.Contains(summary, "--markdown") || !strings.Contains(summary, "--json") {
			t.Errorf("the refusal = %v, want ok false and both flags named", envelope)
		}
		if _, present := result["trace"]; present || strings.Contains(stdout, "| Statement") || stderr != "" {
			t.Errorf("the refusal prints a trace or writes to stderr: %s%s", stdout, stderr)
		}
	}
}

// TestTraceFailures: what the trace cannot read fails it, naming the feature
// or the file and the cause, in text and in the envelope, and no statement is
// printed.
func TestTraceFailures(t *testing.T) {
	const specDir = ".walden/specs/" + verifyTaskFeature + "/"
	const ledger = ".walden/evidence/" + verifyTaskFeature + ".json"
	writeRaw := func(t *testing.T, root, name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name string
		args []string
		// arrange changes the repository and returns the fragments the
		// message must hold.
		arrange func(t *testing.T, root string) []string
	}{
		{
			name: "a feature without a specification",
			args: []string{"nope"},
			arrange: func(t *testing.T, root string) []string {
				return []string{`feature "nope" does not exist`}
			},
		},
		{
			name: "a plan the parser rejects",
			arrange: func(t *testing.T, root string) []string {
				writeRaw(t, root, specDir+"tasks.md", strings.Replace(tracePlan(), "        covers: [\"R1.AC1\"]", "        coverz: [\"R1.AC1\"]", 1))
				feature, err := spec.LoadFeature(root, verifyTaskFeature)
				if err != nil {
					t.Fatalf("load the fixture: %v", err)
				}
				_, parseErr := spec.ParseTaskTree(feature.Tasks)
				if parseErr == nil || !strings.Contains(parseErr.Error(), "coverz") {
					t.Fatalf("the parser does not reject the plan on its line: %v", parseErr)
				}
				// The file, then the message of the parser.
				return []string{specDir + "tasks.md: " + parseErr.Error()}
			},
		},
		{
			name: "a ledger that is not JSON",
			arrange: func(t *testing.T, root string) []string {
				writeRaw(t, root, ledger, "<<<<<<< HEAD\n")
				return []string{ledger + ": parse: "}
			},
		},
		{
			name: "requirements that are missing",
			arrange: func(t *testing.T, root string) []string {
				if err := os.Remove(filepath.Join(root, specDir+"requirements.md")); err != nil {
					t.Fatal(err)
				}
				return []string{specDir + "requirements.md does not exist"}
			},
		},
		{
			name: "requirements without frontmatter",
			arrange: func(t *testing.T, root string) []string {
				writeRaw(t, root, specDir+"requirements.md", "# Requirements Document\n")
				return []string{specDir + "requirements.md: ", "frontmatter"}
			},
		},
		{
			name: "no feature name",
			args: []string{},
			arrange: func(t *testing.T, root string) []string {
				return []string{"exactly one feature name"}
			},
		},
		{
			name: "two feature names",
			args: []string{verifyTaskFeature, "other"},
			arrange: func(t *testing.T, root string) []string {
				return []string{"exactly one feature name"}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := traceFixture(t, traceRequirements, specDiagnosticsDesign, tracePlan())
			fragments := tc.arrange(t, root)
			args := tc.args
			if args == nil {
				args = []string{verifyTaskFeature}
			}

			for _, format := range []string{"", "--markdown"} {
				formatted := args
				if format != "" {
					formatted = append(append([]string{}, args...), format)
				}
				stdout, stderr, code := runTraceCLI(t, formatted...)
				if code == 0 {
					t.Fatalf("trace %v exited 0", formatted)
				}
				for _, fragment := range fragments {
					if !strings.Contains(stderr, fragment) {
						t.Errorf("trace %v says %q, want it to hold %q", formatted, stderr, fragment)
					}
				}
				if stdout != "" {
					t.Errorf("trace %v printed on stdout after failing: %q", formatted, stdout)
				}
			}

			stdout, stderr, code := runTraceCLI(t, append(append([]string{}, args...), "--json")...)
			if code == 0 || stderr != "" {
				t.Fatalf("trace --json exited %d and wrote %q to stderr", code, stderr)
			}
			var envelope map[string]any
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatalf("the failure does not travel in an envelope: %v\n%s", err, stdout)
			}
			result, _ := envelope["result"].(map[string]any)
			summary, _ := result["summary"].(string)
			for _, fragment := range fragments {
				if !strings.Contains(summary, fragment) {
					t.Errorf("the envelope says %q, want it to hold %q", summary, fragment)
				}
			}
			if _, present := result["trace"]; present || envelope["ok"] != false || envelope["command"] != "trace" {
				t.Errorf("the envelope of a failure = %v, want ok false, the command and no trace", envelope)
			}
		})
	}
}
