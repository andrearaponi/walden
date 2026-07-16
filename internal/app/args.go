package app

import (
	"fmt"
	"io"
	"strings"
)

// flagSpec declares one flag a command understands, for parsing and help.
type flagSpec struct {
	Name        string
	Description string
	// Placeholder names the value a value-flag consumes, e.g. "<phase>".
	Placeholder string
}

// commandSpec is the single source of truth for one command: its dispatch
// path, invocation syntax, summary, and known flags. Group commands carry
// subcommands instead of flags. The parser and both help renderers (global
// usage and per-command help) all read from this declaration, so parsing
// behavior and documentation cannot drift apart.
type commandSpec struct {
	Path        string
	Syntax      string
	Summary     string
	BoolFlags   []flagSpec
	ValueFlags  []flagSpec
	Subcommands []commandSpec
}

// leaves returns the runnable commands under this spec: the spec itself for
// leaf commands, or every nested leaf for group commands, in declaration order.
func (s commandSpec) leaves() []commandSpec {
	if len(s.Subcommands) == 0 {
		return []commandSpec{s}
	}
	var out []commandSpec
	for _, sub := range s.Subcommands {
		out = append(out, sub.leaves()...)
	}
	return out
}

// commandArgs is the parsed view of a command invocation. Flag reads go
// through the typed accessors, so a flag the spec never declared cannot be
// read by the command body.
type commandArgs struct {
	Positionals []string
	bools       map[string]bool
	values      map[string]string
}

// Bool reports whether a declared bool flag was passed.
func (a commandArgs) Bool(name string) bool { return a.bools[name] }

// Value returns the argument a declared value flag consumed, or "".
func (a commandArgs) Value(name string) string { return a.values[name] }

// parseOutcome is the triage verdict for one invocation. Exactly one of the
// three states holds: help requested, unusable arguments (Err), or parsed.
type parseOutcome struct {
	// Help is true when --help or -h appears anywhere in the arguments. It
	// is resolved before positional validation, so a help request never
	// turns into a "feature not found" error.
	Help bool
	// Err reports an unknown flag-shaped argument or a value flag missing
	// its value. The command must emit it and exit non-zero without acting.
	Err error
	// JSONMode is true when --json appears anywhere, so even flag errors
	// honor the JSON envelope contract.
	JSONMode bool
}

// parseCommandArgs triages raw arguments against a command's declaration.
// Flags and positionals may interleave in any order.
func parseCommandArgs(spec commandSpec, args []string) (commandArgs, parseOutcome) {
	outcome := parseOutcome{}
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			outcome.Help = true
		}
		if arg == "--json" {
			outcome.JSONMode = true
		}
	}
	if outcome.Help {
		return commandArgs{}, outcome
	}

	boolNames := make(map[string]bool, len(spec.BoolFlags))
	for _, f := range spec.BoolFlags {
		boolNames[f.Name] = true
	}
	valueNames := make(map[string]bool, len(spec.ValueFlags))
	for _, f := range spec.ValueFlags {
		valueNames[f.Name] = true
	}

	parsed := commandArgs{
		Positionals: []string{},
		bools:       map[string]bool{},
		values:      map[string]string{},
	}

	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case boolNames[arg]:
			parsed.bools[arg] = true
		case valueNames[arg]:
			if index+1 >= len(args) {
				outcome.Err = fmt.Errorf("%s requires a value: run 'walden %s --help'", arg, spec.Path)
				return commandArgs{}, outcome
			}
			parsed.values[arg] = args[index+1]
			index++
		case strings.HasPrefix(arg, "-"):
			outcome.Err = fmt.Errorf("unknown flag %s for %q: run 'walden %s --help'", arg, spec.Path, spec.Path)
			return commandArgs{}, outcome
		default:
			parsed.Positionals = append(parsed.Positionals, arg)
		}
	}

	return parsed, outcome
}

// printCommandHelp renders one command's usage: syntax, summary, and flags
// for leaf commands; the subcommand list for group commands.
func printCommandHelp(w io.Writer, spec commandSpec) {
	_, _ = fmt.Fprintf(w, "Usage:\n  %s %s\n      %s\n", binaryName, spec.Syntax, spec.Summary)

	if len(spec.Subcommands) > 0 {
		_, _ = fmt.Fprintln(w, "\nSubcommands:")
		for _, sub := range spec.Subcommands {
			_, _ = fmt.Fprintf(w, "  %s\n      %s\n", sub.Syntax, sub.Summary)
		}
		_, _ = fmt.Fprintf(w, "\nRun '%s <subcommand> --help' for details on a subcommand.\n", binaryName)
		return
	}

	if len(spec.BoolFlags) > 0 || len(spec.ValueFlags) > 0 {
		_, _ = fmt.Fprintln(w, "\nFlags:")
		for _, f := range spec.BoolFlags {
			_, _ = fmt.Fprintf(w, "  %-18s %s\n", f.Name, f.Description)
		}
		for _, f := range spec.ValueFlags {
			label := f.Name
			if f.Placeholder != "" {
				label += " " + f.Placeholder
			}
			_, _ = fmt.Fprintf(w, "  %-18s %s\n", label, f.Description)
		}
	}
}

var (
	jsonFlag    = flagSpec{Name: "--json", Description: "Emit machine-readable JSON instead of text"}
	projectFlag = flagSpec{Name: "--project", Description: "Install into the repository instead of the user home"}
)

// commandRegistry declares every walden command. Dispatch paths, help output,
// and flag parsing all derive from this table.
var commandRegistry = []commandSpec{
	{
		Path:      "version",
		Syntax:    "version [--json]",
		Summary:   "Print the CLI and schema version",
		BoolFlags: []flagSpec{jsonFlag},
	},
	{
		Path:    "update",
		Syntax:  "update [--check] [--version <tag>] [--json]",
		Summary: "Update the walden binary from GitHub releases and re-sync installed skills",
		BoolFlags: []flagSpec{
			{Name: "--check", Description: "Report the available update without applying it"},
			jsonFlag,
		},
		ValueFlags: []flagSpec{
			{Name: "--version", Description: "Pin the update to a release tag", Placeholder: "<tag>"},
		},
	},
	{
		Path:    "repo",
		Syntax:  "repo <subcommand>",
		Summary: "Repository-level operations",
		Subcommands: []commandSpec{
			{
				Path:      "repo init",
				Syntax:    "repo init [--json]",
				Summary:   "Initialize Walden in the current repository",
				BoolFlags: []flagSpec{jsonFlag},
			},
		},
	},
	{
		Path:    "feature",
		Syntax:  "feature <subcommand>",
		Summary: "Feature spec operations",
		Subcommands: []commandSpec{
			{
				Path:      "feature init",
				Syntax:    "feature init <name> [--json]",
				Summary:   "Scaffold a new feature spec",
				BoolFlags: []flagSpec{jsonFlag},
			},
		},
	},
	{
		Path:      "status",
		Syntax:    "status <feature> [--json]",
		Summary:   "Show a feature's phase, blockers, and next action",
		BoolFlags: []flagSpec{jsonFlag},
	},
	{
		Path:      "reconcile",
		Syntax:    "reconcile <feature> [--json]",
		Summary:   "Re-sync the approval chain after upstream edits",
		BoolFlags: []flagSpec{jsonFlag},
	},
	{
		Path:    "lesson",
		Syntax:  "lesson <subcommand>",
		Summary: "Lessons-learned operations",
		Subcommands: []commandSpec{
			{
				Path:      "lesson log",
				Syntax:    "lesson log --feature <name> --phase <phase> --trigger <text> --lesson <text> --guardrail <text> [--json]",
				Summary:   "Append a structured lesson to .walden/lessons.md",
				BoolFlags: []flagSpec{jsonFlag},
				ValueFlags: []flagSpec{
					{Name: "--feature", Description: "Feature the lesson belongs to", Placeholder: "<name>"},
					{Name: "--phase", Description: "Workflow phase that produced the lesson", Placeholder: "<phase>"},
					{Name: "--trigger", Description: "Event that triggered the lesson", Placeholder: "<text>"},
					{Name: "--lesson", Description: "Mistake pattern to remember", Placeholder: "<text>"},
					{Name: "--guardrail", Description: "Rule that would have prevented it", Placeholder: "<text>"},
				},
			},
		},
	},
	{
		Path:    "task",
		Syntax:  "task <subcommand>",
		Summary: "Task execution operations",
		Subcommands: []commandSpec{
			{
				Path:      "task status",
				Syntax:    "task status <feature> [--json]",
				Summary:   "Show execution readiness and the next runnable task",
				BoolFlags: []flagSpec{jsonFlag},
			},
			{
				Path:      "task start",
				Syntax:    "task start <feature> [task-id] [--json]",
				Summary:   "Resolve normalized execution context for a task",
				BoolFlags: []flagSpec{jsonFlag},
			},
			{
				Path:      "task complete",
				Syntax:    "task complete <feature> <task-id> [--json]",
				Summary:   "Run a task's proofs and mark it complete",
				BoolFlags: []flagSpec{jsonFlag},
			},
			{
				Path:      "task complete-all",
				Syntax:    "task complete-all <feature> [--json]",
				Summary:   "Complete all runnable tasks in order",
				BoolFlags: []flagSpec{jsonFlag},
			},
		},
	},
	{
		Path:    "verify",
		Syntax:  "verify <feature> [--all] [--check] [--json]",
		Summary: "Re-prove completed tasks against the current code and refresh evidence",
		BoolFlags: []flagSpec{
			{Name: "--all", Description: "Re-prove every completed task, not just stale ones"},
			{Name: "--check", Description: "Report without persisting evidence"},
			jsonFlag,
		},
	},
	{
		Path:    "adopt",
		Syntax:  "adopt [<feature>] [--apply] [--json]",
		Summary: "Plan or apply brownfield adoption: seal recorded approvals, re-prove unrecorded work",
		BoolFlags: []flagSpec{
			{Name: "--apply", Description: "Execute the plan: seal absent fingerprints and re-prove evidence"},
			jsonFlag,
		},
	},
	{
		Path:    "evidence",
		Syntax:  "evidence <subcommand>",
		Summary: "Execution evidence operations",
		Subcommands: []commandSpec{
			{
				Path:      "evidence status",
				Syntax:    "evidence status <feature> [--json]",
				Summary:   "Report each completed task's derived execution-evidence state",
				BoolFlags: []flagSpec{jsonFlag},
			},
		},
	},
	{
		Path:    "release",
		Syntax:  "release <subcommand>",
		Summary: "Release certification operations",
		Subcommands: []commandSpec{
			{
				Path:    "release check",
				Syntax:  "release check [<feature>] [--strict] [--json]",
				Summary: "Certify the repository (or one feature) as releasable: chain, validation, decisions, evidence, worktree",
				BoolFlags: []flagSpec{
					{Name: "--strict", Description: "Also require every planned task to be executed"},
					jsonFlag,
				},
			},
		},
	},
	{
		Path:    "validate",
		Syntax:  "validate [<feature>] [--all] [--json]",
		Summary: "Check EARS, traceability, and freshness (all features when no name is given)",
		BoolFlags: []flagSpec{
			{Name: "--all", Description: "Validate the full spec, not just the current phase"},
			jsonFlag,
		},
	},
	{
		Path:    "review",
		Syntax:  "review <subcommand>",
		Summary: "Review gate operations",
		Subcommands: []commandSpec{
			{
				Path:      "review open",
				Syntax:    "review open <feature> --phase <phase> [--json]",
				Summary:   "Open the review gate (move to in-review)",
				BoolFlags: []flagSpec{jsonFlag},
				ValueFlags: []flagSpec{
					{Name: "--phase", Description: "Target requirements|design|tasks", Placeholder: "<phase>"},
				},
			},
			{
				Path:      "review approve",
				Syntax:    "review approve <feature> --phase <phase> [--json]",
				Summary:   "Approve the review gate (move to approved)",
				BoolFlags: []flagSpec{jsonFlag},
				ValueFlags: []flagSpec{
					{Name: "--phase", Description: "Target requirements|design|tasks", Placeholder: "<phase>"},
				},
			},
		},
	},
	{
		Path:    "skill",
		Syntax:  "skill <subcommand>",
		Summary: "AI skill distribution operations",
		Subcommands: []commandSpec{
			{
				Path:    "skill install",
				Syntax:  "skill install <agent>|--all [--project] [--json]",
				Summary: "Install the embedded AI skill (claude|codex|copilot|opencode)",
				BoolFlags: []flagSpec{
					{Name: "--all", Description: "Target every supported agent"},
					projectFlag,
					jsonFlag,
				},
			},
			{
				Path:    "skill uninstall",
				Syntax:  "skill uninstall <agent>|--all [--project] [--json]",
				Summary: "Remove an installed AI skill",
				BoolFlags: []flagSpec{
					{Name: "--all", Description: "Target every supported agent"},
					projectFlag,
					jsonFlag,
				},
			},
			{
				Path:      "skill status",
				Syntax:    "skill status [--json]",
				Summary:   "Report installed skills and drift against the embedded copy",
				BoolFlags: []flagSpec{jsonFlag},
			},
			{
				Path:      "skill show",
				Syntax:    "skill show [--json]",
				Summary:   "Print the embedded SKILL.md",
				BoolFlags: []flagSpec{jsonFlag},
			},
		},
	},
}

// commandSpecByPath resolves a registry entry by its dispatch path.
func commandSpecByPath(path string) commandSpec {
	for _, spec := range commandRegistry {
		if spec.Path == path {
			return spec
		}
		for _, sub := range spec.Subcommands {
			if sub.Path == path {
				return sub
			}
		}
	}
	return commandSpec{Path: path, Syntax: path, Summary: ""}
}

// buildCommandUsages derives the flat global usage table from the registry,
// preserving declaration order.
func buildCommandUsages() []commandUsage {
	var usages []commandUsage
	for _, spec := range commandRegistry {
		for _, leaf := range spec.leaves() {
			usages = append(usages, commandUsage{Syntax: leaf.Syntax, Summary: leaf.Summary})
		}
	}
	return usages
}

// triageArgs runs the shared parse for one leaf command and resolves the two
// short-circuit outcomes uniformly: help goes to stdout with exit 0, an
// argument error goes through emitResult (JSON envelope when --json was
// passed) with exit 1. When handled is true the caller must return code.
func triageArgs(commandPath, emitName string, args []string, stdout, stderr io.Writer) (commandArgs, bool, int) {
	spec := commandSpecByPath(commandPath)
	parsed, outcome := parseCommandArgs(spec, args)
	if outcome.Help {
		printCommandHelp(stdout, spec)
		return commandArgs{}, true, 0
	}
	if outcome.Err != nil {
		return commandArgs{}, true, emitResult(emitName, errorResult(outcome.Err), outcome.JSONMode, stdout, stderr)
	}
	return parsed, false, 0
}

// groupHelp answers --help/-h at a group dispatcher when it is the first
// argument (e.g. `walden task --help`). Later positions belong to the
// subcommand, which renders its own leaf help.
func groupHelp(commandPath string, args []string, stdout io.Writer) bool {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		printCommandHelp(stdout, commandSpecByPath(commandPath))
		return true
	}
	return false
}
