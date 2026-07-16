package spec

import (
	"strings"
	"testing"
)

func timeoutFixture(attrLines string) Document {
	return Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Harden proof execution
  - [ ] 1.1 Bound the proof step
    - Requirements: ` + "`R1`" + `
    - Design: Proof executor
    - Verification:
      - command: ["go", "test", "./internal/spec"]
` + attrLines,
	}
}

func TestParseTimeoutAttribute(t *testing.T) {
	cases := []struct {
		name        string
		attrLines   string
		wantTimeout string
		wantErr     string
	}{
		{
			name:        "valid seconds",
			attrLines:   "        timeout: 30s\n",
			wantTimeout: "30s",
		},
		{
			name:        "valid composite duration",
			attrLines:   "        timeout: 1h30m\n",
			wantTimeout: "1h30m",
		},
		{
			name:        "quoted value is unquoted",
			attrLines:   "        timeout: \"45s\"\n",
			wantTimeout: "45s",
		},
		{
			name:      "invalid duration string",
			attrLines: "        timeout: banana\n",
			wantErr:   `invalid timeout value for task "1.1"`,
		},
		{
			name:      "zero duration",
			attrLines: "        timeout: 0s\n",
			wantErr:   "must be positive",
		},
		{
			name:      "negative duration",
			attrLines: "        timeout: -5m\n",
			wantErr:   "must be positive",
		},
		{
			name:      "wrong indentation",
			attrLines: "      timeout: 30s\n",
			wantErr:   "invalid proof attribute indentation",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tree, err := ParseTaskTree(timeoutFixture(testCase.attrLines))
			if testCase.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", testCase.wantErr)
				}
				if !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("expected error containing %q, got %v", testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected parse to succeed, got %v", err)
			}
			task, ok := tree.FindTask("1.1")
			if !ok {
				t.Fatal("expected task 1.1 to exist")
			}
			if len(task.Proof.Steps) != 1 {
				t.Fatalf("expected 1 proof step, got %d", len(task.Proof.Steps))
			}
			timeout := task.Proof.Steps[0].Timeout
			if timeout == nil {
				t.Fatal("expected timeout to be set")
			}
			if *timeout != testCase.wantTimeout {
				t.Fatalf("expected timeout %q, got %q", testCase.wantTimeout, *timeout)
			}
		})
	}
}

func TestParseTimeoutAttributeBeforeAnyStep(t *testing.T) {
	document := Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Harden proof execution
  - [ ] 1.1 Bound the proof step
    - Requirements: ` + "`R1`" + `
    - Design: Proof executor
    - Verification:
        timeout: 30s
`,
	}
	_, err := ParseTaskTree(document)
	if err == nil || !strings.Contains(err.Error(), "timeout declared before any command step") {
		t.Fatalf("expected timeout-before-step error, got %v", err)
	}
}

func TestParseTimeoutAttributeAttachesToLastStep(t *testing.T) {
	document := Document{
		Path:   "tasks.md",
		Exists: true,
		Body: `# Implementation Plan

- [ ] 1. Harden proof execution
  - [ ] 1.1 Bound the proof step
    - Requirements: ` + "`R1`" + `
    - Design: Proof executor
    - Verification:
      - command: ["go", "build", "./..."]
      - command: ["go", "test", "./internal/spec"]
        timeout: 2m
`,
	}
	tree, err := ParseTaskTree(document)
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}
	task, ok := tree.FindTask("1.1")
	if !ok {
		t.Fatal("expected task 1.1 to exist")
	}
	if len(task.Proof.Steps) != 2 {
		t.Fatalf("expected 2 proof steps, got %d", len(task.Proof.Steps))
	}
	if task.Proof.Steps[0].Timeout != nil {
		t.Fatalf("expected first step without timeout, got %q", *task.Proof.Steps[0].Timeout)
	}
	second := task.Proof.Steps[1].Timeout
	if second == nil || *second != "2m" {
		t.Fatalf("expected second step timeout 2m, got %v", second)
	}
}

func TestDisplayTimeoutSuffix(t *testing.T) {
	declared := "2m"
	spec := VerificationSpec{Steps: []VerificationStep{
		{Argv: []string{"go", "build", "./..."}},
		{Argv: []string{"go", "test", "./internal/spec"}, Timeout: &declared},
	}}
	display := spec.Display()
	if want := `command ["go","build","./..."]; command ["go","test","./internal/spec"] timeout=2m`; display != want {
		t.Fatalf("unexpected display:\n got %q\nwant %q", display, want)
	}
}

func TestDisplayWithoutTimeoutIsByteIdentical(t *testing.T) {
	tree, err := ParseTaskTree(timeoutFixture(""))
	if err != nil {
		t.Fatalf("expected parse to succeed, got %v", err)
	}
	task, ok := tree.FindTask("1.1")
	if !ok {
		t.Fatal("expected task 1.1 to exist")
	}
	if want := `command ["go","test","./internal/spec"]`; task.Verification != want {
		t.Fatalf("undeclared timeout changed the rendering:\n got %q\nwant %q", task.Verification, want)
	}
}

func TestTaskFingerprintMovesWithTimeout(t *testing.T) {
	fingerprintFor := func(attrLines string) string {
		tree, err := ParseTaskTree(timeoutFixture(attrLines))
		if err != nil {
			t.Fatalf("expected parse to succeed, got %v", err)
		}
		task, ok := tree.FindTask("1.1")
		if !ok {
			t.Fatal("expected task 1.1 to exist")
		}
		return TaskDefinitionFingerprint(task)
	}

	none := fingerprintFor("")
	thirty := fingerprintFor("        timeout: 30s\n")
	fortyFive := fingerprintFor("        timeout: 45s\n")

	if none == thirty {
		t.Fatal("declaring a timeout must move the task fingerprint")
	}
	if thirty == fortyFive {
		t.Fatal("changing a declared timeout must move the task fingerprint")
	}
	if again := fingerprintFor(""); none != again {
		t.Fatal("fingerprint without timeout must stay deterministic")
	}
}
