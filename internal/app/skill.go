package app

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/skilldist"
	"github.com/andrearaponi/walden/skill"
)

func runSkill(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "show" {
		return runSkillShow(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "install" {
		return runSkillInstall(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "uninstall" {
		return runSkillUninstall(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "status" {
		return runSkillStatus(args[1:], stdout, stderr)
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: skill %s\n\n", strings.Join(args, " "))
	printUsage(stderr)
	return 1
}

func runSkillStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
		}
	}

	opts, err := skillOptions()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	statuses, warnings := skilldist.Status(opts)
	skills := make([]output.SkillStatus, 0, len(statuses))
	for _, status := range statuses {
		skills = append(skills, output.SkillStatus{
			Agent:     status.Agent,
			Scope:     string(status.Scope),
			Path:      status.Path,
			Installed: status.Installed,
			State:     status.State,
			Version:   status.Version,
		})
	}

	// Status is a report, not a gate: drift never changes the exit code.
	result := output.Result{
		Summary:  fmt.Sprintf("skill status against embedded version %s", Version),
		Skills:   skills,
		Warnings: warnings,
		ExitCode: 0,
	}

	if jsonMode {
		if err := output.PrintJSON(stdout, "skill-status", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}
	output.PrintText(stdout, result)
	return 0
}

// skillFlags holds the parsed flag set shared by install and uninstall.
type skillFlags struct {
	jsonMode bool
	project  bool
	all      bool
	agents   []string
}

func parseSkillFlags(args []string) skillFlags {
	flags := skillFlags{}
	for _, arg := range args {
		switch arg {
		case "--json":
			flags.jsonMode = true
		case "--project":
			flags.project = true
		case "--all":
			flags.all = true
		default:
			flags.agents = append(flags.agents, arg)
		}
	}
	return flags
}

// resolveSkillTarget validates the flag combination and returns the selected
// agent (zero Agent when --all). A non-nil error is a usage error.
func resolveSkillTarget(command string, flags skillFlags) (skilldist.Agent, error) {
	supported := strings.Join(skilldist.AgentNames(), ", ")

	if flags.all {
		if flags.project || len(flags.agents) > 0 {
			return skilldist.Agent{}, fmt.Errorf("skill %s --all cannot be combined with --project or an agent", command)
		}
		return skilldist.Agent{}, nil
	}
	if len(flags.agents) != 1 {
		return skilldist.Agent{}, fmt.Errorf("skill %s requires exactly one agent: %s (or --all)", command, supported)
	}

	agent, err := skilldist.Lookup(flags.agents[0])
	if err != nil {
		return skilldist.Agent{}, fmt.Errorf("skill %s: %v", command, err)
	}
	if flags.project {
		if _, ok := agent.ProjectTarget("."); !ok {
			return skilldist.Agent{}, fmt.Errorf("agent %s supports only user scope; --project is not available", agent.Name)
		}
	}
	return agent, nil
}

func skillOptions() (skilldist.Options, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return skilldist.Options{}, fmt.Errorf("resolve working directory: %w", err)
	}
	return skilldist.Options{
		Version: Version,
		WorkDir: workDir,
		Env:     skilldist.EnvFromOS(),
	}, nil
}

func runSkillInstall(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := parseSkillFlags(args)
	agent, err := resolveSkillTarget("install", flags)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	opts, err := skillOptions()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	var reports []skilldist.Report
	if flags.all {
		reports, err = skilldist.InstallAll(opts)
	} else {
		scope := skilldist.ScopeUser
		if flags.project {
			scope = skilldist.ScopeProject
		}
		var report skilldist.Report
		report, err = skilldist.Install(agent, scope, opts)
		reports = []skilldist.Report{report}
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	result := installResult(flags, reports)
	if flags.jsonMode {
		if err := output.PrintJSON(stdout, "skill-install", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}
	output.PrintText(stdout, result)
	return 0
}

func installResult(flags skillFlags, reports []skilldist.Report) output.Result {
	changed := make([]string, 0, len(reports))
	for _, report := range reports {
		changed = append(changed, report.Path)
	}

	result := output.Result{
		Summary:      fmt.Sprintf("skill installed for %s (user scope)", reports[0].Agent),
		ChangedFiles: changed,
		ExitCode:     0,
	}
	if flags.all {
		result.Summary = "skill installed for all supported agents (user scope)"
	}
	if flags.project {
		result.Summary = fmt.Sprintf("skill installed for %s (project scope)", reports[0].Agent)
		result.NextAction = fmt.Sprintf("Commit %s to share the skill with your team", reports[0].Path)
	}
	return result
}

func runSkillUninstall(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := parseSkillFlags(args)
	agent, err := resolveSkillTarget("uninstall", flags)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	opts, err := skillOptions()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	var reports []skilldist.Report
	if flags.all {
		reports, err = skilldist.UninstallAll(opts)
	} else {
		scope := skilldist.ScopeUser
		if flags.project {
			scope = skilldist.ScopeProject
		}
		var report skilldist.Report
		report, err = skilldist.Uninstall(agent, scope, opts)
		reports = []skilldist.Report{report}
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	result := uninstallResult(flags, reports)
	if flags.jsonMode {
		if err := output.PrintJSON(stdout, "skill-uninstall", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}
	output.PrintText(stdout, result)
	return 0
}

func uninstallResult(flags skillFlags, reports []skilldist.Report) output.Result {
	changed := []string{}
	removedAny := false
	for _, report := range reports {
		if !report.NotInstalled {
			changed = append(changed, report.Path)
			removedAny = true
		}
	}

	result := output.Result{ChangedFiles: changed, ExitCode: 0}
	scope := "user scope"
	if flags.project {
		scope = "project scope"
	}
	switch {
	case flags.all && removedAny:
		result.Summary = "skill removed for all installed agents (user scope)"
	case flags.all:
		result.Summary = "skill not installed for any agent (user scope)"
	case removedAny:
		result.Summary = fmt.Sprintf("skill removed for %s (%s)", reports[0].Agent, scope)
	default:
		result.Summary = fmt.Sprintf("skill not installed for %s (%s)", reports[0].Agent, scope)
	}
	return result
}

func runSkillShow(args []string, stdout io.Writer, stderr io.Writer) int {
	jsonMode := false
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
		}
	}

	if jsonMode {
		result := output.Result{
			Summary:  "embedded walden skill",
			Content:  string(skill.Content()),
			ExitCode: 0,
		}
		if err := output.PrintJSON(stdout, "skill-show", result); err != nil {
			_, _ = fmt.Fprintf(stderr, "render json output: %v\n", err)
			return 1
		}
		return 0
	}

	// Verbatim bytes with no framing, so `walden skill show > SKILL.md`
	// yields a clean skill file.
	if _, err := stdout.Write(skill.Content()); err != nil {
		_, _ = fmt.Fprintf(stderr, "write skill content: %v\n", err)
		return 1
	}
	return 0
}
