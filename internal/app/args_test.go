package app

import (
	"bytes"
	"strings"
	"testing"
)

func testSpec() commandSpec {
	return commandSpec{
		Path:    "demo run",
		Syntax:  "demo run <feature> [--fast] [--json]",
		Summary: "Run the demo command",
		BoolFlags: []flagSpec{
			{Name: "--fast", Description: "Skip slow steps"},
			{Name: "--json", Description: "Emit machine-readable JSON instead of text"},
		},
		ValueFlags: []flagSpec{
			{Name: "--phase", Description: "Target phase", Placeholder: "<phase>"},
		},
	}
}

func TestParseCommandArgs(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantHelp    bool
		wantErr     string
		wantJSON    bool
		positionals []string
		bools       map[string]bool
		values      map[string]string
	}{
		{
			name:     "long help flag alone",
			args:     []string{"--help"},
			wantHelp: true,
		},
		{
			name:     "short help flag alone",
			args:     []string{"-h"},
			wantHelp: true,
		},
		{
			name:     "help wins over positionals and other flags",
			args:     []string{"my-feature", "--json", "--help"},
			wantHelp: true,
		},
		{
			name:     "help wins even next to an unknown flag",
			args:     []string{"--bogus", "--help"},
			wantHelp: true,
		},
		{
			name:        "plain positional with bool flag interleaved after",
			args:        []string{"my-feature", "--json"},
			positionals: []string{"my-feature"},
			bools:       map[string]bool{"--json": true},
		},
		{
			name:        "bool flag before positional",
			args:        []string{"--json", "my-feature"},
			positionals: []string{"my-feature"},
			bools:       map[string]bool{"--json": true},
		},
		{
			name:        "value flag consumes its argument",
			args:        []string{"my-feature", "--phase", "design"},
			positionals: []string{"my-feature"},
			values:      map[string]string{"--phase": "design"},
		},
		{
			name:    "value flag missing its argument",
			args:    []string{"my-feature", "--phase"},
			wantErr: "--phase",
		},
		{
			name:    "unknown long flag rejected and named",
			args:    []string{"--jsn", "my-feature"},
			wantErr: "--jsn",
		},
		{
			name:    "unknown short flag rejected and named",
			args:    []string{"-x"},
			wantErr: "-x",
		},
		{
			name:     "unknown flag with json flag keeps json mode",
			args:     []string{"--bogus", "--json"},
			wantErr:  "--bogus",
			wantJSON: true,
		},
		{
			name:    "unknown flag error points to command help",
			args:    []string{"--bogus"},
			wantErr: "walden demo run --help",
		},
		{
			name:        "empty args parse to empty result",
			args:        []string{},
			positionals: []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, outcome := parseCommandArgs(testSpec(), tc.args)

			if tc.wantHelp {
				if !outcome.Help {
					t.Fatalf("expected help outcome for args %v", tc.args)
				}
				return
			}
			if outcome.Help {
				t.Fatalf("unexpected help outcome for args %v", tc.args)
			}

			if tc.wantErr != "" {
				if outcome.Err == nil {
					t.Fatalf("expected error containing %q, got none", tc.wantErr)
				}
				if !strings.Contains(outcome.Err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", outcome.Err.Error(), tc.wantErr)
				}
				if outcome.JSONMode != tc.wantJSON {
					t.Fatalf("JSONMode = %v, want %v", outcome.JSONMode, tc.wantJSON)
				}
				return
			}
			if outcome.Err != nil {
				t.Fatalf("unexpected error: %v", outcome.Err)
			}

			if len(parsed.Positionals) != len(tc.positionals) {
				t.Fatalf("positionals = %v, want %v", parsed.Positionals, tc.positionals)
			}
			for i, want := range tc.positionals {
				if parsed.Positionals[i] != want {
					t.Fatalf("positionals = %v, want %v", parsed.Positionals, tc.positionals)
				}
			}
			for name, want := range tc.bools {
				if parsed.Bool(name) != want {
					t.Fatalf("Bool(%q) = %v, want %v", name, parsed.Bool(name), want)
				}
			}
			for name, want := range tc.values {
				if parsed.Value(name) != want {
					t.Fatalf("Value(%q) = %q, want %q", name, parsed.Value(name), want)
				}
			}
		})
	}
}

// TestParseCommandArgsUndeclaredAccess asserts the typed accessors cannot read
// flags the spec never declared: an undeclared read reports zero values.
func TestParseCommandArgsUndeclaredAccess(t *testing.T) {
	parsed, outcome := parseCommandArgs(testSpec(), []string{"my-feature", "--fast"})
	if outcome.Err != nil || outcome.Help {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
	if parsed.Bool("--undeclared") {
		t.Fatal("undeclared bool flag must read false")
	}
	if parsed.Value("--undeclared") != "" {
		t.Fatal("undeclared value flag must read empty")
	}
}

func TestPrintCommandHelp(t *testing.T) {
	var buf bytes.Buffer
	printCommandHelp(&buf, testSpec())
	out := buf.String()

	for _, want := range []string{
		"Usage:",
		"walden demo run <feature> [--fast] [--json]",
		"Run the demo command",
		"--fast",
		"Skip slow steps",
		"--phase",
		"Target phase",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("help output missing %q:\n%s", want, out)
		}
	}
}

// TestPrintCommandHelpGroup asserts a group spec lists its subcommands so a
// user can discover the next token.
func TestPrintCommandHelpGroup(t *testing.T) {
	group := commandSpec{
		Path:    "demo",
		Syntax:  "demo <subcommand>",
		Summary: "Demo command group",
		Subcommands: []commandSpec{
			{Path: "demo run", Syntax: "demo run <feature>", Summary: "Run the demo"},
			{Path: "demo stop", Syntax: "demo stop <feature>", Summary: "Stop the demo"},
		},
	}

	var buf bytes.Buffer
	printCommandHelp(&buf, group)
	out := buf.String()

	for _, want := range []string{"demo run", "Run the demo", "demo stop", "Stop the demo"} {
		if !strings.Contains(out, want) {
			t.Fatalf("group help missing %q:\n%s", want, out)
		}
	}
}

// TestUsageRebuiltFromRegistry asserts the global usage table is derived from
// the command registry, so global and per-command help share one source.
func TestUsageRebuiltFromRegistry(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf)
	out := buf.String()

	for _, spec := range commandRegistry {
		leaves := spec.leaves()
		for _, leaf := range leaves {
			if !strings.Contains(out, leaf.Syntax) {
				t.Fatalf("global usage missing syntax %q", leaf.Syntax)
			}
		}
	}
}
