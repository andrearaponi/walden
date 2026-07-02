package skilldist

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/skill"
)

func statusByAgentScope(statuses []SkillStatus, agent string, scope Scope) *SkillStatus {
	for i := range statuses {
		if statuses[i].Agent == agent && statuses[i].Scope == scope {
			return &statuses[i]
		}
	}
	return nil
}

func TestStatusCoversEveryAgentAndSupportedScope(t *testing.T) {
	opts, _ := testOptions(t)
	statuses, _ := Status(opts)

	// claude and codex expose both scopes; copilot and opencode user only.
	if len(statuses) != 6 {
		t.Fatalf("expected 6 slots (2+2+1+1), got %d", len(statuses))
	}
	for _, name := range AgentNames() {
		if statusByAgentScope(statuses, name, ScopeUser) == nil {
			t.Fatalf("missing user-scope slot for %s", name)
		}
	}
	for _, name := range []string{"claude", "codex"} {
		if statusByAgentScope(statuses, name, ScopeProject) == nil {
			t.Fatalf("missing project-scope slot for %s", name)
		}
	}
}

func TestStatusFreshEnvironmentReportsNotInstalled(t *testing.T) {
	opts, _ := testOptions(t)
	statuses, warnings := Status(opts)
	for _, status := range statuses {
		if status.Installed || status.State != StateNotInstalled {
			t.Fatalf("%s/%s: expected not-installed, got %+v", status.Agent, status.Scope, status)
		}
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestStatusInSyncAfterInstallWithVersion(t *testing.T) {
	opts, _ := testOptions(t)
	if _, err := InstallAll(opts); err != nil {
		t.Fatalf("InstallAll: %v", err)
	}

	statuses, warnings := Status(opts)
	for _, name := range AgentNames() {
		status := statusByAgentScope(statuses, name, ScopeUser)
		if status == nil || !status.Installed {
			t.Fatalf("%s: expected installed user-scope slot", name)
		}
		if status.State != StateInSync {
			t.Fatalf("%s: expected in-sync, got %s", name, status.State)
		}
		if status.Version != "v9.9.9" {
			t.Fatalf("%s: expected version v9.9.9, got %q", name, status.Version)
		}
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestStatusDriftedAfterLocalEdit(t *testing.T) {
	opts, home := testOptions(t)
	agent := mustLookup(t, "claude")
	if _, err := Install(agent, ScopeUser, opts); err != nil {
		t.Fatalf("Install: %v", err)
	}

	target := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
	edited := append(Stamp(skill.Content(), "v9.9.9"), []byte("\nlocal customization\n")...)
	if err := os.WriteFile(target, edited, 0o644); err != nil {
		t.Fatalf("edit installed skill: %v", err)
	}

	statuses, _ := Status(opts)
	status := statusByAgentScope(statuses, "claude", ScopeUser)
	if status == nil || status.State != StateDrifted {
		t.Fatalf("expected drifted claude user slot, got %+v", status)
	}
}

func TestStatusLegacyInstallWithoutMarker(t *testing.T) {
	opts, home := testOptions(t)
	target := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A legacy setup.sh install: the raw skill, no version marker.
	if err := os.WriteFile(target, skill.Content(), 0o644); err != nil {
		t.Fatalf("seed legacy install: %v", err)
	}

	statuses, _ := Status(opts)
	status := statusByAgentScope(statuses, "claude", ScopeUser)
	if status == nil || !status.Installed {
		t.Fatal("expected an installed legacy slot")
	}
	if status.Version != "" {
		t.Fatalf("legacy installs carry no version, got %q", status.Version)
	}
	if status.State != StateInSync {
		t.Fatalf("an identical legacy install must read in-sync, got %s", status.State)
	}
}

func TestStatusDualScopeDivergenceWarns(t *testing.T) {
	opts, _ := testOptions(t)
	agent := mustLookup(t, "claude")
	if _, err := Install(agent, ScopeUser, opts); err != nil {
		t.Fatalf("Install user: %v", err)
	}
	if _, err := Install(agent, ScopeProject, opts); err != nil {
		t.Fatalf("Install project: %v", err)
	}

	// Identical installations: no divergence warning expected.
	_, warnings := Status(opts)
	if len(warnings) != 0 {
		t.Fatalf("identical dual-scope installs must not warn, got %v", warnings)
	}

	projectTarget := filepath.Join(opts.WorkDir, ".claude", "skills", "walden", "SKILL.md")
	edited := append(Stamp(skill.Content(), "v9.9.9"), []byte("\nproject tweak\n")...)
	if err := os.WriteFile(projectTarget, edited, 0o644); err != nil {
		t.Fatalf("edit project skill: %v", err)
	}

	_, warnings = Status(opts)
	foundDivergence := false
	for _, warning := range warnings {
		if strings.Contains(warning, "claude") && strings.Contains(warning, "differ") {
			foundDivergence = true
		}
	}
	if !foundDivergence {
		t.Fatalf("expected a divergence warning naming claude, got %v", warnings)
	}
}

func TestStatusCorruptBlockReportsDriftedWithWarning(t *testing.T) {
	opts, home := testOptions(t)
	target := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("notes\n"+blockBegin+"\nno end\n"), 0o644); err != nil {
		t.Fatalf("seed corrupt AGENTS.md: %v", err)
	}

	statuses, warnings := Status(opts)
	status := statusByAgentScope(statuses, "codex", ScopeUser)
	if status == nil || status.State != StateDrifted {
		t.Fatalf("expected drifted codex slot for a corrupt block, got %+v", status)
	}
	if len(warnings) == 0 {
		t.Fatal("expected a warning describing the corrupt block")
	}
}
