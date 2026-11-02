package workflow

import (
	"fmt"
	"strings"
)

// emptyRunWarning returns the warning for a passing step whose output shows
// that go test ran no test, or "" when the step stays silent. It quotes the
// declared command and nothing of the output.
func emptyRunWarning(taskID string, step proofCommand, output string) string {
	if !emptyTestRun(output) || emptyRunIntended(step) {
		return ""
	}
	return fmt.Sprintf("task %s: %s passed without running any test: check its selector and assert the test by name with -v and expect_output", taskID, step.Display)
}

// emptyTestRun reports whether output holds at least one package line of
// go test and all of them say that no test ran. A package line starts at the
// beginning of a line, so indented logs and quotations are never read as one.
func emptyTestRun(output string) bool {
	found := false
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimRight(line, " \t\r")
		switch {
		case line == "FAIL" || strings.HasPrefix(line, "FAIL\t"):
			return false
		case strings.HasPrefix(line, "ok  \t"):
			if !strings.HasSuffix(line, "[no tests to run]") {
				return false
			}
			found = true
		case strings.HasPrefix(line, "?   \t"):
			found = found || strings.HasSuffix(line, "[no test files]")
		}
	}
	return found
}

// emptyRunIntended reports a step that declares its empty run: an expect_output
// asserting the marker, or the compile-only selector ^$.
func emptyRunIntended(step proofCommand) bool {
	if strings.Contains(step.ExpectOutput, "no tests to run") || strings.Contains(step.ExpectOutput, "no test files") {
		return true
	}
	fields := strings.Fields(strings.Join(append([]string{step.Name}, step.Args...), " "))
	for i, field := range fields {
		value, found := strings.CutPrefix(field, "-run=")
		if !found && field == "-run" && i+1 < len(fields) {
			value, found = fields[i+1], true
		}
		if found && strings.Trim(value, `'"`) == "^$" {
			return true
		}
	}
	return false
}
