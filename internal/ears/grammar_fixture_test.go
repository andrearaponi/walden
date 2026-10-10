package ears

import (
	"bufio"
	"os"
	"slices"
	"strings"
	"testing"
)

const invalidVerdict = "invalid"

// grammarRow is one criterion of testdata/grammar.txt with its v0.13.1
// verdict, its expected verdict and its expected warning kinds.
type grammarRow struct {
	line     int
	legacy   string
	expected string
	warnings []string
	text     string
}

func loadGrammarFixture(t *testing.T) []grammarRow {
	t.Helper()
	file, err := os.Open("testdata/grammar.txt")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()

	var rows []grammarRow
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.SplitN(text, " | ", 4)
		if len(fields) != 4 {
			t.Fatalf("testdata/grammar.txt:%d: want 4 fields, got %d", line, len(fields))
		}
		row := grammarRow{line: line, legacy: fields[0], expected: fields[1], text: fields[3]}
		if fields[2] != "-" {
			row.warnings = strings.Split(fields[2], ",")
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("fixture holds no criteria")
	}
	return rows
}

// verdictOf names a parse result the way the fixture does.
func verdictOf(result ParsedCriterion) string {
	if !result.Valid {
		return invalidVerdict
	}
	return result.Form
}

// TestGrammarFixtureKeepsValidCriteriaValid pins NFR1: every criterion that
// v0.13.1 validates stays valid. It holds before and after the clause scan.
func TestGrammarFixtureKeepsValidCriteriaValid(t *testing.T) {
	checked := 0
	for _, row := range loadGrammarFixture(t) {
		if row.legacy == invalidVerdict {
			continue
		}
		checked++
		if got := ParseCriterion("R1.AC1", row.text); !got.Valid {
			t.Errorf("testdata/grammar.txt:%d: valid in v0.13.1 as %s, now invalid: %v\n  %s", row.line, row.legacy, got.Errors, row.text)
		}
	}
	if checked == 0 {
		t.Fatal("fixture holds no criterion valid in v0.13.1")
	}
}

// TestGrammarFixtureVerdicts asserts each criterion's expected verdict.
func TestGrammarFixtureVerdicts(t *testing.T) {
	for _, row := range loadGrammarFixture(t) {
		if got := verdictOf(ParseCriterion("R1.AC1", row.text)); got != row.expected {
			t.Errorf("testdata/grammar.txt:%d: want %s, got %s\n  %s", row.line, row.expected, got, row.text)
		}
	}
}

// warningKind names a warning the way the fixture does.
func warningKind(warning string) string {
	switch {
	case strings.Contains(warning, "appears after SHALL"):
		return "after-shall"
	case strings.Contains(warning, "out of EARS order"):
		return "order"
	case strings.Contains(warning, "is a pronoun"):
		return "subject"
	default:
		return "unknown: " + warning
	}
}

// TestGrammarFixtureWarnings asserts each criterion's exact set of warnings.
func TestGrammarFixtureWarnings(t *testing.T) {
	for _, row := range loadGrammarFixture(t) {
		var kinds []string
		for _, warning := range ParseCriterion("R1.AC1", row.text).Warnings {
			kinds = append(kinds, warningKind(warning))
		}
		slices.Sort(kinds)
		want := slices.Clone(row.warnings)
		slices.Sort(want)
		if !slices.Equal(kinds, want) {
			t.Errorf("testdata/grammar.txt:%d: want warnings %v, got %v\n  %s", row.line, want, kinds, row.text)
		}
	}
}
