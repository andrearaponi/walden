package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillStatusFreshEnvironment(t *testing.T) {
	setSkillTestEnv(t)
	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	code := Run([]string{"skill", "status"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Skills:") {
		t.Fatalf("expected a Skills block, got:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "not-installed") {
		t.Fatalf("expected not-installed states, got:\n%s", stdout.String())
	}
}

func TestSkillStatusJSONCarriesSkillsAndExitsZeroOnDrift(t *testing.T) {
	home := setSkillTestEnv(t)
	t.Chdir(t.TempDir())

	var buf bytes.Buffer
	if code := Run([]string{"skill", "install", "claude"}, &buf, &buf); code != 0 {
		t.Fatalf("install failed:\n%s", buf.String())
	}

	// Drift the installation on purpose: status must still exit 0.
	target := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
	if err := os.WriteFile(target, []byte("locally rewritten\n"), 0o644); err != nil {
		t.Fatalf("edit installed skill: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"skill", "status", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("status must exit 0 even with drift, got %d (stderr: %s)", code, stderr.String())
	}

	var envelope struct {
		Command string `json:"command"`
		OK      bool   `json:"ok"`
		Result  struct {
			Skills []struct {
				Agent string `json:"agent"`
				Scope string `json:"scope"`
				State string `json:"state"`
			} `json:"skills"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	if envelope.Command != "skill-status" || !envelope.OK {
		t.Fatalf("unexpected envelope: %s", stdout.String())
	}
	if len(envelope.Result.Skills) == 0 {
		t.Fatal("expected skills in the JSON envelope")
	}
	foundDrifted := false
	for _, s := range envelope.Result.Skills {
		if s.Agent == "claude" && s.Scope == "user" && s.State == "drifted" {
			foundDrifted = true
		}
	}
	if !foundDrifted {
		t.Fatalf("expected the edited claude installation to read drifted: %s", stdout.String())
	}
}

func TestUsageListsSkillCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	for _, needle := range []string{"skill install", "skill uninstall", "skill status", "skill show"} {
		if !strings.Contains(stdout.String(), needle) {
			t.Fatalf("usage must list %q, got:\n%s", needle, stdout.String())
		}
	}
}
