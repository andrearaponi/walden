package skilldist

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/skill"
)

func accuracyWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func accuracySlot(t *testing.T, slots []SkillStatus, agent string, scope Scope, state string, installed bool, version string) {
	t.Helper()
	slot := statusByAgentScope(slots, agent, scope)
	if slot == nil || slot.State != state || slot.Installed != installed || slot.Version != version {
		t.Errorf("%s/%s: got %+v, want state=%s installed=%t version=%q", agent, scope, slot, state, installed, version)
	}
}

func accuracyDiverges(warnings []string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, "installations differ") {
			return true
		}
	}
	return false
}

func TestSkillStatusReadFailures(t *testing.T) {
	for _, cause := range []struct {
		name string
		err  error
	}{
		{"permission", os.ErrPermission},
		{"traversal", errors.New("fixture: untrusted mount point (448)")},
	} {
		t.Run(cause.name, func(t *testing.T) {
			opts, home := testOptions(t)
			target := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
			project := filepath.Join(opts.WorkDir, ".claude", "skills", "walden", "SKILL.md")
			alternate := filepath.Join(home, ".agents", "skills", "walden", "SKILL.md")
			accuracyWrite(t, target, Stamp(skill.Content(), "v8.8.8"))
			accuracyWrite(t, project, skill.Content())
			accuracyWrite(t, alternate, skill.Content())
			readErr := &os.PathError{Op: "open", Path: target, Err: cause.err}
			allowed := map[string]bool{}
			for _, agent := range Agents() {
				for _, scope := range []Scope{ScopeUser, ScopeProject} {
					if path, err := resolveTarget(agent, scope, opts); err == nil {
						allowed[path] = true
					}
				}
			}
			calls := map[string]int{}
			slots, warnings := statusWithReader(opts, func(path string) ([]byte, error) {
				calls[path]++
				if !allowed[path] || path == alternate {
					t.Errorf("unexpected fallback/read outside native targets: %s", path)
				}
				if path == target {
					return nil, readErr
				}
				return os.ReadFile(path)
			})
			if len(slots) != 6 || calls[target] != 1 {
				t.Fatalf("wrong inventory/read count: %d slots; calls=%v", len(slots), calls)
			}
			accuracySlot(t, slots, "claude", ScopeUser, "unreadable", true, "")
			accuracySlot(t, slots, "claude", ScopeProject, StateInSync, true, "")
			accuracySlot(t, slots, "copilot", ScopeUser, StateNotInstalled, false, "")
			if len(warnings) != 1 || accuracyDiverges(warnings) {
				t.Errorf("unavailable body must not claim divergence: %v", warnings)
			}
			text := strings.Join(warnings, "\n")
			for _, want := range []string{"claude", "user", target, cause.err.Error(), "comparison not performed"} {
				if !strings.Contains(text, want) {
					t.Errorf("warning missing %q: %s", want, text)
				}
			}
		})
	}
	t.Run("real filesystem read failure", func(t *testing.T) {
		opts, home := testOptions(t)
		target := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
		if err := os.MkdirAll(target, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := os.ReadFile(target); err == nil || errors.Is(err, os.ErrNotExist) {
			t.Fatalf("fixture did not establish a non-absence read error: %v", err)
		}
		slots, warnings := Status(opts)
		accuracySlot(t, slots, "claude", ScopeUser, "unreadable", true, "")
		if len(warnings) != 1 {
			t.Fatalf("missing read diagnostic: %v", warnings)
		}
	})
	t.Run("absence versus readable corrupt block", func(t *testing.T) {
		opts, home := testOptions(t)
		accuracyWrite(t, filepath.Join(home, ".codex", "AGENTS.md"), []byte("UNRELATED-NOTES\n"))
		slots, warnings := Status(opts)
		accuracySlot(t, slots, "claude", ScopeUser, StateNotInstalled, false, "")
		accuracySlot(t, slots, "codex", ScopeUser, StateNotInstalled, false, "")
		if len(warnings) != 0 {
			t.Fatal(warnings)
		}
		accuracyWrite(t, filepath.Join(home, ".codex", "AGENTS.md"), []byte(blockBegin+"\nMISSING-END-SENTINEL\n"))
		accuracyWrite(t, filepath.Join(opts.WorkDir, "AGENTS.md"), buildBlock(skill.Content()))
		slots, warnings = Status(opts)
		accuracySlot(t, slots, "codex", ScopeUser, StateDrifted, true, "")
		accuracySlot(t, slots, "codex", ScopeProject, StateInSync, true, "")
		if len(warnings) != 1 || !strings.Contains(warnings[0], "END marker") || accuracyDiverges(warnings) {
			t.Fatalf("corrupt body has not been extracted for comparison: %v", warnings)
		}
	})
	t.Run("empty bodies remain comparable", func(t *testing.T) {
		opts, home := testOptions(t)
		user := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
		project := filepath.Join(opts.WorkDir, ".claude", "skills", "walden", "SKILL.md")
		accuracyWrite(t, user, nil)
		accuracyWrite(t, project, nil)
		slots, warnings := Status(opts)
		accuracySlot(t, slots, "claude", ScopeUser, StateDrifted, true, "")
		accuracySlot(t, slots, "claude", ScopeProject, StateDrifted, true, "")
		if accuracyDiverges(warnings) {
			t.Fatal(warnings)
		}
		accuracyWrite(t, project, skill.Content())
		_, warnings = Status(opts)
		if !accuracyDiverges(warnings) {
			t.Fatal("readable empty body was dropped from scope comparison")
		}
	})
	t.Run("same path retains both scopes", func(t *testing.T) {
		opts, home := testOptions(t)
		opts.WorkDir = home
		accuracyWrite(t, filepath.Join(home, ".claude", "skills", "walden", "SKILL.md"), skill.Content())
		slots, warnings := Status(opts)
		accuracySlot(t, slots, "claude", ScopeUser, StateInSync, true, "")
		accuracySlot(t, slots, "claude", ScopeProject, StateInSync, true, "")
		if len(slots) != 6 || len(warnings) != 0 {
			t.Fatalf("scope inventory changed: %+v %v", slots, warnings)
		}
		if statusByAgentScope(slots, "claude", ScopeUser).Path != statusByAgentScope(slots, "claude", ScopeProject).Path {
			t.Fatal("same requested path was redirected")
		}
	})
}
