package selfupdate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/andrearaponi/walden/internal/skilldist"
	"github.com/andrearaponi/walden/skill"
)

func TestSkillStatusSyncSelectionCompatibility(t *testing.T) {
	home, work := t.TempDir(), t.TempDir()
	unreadable := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
	readable := filepath.Join(work, ".claude", "skills", "walden", "SKILL.md")
	corrupt := filepath.Join(home, ".codex", "AGENTS.md")
	for _, path := range []string{unreadable, filepath.Dir(readable), filepath.Dir(corrupt)} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.ReadFile(unreadable); err == nil {
		t.Fatal("fixture must establish a read error")
	}
	if err := os.WriteFile(readable, skill.Content(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corrupt, []byte("# --- BEGIN WALDEN SKILL ---\nCORRUPT-BLOCK-SENTINEL\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := skilldist.Options{Version: "v9.9.9", WorkDir: work, Env: skilldist.Env{Home: home}}
	statuses, _ := skilldist.Status(opts)
	if statuses[0].State != "unreadable" || !statuses[0].Installed {
		t.Fatalf("test must exercise the new diagnostic with its legacy flag: %+v", statuses[0])
	}
	want := []skillSlot{
		{Agent: "claude", Scope: skilldist.ScopeUser, Path: unreadable},
		{Agent: "claude", Scope: skilldist.ScopeProject, Path: readable},
		{Agent: "codex", Scope: skilldist.ScopeUser, Path: corrupt},
	}
	if got := snapshotSkillSlots(opts); !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostic change altered sync selection: got=%+v want=%+v", got, want)
	}
	// Snapshot selection only: no apply, downloads, skill writes or native sync.
}
