package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunNoArgsPrintsUsageAndCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(nil, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := stdout.String()
	if !strings.Contains(output, "Usage:") {
		t.Fatalf("expected usage output to contain %q, got %q", "Usage:", output)
	}
	if strings.Contains(output, "Planned commands:") {
		t.Fatalf("expected usage output not to contain %q, got %q", "Planned commands:", output)
	}
	if !strings.Contains(output, "repo init") {
		t.Fatalf("expected usage output to mention repo init, got %q", output)
	}
	if !strings.Contains(output, "review approve") {
		t.Fatalf("expected usage output to mention review approve, got %q", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

// TestUsageContent asserts the usage block is honest and useful: a Usage:
// heading (not the misleading "Planned commands:"), every command listed with
// its summary, and the key global flags documented.
func TestUsageContent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Run([]string{"--help"}, &stdout, &stderr)
	out := stdout.String()

	if !strings.Contains(out, "Usage:") {
		t.Fatalf("expected %q heading, got %q", "Usage:", out)
	}
	if strings.Contains(out, "Planned commands:") {
		t.Fatalf("usage must not contain the misleading %q header", "Planned commands:")
	}
	for _, c := range commandUsages {
		if !strings.Contains(out, c.Syntax) {
			t.Fatalf("expected usage to list command %q", c.Syntax)
		}
		if !strings.Contains(out, c.Summary) {
			t.Fatalf("expected usage to describe %q with %q", c.Syntax, c.Summary)
		}
	}
	for _, flag := range []string{"--json", "--all", "--phase"} {
		if !strings.Contains(out, flag) {
			t.Fatalf("expected usage to document flag %q", flag)
		}
	}
}

func TestRunHelpFlag(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		var stdout, stderr bytes.Buffer
		exitCode := Run([]string{arg}, &stdout, &stderr)

		if exitCode != 0 {
			t.Fatalf("%s: expected exit code 0, got %d", arg, exitCode)
		}
		if !strings.Contains(stdout.String(), "Usage:") {
			t.Fatalf("%s: expected usage on stdout, got %q", arg, stdout.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("%s: expected empty stderr, got %q", arg, stderr.String())
		}
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"bogus"}, &stdout, &stderr)

	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code for unknown command")
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("expected error on stderr, got %q", stderr.String())
	}
}
