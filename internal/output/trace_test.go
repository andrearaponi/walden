package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/trace"
)

// traceRepeats is how many times each whole-output comparison runs: a
// renderer that iterated a map would change its output between runs.
const traceRepeats = 50

// longCommand is shown as 400 characters, sh -c and the quotes included: 392
// inside the quotes. It holds && and >.
var longCommand = "true" + strings.Repeat(" && true", 48) + " >&2"

// fixedTrace is a report written by hand, not built by the join. It holds a
// delivery leaf, a statement no step covers, two statements no leaf names, a
// command longer than the limit, and the commands whose Markdown GitHub was
// shown to render as rows of five cells: a pipe, a backslash before a pipe,
// backticks and escaped quotes.
func fixedTrace() trace.Report {
	shTrue := []string{"sh", "-c", "true"}
	return trace.Report{
		Feature: "demo",
		Statements: []trace.Statement{
			{ID: "R1.AC1", Kind: trace.KindCriterion, NamedBy: []string{"1.1"}, Proofs: []trace.Proof{
				{TaskID: "1.1", Step: 1, Command: []string{"grep", "-q", "released", "notes.txt"}, Delivery: true, State: "delivered"},
			}},
			{ID: "R1.AC2", Kind: trace.KindCriterion, NamedBy: []string{"1.2", "1.4"}, Proofs: []trace.Proof{
				{TaskID: "1.2", Step: 1, Command: shTrue, State: "stale-code"},
				{TaskID: "1.4", Step: 1, Command: shTrue, State: "pending"},
			}},
			{ID: "R1.AC3", Kind: trace.KindCriterion, NamedBy: []string{"1.1", "1.3"}, Proofs: []trace.Proof{
				{TaskID: "1.3", Step: 1, Command: []string{"sh", "-c", `printf 'a|b' | grep -q 'a\|b'`}, State: "verified"},
				{TaskID: "1.3", Step: 2, Command: []string{"sh", "-c", "test \"`echo ok`\" = ok"}, State: "verified"},
				{TaskID: "1.3", Step: 3, Command: []string{"sh", "-c", longCommand}, State: "verified"},
			}},
			{ID: "R1.AC4", Kind: trace.KindCriterion, NamedBy: []string{}, Proofs: []trace.Proof{}},
			{ID: "R1.AC5", Kind: trace.KindCriterion, NamedBy: []string{"1.2"}, Proofs: []trace.Proof{}},
			{ID: "NFR1", Kind: trace.KindNFR, NamedBy: []string{"1.2"}, Proofs: []trace.Proof{
				{TaskID: "1.2", Step: 1, Command: shTrue, State: "stale-code"},
			}},
			{ID: "C1", Kind: trace.KindConstraint, NamedBy: []string{}, Proofs: []trace.Proof{
				{TaskID: "1.4", Step: 1, Command: shTrue, State: "pending"},
			}},
		},
		Leaves: []trace.Leaf{
			{ID: "1.1", DesignRefs: []string{"Architecture"}, Delivery: true, State: "delivered"},
			{ID: "1.2", DesignRefs: []string{"Architecture", "Verification Plan"}, State: "stale-code"},
			{ID: "1.3", DesignRefs: []string{"Architecture"}, State: "verified"},
			{ID: "1.4", DesignRefs: []string{"Architecture"}, State: "pending"},
		},
		Holes: []trace.Hole{
			{Kind: trace.HoleUnnamed, Statement: "R1.AC4"},
			// The corner of the two readers: a leaf names it for the parser.
			{Kind: trace.HoleUnnamed, Statement: "R1.AC7", NamedBy: []string{"1.3"}},
			{Kind: trace.HoleUncovered, Statement: "R1.AC5", NamedBy: []string{"1.2"}},
			{Kind: trace.HoleNotStanding, Statement: "R1.AC2", TaskID: "1.2", Step: 1, State: "stale-code"},
			{Kind: trace.HoleNotStanding, Statement: "R1.AC2", TaskID: "1.4", Step: 1, State: "pending"},
			{Kind: trace.HoleNotStanding, Statement: "NFR1", TaskID: "1.2", Step: 1, State: "stale-code"},
			{Kind: trace.HoleNotStanding, Statement: "C1", TaskID: "1.4", Step: 1, State: "pending"},
		},
	}
}

// cutCommand is the long command as a reader sees it: its first 119
// characters, sh -c and the opening quote among them, and the mark.
var cutCommand = `sh -c "` + strings.Repeat("true && ", 14) + "…"

const fixedTraceText = `Summary: trace for demo: 7 statement(s), 7 hole(s): 2 unnamed, 1 uncovered, 4 not-standing
Trace:
- R1.AC1: named by 1.1
  - 1.1 step 1, delivered (delivery proof): grep -q released notes.txt
- R1.AC2: named by 1.2, 1.4
  - 1.2 step 1, stale-code: sh -c true
  - 1.4 step 1, pending: sh -c true
- R1.AC3: named by 1.1, 1.3
  - 1.3 step 1, verified: sh -c "printf 'a|b' | grep -q 'a\\|b'"
  - 1.3 step 2, verified: sh -c "test \"` + "`echo ok`" + `\" = ok"
  - 1.3 step 3, verified: CUT
- R1.AC4: named by no leaf
- R1.AC5: named by 1.2
- NFR1: named by 1.2
  - 1.2 step 1, stale-code: sh -c true
- C1: named by no leaf
  - 1.4 step 1, pending: sh -c true
Holes:
- R1.AC4: no leaf names it
- R1.AC7: named by 1.3, which walden validate does not count
- R1.AC5: named by 1.2, covered by no proof step
- R1.AC2: 1.2 step 1 is stale-code
- R1.AC2: 1.4 step 1 is pending
- NFR1: 1.2 step 1 is stale-code
- C1: 1.4 step 1 is pending
Exit code: 0
`

const fixedTraceMarkdown = `| Statement | Named by | Proof step | Evidence | Command |
| --- | --- | --- | --- | --- |
| ` + "`R1.AC1`" + ` | 1.1 | 1.1 step 1 (delivery proof) | delivered | ` + "`grep -q released notes.txt`" + ` |
| ` + "`R1.AC2`" + ` | 1.2, 1.4 | 1.2 step 1 | stale-code | ` + "`sh -c true`" + ` |
| | | 1.4 step 1 | pending | ` + "`sh -c true`" + ` |
| ` + "`R1.AC3`" + ` | 1.1, 1.3 | 1.3 step 1 | verified | ` + "`sh -c \"printf 'a\\|b' \\| grep -q 'a\\\\\\|b'\"`" + ` |
| | | 1.3 step 2 | verified | ` + "`` sh -c \"test \\\"`echo ok`\\\" = ok\" ``" + ` |
| | | 1.3 step 3 | verified | ` + "`CUT`" + ` |
| ` + "`R1.AC4`" + ` | no leaf | | | |
| ` + "`R1.AC5`" + ` | 1.2 | | | |
| ` + "`NFR1`" + ` | 1.2 | 1.2 step 1 | stale-code | ` + "`sh -c true`" + ` |
| ` + "`C1`" + ` | no leaf | 1.4 step 1 | pending | ` + "`sh -c true`" + ` |

Holes: 7 (2 unnamed, 1 uncovered, 4 not-standing)

- ` + "`R1.AC4`" + `: no leaf names it
- ` + "`R1.AC7`" + `: named by 1.3, which walden validate does not count
- ` + "`R1.AC5`" + `: named by 1.2, covered by no proof step
- ` + "`R1.AC2`" + `: 1.2 step 1 is stale-code
- ` + "`R1.AC2`" + `: 1.4 step 1 is pending
- ` + "`NFR1`" + `: 1.2 step 1 is stale-code
- ` + "`C1`" + `: 1.4 step 1 is pending
`

// firstTraceDifference names the first line at which two outputs differ.
func firstTraceDifference(got, want string) string {
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for index := 0; index < len(gotLines) || index < len(wantLines); index++ {
		var gotLine, wantLine string
		if index < len(gotLines) {
			gotLine = gotLines[index]
		}
		if index < len(wantLines) {
			wantLine = wantLines[index]
		}
		if gotLine != wantLine {
			return fmt.Sprintf("line %d:\n  got:  %s\n  want: %s", index+1, gotLine, wantLine)
		}
	}
	return "no difference"
}

// TestPrintTextTrace: the text of a fixed report, compared whole.
func TestPrintTextTrace(t *testing.T) {
	report := fixedTrace()
	want := strings.Replace(fixedTraceText, "CUT", cutCommand, 1)
	if len([]rune(cutCommand)) != trace.CommandLimit {
		t.Fatalf("the cut command written here is %d characters, want %d", len([]rune(cutCommand)), trace.CommandLimit)
	}
	for run := 0; run < traceRepeats; run++ {
		var out bytes.Buffer
		PrintText(&out, Result{Summary: report.Summary(), Trace: &report, ExitCode: 0})
		if got := out.String(); got != want {
			t.Fatalf("the text of the trace differs, %s", firstTraceDifference(got, want))
		}
	}

	t.Run("without holes", func(t *testing.T) {
		clean := trace.Report{Feature: "demo", Leaves: []trace.Leaf{}, Holes: []trace.Hole{}, Statements: []trace.Statement{
			{ID: "R1.AC1", Kind: trace.KindCriterion, NamedBy: []string{"1.1"}, Proofs: []trace.Proof{
				{TaskID: "1.1", Step: 1, Command: []string{"go", "test", "./..."}, State: "verified"},
			}},
		}}
		const want = "Summary: trace for demo: 1 statement(s), 0 hole(s)\nTrace:\n- R1.AC1: named by 1.1\n  - 1.1 step 1, verified: go test ./...\nHoles: none\nExit code: 0\n"
		var out bytes.Buffer
		PrintText(&out, Result{Summary: clean.Summary(), Trace: &clean, ExitCode: 0})
		if got := out.String(); got != want {
			t.Errorf("the text of a trace without holes differs, %s", firstTraceDifference(got, want))
		}
	})

	t.Run("a result without a trace prints no trace section", func(t *testing.T) {
		var out bytes.Buffer
		PrintText(&out, Result{Summary: "workflow status for demo", ExitCode: 0})
		if got, want := out.String(), "Summary: workflow status for demo\nExit code: 0\n"; got != want {
			t.Errorf("the text of another result = %q, want %q", got, want)
		}
	})
}

// TestPrintTraceMarkdown: the Markdown of the same report, compared whole
// with the bytes fixed here. GitHub's Markdown API rendered these row forms
// as rows of five cells with each command intact; no test repeats that
// rendering, which would need the network.
func TestPrintTraceMarkdown(t *testing.T) {
	report := fixedTrace()
	want := strings.Replace(fixedTraceMarkdown, "CUT", cutCommand, 1)
	for run := 0; run < traceRepeats; run++ {
		var out bytes.Buffer
		PrintTraceMarkdown(&out, report)
		if got := out.String(); got != want {
			t.Fatalf("the Markdown of the trace differs, %s", firstTraceDifference(got, want))
		}
	}

	t.Run("without holes", func(t *testing.T) {
		clean := trace.Report{Feature: "demo", Leaves: []trace.Leaf{}, Holes: []trace.Hole{}, Statements: []trace.Statement{
			{ID: "R1.AC1", Kind: trace.KindCriterion, NamedBy: []string{"1.1"}, Proofs: []trace.Proof{
				{TaskID: "1.1", Step: 1, Command: []string{"go", "test", "./..."}, State: "verified"},
			}},
		}}
		const want = "| Statement | Named by | Proof step | Evidence | Command |\n| --- | --- | --- | --- | --- |\n| `R1.AC1` | 1.1 | 1.1 step 1 | verified | `go test ./...` |\n\nHoles: none\n"
		var out bytes.Buffer
		PrintTraceMarkdown(&out, clean)
		if got := out.String(); got != want {
			t.Errorf("the Markdown of a trace without holes differs, %s", firstTraceDifference(got, want))
		}
	})

	t.Run("nothing of the text output", func(t *testing.T) {
		var out bytes.Buffer
		PrintTraceMarkdown(&out, report)
		got := out.String()
		if !strings.HasPrefix(got, "| Statement | Named by | Proof step | Evidence | Command |\n") {
			t.Errorf("the Markdown does not open with the heading row of the table: %q", strings.SplitN(got, "\n", 2)[0])
		}
		for _, line := range []string{"Summary:", "Exit code:", "Trace:"} {
			if strings.Contains(got, line) {
				t.Errorf("the Markdown holds %q, a line of the text output", line)
			}
		}
		if strings.Contains(got, "Architecture") {
			t.Errorf("the Markdown holds a design reference")
		}
	})
}

// TestResultTraceField: the report travels in the result under one key,
// trace, and a result without a report has no such key.
func TestResultTraceField(t *testing.T) {
	keys := func(t *testing.T, result Result) (map[string]any, []string) {
		t.Helper()
		var out bytes.Buffer
		if err := PrintJSON(&out, "trace", result); err != nil {
			t.Fatalf("PrintJSON: %v", err)
		}
		var envelope struct {
			Result map[string]any `json:"result"`
		}
		if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
			t.Fatalf("unmarshal the envelope: %v", err)
		}
		names := []string{}
		for name := range envelope.Result {
			names = append(names, name)
		}
		sort.Strings(names)
		return envelope.Result, names
	}

	report := fixedTrace()
	with, names := keys(t, Result{Summary: report.Summary(), Trace: &report})
	block, present := with["trace"].(map[string]any)
	if !present {
		t.Fatalf("a result with a report has the keys %v, want trace among them", names)
	}
	blockKeys := []string{}
	for name := range block {
		blockKeys = append(blockKeys, name)
	}
	sort.Strings(blockKeys)
	if want := []string{"feature", "holes", "leaves", "statements"}; !reflect.DeepEqual(blockKeys, want) {
		t.Errorf("the keys of the block = %v, want %v", blockKeys, want)
	}

	without, names := keys(t, Result{Summary: "workflow status for demo"})
	if _, present := without["trace"]; present {
		t.Errorf("a result without a report has the keys %v, want no trace", names)
	}
	if want := []string{"changed_files", "exit_code", "skipped_files", "summary", "warnings"}; !reflect.DeepEqual(names, want) {
		t.Errorf("the keys of a result without a report = %v, want %v", names, want)
	}
}
