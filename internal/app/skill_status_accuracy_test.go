package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/skill"
)

func skillStatusWriteFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Reading may update access time; content, kind, mode, modification time and
// link targets are the preservation facts relevant to these read-only checks.
func skillStatusSnapshot(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for i, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			value := fmt.Sprintf("%v %d", info.Mode(), info.ModTime().UnixNano())
			switch {
			case info.Mode()&os.ModeSymlink != 0:
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				value += " link:" + target
			case info.Mode().IsRegular():
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				value += fmt.Sprintf(" sha256:%x", sha256.Sum256(data))
			}
			result[fmt.Sprintf("%d/%s", i, filepath.ToSlash(rel))] = value
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func skillStatusReadJSON(t *testing.T) (string, []map[string]any, []string) {
	t.Helper()
	var out, stderr bytes.Buffer
	if code := Run([]string{"skill", "status", "--json"}, &out, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("status is a report, not a gate: exit=%d stderr=%s out=%s", code, &stderr, &out)
	}
	var result struct {
		SchemaVersion string `json:"schema_version"`
		Command       string `json:"command"`
		OK            bool   `json:"ok"`
		Result        struct {
			Skills   []map[string]any `json:"skills"`
			Warnings []string         `json:"warnings"`
			ExitCode int              `json:"exit_code"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != "v0beta1" || result.Command != "skill-status" || !result.OK || result.Result.ExitCode != 0 || len(result.Result.Skills) != 6 {
		t.Fatalf("report shape changed: %s", &out)
	}
	return out.String(), result.Result.Skills, result.Result.Warnings
}

func TestSkillStatusReadOnlyOutput(t *testing.T) {
	home, work := setSkillTestEnv(t), t.TempDir()
	t.Chdir(work)
	actualHome, err := os.UserHomeDir()
	if err != nil || actualHome != home {
		t.Fatalf("test home is not isolated: %q %v", actualHome, err)
	}
	target := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	_, readErr := os.ReadFile(target)
	if readErr == nil || errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("required filesystem error was not established: %v", readErr)
	}
	rootCause := readErr.Error()
	var pathErr *os.PathError
	if errors.As(readErr, &pathErr) {
		rootCause = pathErr.Err.Error()
	}
	skillStatusWriteFixture(t, filepath.Join(work, ".claude", "skills", "walden", "SKILL.md"), skill.Content())
	skillStatusWriteFixture(t, filepath.Join(home, ".copilot", "skills", "walden", "SKILL.md"), []byte("ALTERED-CONTENT-SENTINEL-395\n"))
	skillStatusWriteFixture(t, filepath.Join(home, ".agents", ".skill-lock.json"), []byte("{\"unrelated\":\"registry-sentinel\"}\n"))
	skillStatusWriteFixture(t, filepath.Join(home, ".bash_profile"), []byte("PROFILE-SENTINEL\n"))
	before := skillStatusSnapshot(t, home, work)
	raw, slots, warnings := skillStatusReadJSON(t)
	var text, stderr bytes.Buffer
	if code := Run([]string{"skill", "status"}, &text, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("text status failed: %d %s", code, &stderr)
	}
	wantStates := map[string]string{
		"claude/user": "unreadable", "claude/project": "in-sync",
		"codex/user": "not-installed", "codex/project": "not-installed",
		"copilot/user": "drifted", "opencode/user": "not-installed",
	}
	for _, slot := range slots {
		agent, okAgent := slot["agent"].(string)
		scope, okScope := slot["scope"].(string)
		_, okPath := slot["path"].(string)
		installed, okInstalled := slot["installed"].(bool)
		state, okState := slot["state"].(string)
		if !okAgent || !okScope || !okPath || !okInstalled || !okState {
			t.Fatalf("field names/types changed: %s", raw)
		}
		key := agent + "/" + scope
		if state != wantStates[key] {
			t.Errorf("%s state=%s, want %s", key, state, wantStates[key])
		}
		if !strings.Contains(text.String(), fmt.Sprintf("%s (%s): %s", agent, scope, wantStates[key])) {
			t.Errorf("text/JSON states disagree: %s", &text)
		}
		if key == "claude/user" {
			if !installed || slot["path"] != target {
				t.Errorf("legacy slot flag or requested path changed: %+v", slot)
			}
			if _, exists := slot["version"]; exists {
				t.Errorf("read error manufactured a marker: %+v", slot)
			}
		}
	}
	if len(warnings) != 1 {
		t.Errorf("expected only the read failure, not scope divergence: %v", warnings)
	}
	for _, want := range []string{"claude", "user", target, rootCause, "comparison not performed"} {
		if !strings.Contains(strings.Join(warnings, "\n"), want) || !strings.Contains(text.String(), want) {
			t.Errorf("missing diagnostic %q in text/JSON: %s %v", want, &text, warnings)
		}
	}
	if after := skillStatusSnapshot(t, home, work); !reflect.DeepEqual(before, after) {
		t.Fatalf("read-only status changed the fixture: before=%v after=%v", before, after)
	}
}
