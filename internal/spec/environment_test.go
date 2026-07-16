package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProbeDeclaration(t *testing.T, root, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".walden"), 0o755); err != nil {
		t.Fatalf("mkdir .walden: %v", err)
	}
	if err := os.WriteFile(EnvironmentProbesPath(root), []byte(content), 0o644); err != nil {
		t.Fatalf("write environment.md: %v", err)
	}
}

func TestLoadEnvironmentProbes(t *testing.T) {
	t.Run("absent file means no probes", func(t *testing.T) {
		probes, err := LoadEnvironmentProbes(t.TempDir())
		if err != nil || len(probes) != 0 {
			t.Fatalf("expected empty list without error, got %v / %v", probes, err)
		}
	})

	t.Run("valid declaration with documentation prose", func(t *testing.T) {
		root := t.TempDir()
		writeProbeDeclaration(t, root, `# Environment Probes

These commands describe the toolchain this project's proofs depend on.

- go: ["go", "version"]
- node: ["node", "--version"]
`)
		probes, err := LoadEnvironmentProbes(root)
		if err != nil {
			t.Fatalf("expected parse to succeed, got %v", err)
		}
		if len(probes) != 2 || probes[0].Name != "go" || probes[1].Name != "node" {
			t.Fatalf("unexpected probes: %+v", probes)
		}
		if strings.Join(probes[0].Argv, " ") != "go version" {
			t.Fatalf("unexpected argv: %v", probes[0].Argv)
		}
	})

	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{"malformed list item", "- go [\"go\", \"version\"]\n", "malformed probe line"},
		{"uppercase name", "- Go: [\"go\", \"version\"]\n", "malformed probe line"},
		{"reserved name platform", "- platform: [\"uname\", \"-m\"]\n", "reserved for the kernel profile"},
		{"reserved name walden", "- walden: [\"walden\", \"version\"]\n", "reserved for the kernel profile"},
		{"duplicate name", "- go: [\"go\", \"version\"]\n- go: [\"go\", \"env\"]\n", "duplicate probe name"},
		{"invalid argv payload", "- go: [go version]\n", "invalid argv"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			writeProbeDeclaration(t, root, testCase.content)
			_, err := LoadEnvironmentProbes(root)
			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("expected error containing %q, got %v", testCase.wantErr, err)
			}
			if err != nil && !strings.Contains(err.Error(), EnvironmentProbesPath(root)) {
				t.Fatalf("error does not name the file: %v", err)
			}
		})
	}

	t.Run("line numbers are reported", func(t *testing.T) {
		root := t.TempDir()
		writeProbeDeclaration(t, root, "# Probes\n\n- ok-probe: [\"true\"]\n- broken here\n")
		_, err := LoadEnvironmentProbes(root)
		if err == nil || !strings.Contains(err.Error(), ":4:") {
			t.Fatalf("expected the error to name line 4, got %v", err)
		}
	})
}
