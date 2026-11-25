package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	readmeOpening = "Walden proves that your code meets its spec"
	readmeBanner  = `<p align="center">`
	readmeTrace   = "after-change"
	linkedPull    = "](https://github.com/andrearaponi/walden/pull/87)"
)

var (
	linkTarget  = regexp.MustCompile(`\]\([^)]*\)`)
	sentenceEnd = regexp.MustCompile(`[.:](\s|$)`)
)

// shownTrace returns the trace a document shows between the markers of the
// given name.
func shownTrace(document, name string) (string, error) {
	blocks, err := docBlocks(document)
	if err != nil {
		return "", err
	}
	var found []docBlock
	for _, block := range blocks {
		if block.kind == "trace" && block.name == name {
			found = append(found, block)
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("the trace %s is marked %d time(s), want once", name, len(found))
	}
	return found[0].body, nil
}

// readmeProblems checks the README against the trace the command printed for
// the example after the documented change. It returns what is wrong: nothing
// when the page shows that trace where and how it must.
func readmeProblems(readme, printed string) []string {
	var problems []string
	open, shut := "<!-- trace:"+readmeTrace+" -->", "<!-- /trace:"+readmeTrace+" -->"
	opening, banner := strings.Index(readme, readmeOpening), strings.Index(readme, readmeBanner)
	start, end := strings.Index(readme, open), strings.Index(readme, shut)
	if opening < 0 || banner < 0 || start < 0 || end < 0 || !(opening < start && start < end && end < banner) {
		return []string{fmt.Sprintf("README.md: the trace must stand between the opening paragraph (at %d) and the banner (at %d); its markers stand at %d and %d", opening, banner, start, end)}
	}
	shown, err := shownTrace(readme, readmeTrace)
	if err != nil {
		return []string{"README.md: " + err.Error()}
	}
	if strings.Contains(shown, "```") || !strings.HasPrefix(shown, "| Statement |") {
		problems = append(problems, "README.md: the trace must be Markdown the page renders as a table, not a code block")
	}
	if drift := traceDrift("README.md", shown, printed); drift != "" {
		problems = append(problems, drift)
	}

	// The text beside the table: from the end of the opening paragraph to
	// the banner, without the trace.
	paragraphEnd := opening + strings.Index(readme[opening:], "\n")
	beside := readme[paragraphEnd:start] + readme[end+len(shut):banner]
	for _, fact := range []string{"`walden trace`", "](" + exampleDir + ")", "a typo broke one of its three commands", "Before the typo every row read `verified`", linkedPull} {
		if !strings.Contains(beside, fact) {
			problems = append(problems, fmt.Sprintf("README.md: the text beside the table does not hold %q", fact))
		}
	}

	// Brevity: one table of ten rows at most, four sentences at most.
	rows := 0
	for _, line := range strings.Split(shown, "\n") {
		if strings.HasPrefix(line, "| `") {
			rows++
		}
	}
	if rows == 0 || rows > 10 {
		problems = append(problems, fmt.Sprintf("README.md: the table has %d row(s), want between one and ten", rows))
	}
	if tables := strings.Count(readme, "| Statement |"); tables != 1 {
		problems = append(problems, fmt.Sprintf("README.md: %d trace tables, want one", tables))
	}
	if sentences := len(sentenceEnd.FindAllString(linkTarget.ReplaceAllString(strings.TrimSpace(beside), "]"), -1)); sentences > 4 {
		problems = append(problems, fmt.Sprintf("README.md: %d sentences beside the table, want four at most", sentences))
	}
	return problems
}

// The README shows, under its opening paragraph and above its banner, the
// trace of the example after the documented change, as the command prints
// it, with a few sentences that say what it is.
func TestReadmeShowsExampleTrace(t *testing.T) {
	walk := walkGuide(t, readShipped(t, exampleGuide), nil)
	if len(walk.problems) != 0 {
		t.Fatalf("the guide could not be followed:\n%s", strings.Join(walk.problems, "\n"))
	}
	for _, problem := range readmeProblems(readShipped(t, "README.md"), walk.printed(readmeTrace)) {
		t.Error(problem)
	}
}

// A page that shows a trace the command does not print fails the suite, and
// so does a guide whose trace or whose change is not what the walk finds.
// Each case alters one thing and runs the check the other tests report.
func TestTraceBlocksFailOnDrift(t *testing.T) {
	readme := readShipped(t, "README.md")
	guide := readShipped(t, exampleGuide)
	shown, err := shownTrace(readme, readmeTrace)
	if err != nil {
		t.Fatal(err)
	}
	if problems := readmeProblems(readme, shown); len(problems) != 0 {
		t.Fatalf("the README as shipped has problems of its own: %q", problems)
	}
	reported := func(problems []string, wants ...string) bool {
		text := strings.Join(problems, "\n")
		for _, want := range wants {
			if !strings.Contains(text, want) {
				return false
			}
		}
		return len(problems) != 0
	}

	t.Run("one cell of the table changed in the README", func(t *testing.T) {
		row := "| `R3.AC1` | 3 | 3 step 1 | failed |"
		if strings.Count(readme, row) != 1 {
			t.Fatalf("the README does not hold the row %q once", row)
		}
		problems := readmeProblems(strings.Replace(readme, row, "| `R3.AC1` | 3 | 3 step 1 | verified |", 1), shown)
		if !reported(problems, "line 7 shows:  | `R3.AC1` | 3 | 3 step 1 | verified |", "line 7 prints: | `R3.AC1` | 3 | 3 step 1 | failed |") {
			t.Errorf("a changed cell is not reported with its line: %q", problems)
		}
	})

	t.Run("one proof of the example changed", func(t *testing.T) {
		walk := walkGuide(t, guide, func(t *testing.T, root string) {
			path := filepath.Join(root, ".walden", "specs", exampleFeature, "tasks.md")
			plan, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			step := "    - command: [\"sh\", \"tests/test_todo.sh\", \"list\"]\n"
			if strings.Count(string(plan), step) != 1 {
				t.Fatal("the plan of the example does not hold the proof step of list once")
			}
			changed := strings.Replace(string(plan), step, "    - command: [\"sh\", \"tests/test_todo.sh\"]\n", 1)
			if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
				t.Fatal(err)
			}
			integrityMustCLI(t, "reconcile", exampleFeature)
			integrityMustCLI(t, "review", "open", exampleFeature, "--phase", "tasks")
			integrityMustCLI(t, "review", "approve", exampleFeature, "--phase", "tasks")
		})
		problems := readmeProblems(readme, walk.printed(readmeTrace))
		if !reported(problems, "shows:  | `R2.AC1` | 2 | 2 step 1 | verified | `sh tests/test_todo.sh list` |", "prints: | `R2.AC1` | 2 | 2 step 1 | failed | `sh tests/test_todo.sh` |") {
			t.Errorf("a changed proof is not reported with the row it moves: %q", problems)
		}
		if !reported(walk.problems, "trace:delivered", "trace:after-change") {
			t.Errorf("a changed proof is not reported for the traces of the guide: %q", walk.problems)
		}
	})

	t.Run("one line of a trace changed in the guide", func(t *testing.T) {
		if strings.Count(guide, "\nHoles: none\n") != 1 {
			t.Fatal("the guide does not hold the line of the trace without holes once")
		}
		walk := walkGuide(t, strings.Replace(guide, "\nHoles: none\n", "\nHoles: 1\n", 1), nil)
		if len(walk.problems) != 1 || !reported(walk.problems, "trace:delivered", "shows:  Holes: 1", "prints: Holes: none") {
			t.Errorf("a changed line of a trace is not reported with its block and nothing else: %q", walk.problems)
		}
	})

	t.Run("the line to replace changed in the guide", func(t *testing.T) {
		line := "-    sed -i.bak \"${index}s/^TODO/DONE/\""
		if strings.Count(guide, line) != 1 {
			t.Fatal("the guide does not hold the line to replace once")
		}
		walk := walkGuide(t, strings.Replace(guide, line, "-    sed -i \"${index}s/^TODO/DONE/\"", 1), nil)
		if !reported(walk.problems, "diff block at line", "src/todo.sh does not hold the line to replace once") {
			t.Errorf("a line the example does not hold is not reported with its block: %q", walk.problems)
		}
	})

	t.Run("a change that breaks nothing", func(t *testing.T) {
		line := "+    sed -i.bak \"${index}s/^TODO/DOEN/\""
		if strings.Count(guide, line) != 1 {
			t.Fatal("the guide does not hold the replacing line once")
		}
		walk := walkGuide(t, strings.Replace(guide, line, "+    sed -i.bak  \"${index}s/^TODO/DONE/\"", 1), nil)
		if !reported(walk.problems, "trace:after-change", "prints: Holes: none") {
			t.Errorf("a change that breaks no proof is not reported at the trace after it: %q", walk.problems)
		}
	})

	t.Run("a marker removed", func(t *testing.T) {
		problems := readmeProblems(strings.Replace(readme, "<!-- /trace:"+readmeTrace+" -->", "", 1), shown)
		if !reported(problems, "the trace must stand between") {
			t.Errorf("a trace without its closing marker is not reported: %q", problems)
		}
	})
}

// The changelog says that the example was rewritten and that the README
// shows its trace.
func TestReadmeTraceChangelog(t *testing.T) {
	changelog := readShipped(t, "CHANGELOG.md")
	released := strings.Index(changelog, "\n## [0.16.1]")
	if released < 0 {
		t.Fatal("CHANGELOG.md has no heading for v0.16.1")
	}
	var entry string
	for _, line := range strings.Split(changelog[:released], "\n") {
		if strings.HasPrefix(line, "- ") && strings.Contains(line, "`"+exampleDir+"`") {
			entry = line
		}
	}
	if entry == "" {
		t.Fatalf("above the heading of v0.16.1, CHANGELOG.md holds no entry that names `%s`", exampleDir)
	}
	for _, fact := range []string{"one proof step for each acceptance criterion", "README", "`walden trace`"} {
		if !strings.Contains(entry, fact) {
			t.Errorf("the entry does not mention %q", fact)
		}
	}
}
