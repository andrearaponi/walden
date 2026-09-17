package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/andrearaponi/walden/internal/skilldist"
	"github.com/andrearaponi/walden/internal/spec"
)

type authoringInvoke func(args ...string) (string, int)

func authoringSourceRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source location unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func authoringTestEnv(t *testing.T) {
	t.Helper()
	// Keep the existing external Go build cache while isolating user settings.
	if os.Getenv("GOCACHE") == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("GOCACHE", filepath.Join(cache, "go-build"))
	}
	setSkillTestEnv(t)
	t.Setenv("PATH", filepath.Join(runtime.GOROOT(), "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOWORK", "off")
}

func authoringInProcess(args ...string) (string, int) {
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return stdout.String() + stderr.String(), code
}

func authoringMust(t *testing.T, invoke authoringInvoke, args ...string) string {
	t.Helper()
	out, code := invoke(args...)
	if code != 0 {
		t.Fatalf("walden %v exited %d: %s", args, code, out)
	}
	return out
}

func authoringWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Replace prose placeholders in the generated BODY, never workflow metadata,
// checkboxes, or JSON argv arrays. The test still consumes actual scaffolds.
func authoringFillScaffolds(t *testing.T, root string) {
	t.Helper()
	placeholder := regexp.MustCompile(`\[[A-Za-z][^\]\n]*\]`)
	for _, name := range []string{"requirements", "design", "tasks"} {
		path := filepath.Join(root, ".walden/specs/authoring-example", name+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		end := strings.Index(string(data), "\n---\n")
		if end < 0 {
			t.Fatal("missing generated frontmatter terminator")
		}
		body := strings.ReplaceAll(string(data[end+5:]), "[Relevant section]", "Architecture")
		body = placeholder.ReplaceAllString(body, "local fixture behavior")
		authoringWrite(t, path, string(data[:end+5])+body)
	}
	authoringWrite(t, filepath.Join(root, "go.mod"), "module example.test/authoring\n\ngo 1.25.0\n")
	authoringWrite(t, filepath.Join(root, "pkg/example/example_test.go"), "package example\nimport \"testing\"\nfunc TestExample(t *testing.T) { if 2 + 2 != 4 { t.Fatal(\"arithmetic\") } }\n")
}

func authoringApprove(t *testing.T, invoke authoringInvoke) {
	t.Helper()
	for _, phase := range []string{"requirements", "design", "tasks"} {
		authoringMust(t, invoke, "review", "open", "authoring-example", "--phase", phase, "--json")
		authoringMust(t, invoke, "review", "approve", "authoring-example", "--phase", phase, "--json")
	}
}

func TestAuthoringScaffoldLifecycle(t *testing.T) {
	authoringTestEnv(t)
	root := t.TempDir()
	t.Chdir(root)
	authoringMust(t, authoringInProcess, "repo", "init", "--json")
	authoringMust(t, authoringInProcess, "feature", "init", "authoring-example", "--json")
	authoringFillScaffolds(t, root)
	authoringApprove(t, authoringInProcess)
	out := authoringMust(t, authoringInProcess, "validate", "authoring-example", "--all", "--json")
	var validation struct {
		Result struct {
			Coverage struct {
				Proof struct {
					Complete bool `json:"complete"`
				} `json:"proof_reference_coverage"`
			} `json:"coverage"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &validation); err != nil {
		t.Fatal(err)
	}
	if !validation.Result.Coverage.Proof.Complete {
		t.Fatal("generated proof does not cover the fixture AC")
	}
	feature, err := spec.LoadFeature(root, "authoring-example")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(feature.Requirements.Body, "   - Acceptance check:") {
		t.Fatal("acceptance sketch did not survive the normal approval cycle")
	}
	authoringMust(t, authoringInProcess, "task", "complete", "authoring-example", "1.1", "--json")
}

func authoringReplaceBody(t *testing.T, path, body string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	end := strings.Index(string(data), "\n---\n")
	if end < 0 {
		t.Fatal("missing scaffold frontmatter")
	}
	authoringWrite(t, path, string(data[:end+5])+"\n"+body)
}

// Lift actual published command/attribute lines into a minimal task so the
// production proof parser interprets them. No parallel grammar for attributes.
func authoringExampleSteps(t *testing.T, source string) []spec.VerificationStep {
	t.Helper()
	lines := strings.Split(source, "\n")
	var steps []spec.VerificationStep
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- command:") {
			continue
		}
		var argv []string
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(trimmed, "- command:"))), &argv); err != nil {
			t.Fatal(err)
		}
		if len(argv) < 2 || argv[0] != "go" || argv[1] != "test" {
			continue
		}
		body := "- [ ] 1. Published proof\n  - Requirements: `R1.AC1`\n  - Design: Architecture\n  - Verification:\n    " + trimmed + "\n"
		for _, following := range lines[i+1:] {
			attr := strings.TrimSpace(following)
			if !strings.HasPrefix(attr, "expect_output:") && !strings.HasPrefix(attr, "expect_exit:") && !strings.HasPrefix(attr, "covers:") && !strings.HasPrefix(attr, "timeout:") {
				break
			}
			body += "      " + attr + "\n"
		}
		tree, err := spec.ParseTaskTree(spec.Document{Exists: true, Body: body})
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, tree.LeafTasks()[0].Proof.Steps[0])
	}
	return steps
}

func TestAuthoringProofExamples(t *testing.T) {
	root := authoringSourceRoot(t)
	for _, name := range []string{"skill/walden/SKILL.md", "templates/spec/tasks.md.tmpl", "docs/quickstart.md", "docs/workflow.md", "docs/lifecycle.md", "docs/reference/spec-format.md"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			steps := authoringExampleSteps(t, string(data))
			if len(steps) == 0 {
				t.Fatal("no actual Go proof example found")
			}
			for i, step := range steps {
				t.Run(fmt.Sprint(i), func(t *testing.T) {
					command := " " + strings.Join(step.Argv, " ") + " "
					if !strings.Contains(command, " -v ") || !strings.Contains(command, " -count=1 ") || step.ExpectOutput == nil || len(step.Covers) == 0 {
						t.Fatalf("published proof lacks non-vacuity/coverage: %+v", step)
					}
					selector := ""
					for j, arg := range step.Argv {
						if arg == "-run" && j+1 < len(step.Argv) {
							selector = step.Argv[j+1]
						}
					}
					if !regexp.MustCompile(`^\^Test[A-Za-z0-9_]+\$$`).MatchString(selector) {
						t.Fatalf("expected an anchored named-test selector, got %q", selector)
					}
					testName := strings.TrimSuffix(strings.TrimPrefix(selector, "^"), "$")
					pkg := strings.TrimPrefix(step.Argv[len(step.Argv)-1], "./")
					if pkg == "..." {
						pkg = "."
					}
					if !filepath.IsLocal(pkg) {
						t.Fatalf("unsafe fixture package %q", pkg)
					}
					for _, scenario := range []string{"pass", "zero-tests", "failing-test"} {
						t.Run(scenario, func(t *testing.T) {
							authoringTestEnv(t)
							work := t.TempDir()
							t.Chdir(work)
							authoringMust(t, authoringInProcess, "repo", "init", "--json")
							authoringMust(t, authoringInProcess, "feature", "init", "authoring-example", "--json")
							base := filepath.Join(work, ".walden/specs/authoring-example")
							requirements := "# Requirements Document\n\n### R1 Observable proof fixture\n\n"
							refs := []string{}
							for j, id := range step.Covers {
								requirements += fmt.Sprintf("%d. `%s` WHEN the check runs, the system SHALL assert the fixture property.\n", j+1, id)
								refs = append(refs, "`"+id+"`")
							}
							authoringReplaceBody(t, filepath.Join(base, "requirements.md"), requirements)
							design, err := os.ReadFile(filepath.Join(root, "skill/testdata/authoring-eval/seed/design.md"))
							if err != nil {
								t.Fatal(err)
							}
							authoringReplaceBody(t, filepath.Join(base, "design.md"), string(design))
							argv, _ := json.Marshal(step.Argv)
							covers, _ := json.Marshal(step.Covers)
							tasks := "# Implementation Plan\n\n- [ ] 1. Exercise actual example\n  - Requirements: " + strings.Join(refs, ", ") + "\n  - Design: Architecture\n  - Verification:\n    - command: " + string(argv) + "\n      expect_output: " + strconvQuote(*step.ExpectOutput) + "\n      covers: " + string(covers) + "\n"
							if step.ExpectExit != nil {
								tasks += fmt.Sprintf("      expect_exit: %d\n", *step.ExpectExit)
							}
							if step.Timeout != nil {
								tasks += "      timeout: " + *step.Timeout + "\n"
							}
							authoringReplaceBody(t, filepath.Join(base, "tasks.md"), tasks)
							authoringWrite(t, filepath.Join(work, "go.mod"), "module example.test/published\n\ngo 1.25.0\n")
							function, assertion := testName, "if 2+2 != 4 { t.Fatal(\"fixture\") }"
							if scenario == "zero-tests" {
								function = "TestUnrelated"
							}
							if scenario == "failing-test" {
								assertion = "t.Fatal(\"intentional failing-test fixture\")"
							}
							authoringWrite(t, filepath.Join(work, pkg, "example_test.go"), fmt.Sprintf("package example\nimport \"testing\"\nfunc %s(t *testing.T) { %s }\n", function, assertion))
							authoringApprove(t, authoringInProcess)
							out, code := authoringInProcess("task", "complete", "authoring-example", "1", "--json")
							if (code == 0) != (scenario == "pass") {
								t.Fatalf("%s: code=%d, %s", scenario, code, out)
							}
							if scenario == "zero-tests" && !strings.Contains(out, "output does not contain expected content") {
								t.Fatalf("did not fail on the anti-vacuity assertion: %s", out)
							}
						})
					}
				})
			}
		})
	}
}

func strconvQuote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func authoringBuildBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "walden")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin/go"), "build", "-ldflags", "-X github.com/andrearaponi/walden/internal/app.Version=v0.10.1-authoring-test", "-o", binary, "./cmd/walden")
	cmd.Dir = authoringSourceRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build candidate: %v: %s", err, out)
	}
	return binary
}

func authoringProcess(root, binary string) authoringInvoke {
	return func(args ...string) (string, int) {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out), 0
		}
		if exit, ok := err.(*exec.ExitError); ok {
			return string(out), exit.ExitCode()
		}
		return string(out) + err.Error(), -1
	}
}

func authoringInstallationState(t *testing.T, invoke authoringInvoke, agent, scope, expected string) {
	t.Helper()
	out := authoringMust(t, invoke, "skill", "status", "--json")
	var envelope struct {
		Result struct {
			Skills []struct {
				Agent, Scope, State, Version string
				Installed                    bool
			} `json:"skills"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatal(err)
	}
	for _, slot := range envelope.Result.Skills {
		if slot.Agent == agent && slot.Scope == scope {
			if !slot.Installed || slot.State != expected || slot.Version != "v0.10.1-authoring-test" {
				t.Fatalf("unexpected installation state: %+v", slot)
			}
			return
		}
	}
	t.Fatalf("missing installation slot %s/%s: %s", agent, scope, out)
}

func TestAuthoringCompiledBundle(t *testing.T) {
	authoringTestEnv(t)
	binary := authoringBuildBinary(t)
	root := t.TempDir()
	invoke := authoringProcess(root, binary)
	canonical, err := os.ReadFile(filepath.Join(authoringSourceRoot(t), "skill/walden/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if shown := authoringMust(t, invoke, "skill", "show"); shown != string(canonical) {
		t.Fatal("fresh binary does not expose the current canonical skill")
	}
	authoringMust(t, invoke, "repo", "init", "--json")
	authoringMust(t, invoke, "feature", "init", "authoring-example", "--json")
	feature, err := spec.LoadFeature(root, "authoring-example")
	if err != nil {
		t.Fatal(err)
	}
	headings := 0
	for _, line := range strings.Split(feature.Design.Body, "\n") {
		if strings.HasPrefix(line, "## ") {
			headings++
		}
	}
	if headings != 6 || !strings.Contains(feature.Requirements.Body, "Acceptance check:") {
		t.Fatal("fresh binary still contains old scaffold content")
	}
	authoringFillScaffolds(t, root)
	authoringApprove(t, invoke)
	authoringMust(t, invoke, "validate", "authoring-example", "--all", "--json")
	authoringMust(t, invoke, "task", "complete", "authoring-example", "1.1", "--json")

	authoringMust(t, invoke, "skill", "install", "--all", "--json")
	for _, agent := range []string{"claude", "codex", "copilot", "opencode"} {
		authoringInstallationState(t, invoke, agent, "user", "in-sync")
	}
	authoringMust(t, invoke, "skill", "install", "claude", "--project", "--json")
	path := filepath.Join(root, ".claude/skills/walden/SKILL.md")
	installed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body, version := skilldist.Strip(installed)
	if !bytes.Equal(body, canonical) || version != "v0.10.1-authoring-test" {
		t.Fatal("installed bytes/version differ from the fresh binary")
	}
	authoringInstallationState(t, invoke, "claude", "project", "in-sync")

	const sentinel = "USER-OWNED-SENTINEL-8f2a"
	authoringWrite(t, filepath.Join(root, "AGENTS.md"), sentinel+"\n")
	authoringMust(t, invoke, "skill", "install", "codex", "--project", "--json")
	managed, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(managed), sentinel) || !strings.Contains(string(managed), string(canonical)) {
		t.Fatal("managed-block installation lost user text or embedded content")
	}
	authoringInstallationState(t, invoke, "codex", "project", "in-sync")

	// Status intentionally exits zero on drift; the JSON state is the oracle.
	authoringWrite(t, path, strings.Replace(string(installed), "# Walden", "# DRIFT-SENTINEL-97b3", 1))
	authoringInstallationState(t, invoke, "claude", "project", "drifted")
	authoringInstallationState(t, invoke, "claude", "user", "in-sync")
}

func TestAuthoringBaselineCLICompatibility(t *testing.T) {
	baseline := os.Getenv("WALDEN_BASELINE_CLI")
	if baseline == "" {
		t.Skip("baseline CLI compatibility not run: set WALDEN_BASELINE_CLI")
	}
	if !filepath.IsAbs(baseline) {
		baseline = filepath.Join(authoringSourceRoot(t), baseline)
	}
	authoringTestEnv(t)
	root := t.TempDir()
	old := authoringProcess(root, baseline)
	version := authoringMust(t, old, "version", "--json")
	if !strings.Contains(version, "walden v0.10.1 (") {
		t.Fatalf("wrong baseline version: %s", version)
	}
	candidate := authoringProcess(root, authoringBuildBinary(t))
	authoringMust(t, candidate, "repo", "init", "--json")
	authoringMust(t, candidate, "feature", "init", "authoring-example", "--json")
	authoringFillScaffolds(t, root)
	authoringApprove(t, old)
	authoringMust(t, old, "validate", "authoring-example", "--all", "--json")
	authoringMust(t, old, "task", "complete", "authoring-example", "1.1", "--json")
}
