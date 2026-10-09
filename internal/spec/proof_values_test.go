package spec

import (
	"strings"
	"testing"
)

// malformedValuePlan holds one proof line on line 9, after a valid first step
// and before a second one.
const malformedValuePlan = `# Implementation Plan

- [ ] 1. Container
  - [ ] 1.1 Child
    - Requirements: ` + "`R1.AC1`" + `
    - Design: Parser
    - Verification:
      - command: ["go", "test", "./..."]
%s
      - command: ["go", "vet", "./..."]
`

func TestParseTaskTreeRejectsMalformedProofValues(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{
			name: "covers in backticks",
			line: "        covers: `R1.AC1`",
			want: `line 9: invalid covers for task "1.1": expected a JSON array of criterion IDs such as ["R1.AC1"], or [] for none; got "` + "`R1.AC1`" + `"`,
		},
		{
			name: "covers null",
			line: "        covers: null",
			want: `line 9: invalid covers for task "1.1": expected a JSON array of criterion IDs such as ["R1.AC1"], or [] for none; got "null"`,
		},
		{
			name: "expect_exit word",
			line: "        expect_exit: zero",
			want: `line 9: invalid expect_exit for task "1.1": expected a non-negative integer exit code; got "zero"`,
		},
		{
			name: "expect_exit negative",
			line: "        expect_exit: -1",
			want: `line 9: invalid expect_exit for task "1.1": expected a non-negative integer exit code; got "-1"`,
		},
		{
			name: "timeout empty",
			line: "        timeout:",
			want: `line 9: invalid timeout value for task "1.1": time: invalid duration ""; expected a positive Go duration such as 90s or 30m`,
		},
		{
			name: "expect_output empty",
			line: "        expect_output:",
			want: `line 9: invalid expect_output for task "1.1": expected non-empty text, optionally in double quotes; got ""`,
		},
		{
			name: "command empty array",
			line: "      - command: []",
			want: `line 9: invalid argv verification step for task "1.1": expected a non-empty JSON array of non-empty strings such as ["go", "test", "./..."]; got "[]"`,
		},
		{
			name: "command not JSON",
			line: "      - command: npm test",
			want: `line 9: invalid argv verification step for task "1.1": expected a non-empty JSON array of non-empty strings such as ["go", "test", "./..."]; got "npm test"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(malformedValuePlan, "%s", tc.line, 1)
			_, err := ParseTaskTree(Document{Exists: true, Path: "tasks.md", Body: body})
			if err == nil {
				t.Fatalf("the plan parsed, so %q was dropped instead of rejected", strings.TrimSpace(tc.line))
			}
			if err.Error() != tc.want {
				t.Fatalf("error does not name the line, the task and the form:\n got: %v\nwant: %s", err, tc.want)
			}
		})
	}
}

func TestParseTaskTreeReadsEmptyCovers(t *testing.T) {
	task := parsePlanTask(t, emptyCoversPlan, "1")
	if len(task.Proof.Steps) != 2 {
		t.Fatalf("read %d steps; the plan declares 2", len(task.Proof.Steps))
	}
	if task.Proof.Steps[0].Covers != nil {
		t.Fatalf("covers: [] read as %q, want no coverage", task.Proof.Steps[0].Covers)
	}
	if got := strings.Join(task.Proof.Steps[1].Argv, " "); got != "go vet ./internal/spec" {
		t.Fatalf("second step is %q", got)
	}
	if TaskDefinitionFingerprint(task) == v0130EmptyCoversFingerprint {
		t.Fatal("the definition fingerprint equals the v0.13.0 reading, so evidence minted from it would stay verified")
	}
}

func TestEmptyCoversFingerprintMatchesAbsentCovers(t *testing.T) {
	const plan = `# Implementation Plan

- [ ] 1. One step
  - Requirements: ` + "`R1.AC1`" + `
  - Design: Parser
  - Verification:
    - command: ["go", "test", "./internal/spec"]
%s      timeout: 2m
`
	withEmpty := parsePlanTask(t, strings.Replace(plan, "%s", "      covers: []\n", 1), "1")
	without := parsePlanTask(t, strings.Replace(plan, "%s", "", 1), "1")

	if withEmpty.Proof.Steps[0].Timeout == nil {
		t.Fatal("the timeout after covers: [] was dropped")
	}
	if TaskDefinitionFingerprint(withEmpty) != TaskDefinitionFingerprint(without) {
		t.Fatal("covers: [] and an absent covers give different definition fingerprints")
	}
}
