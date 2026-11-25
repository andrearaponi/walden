package app

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/trace"
)

const (
	exampleDir     = "examples/todo-app-demo"
	exampleFeature = "todo-app"
	exampleGuide   = exampleDir + "/README.md"

	exampleTraceCommand = "walden trace " + exampleFeature + " --markdown"
)

// docBlock is one fenced block of a Markdown document, or one trace between
// its markers. The markers are HTML comments: the page shows the table and
// not the markers.
type docBlock struct {
	kind string // the language of the fence, or "trace"
	name string // for a trace, the name its markers carry
	body string
	line int // the line the block opens at
}

var traceMarker = regexp.MustCompile(`^<!-- (/?)trace:([a-z-]+) -->$`)

// docBlocks returns the fenced blocks and the marked traces of a document,
// in the order the document gives them.
func docBlocks(document string) ([]docBlock, error) {
	var blocks []docBlock
	var open *docBlock
	var body []string
	for index, line := range strings.Split(document, "\n") {
		marker := traceMarker.FindStringSubmatch(line)
		switch {
		case open == nil && strings.HasPrefix(line, "```"):
			open = &docBlock{kind: strings.TrimPrefix(line, "```"), line: index + 1}
		case open == nil && marker != nil && marker[1] == "":
			open = &docBlock{kind: "trace", name: marker[2], line: index + 1}
		case open == nil && marker != nil:
			return nil, fmt.Errorf("line %d closes the trace %s, which is not open", index+1, marker[2])
		case open == nil:
			// Prose.
		case open.kind == "trace" && marker != nil && marker[1] == "/" && marker[2] == open.name:
			open.body = strings.Trim(strings.Join(body, "\n"), "\n")
			blocks, open, body = append(blocks, *open), nil, nil
		case open.kind != "trace" && line == "```":
			open.body = strings.Join(body, "\n")
			blocks, open, body = append(blocks, *open), nil, nil
		default:
			body = append(body, line)
		}
	}
	if open != nil {
		return nil, fmt.Errorf("the block opened at line %d is not closed", open.line)
	}
	return blocks, nil
}

// traceDrift compares the trace a document shows with the one the command
// printed. It returns nothing when they are the same bytes; otherwise the
// lines that differ, and the whole trace as printed, ready to paste.
func traceDrift(where, shown, printed string) string {
	printed = strings.Trim(printed, "\n")
	if shown == printed {
		return ""
	}
	shownLines, printedLines := strings.Split(shown, "\n"), strings.Split(printed, "\n")
	var out strings.Builder
	fmt.Fprintf(&out, "%s shows a trace the command does not print:\n", where)
	for index := 0; index < len(shownLines) || index < len(printedLines); index++ {
		var a, b string
		if index < len(shownLines) {
			a = shownLines[index]
		}
		if index < len(printedLines) {
			b = printedLines[index]
		}
		if a != b {
			fmt.Fprintf(&out, "  line %d shows:  %s\n  line %d prints: %s\n", index+1, a, index+1, b)
		}
	}
	fmt.Fprintf(&out, "the command prints:\n%s\n", printed)
	return out.String()
}

// changeOf reads a diff block of the guide: the line of src/todo.sh to
// replace and the line that replaces it.
func changeOf(block docBlock) (from, to string, err error) {
	lines := strings.Split(block.body, "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "-") || !strings.HasPrefix(lines[1], "+") {
		return "", "", fmt.Errorf("want one removed line and one added line, got %q", lines)
	}
	return lines[0][1:], lines[1][1:], nil
}

// applyChange replaces one line of src/todo.sh in the example at root.
func applyChange(root, from, to string) error {
	path := filepath.Join(root, "src", "todo.sh")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Count(string(data), from+"\n") != 1 {
		return fmt.Errorf("src/todo.sh does not hold the line to replace once: %q", from)
	}
	return os.WriteFile(path, []byte(strings.Replace(string(data), from+"\n", to+"\n", 1)), 0o644)
}

// applyDocumentedChange makes, in the example at root, the change the guide
// gives in its diff block.
func applyDocumentedChange(root, guide string) error {
	blocks, err := docBlocks(guide)
	if err != nil {
		return err
	}
	for _, block := range blocks {
		if block.kind != "diff" {
			continue
		}
		from, to, err := changeOf(block)
		if err != nil {
			return err
		}
		return applyChange(root, from, to)
	}
	return fmt.Errorf("the guide holds no diff block")
}

// cloneLines returns the two lines of the guide that the copy made by the
// test stands for: the clone of this repository, from the module path of
// go.mod, and the change of directory into the example of that clone.
func cloneLines(t *testing.T) (clone, enter string) {
	t.Helper()
	for _, line := range strings.Split(readShipped(t, "go.mod"), "\n") {
		if module, found := strings.CutPrefix(line, "module "); found {
			module = strings.TrimSpace(module)
			name := module[strings.LastIndex(module, "/")+1:]
			return "git clone https://" + module + ".git", "cd " + name + "/" + exampleDir
		}
	}
	t.Fatal("go.mod names no module")
	return "", ""
}

// readShipped reads a file of this repository, by its path from the top.
func readShipped(t *testing.T, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(authoringSourceRoot(t), filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return string(data)
}

// shippedExample copies the example as this working tree holds it into a new
// repository, at the path it has here, commits it and enters it. The copy
// leaves out the evidence a run in place would have written, which this
// repository ignores. The example is a root in a subdirectory of its
// repository, here as in the repository that ships it.
func shippedExample(t *testing.T) (top, root string) {
	t.Helper()
	for _, tool := range []string{"git", "sh"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not available in PATH")
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	source := filepath.Join(authoringSourceRoot(t), filepath.FromSlash(exampleDir))
	top = t.TempDir()
	root = filepath.Join(top, filepath.FromSlash(exampleDir))
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if filepath.ToSlash(relative) == ".walden/evidence" {
			return filepath.SkipDir
		}
		target := filepath.Join(root, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatalf("copy the example: %v", err)
	}
	gitify(t, top)
	t.Chdir(root)
	return top, root
}

// guideEvent is one thing the guide gives, in its order: a command and how
// it ended, the documented change, or a trace it shows.
type guideEvent struct {
	label  string // the command, "change", or "trace:<name>"
	code   int
	output string // what the command printed; for a shown trace, what the trace command before it printed
}

// guideWalk is what happened when the guide was followed on a copy of the
// shipped example.
type guideWalk struct {
	top, root string
	events    []guideEvent
	named     []string        // the paths of the example the guide names
	held      map[string]bool // those the example held at some point of the walk
	problems  []string
}

func (w guideWalk) labels() []string {
	labels := make([]string, len(w.events))
	for index, event := range w.events {
		labels[index] = event.label
	}
	return labels
}

// find returns the index of the first event with the label at or after from.
func (w guideWalk) find(label string, from int) int {
	for index := from; index < len(w.events); index++ {
		if w.events[index].label == label {
			return index
		}
	}
	return -1
}

// printed returns what the command printed for the trace the guide shows
// under the given name.
func (w guideWalk) printed(name string) string {
	if index := w.find("trace:"+name, 0); index >= 0 {
		return w.events[index].output
	}
	return ""
}

var guidePath = regexp.MustCompile(`(?:\.walden|src|tests|scripts)/[A-Za-z0-9_./-]*[A-Za-z0-9_]`)

// runGuideCommand runs one command line of the guide in the example at root.
// It knows the commands a reader types there and refuses any other. clone and
// enter are the two lines the copy stands for.
func runGuideCommand(root, line, clone, enter string) (guideEvent, error) {
	event := guideEvent{label: line}
	fields := strings.Fields(line)
	switch {
	case line == clone, line == enter:
		// The copy stands for the clone, and the test is already in it.
	case fields[0] == "walden":
		var stdout, stderr bytes.Buffer
		event.code = Run(fields[1:], &stdout, &stderr)
		event.output = stdout.String() + stderr.String()
	case len(fields) == 3 && fields[0] == "git" && fields[1] == "checkout":
		out, err := exec.Command("git", "-C", root, "checkout", fields[2]).CombinedOutput()
		event.output = string(out)
		if err != nil {
			event.code = 1
		}
	case len(fields) == 3 && fields[0] == "rm" && fields[1] == "-rf" && filepath.IsLocal(fields[2]):
		if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(fields[2]))); err != nil {
			return event, err
		}
	default:
		return event, fmt.Errorf("the guide gives a command this test cannot run: %s", line)
	}
	return event, nil
}

// walkGuide follows the guide on a copy of the shipped example: it runs each
// command of its bash blocks, makes the change of its diff block and compares
// each trace it shows with what the trace command before it printed. The
// proofs are the shell tests of the example, run for real. prepare, when
// given, alters the copy before the first command.
func walkGuide(t *testing.T, guide string, prepare func(t *testing.T, root string)) guideWalk {
	t.Helper()
	clone, enter := cloneLines(t)
	top, root := shippedExample(t)
	if prepare != nil {
		prepare(t, root)
	}
	walk := guideWalk{top: top, root: root, held: map[string]bool{}, named: guidePath.FindAllString(guide, -1)}
	note := func() {
		for _, path := range walk.named {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err == nil {
				walk.held[path] = true
			}
		}
	}
	note()
	blocks, err := docBlocks(guide)
	if err != nil {
		walk.problems = append(walk.problems, exampleGuide+": "+err.Error())
		return walk
	}
	printed, asked := "", false
	for _, block := range blocks {
		switch block.kind {
		case "bash":
			for _, line := range strings.Split(block.body, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				event, err := runGuideCommand(root, line, clone, enter)
				if err != nil {
					walk.problems = append(walk.problems, fmt.Sprintf("%s, bash block at line %d: %v", exampleGuide, block.line, err))
					return walk
				}
				walk.events = append(walk.events, event)
				if line == exampleTraceCommand {
					printed, asked = event.output, true
				}
				note()
			}
		case "diff":
			from, to, err := changeOf(block)
			if err == nil {
				err = applyChange(root, from, to)
			}
			if err != nil {
				walk.problems = append(walk.problems, fmt.Sprintf("%s, diff block at line %d: %v", exampleGuide, block.line, err))
				return walk
			}
			walk.events = append(walk.events, guideEvent{label: "change"})
		case "trace":
			where := fmt.Sprintf("%s, trace:%s at line %d", exampleGuide, block.name, block.line)
			if !asked {
				walk.problems = append(walk.problems, where+": no `"+exampleTraceCommand+"` comes before it")
				continue
			}
			walk.events = append(walk.events, guideEvent{label: "trace:" + block.name, output: printed})
			if drift := traceDrift(where, block.body, printed); drift != "" {
				walk.problems = append(walk.problems, drift)
			}
			asked = false
		}
	}
	return walk
}

func exampleTrace(t *testing.T) *trace.Report {
	t.Helper()
	result := integrityMustCLI(t, "trace", exampleFeature)
	if result.Trace == nil {
		t.Fatal("walden trace returned no report")
	}
	return result.Trace
}

// The example gives a complete trace: every criterion has a proof step of its
// own, no statement the plan names lacks one, and a change that breaks one
// command fails the criteria of that command and no other.
func TestShippedExampleTrace(t *testing.T) {
	guide := readShipped(t, exampleGuide)
	_, root := shippedExample(t)

	// As shipped: an approved and fresh chain, nothing completed, no ledger.
	status := integrityMustCLI(t, "status", exampleFeature)
	if len(status.Documents) != 3 {
		t.Fatalf("status reports %d document(s), want three", len(status.Documents))
	}
	for _, document := range status.Documents {
		if document.Status != "approved" || !document.Fresh {
			t.Errorf("%s ships %s, fresh=%t; want approved and fresh", document.Name, document.Status, document.Fresh)
		}
	}
	if validated := integrityMustCLI(t, "validate", exampleFeature, "--all"); len(validated.Warnings) != 0 {
		t.Errorf("validate --all warns about the example: %v", validated.Warnings)
	}
	plan, err := os.ReadFile(filepath.Join(root, ".walden", "specs", exampleFeature, "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plan), "- [x]") || !strings.Contains(string(plan), "- [ ]") {
		t.Fatalf("the plan of the example must ship with every task unchecked (was the example run in place? see git status %s)", exampleDir)
	}
	source := authoringSourceRoot(t)
	if _, err := os.Stat(filepath.Join(source, ".git")); err == nil {
		tracked, err := exec.Command("git", "-C", source, "ls-files", "--", exampleDir+"/.walden/evidence").Output()
		if err != nil {
			t.Fatalf("git ls-files: %v", err)
		}
		if strings.TrimSpace(string(tracked)) != "" {
			t.Errorf("the repository tracks evidence of the example: %s", tracked)
		}
	} else {
		t.Log("not a git checkout: whether git tracks evidence of the example was not checked")
	}
	requirements, err := os.ReadFile(filepath.Join(root, ".walden", "specs", exampleFeature, "requirements.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(requirements), "- `NFR1`") {
		t.Error("the requirements of the example no longer define their non-functional requirement")
	}

	// The tasks completed: seven statements, each with a proof, no hole.
	integrityMustCLI(t, "task", "complete-all", exampleFeature)
	report := exampleTrace(t)
	if len(report.Statements) != 7 || len(report.Holes) != 0 {
		t.Fatalf("the trace has %d statement(s) and %d hole(s), want seven and none", len(report.Statements), len(report.Holes))
	}
	owner := map[string]string{}
	criteria := 0
	for _, statement := range report.Statements {
		if len(statement.Proofs) == 0 {
			t.Errorf("%s is named by the plan and no proof step covers it", statement.ID)
		}
		if statement.Kind == trace.KindNFR {
			t.Errorf("the plan names %s, which no proof of the example covers", statement.ID)
		}
		for _, proof := range statement.Proofs {
			if proof.State != evidence.StateVerified {
				t.Errorf("%s: %s step %d reads %s after completion", statement.ID, proof.TaskID, proof.Step, proof.State)
			}
		}
		if statement.Kind != trace.KindCriterion {
			continue
		}
		criteria++
		if len(statement.Proofs) != 1 {
			t.Errorf("%s is covered by %d proof step(s), want one of its own", statement.ID, len(statement.Proofs))
			continue
		}
		step := fmt.Sprintf("%s step %d", statement.Proofs[0].TaskID, statement.Proofs[0].Step)
		if other, taken := owner[step]; taken {
			t.Errorf("%s and %s share the proof step %s", other, statement.ID, step)
		}
		owner[step] = statement.ID
	}
	if criteria != 6 {
		t.Errorf("the trace lists %d criteria, want the six of the example", criteria)
	}

	// The documented change: the task of the changed command fails, alone.
	if err := applyDocumentedChange(root, guide); err != nil {
		t.Fatal(err)
	}
	verified, code := integrityCLI(t, "verify", exampleFeature)
	if code == 0 || verified.Summary != "verification failed for "+exampleFeature+": 3" {
		t.Fatalf("verify after the change: exit %d, %q; want the failure of task 3 alone", code, verified.Summary)
	}
	report = exampleTrace(t)
	var failed []string
	for _, statement := range report.Statements {
		for _, proof := range statement.Proofs {
			want := evidence.StateVerified
			if proof.TaskID == "3" {
				want = evidence.StateFailed
			}
			if proof.State != want {
				t.Errorf("%s: %s step %d reads %s, want %s", statement.ID, proof.TaskID, proof.Step, proof.State, want)
			}
			if proof.State == evidence.StateFailed {
				failed = append(failed, statement.ID)
			}
		}
	}
	if strings.Join(failed, " ") != "R3.AC1 R3.AC2" {
		t.Errorf("the statements that read failed are %v, want the two criteria of the changed command", failed)
	}
	if len(report.Holes) != 2 {
		t.Fatalf("the trace lists %d hole(s), want two", len(report.Holes))
	}
	for _, hole := range report.Holes {
		if hole.Kind != trace.HoleNotStanding {
			t.Errorf("the hole of %s is %s, want not-standing", hole.Statement, hole.Kind)
		}
	}
}

// The guide of the example leads a reader to both traces and back, shows the
// traces as the command prints them, and says where the evidence is written.
func TestExampleGuide(t *testing.T) {
	guide := readShipped(t, exampleGuide)
	walk := walkGuide(t, guide, nil)
	for _, problem := range walk.problems {
		t.Error(problem)
	}

	// The steps, as the guide gives them and in its order.
	clone, enter := cloneLines(t)
	steps := []string{
		clone,
		enter,
		"walden task complete-all " + exampleFeature,
		exampleTraceCommand,
		"trace:delivered",
		"change",
		"walden verify " + exampleFeature,
		exampleTraceCommand,
		"trace:after-change",
		"git checkout src/todo.sh",
		"walden verify " + exampleFeature,
		"walden trace " + exampleFeature,
	}
	at := make([]int, len(steps))
	from := 0
	for index, step := range steps {
		at[index] = walk.find(step, from)
		if at[index] < 0 {
			t.Fatalf("the guide does not give %q after the steps before it; it gives: %q", step, walk.labels())
		}
		from = at[index] + 1
	}

	// Each command ends as the guide says: the verify after the change
	// fails and names task 3, and every other command succeeds.
	failing := at[6]
	for index, event := range walk.events {
		switch {
		case index == failing:
			if event.code == 0 || !strings.Contains(event.output, "verification failed for "+exampleFeature+": 3\n") {
				t.Errorf("the guide says %q fails and names task 3; it exited %d and printed:\n%s", event.label, event.code, event.output)
			}
		case event.code != 0:
			t.Errorf("%q exited %d and printed:\n%s", event.label, event.code, event.output)
		}
	}
	if !strings.Contains(guide, "The run fails and names task 3") {
		t.Error("the guide no longer says that the verify after the change fails and names task 3")
	}
	for name, want := range map[string]string{
		"delivered":    "\nHoles: none",
		"after-change": "\nHoles: 2 (0 unnamed, 0 uncovered, 2 not-standing)",
	} {
		if !strings.Contains(walk.printed(name), want) {
			t.Errorf("the trace %s does not read %q:\n%s", name, strings.TrimSpace(want), walk.printed(name))
		}
	}
	if back := walk.events[at[11]].output; !strings.Contains(back, "7 statement(s), 0 hole(s)") {
		t.Errorf("after the way back the trace still lists holes:\n%s", back)
	}

	// And back: the last commands leave the example as it shipped.
	if left := integrityGit(t, walk.top, "status", "--porcelain"); left != "" {
		t.Errorf("after the last command of the guide the example is not as it shipped:\n%s", left)
	}

	// The guide names no file the example does not hold.
	for _, path := range walk.named {
		if !walk.held[path] {
			t.Errorf("the guide names %s, which the example does not hold at any step", path)
		}
	}

	// What the reader needs, and the ledger: written on the reader's
	// machine, not in the repository.
	for _, fact := range []string{
		"git, a POSIX shell and the `walden` CLI, v0.16.1 or newer",
		"`.walden/evidence/todo-app.json`",
		"Completing the tasks writes it on your machine",
		"it is not part of this repository, which ignores it",
	} {
		if !strings.Contains(guide, fact) {
			t.Errorf("the guide does not say %q", fact)
		}
	}
	source := authoringSourceRoot(t)
	if _, err := os.Stat(filepath.Join(source, ".git")); err == nil {
		ledger := exampleDir + "/.walden/evidence/" + exampleFeature + ".json"
		if err := exec.Command("git", "-C", source, "check-ignore", "-q", ledger).Run(); err != nil {
			t.Errorf("the guide says the repository ignores the ledger; git check-ignore %s: %v", ledger, err)
		}
	}
}
