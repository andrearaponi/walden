package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/andrearaponi/walden/skill"
)

func skillStatusBuildCandidate(t *testing.T, goos, arch string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "walden")
	if goos == "windows" {
		binary += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goBinary += ".exe"
	}
	command := exec.CommandContext(ctx, goBinary, "build", "-trimpath", "-buildvcs=false", "-ldflags", "-X github.com/andrearaponi/walden/internal/app.Version=v0.10.5", "-o", binary, "./cmd/walden")
	command.Dir = authoringSourceRoot(t)
	overrides := map[string]string{"GOOS": goos, "GOARCH": arch, "CGO_ENABLED": "0", "GOFLAGS": "", "GOWORK": "off", "GOTOOLCHAIN": "local", "GOAMD64": "v1", "GOARM64": "v8.0"}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, override := overrides[key]; !override {
			command.Env = append(command.Env, item)
		}
	}
	for key, value := range overrides {
		command.Env = append(command.Env, key+"="+value)
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s/%s candidate: %v\n%s", goos, arch, err, output)
	}
	return binary
}

func TestSkillStatusLineEndingCLIContract(t *testing.T) {
	binary := skillStatusBuildCandidate(t, runtime.GOOS, runtime.GOARCH)
	home, work := setSkillTestEnv(t), t.TempDir()
	t.Chdir(work)
	canonical := bytes.ReplaceAll(skill.Content(), []byte("\r\n"), []byte("\n"))
	stamp := append(append([]byte(nil), canonical...), []byte("<!-- walden-skill-version: v1.2.3 -->\n")...)
	crlf := bytes.ReplaceAll(stamp, []byte("\n"), []byte("\r\n"))
	skillStatusWriteFixture(t, filepath.Join(home, ".claude", "skills", "walden", "SKILL.md"), crlf)
	skillStatusWriteFixture(t, filepath.Join(work, ".claude", "skills", "walden", "SKILL.md"), canonical)
	block := append([]byte("PREFIX-PRESERVED-491\r\n# --- BEGIN WALDEN SKILL ---\r\n"), crlf...)
	block = append(block, []byte("# --- END WALDEN SKILL ---\r\nSUFFIX-PRESERVED-369\r\n")...)
	skillStatusWriteFixture(t, filepath.Join(home, ".codex", "AGENTS.md"), block)
	changed := append(append([]byte(nil), crlf...), []byte("REAL-CONTENT-CHANGE-185\r\n")...)
	skillStatusWriteFixture(t, filepath.Join(home, ".copilot", "skills", "walden", "SKILL.md"), changed)
	before := skillStatusSnapshot(t, home, work)
	_, inProcess, warnings := skillStatusReadJSON(t)
	if len(warnings) != 0 {
		t.Errorf("newline-only dual-scope difference warned: %v", warnings)
	}
	var nativeSlots []map[string]any
	for _, jsonMode := range []bool{false, true} {
		args := []string{"skill", "status"}
		if jsonMode {
			args = append(args, "--json")
		}
		command := exec.Command(binary, args...)
		command.Dir = work
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("compiled status: %v\n%s", err, output)
		}
		if jsonMode {
			var envelope struct {
				OK     bool `json:"ok"`
				Result struct {
					Summary  string           `json:"summary"`
					Skills   []map[string]any `json:"skills"`
					Warnings []string         `json:"warnings"`
					Exit     int              `json:"exit_code"`
				} `json:"result"`
			}
			if err := json.Unmarshal(output, &envelope); err != nil {
				t.Fatal(err)
			}
			if !envelope.OK || envelope.Result.Exit != 0 || len(envelope.Result.Warnings) != 0 || envelope.Result.Summary != "skill status against embedded version v0.10.5" {
				t.Errorf("compiled report invalid: %s", output)
			}
			nativeSlots = envelope.Result.Skills
		} else {
			for _, want := range []string{"claude (user): in-sync version=v1.2.3", "claude (project): in-sync", "codex (user): in-sync version=v1.2.3", "copilot (user): drifted"} {
				if !strings.Contains(string(output), want) {
					t.Errorf("text report missing %q: %s", want, output)
				}
			}
		}
	}
	if !reflect.DeepEqual(nativeSlots, inProcess) {
		t.Errorf("compiled and app.Run slots differ: native=%+v in-process=%+v", nativeSlots, inProcess)
	}
	for _, slot := range nativeSlots {
		key := slot["agent"].(string) + "/" + slot["scope"].(string)
		want := map[string]string{"claude/user": "in-sync", "claude/project": "in-sync", "codex/user": "in-sync", "codex/project": "not-installed", "copilot/user": "drifted", "opencode/user": "not-installed"}
		if slot["state"] != want[key] {
			t.Errorf("%s state=%v want=%s", key, slot["state"], want[key])
		}
		if key == "claude/user" || key == "codex/user" {
			if slot["version"] != "v1.2.3" {
				t.Errorf("CRLF marker lost: %+v", slot)
			}
		}
	}
	if after := skillStatusSnapshot(t, home, work); !reflect.DeepEqual(before, after) {
		t.Fatal("compiled/in-process status changed fixture bytes or metadata")
	}
}
