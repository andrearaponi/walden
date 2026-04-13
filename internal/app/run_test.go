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
	if !strings.Contains(output, "walden") {
		t.Fatalf("expected usage output to mention walden, got %q", output)
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
