package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/skill"
)

func TestSkillInstallClaudeUserScope(t *testing.T) {
	home := setSkillTestEnv(t)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"skill", "install", "claude"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr.String())
	}

	target := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
	installed, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if !bytes.HasPrefix(installed, skill.Content()) {
		t.Fatal("installed skill must start with the embedded content")
	}
	if !bytes.Contains(installed, []byte("walden-skill-version")) {
		t.Fatal("installed skill must carry the version stamp")
	}
	if !strings.Contains(stdout.String(), target) {
		t.Fatalf("output must name the installed path, got:\n%s", stdout.String())
	}
}

func TestSkillInstallReinstallSucceeds(t *testing.T) {
	setSkillTestEnv(t)
	for i := 0; i < 2; i++ {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"skill", "install", "claude"}, &stdout, &stderr); code != 0 {
			t.Fatalf("run %d: expected exit 0, got %d (stderr: %s)", i, code, stderr.String())
		}
	}
}

func TestSkillInstallAllInstallsEveryAgent(t *testing.T) {
	home := setSkillTestEnv(t)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"skill", "install", "--all"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr.String())
	}
	for _, suffix := range []string{
		".claude/skills/walden/SKILL.md",
		".codex/AGENTS.md",
		".copilot/skills/walden/SKILL.md",
		".config/opencode/skills/walden/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(home, suffix)); err != nil {
			t.Fatalf("expected %s to be installed: %v", suffix, err)
		}
	}
}

func TestSkillInstallProjectScopeEmitsCommitHint(t *testing.T) {
	setSkillTestEnv(t)
	work := t.TempDir()
	t.Chdir(work)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"skill", "install", "claude", "--project"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(work, ".claude", "skills", "walden", "SKILL.md")); err != nil {
		t.Fatalf("expected project-scope skill under the working directory: %v", err)
	}
	if !strings.Contains(stdout.String(), "Commit") {
		t.Fatalf("project install must hint at committing the file, got:\n%s", stdout.String())
	}
}

func TestSkillInstallJSONEnvelope(t *testing.T) {
	setSkillTestEnv(t)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"skill", "install", "claude", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr.String())
	}
	var envelope struct {
		Command string `json:"command"`
		OK      bool   `json:"ok"`
		Result  struct {
			ChangedFiles []string `json:"changed_files"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	if envelope.Command != "skill-install" || !envelope.OK {
		t.Fatalf("unexpected envelope: %s", stdout.String())
	}
	if len(envelope.Result.ChangedFiles) != 1 {
		t.Fatalf("expected one changed file, got %v", envelope.Result.ChangedFiles)
	}
}

func TestSkillInstallRejectsMissingAndUnknownAgents(t *testing.T) {
	setSkillTestEnv(t)
	for _, args := range [][]string{
		{"skill", "install"},
		{"skill", "install", "cursor"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("%v: expected exit 1, got %d", args, code)
		}
		if !strings.Contains(stderr.String(), "claude, codex, copilot, opencode") {
			t.Fatalf("%v: error must list the supported agents, got: %s", args, stderr.String())
		}
	}
}

func TestSkillInstallRejectsProjectForUserOnlyAgents(t *testing.T) {
	setSkillTestEnv(t)
	for _, agent := range []string{"copilot", "opencode"} {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"skill", "install", agent, "--project"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("%s: expected exit 1, got %d", agent, code)
		}
		if !strings.Contains(stderr.String(), "only user scope") {
			t.Fatalf("%s: error must explain user-only support, got: %s", agent, stderr.String())
		}
	}
}

func TestSkillInstallRejectsAllWithProjectOrAgent(t *testing.T) {
	setSkillTestEnv(t)
	for _, args := range [][]string{
		{"skill", "install", "--all", "--project"},
		{"skill", "install", "--all", "claude"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("%v: expected exit 1, got %d", args, code)
		}
		if !strings.Contains(stderr.String(), "--all cannot be combined") {
			t.Fatalf("%v: expected a flag-conflict error, got: %s", args, stderr.String())
		}
	}
}

func TestSkillUninstallRemovesInstalledSkill(t *testing.T) {
	home := setSkillTestEnv(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"skill", "install", "claude"}, &stdout, &stderr); code != 0 {
		t.Fatalf("install: expected exit 0 (stderr: %s)", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"skill", "uninstall", "claude"}, &stdout, &stderr); code != 0 {
		t.Fatalf("uninstall: expected exit 0, got stderr: %s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "walden")); !os.IsNotExist(err) {
		t.Fatal("uninstall must remove the skill directory")
	}
}

func TestSkillUninstallAbsentSkillIsIdempotent(t *testing.T) {
	setSkillTestEnv(t)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"skill", "uninstall", "claude"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0 for an absent installation, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "not installed") {
		t.Fatalf("expected a not-installed summary, got:\n%s", stdout.String())
	}
}

func TestSkillUninstallAllRemovesEverything(t *testing.T) {
	home := setSkillTestEnv(t)

	var buf bytes.Buffer
	if code := Run([]string{"skill", "install", "--all"}, &buf, &buf); code != 0 {
		t.Fatalf("install --all failed:\n%s", buf.String())
	}
	buf.Reset()
	if code := Run([]string{"skill", "uninstall", "--all"}, &buf, &buf); code != 0 {
		t.Fatalf("uninstall --all failed:\n%s", buf.String())
	}
	for _, suffix := range []string{".claude/skills/walden", ".codex/AGENTS.md", ".copilot/skills/walden", ".config/opencode/skills/walden"} {
		if _, err := os.Stat(filepath.Join(home, suffix)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed", suffix)
		}
	}
}
