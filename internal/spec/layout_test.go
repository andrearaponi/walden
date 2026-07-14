package spec

import (
	"strings"
	"testing"
)

// TestCheckTaskLayoutMatchesParserVerdicts feeds identical, completeness-legal
// fixtures to CheckTaskLayout and ParseTaskTree: the layout check must accept
// exactly what the executor accepts and reject offset defects with the same
// message the executor reports.
func TestCheckTaskLayoutMatchesParserVerdicts(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{
			name: "natural top-level leaf 2/4/6",
			ok:   true,
			body: `# Implementation Plan

- [ ] 1. Flat task
  - Requirements: ` + "`R1`" + `
  - Design: Parser
  - Verification:
    - command: ["go", "test", "./..."]
      expect_exit: 0
`,
		},
		{
			name: "legacy top-level leaf 4/6/8",
			ok:   true,
			body: `# Implementation Plan

- [ ] 1. Flat task
    - Requirements: ` + "`R1`" + `
    - Design: Parser
    - Verification:
      - command: ["go", "test", "./..."]
        expect_exit: 0
`,
		},
		{
			name: "classic child profile 4/6/8",
			ok:   true,
			body: `# Implementation Plan

- [ ] 1. Container
  - [ ] 1.1 Child
    - Requirements: ` + "`R1`" + `
    - Design: Parser
    - Verification:
      - command: ["go", "test", "./..."]
`,
		},
		{
			name: "top-level metadata at six spaces",
			ok:   false,
			body: `# Implementation Plan

- [ ] 1. Flat task
      - Requirements: ` + "`R1`" + `
      - Design: Parser
      - Verification: go test ./...
`,
		},
		{
			name: "child metadata at parent offset",
			ok:   false,
			body: `# Implementation Plan

- [ ] 1. Container
  - [ ] 1.1 Child
  - Requirements: ` + "`R1`" + `
  - Design: Parser
  - Verification: go test ./...
`,
		},
		{
			name: "proof step deeper than its block",
			ok:   false,
			body: `# Implementation Plan

- [ ] 1. Flat task
  - Requirements: ` + "`R1`" + `
  - Design: Parser
  - Verification:
      - command: ["go", "test", "./..."]
`,
		},
		{
			name: "proof attribute shallower than its step",
			ok:   false,
			body: `# Implementation Plan

- [ ] 1. Container
  - [ ] 1.1 Child
    - Requirements: ` + "`R1`" + `
    - Design: Parser
    - Verification:
      - command: ["go", "test", "./..."]
      expect_exit: 1
`,
		},
		{
			name: "column-zero metadata under a task",
			ok:   false,
			body: `# Implementation Plan

- [ ] 1. Flat task
- Requirements: ` + "`R1`" + `
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			layoutErr := CheckTaskLayout(tc.body)
			_, parseErr := ParseTaskTree(Document{Path: "tasks.md", Exists: true, Body: tc.body})

			if tc.ok {
				if layoutErr != nil {
					t.Fatalf("layout check rejected a parser-legal document: %v", layoutErr)
				}
				if parseErr != nil {
					t.Fatalf("fixture is not parser-legal: %v", parseErr)
				}
				return
			}

			if layoutErr == nil {
				t.Fatal("layout check accepted a parser-illegal document")
			}
			if parseErr == nil {
				t.Fatal("fixture unexpectedly parser-legal")
			}
			if layoutErr.Error() != parseErr.Error() {
				t.Fatalf("verdict messages diverge:\n layout: %v\n parser: %v", layoutErr, parseErr)
			}
			if !strings.Contains(layoutErr.Error(), "expected") {
				t.Fatalf("layout error does not name the expectation: %v", layoutErr)
			}
		})
	}
}
