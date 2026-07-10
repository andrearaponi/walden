package selfupdate

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/skilldist"
)

func TestSyncSnapshotRecordsInstalledSlots(t *testing.T) {
	home := t.TempDir()
	skillPath := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("skill body\n"), 0o644); err != nil {
		t.Fatalf("seed skill file: %v", err)
	}

	opts := skilldist.Options{
		Version: "v0.6.0",
		WorkDir: t.TempDir(),
		Env:     skilldist.Env{Home: home},
	}

	slots := snapshotSkillSlots(opts)

	if len(slots) != 1 {
		t.Fatalf("snapshot recorded %d slots, want 1: %+v", len(slots), slots)
	}
	if slots[0].Agent != "claude" || slots[0].Scope != skilldist.ScopeUser || slots[0].Path != skillPath {
		t.Fatalf("snapshot slot = %+v, want claude/user at %s", slots[0], skillPath)
	}
}

func TestSyncReinstallsRecordedSlots(t *testing.T) {
	runner := &fakeRunner{}
	slots := []skillSlot{
		{Agent: "claude", Scope: skilldist.ScopeUser, Path: "/home/u/.claude/skills/walden/SKILL.md"},
		{Agent: "codex", Scope: skilldist.ScopeProject, Path: "/repo/AGENTS.md"},
	}

	synced, warnings := resyncSkills(context.Background(), runner, "/usr/local/bin/walden", "v0.7.0", slots)

	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(synced) != 2 {
		t.Fatalf("synced %d slots, want 2", len(synced))
	}

	wantCalls := [][]string{
		{"/usr/local/bin/walden", "skill", "install", "claude"},
		{"/usr/local/bin/walden", "skill", "install", "codex", "--project"},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("runner calls = %v, want %v", runner.calls, wantCalls)
	}
}

func TestSyncFailureBecomesWarning(t *testing.T) {
	runner := &fakeRunner{respond: func(_ string, args []string) (shell.Response, error) {
		if len(args) >= 3 && args[2] == "codex" {
			return shell.Response{Stderr: "permission denied", ExitCode: 1}, nil
		}
		return shell.Response{ExitCode: 0}, nil
	}}
	slots := []skillSlot{
		{Agent: "claude", Scope: skilldist.ScopeUser},
		{Agent: "codex", Scope: skilldist.ScopeUser},
	}

	synced, warnings := resyncSkills(context.Background(), runner, "/bin/walden", "v0.7.0", slots)

	if len(synced) != 1 || synced[0].Agent != "claude" {
		t.Fatalf("synced = %+v, want only claude", synced)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
	if !strings.Contains(warnings[0], "codex") || !strings.Contains(warnings[0], "user") {
		t.Fatalf("warning %q does not name the agent and scope", warnings[0])
	}
}

func TestSyncSkipsPreSkillReleases(t *testing.T) {
	runner := &fakeRunner{}
	slots := []skillSlot{{Agent: "claude", Scope: skilldist.ScopeUser}}

	synced, warnings := resyncSkills(context.Background(), runner, "/bin/walden", "v0.4.0", slots)

	if len(runner.calls) != 0 {
		t.Fatalf("re-sync invoked the binary %d times for a pre-skill release, want 0", len(runner.calls))
	}
	if len(synced) != 0 {
		t.Fatalf("synced = %+v, want none", synced)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "skip") {
		t.Fatalf("warnings = %v, want one skip warning", warnings)
	}
}
